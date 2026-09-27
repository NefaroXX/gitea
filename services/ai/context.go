// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ai

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	issue_template "gitea.dev/modules/issue/template"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	git_service "gitea.dev/services/git"
	"gitea.dev/services/gitdiff"
)

// BuildInputFromCompare builds a DescriptionInput from compare information with limits.
func BuildInputFromCompare(ctx context.Context, repo *repo_model.Repository, compareInfo *git_service.CompareInfo, gitRepo *git.Repository) (DescriptionInput, error) {
	aiCfg := setting.Config().AI
	maxCommits := aiCfg.MaxCommits.Value(ctx)
	maxFiles := aiCfg.MaxFiles.Value(ctx)
	input := DescriptionInput{
		RepositoryName: repo.FullName(),
		BaseBranch:     compareInfo.BaseRef.ShortName(),
		HeadBranch:     compareInfo.HeadRef.ShortName(),
	}

	// Commits with limits
	commits := compareInfo.Commits
	limitedCommits, truncatedCommits := LimitCommits(commitInfoFromGitCommits(commits), maxCommits)
	input.Commits = limitedCommits
	if truncatedCommits {
		// truncated commits already handled, diffStat will reflect full but we note truncation? Prompt truncation refers to diff.
	}

	// Diff stat
	if compareInfo.CompareBase != "" && compareInfo.HeadCommitID != "" {
		shortStat, err := gitdiff.GetDiffShortStat(ctx, gitRepo, compareInfo.CompareBase, compareInfo.HeadCommitID)
		if err != nil {
			log.Error("GetDiffShortStat failed: %v", err)
		} else if shortStat != nil {
			input.DiffStat = fmt.Sprintf("%d files changed, %d insertions(+), %d deletions(-)", shortStat.NumFiles, shortStat.TotalAddition, shortStat.TotalDeletion)
		}
	}

	// Changed files
	changedFiles, err := getChangedFilesWithStatus(ctx, gitRepo, compareInfo.CompareBase, compareInfo.HeadCommitID, maxFiles)
	if err != nil {
		log.Error("getChangedFiles failed: %v", err)
	} else {
		limitedFiles, _ := LimitFiles(changedFiles, maxFiles)
		input.ChangedFiles = limitedFiles
	}

	// Diff patch with limit
	if compareInfo.CompareBase != "" && compareInfo.HeadCommitID != "" {
		diff, truncated := getDiffWithLimit(ctx, gitRepo, compareInfo.CompareBase, compareInfo.HeadCommitID, aiCfg.MaxDiff.Value(ctx))
		input.Diff = diff
		input.Truncated = truncated || truncatedCommits
	}

	// PR template
	templateContent := getPRTemplate(ctx, gitRepo, repo.DefaultBranch)
	input.PRTemplate = templateContent

	// Ensure prompt limit
	EnsurePromptWithinLimit(&input, aiCfg.MaxPrompt.Value(ctx))

	return input, nil
}

func commitInfoFromGitCommits(commits []*git.Commit) []CommitInfo {
	out := make([]CommitInfo, 0, len(commits))
	for _, c := range commits {
		subject := c.MessageTitle()
		body := c.MessageBody()
		out = append(out, CommitInfo{
			SHA:     c.ID.String(),
			Subject: subject,
			Body:    body,
		})
	}
	return out
}

func getChangedFilesWithStatus(ctx context.Context, gitRepo *git.Repository, base, head string, maxFiles int) ([]ChangedFile, error) {
	if base == "" || head == "" {
		return nil, nil
	}
	// Use git diff --name-status --diff-filter=AMDR etc.
	cmd := gitcmd.NewCommand("diff", "--name-status").AddDynamicArguments(base + "..." + head).AddArguments("--")
	stdout, _, err := cmd.WithRepo(gitRepo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	var files []ChangedFile
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// format: "M\tpath" or "R100\told\tnew"
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		status := parts[0]
		var path string
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if len(parts) >= 3 {
				path = parts[1] + " -> " + parts[2]
			} else {
				path = parts[1]
			}
		} else {
			path = parts[1]
		}
		// Normalize status letter
		s := "modified"
		switch status[0] {
		case 'A':
			s = "added"
		case 'D':
			s = "deleted"
		case 'R':
			s = "renamed"
		case 'M':
			s = "modified"
		case 'C':
			s = "copied"
		}
		files = append(files, ChangedFile{Path: path, Status: s})
		if len(files) >= maxFiles {
			break
		}
	}
	return files, nil
}

func getDiffWithLimit(ctx context.Context, gitRepo *git.Repository, base, head string, maxBytes int) (string, bool) {
	if base == "" || head == "" {
		return "", false
	}
	var buf bytes.Buffer
	compareArg := base + "..." + head
	// Use non-binary diff, limited
	err := gitRepo.GetDiff(ctx, compareArg, &buf)
	if err != nil {
		log.Error("GetDiff failed: %v", err)
		return "", false
	}
	diffStr := buf.String()
	if maxBytes > 0 && len(diffStr) > maxBytes {
		truncated, _ := TruncateDiff(diffStr, maxBytes)
		return truncated, true
	}
	return diffStr, false
}

// getPRTemplate attempts to load PR template from default branch.
func getPRTemplate(ctx context.Context, gitRepo *git.Repository, defaultBranch string) string {
	candidates := []string{
		"PULL_REQUEST_TEMPLATE.md",
		"PULL_REQUEST_TEMPLATE.yaml",
		"PULL_REQUEST_TEMPLATE.yml",
		"pull_request_template.md",
		"pull_request_template.yaml",
		"pull_request_template.yml",
		".gitea/PULL_REQUEST_TEMPLATE.md",
		".gitea/PULL_REQUEST_TEMPLATE.yaml",
		".gitea/PULL_REQUEST_TEMPLATE.yml",
		".gitea/pull_request_template.md",
		".gitea/pull_request_template.yaml",
		".gitea/pull_request_template.yml",
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/PULL_REQUEST_TEMPLATE.yaml",
		".github/PULL_REQUEST_TEMPLATE.yml",
		".github/pull_request_template.md",
		".github/pull_request_template.yaml",
		".github/pull_request_template.yml",
	}
	for _, cand := range candidates {
		tmpl, err := issue_template.UnmarshalFromRepo(ctx, gitRepo, defaultBranch, cand)
		if err != nil {
			continue
		}
		if tmpl != nil {
			// For markdown type, content is the template body; for yaml, need to render?
			// Use tmpl.Content if available, else try to read raw
			if tmpl.Content != "" {
				return tmpl.Content
			}
			return tmpl.FileName
		}
	}
	return ""
}
