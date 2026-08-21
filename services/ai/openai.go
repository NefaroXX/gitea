// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

// OpenAIProvider is an OpenAI-compatible HTTP client.
type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
	Timeout time.Duration
}

// NewOpenAIProvider creates a provider from current setting.AI.
func NewOpenAIProvider() (*OpenAIProvider, error) {
	if !setting.AI.Enabled {
		return nil, ErrAIDisabled
	}
	if setting.AI.BaseURL == "" || setting.AI.Model == "" {
		return nil, ErrAIConfigMissing
	}
	if setting.AI.APIKey == "" {
		// allow empty for local Ollama etc. but warn
		log.Warn("AI API key is empty, proceeding for local provider")
	}
	timeout := time.Duration(setting.AI.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &OpenAIProvider{
		BaseURL: setting.AI.BaseURL,
		APIKey:  setting.AI.APIKey,
		Model:   setting.AI.Model,
		Client:  &http.Client{Timeout: timeout},
		Timeout: timeout,
	}, nil
}

// GetGenerator returns a configured DescriptionGenerator.
func GetGenerator() (DescriptionGenerator, error) {
	return NewOpenAIProvider()
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []map[string]string `json:"messages"`
	Temperature float32             `json:"temperature,omitempty"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	// alternative error format
	ErrorMsg string `json:"error_msg"`
}

// Generate generates a description using OpenAI-compatible API.
func (p *OpenAIProvider) Generate(ctx context.Context, input DescriptionInput) (string, error) {
	// Ensure prompt limits
	EnsurePromptWithinLimit(&input, setting.AI.MaxPromptBytes)

	messages := BuildMessages(input)

	reqBody := openAIChatRequest{
		Model:    p.Model,
		Messages: messages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := p.BaseURL + "/chat/completions"

	// Use context with timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		// Do not expose URL or key
		log.Error("AI provider request failed: %v", err)
		if ctxTimeout.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("AI provider timeout")
		}
		return "", fmt.Errorf("AI provider unavailable")
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024)) // limit 2MB
	if err != nil {
		log.Error("AI provider read response failed: %v", err)
		return "", fmt.Errorf("AI provider returned invalid response")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error("AI provider returned status %d: %s", resp.StatusCode, string(respBytes))
		// try to parse error message safely
		var errResp openAIChatResponse
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("AI provider error: %s", errResp.Error.Message)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("AI authentication failed")
		}
		return "", fmt.Errorf("AI provider unavailable")
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		log.Error("AI provider invalid JSON: %v", err)
		return "", fmt.Errorf("AI provider returned invalid response")
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		log.Error("AI provider error: %s", chatResp.Error.Message)
		return "", fmt.Errorf("AI provider error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		log.Error("AI provider returned no choices")
		return "", fmt.Errorf("AI provider returned empty response")
	}
	content := chatResp.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("AI provider returned empty response")
	}
	return content, nil
}
