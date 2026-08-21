// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"

	"gitea.dev/modules/log"
)

// AI settings
var AI = struct {
	Enabled        bool   `ini:"ENABLED"`
	BaseURL        string `ini:"BASE_URL"`
	APIKey         string `ini:"API_KEY"`
	Model          string `ini:"MODEL"`
	MaxDiffBytes   int    `ini:"MAX_DIFF_BYTES"`
	MaxCommits     int    `ini:"MAX_COMMITS"`
	MaxFiles       int    `ini:"MAX_FILES"`
	MaxPromptBytes int    `ini:"MAX_PROMPT_BYTES"`
	RequestTimeout int    `ini:"REQUEST_TIMEOUT"`
}{
	Enabled:        false,
	BaseURL:        "https://api.openai.com/v1",
	APIKey:         "",
	Model:          "gpt-3.5-turbo",
	MaxDiffBytes:   50000,
	MaxCommits:     20,
	MaxFiles:       50,
	MaxPromptBytes: 100000,
	RequestTimeout: 30,
}

func loadAISettingFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("ai")
	if err := sec.MapTo(&AI); err != nil {
		log.Fatal("Failed to map AI settings: %v", err)
	}
	// normalize BaseURL: trim trailing slash
	AI.BaseURL = strings.TrimRight(strings.TrimSpace(AI.BaseURL), "/")
	if AI.MaxDiffBytes <= 0 {
		AI.MaxDiffBytes = 50000
	}
	if AI.MaxCommits <= 0 {
		AI.MaxCommits = 20
	}
	if AI.MaxFiles <= 0 {
		AI.MaxFiles = 50
	}
	if AI.MaxPromptBytes <= 0 {
		AI.MaxPromptBytes = 100000
	}
	if AI.RequestTimeout <= 0 {
		AI.RequestTimeout = 30
	}
}
