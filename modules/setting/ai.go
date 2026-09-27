// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"gitea.dev/modules/setting/config"
)

// AIStruct holds the AI settings. They are registered as dynamic config options so that they
// can be edited from the admin "Config Settings" page, while still being configurable from
// app.ini through the file-config binding.
type AIStruct struct {
	Enabled    *config.Option[bool]
	BaseURL    *config.Option[string]
	APIKey     *config.Option[string]
	Model      *config.Option[string]
	MaxDiff    *config.Option[int]
	MaxFiles   *config.Option[int]
	MaxCommits *config.Option[int]
	MaxPrompt  *config.Option[int]
	Timeout    *config.Option[int]
}

func newAIStruct() *AIStruct {
	return &AIStruct{
		Enabled:    config.NewOption[bool]("ai.enabled").WithDefaultSimple(false).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "ENABLED"}),
		BaseURL:    config.NewOption[string]("ai.base_url").WithDefaultSimple("https://api.openai.com/v1").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "BASE_URL"}),
		APIKey:     config.NewOption[string]("ai.api_key").WithEmptyAsDefault().WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "API_KEY"}),
		Model:      config.NewOption[string]("ai.model").WithDefaultSimple("gpt-3.5-turbo").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MODEL"}),
		MaxDiff:    config.NewOption[int]("ai.max_diff_bytes").WithDefaultSimple(50000).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_DIFF_BYTES"}),
		MaxFiles:   config.NewOption[int]("ai.max_files").WithDefaultSimple(50).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_FILES"}),
		MaxCommits: config.NewOption[int]("ai.max_commits").WithDefaultSimple(20).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_COMMITS"}),
		MaxPrompt:  config.NewOption[int]("ai.max_prompt_bytes").WithDefaultSimple(100000).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_PROMPT_BYTES"}),
		Timeout:    config.NewOption[int]("ai.request_timeout").WithDefaultSimple(30).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "REQUEST_TIMEOUT"}),
	}
}
