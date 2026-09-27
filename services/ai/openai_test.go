// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildPrompt(t *testing.T) {
	input := DescriptionInput{
		RepositoryName: "owner/repo",
		BaseBranch:     "main",
		HeadBranch:     "feature",
		Commits: []CommitInfo{
			{SHA: "abc123", Subject: "feat: add feature", Body: "details"},
		},
		ChangedFiles: []ChangedFile{{Path: "main.go", Status: "modified"}},
		DiffStat:     "1 files changed, 10 insertions(+), 2 deletions(-)",
		Diff:         "diff --git a/main.go b/main.go\n+foo",
		PRTemplate:   "## Summary\n\n{{content}}",
	}
	prompt := BuildPrompt(input)
	if len(prompt) == 0 {
		t.Fatal("expected prompt not empty")
	}
	if !contains(prompt, "owner/repo") {
		t.Fatalf("prompt should contain repo name")
	}
	if !contains(prompt, "abc123") {
		t.Fatalf("prompt should contain commit SHA")
	}
	// Test truncation notice
	input.Truncated = true
	prompt = BuildPrompt(input)
	if !contains(prompt, "truncated") {
		t.Fatalf("expected truncation notice")
	}
}

func TestTruncateDiff(t *testing.T) {
	diff := "a\nb\nc\nd\ne\nf"
	truncated, isTruncated := TruncateDiff(diff, 5)
	if !isTruncated {
		t.Fatal("expected truncated")
	}
	if len(truncated) > 5 {
		t.Fatalf("truncated length %d > 5", len(truncated))
	}
	// Not truncated when within limit
	_, isTruncated = TruncateDiff(diff, 100)
	if isTruncated {
		t.Fatal("should not be truncated")
	}
}

func TestOpenAIProviderSuccess(t *testing.T) {
	var gotAuth, gotModel string
	var gotBody map[string]any
	captured := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		gotBody = req
		if m, ok := req["model"].(string); ok {
			gotModel = m
		}
		captured = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "## Summary\n\nTest description"}},
			},
		})
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 5 * time.Second,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
	input := DescriptionInput{
		RepositoryName: "owner/repo",
		BaseBranch:     "main",
		HeadBranch:     "feature",
		Commits:        []CommitInfo{{SHA: "abc", Subject: "test"}},
		Diff:           "diff",
	}
	desc, err := provider.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !captured {
		t.Fatal("handler not captured")
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected Bearer test-key, got %q", gotAuth)
	}
	if gotModel != "test-model" {
		t.Fatalf("expected test-model, got %q", gotModel)
	}
	if gotBody == nil {
		t.Fatal("body not captured")
	}
	if _, ok := gotBody["messages"]; !ok {
		t.Fatal("expected messages in body")
	}
	if desc != "## Summary\n\nTest description" {
		t.Fatalf("unexpected desc %q", desc)
	}
}

func TestOpenAIProviderErrors(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		expectErr string
	}{
		{
			name: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "invalid key"}})
			},
			expectErr: "authentication",
		},
		{
			name: "empty choices",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
			},
			expectErr: "empty",
		},
		{
			name: "empty content",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": ""}}}})
			},
			expectErr: "empty",
		},
		{
			name: "invalid json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not json"))
			},
			expectErr: "invalid",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("oops"))
			},
			expectErr: "unavailable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			provider := &OpenAIProvider{
				BaseURL: server.URL,
				APIKey:  "key",
				Model:   "m",
				Timeout: 2 * time.Second,
				Client:  &http.Client{Timeout: 2 * time.Second},
			}
			_, err = provider.Generate(context.Background(), DescriptionInput{RepositoryName: "a/b", BaseBranch: "main", HeadBranch: "feat", Diff: "d"})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.expectErr)
			}
			if !contains(err.Error(), tc.expectErr) {
				t.Fatalf("expected error containing %q, got %q", tc.expectErr, err.Error())
			}
		})
	}
}

func TestOpenAIProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": "hi"}}}})
	}))
	defer server.Close()
	provider := &OpenAIProvider{
		BaseURL: server.URL,
		APIKey:  "key",
		Model:   "m",
		Timeout: time.Second,
		Client:  &http.Client{Timeout: time.Second},
	}
	_, err = provider.Generate(context.Background(), DescriptionInput{RepositoryName: "a/b", BaseBranch: "main", HeadBranch: "feat"})
	if err == nil || !contains(err.Error(), "timeout") && !contains(err.Error(), "unavailable") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestAIDisabled(t *testing.T) {
	// a disabled provider must never reach the network; NewOpenAIProvider reads the
	// dynamic config which defaults to disabled
	_, err := NewOpenAIProvider(t.Context())
	if err != ErrAIDisabled {
		t.Fatalf("expected ErrAIDisabled, got %v", err)
	}
}

func TestEnsurePromptWithinLimit(t *testing.T) {
	input := DescriptionInput{
		RepositoryName: "owner/repo",
		BaseBranch:     "main",
		HeadBranch:     "feature",
		Diff:           string(make([]byte, 10000)),
	}
	for i := range input.Diff {
		// fill with 'a'
		b := []byte(input.Diff)
		b[i] = 'a'
		input.Diff = string(b)
	}
	EnsurePromptWithinLimit(&input, 5000)
	prompt := BuildPrompt(input)
	if len(prompt) > 5000 {
		t.Fatalf("prompt length %d exceeds limit 5000", len(prompt))
	}
	if !input.Truncated {
		t.Fatal("expected truncated true")
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
