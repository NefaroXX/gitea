// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"net/http"
	"strings"

	"gitea.dev/models/unit"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/services/ai"
	"gitea.dev/services/context"
)

// GenerateDescription handles AI generation for web UI.
// It expects POST with JSON body {base, head} or form values.
func GenerateDescription(ctx *context.Context) {
	if !ctx.Repo.Permission.CanWrite(unit.TypePullRequests) {
		ctx.JSONError(ctx.Tr("repo.pulls.no_permission"))
		return
	}
	if !setting.AI.Enabled {
		ctx.JSON(http.StatusServiceUnavailable, map[string]string{"message": "AI is disabled"})
		return
	}
	if setting.AI.BaseURL == "" || setting.AI.Model == "" {
		ctx.JSON(http.StatusInternalServerError, map[string]string{"message": "AI configuration missing"})
		return
	}

	// Try JSON body first, then form
	var base, head string
	if ctx.Req.Method == http.MethodPost {
		// Try to decode JSON if content-type is json
		if strings.Contains(ctx.Req.Header.Get("Content-Type"), "application/json") {
			var req struct {
				Base string `json:"base"`
				Head string `json:"head"`
			}
			if err := json.NewDecoder(ctx.Req.Body).Decode(&req); err == nil {
				base = req.Base
				head = req.Head
			}
		}
		if base == "" {
			base = ctx.FormString("base")
		}
		if head == "" {
			head = ctx.FormString("head")
		}
	}
	if base == "" || head == "" {
		// Try to infer from compare path if available in query
		compareParam := ctx.PathParam("*")
		if compareParam != "" {
			// compareParam like "main...feature" - try to split
			// For simplicity, require explicit base/head
		}
		ctx.JSON(http.StatusBadRequest, map[string]string{"message": "base and head are required"})
		return
	}
	if base == head {
		ctx.JSON(http.StatusBadRequest, map[string]string{"message": "base and head must be different"})
		return
	}

	// Build compare info similar to compare page
	// Use git_service.GetCompareInfo with resolved refs
	comparePageInfo := newComparePageInfo()
	// We need to construct compareParam for parseCompareInfo: "base...head"
	compareParam := base + "..." + head
	err := comparePageInfo.parseCompareInfo(ctx, compareParam)
	if err != nil {
		log.Error("parseCompareInfo failed: %v", err)
		ctx.JSON(http.StatusBadRequest, map[string]string{"message": "invalid base or head"})
		return
	}
	if ctx.Written() {
		return
	}
	ci := comparePageInfo.compareInfo
	if ci == nil || ci.CompareBase == "" {
		ctx.JSON(http.StatusBadRequest, map[string]string{"message": "no common history"})
		return
	}

	gitRepo := ctx.Repo.GitRepo
	if ci.HeadGitRepo != nil {
		gitRepo = ci.HeadGitRepo
	}

	input, err := ai.BuildInputFromCompare(ctx, ctx.Repo.Repository, ci, gitRepo)
	if err != nil {
		log.Error("BuildInputFromCompare failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to build context"})
		return
	}

	generator, err := ai.GetGenerator()
	if err != nil {
		log.Error("GetGenerator failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]string{"message": "AI provider not configured"})
		return
	}

	desc, err := generator.Generate(ctx, input)
	if err != nil {
		log.Error("AI generate failed: %v", err)
		// Map to user-friendly message without leaking keys
		msg := "failed to generate description"
		if err.Error() == "AI provider timeout" {
			msg = "AI provider timeout"
			ctx.JSON(http.StatusGatewayTimeout, map[string]string{"message": msg})
			return
		}
		if err.Error() == "AI provider unavailable" || err.Error() == "AI authentication failed" {
			msg = err.Error()
			ctx.JSON(http.StatusServiceUnavailable, map[string]string{"message": msg})
			return
		}
		ctx.JSON(http.StatusInternalServerError, map[string]string{"message": msg})
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"description": desc})
}
