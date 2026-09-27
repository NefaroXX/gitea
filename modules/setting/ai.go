// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"gitea.dev/modules/setting/config"
)

// AIStruct holds the AI settings. They are registered as dynamic config options so that they
// can be edited from the admin "Config Settings" page, while still being configurable from
// app.ini through the file-config binding.
//
// The numeric limits are declared as strings because the admin config form only round-trips
// strings and booleans faithfully; they are parsed back to numbers by the AI service.
type AIStruct struct {
	Enabled    *config.Option[bool]
	BaseURL    *config.Option[string]
	APIKey     *config.Option[string]
	Model      *config.Option[string]
	MaxDiff    *config.Option[string]
	MaxFiles   *config.Option[string]
	MaxCommits *config.Option[string]
	MaxPrompt  *config.Option[string]
	Timeout    *config.Option[string]
}

func newAIStruct() *AIStruct {
	return &AIStruct{
		Enabled:    config.NewOption[bool]("ai.enabled").WithDefaultSimple(false).WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "ENABLED"}),
		BaseURL:    config.NewOption[string]("ai.base_url").WithDefaultSimple("https://api.openai.com/v1").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "BASE_URL"}),
		APIKey:     config.NewOption[string]("ai.api_key").WithEmptyAsDefault().WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "API_KEY"}),
		Model:      config.NewOption[string]("ai.model").WithDefaultSimple("gpt-3.5-turbo").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MODEL"}),
		MaxDiff:    config.NewOption[string]("ai.max_diff_bytes").WithDefaultSimple("50000").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_DIFF_BYTES"}),
		MaxFiles:   config.NewOption[string]("ai.max_files").WithDefaultSimple("50").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_FILES"}),
		MaxCommits: config.NewOption[string]("ai.max_commits").WithDefaultSimple("20").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_COMMITS"}),
		MaxPrompt:  config.NewOption[string]("ai.max_prompt_bytes").WithDefaultSimple("100000").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "MAX_PROMPT_BYTES"}),
		Timeout:    config.NewOption[string]("ai.request_timeout").WithDefaultSimple("30").WithFileConfig(config.CfgSecKey{Sec: "ai", Key: "REQUEST_TIMEOUT"}),
	}
}
