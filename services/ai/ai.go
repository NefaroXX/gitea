// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ai

import (
	"context"
	"errors"
)

// Sentinel errors
var (
	ErrAIDisabled      = errors.New("AI is disabled")
	ErrAIConfigMissing = errors.New("AI configuration missing")
)

// CommitInfo represents a single commit in the PR.
type CommitInfo struct {
	SHA     string
	Subject string
	Body    string
}

// ChangedFile represents a file changed in the PR.
type ChangedFile struct {
	Path      string
	Status    string
	Additions int
	Deletions int
}

// DescriptionInput is the provider-neutral input for generating a PR description.
type DescriptionInput struct {
	RepositoryName string
	BaseBranch     string
	HeadBranch     string

	Commits      []CommitInfo
	ChangedFiles []ChangedFile
	DiffStat     string
	Diff         string

	PRTemplate string
	Truncated  bool
}

// DescriptionGenerator generates a PR description from the given input.
type DescriptionGenerator interface {
	Generate(ctx context.Context, input DescriptionInput) (string, error)
}
