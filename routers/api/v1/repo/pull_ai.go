// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"gitea.dev/models/unit"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/ai"
	"gitea.dev/services/context"
)

// GeneratePullDescription generates a pull request description using AI.
// swagger:operation POST /repos/{owner}/{repo}/pulls/generate-description repository repoGeneratePullDescription
// ---
// summary: Generate pull request description using AI
// consumes:
// - application/json
// produces:
// - application/json
// parameters:
//   - name: owner
//     in: path
//     description: owner of the repo
//     type: string
//     required: true
//   - name: repo
//     in: path
//     description: name of the repo
//     type: string
//     required: true
//   - name: body
//     in: body
//     schema:
//     "$ref": "#/definitions/GeneratePullRequestDescriptionOption"
//
// responses:
//
//	"200":
//	  "$ref": "#/responses/GeneratePullRequestDescriptionResponse"
//	"400":
//	  "$ref": "#/responses/error"
//	"403":
//	  "$ref": "#/responses/forbidden"
//	"404":
//	  "$ref": "#/responses/notFound"
//	"500":
//	  "$ref": "#/responses/error"
//	"503":
//	  "$ref": "#/responses/error"
func GeneratePullDescription(ctx *context.APIContext) {
	// Permission check: user must be allowed to create a PR (write to pulls)
	if !ctx.Repo.Permission.CanWrite(unit.TypePullRequests) {
		ctx.APIError(http.StatusForbidden, "user does not have permission to create pull request")
		return
	}

	if !setting.AI.Enabled {
		ctx.APIError(http.StatusServiceUnavailable, "AI is disabled")
		return
	}
	if setting.AI.BaseURL == "" || setting.AI.Model == "" {
		ctx.APIError(http.StatusInternalServerError, "AI configuration missing")
		return
	}
	// APIKey may be empty for local providers (e.g., Ollama), still allow

	form := web.GetForm[*api.GeneratePullRequestDescriptionOption](ctx)
	if form == nil {
		ctx.APIError(http.StatusBadRequest, "invalid request")
		return
	}
	if form.Base == "" || form.Head == "" {
		ctx.APIError(http.StatusBadRequest, "base and head are required")
		return
	}
	if form.Base == form.Head {
		ctx.APIError(http.StatusBadRequest, "base and head must be different")
		return
	}

	// Obtain compare info
	compareParam := form.Base + "..." + form.Head
	compareInfo, closer := parseCompareInfo(ctx, compareParam)
	if ctx.Written() {
		return
	}
	defer closer()

	if compareInfo == nil {
		ctx.APIErrorInternal(nil)
		return
	}
	if compareInfo.CompareBase == "" {
		ctx.APIError(http.StatusBadRequest, "no common history between base and head")
		return
	}

	// Determine git repo for diff (head repo or base repo sharing)
	gitRepo := ctx.Repo.GitRepo
	if compareInfo.HeadGitRepo != nil {
		gitRepo = compareInfo.HeadGitRepo
	}

	// Build AI input
	input, err := ai.BuildInputFromCompare(ctx, ctx.Repo.Repository, compareInfo, gitRepo)
	if err != nil {
		log.Error("BuildInputFromCompare failed: %v", err)
		ctx.APIErrorInternal(err)
		return
	}

	generator, err := ai.GetGenerator()
	if err != nil {
		if err == ai.ErrAIDisabled {
			ctx.APIError(http.StatusServiceUnavailable, "AI is disabled")
			return
		}
		log.Error("GetGenerator failed: %v", err)
		ctx.APIErrorInternal(err)
		return
	}

	description, err := generator.Generate(ctx, input)
	if err != nil {
		// Map known errors to appropriate status
		log.Error("Generate failed: %v", err)
		// Use generic messages to avoid leaking details
		if err.Error() == "AI provider timeout" {
			ctx.APIError(http.StatusGatewayTimeout, "AI provider timeout")
			return
		}
		if err.Error() == "AI authentication failed" {
			ctx.APIError(http.StatusInternalServerError, "AI authentication failed")
			return
		}
		if err.Error() == "AI provider unavailable" {
			ctx.APIError(http.StatusServiceUnavailable, "AI provider unavailable")
			return
		}
		ctx.APIError(http.StatusInternalServerError, "failed to generate description")
		return
	}

	ctx.JSON(http.StatusOK, &api.GeneratePullRequestDescriptionResponse{
		Description: description,
	})
}
