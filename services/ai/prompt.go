// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ai

import (
	"fmt"
	"strings"
)

// BuildPrompt constructs a focused prompt for the LLM.
func BuildPrompt(input DescriptionInput) string {
	var sb strings.Builder

	sb.WriteString("Generate a Pull Request description from the supplied repository changes.\n\n")
	sb.WriteString("Only describe information supported by the supplied commit history, changed files, and diff.\n")
	sb.WriteString("Do not invent:\n")
	sb.WriteString("- tests that were not shown\n")
	sb.WriteString("- functionality that was not shown\n")
	sb.WriteString("- performance improvements that were not demonstrated\n")
	sb.WriteString("- bug fixes that cannot be established\n")
	sb.WriteString("- breaking changes without evidence\n\n")
	sb.WriteString("Produce Markdown suitable for direct insertion into a Git hosting Pull Request description.\n\n")

	if strings.TrimSpace(input.PRTemplate) != "" {
		sb.WriteString("Use the repository's PR template if one exists. Preserve its intended structure and fill the template rather than arbitrarily replacing its structure.\n")
		sb.WriteString("PR Template:\n")
		sb.WriteString("```markdown\n")
		sb.WriteString(strings.TrimSpace(input.PRTemplate))
		sb.WriteString("\n```\n\n")
		sb.WriteString("Fill the template with content based on the changes. Keep headings from the template.\n\n")
	} else {
		sb.WriteString("Structure the description with sections when appropriate:\n")
		sb.WriteString("## Summary\n\n## Changes\n\n## Testing\n\n## Breaking Changes\n\n")
		sb.WriteString("Omit sections that have no relevant information (e.g., if there is no evidence of testing, note that).\n\n")
	}

	sb.WriteString("Repository: ")
	sb.WriteString(input.RepositoryName)
	sb.WriteString("\nBase branch: ")
	sb.WriteString(input.BaseBranch)
	sb.WriteString("\nHead branch: ")
	sb.WriteString(input.HeadBranch)
	sb.WriteString("\n\n")

	if len(input.Commits) > 0 {
		sb.WriteString("Commits:\n")
		for _, c := range input.Commits {
			sb.WriteString("- ")
			sb.WriteString(c.SHA)
			if c.Subject != "" {
				sb.WriteString(" ")
				sb.WriteString(c.Subject)
			}
			sb.WriteString("\n")
			if strings.TrimSpace(c.Body) != "" {
				sb.WriteString("  ")
				sb.WriteString(strings.ReplaceAll(strings.TrimSpace(c.Body), "\n", "\n  "))
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	if len(input.ChangedFiles) > 0 {
		sb.WriteString("Changed files:\n")
		for _, f := range input.ChangedFiles {
			sb.WriteString("- ")
			sb.WriteString(f.Path)
			if f.Status != "" {
				sb.WriteString(" (")
				sb.WriteString(f.Status)
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if strings.TrimSpace(input.DiffStat) != "" {
		sb.WriteString("Diff statistics:\n")
		sb.WriteString(strings.TrimSpace(input.DiffStat))
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(input.Diff) != "" {
		sb.WriteString("Diff:\n```diff\n")
		sb.WriteString(strings.TrimSpace(input.Diff))
		sb.WriteString("\n```\n")
	}

	if input.Truncated {
		sb.WriteString("\nThe supplied diff has been truncated. Do not make claims about changes that are not represented in the supplied context.\n")
	}

	return sb.String()
}

// BuildMessages returns system and user messages for OpenAI-compatible chat completions.
func BuildMessages(input DescriptionInput) []map[string]string {
	system := "You are a helpful assistant that generates Pull Request descriptions. Be concise, accurate, and avoid hallucinations."
	user := BuildPrompt(input)
	return []map[string]string{
		{"role": "system", "content": system},
		{"role": "user", "content": user},
	}
}

// TruncateDiff intelligently truncates diff to fit MaxDiffBytes, preserving beginning portion.
func TruncateDiff(diff string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(diff) <= maxBytes {
		return diff, false
	}
	// Reserve space for truncation note
	truncated := diff[:maxBytes]
	// Try to cut at last newline to avoid breaking line in middle
	if idx := strings.LastIndex(truncated, "\n"); idx > maxBytes/2 {
		truncated = truncated[:idx]
	}
	return truncated, true
}

// LimitCommits limits commits to max.
func LimitCommits(commits []CommitInfo, max int) ([]CommitInfo, bool) {
	if max <= 0 || len(commits) <= max {
		return commits, false
	}
	return commits[:max], true
}

// LimitFiles limits changed files to max.
func LimitFiles(files []ChangedFile, max int) ([]ChangedFile, bool) {
	if max <= 0 || len(files) <= max {
		return files, false
	}
	return files[:max], true
}

// EnsurePromptWithinLimit ensures the final prompt is within max bytes, truncating diff further if needed.
func EnsurePromptWithinLimit(input *DescriptionInput, maxPromptBytes int) {
	if maxPromptBytes <= 0 {
		return
	}
	prompt := BuildPrompt(*input)
	if len(prompt) <= maxPromptBytes {
		return
	}
	// First try to reduce diff size proportionally
	over := len(prompt) - maxPromptBytes
	diffLen := len(input.Diff)
	if diffLen > over {
		newDiffLen := diffLen - over - 500 // buffer
		if newDiffLen < 0 {
			newDiffLen = 0
		}
		truncated, _ := TruncateDiff(input.Diff, newDiffLen)
		input.Diff = truncated
		input.Truncated = true
		prompt = BuildPrompt(*input)
		if len(prompt) <= maxPromptBytes {
			return
		}
	}
	// If still too large, truncate entire prompt representation by cutting diff to fit
	if len(prompt) > maxPromptBytes {
		allowedDiff := maxPromptBytes - (len(prompt) - len(input.Diff)) - 500
		if allowedDiff < 0 {
			allowedDiff = 0
		}
		if allowedDiff < len(input.Diff) {
			input.Diff = input.Diff[:allowedDiff]
			input.Truncated = true
		}
	}
	_ = fmt.Sprintf // keep import used
}
