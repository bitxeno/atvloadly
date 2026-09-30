package web

import (
	"net/http"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/task"
	"github.com/gofiber/fiber/v2"
)

// safeUpdateSettings is the update section exposed through GET /api/settings.
// The plaintext token, its sealed blob and any mask are never returned;
// callers only learn whether a token is configured.
type safeUpdateSettings struct {
	CheckInterval  int  `json:"check_interval"`
	GitHubTokenSet bool `json:"github_token_set"`
}

// safeSettingsResponse mirrors app.SettingsConfiguration but replaces the
// update section with its safe projection.
type safeSettingsResponse struct {
	App          any                `json:"app"`
	Task         any                `json:"task"`
	Update       safeUpdateSettings `json:"update"`
	Notification any                `json:"notification"`
	Network      any                `json:"network"`
}

func safeSettings() safeSettingsResponse {
	s := app.Settings
	update := safeUpdateSettings{}
	if s != nil {
		update.CheckInterval = s.Update.CheckInterval
		update.GitHubTokenSet = app.GitHubTokenConfigured()
		return safeSettingsResponse{
			App:          s.App,
			Task:         s.Task,
			Update:       update,
			Notification: s.Notification,
			Network:      s.Network,
		}
	}
	return safeSettingsResponse{Update: update}
}

// updateSettingsRequest accepts the new flat payload and the legacy nested
// update object sent by older frontends. A present but blank github_token
// removes the stored token; an absent one keeps it.
type updateSettingsRequest struct {
	CheckInterval    *int    `json:"check_interval"`
	GitHubToken      *string `json:"github_token"`
	ClearGitHubToken bool    `json:"clear_github_token"`
	Update           *struct {
		CheckInterval *int    `json:"check_interval"`
		GitHubToken   *string `json:"github_token"`
	} `json:"update"`
}

func saveUpdateSettings(c *fiber.Ctx) error {
	var req updateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusOK).JSON(apiError("Invalid argument. error: " + err.Error()))
	}

	interval := app.Settings.Update.CheckInterval
	if req.CheckInterval != nil {
		interval = *req.CheckInterval
	} else if req.Update != nil && req.Update.CheckInterval != nil {
		interval = *req.Update.CheckInterval
	}
	switch interval {
	case 0, 1, 3, 6, 12, 24:
	default:
		return c.Status(http.StatusOK).JSON(apiError("invalid check interval"))
	}
	app.Settings.Update.CheckInterval = interval

	tokenProvided := req.ClearGitHubToken
	tokenValue := ""
	if req.GitHubToken != nil {
		tokenProvided = true
		tokenValue = strings.TrimSpace(*req.GitHubToken)
	} else if req.Update != nil && req.Update.GitHubToken != nil {
		tokenProvided = true
		tokenValue = strings.TrimSpace(*req.Update.GitHubToken)
	}

	if req.ClearGitHubToken {
		app.DeleteGitHubToken()
	} else if tokenProvided {
		// Saving an empty value removes the token.
		if tokenValue == "" {
			app.DeleteGitHubToken()
		} else if err := app.SetGitHubToken(tokenValue); err != nil {
			return c.Status(http.StatusOK).JSON(apiError(err.Error()))
		}
	}
	// An absent github_token keeps the stored token.

	if err := task.ReloadTask(); err != nil {
		return c.Status(http.StatusOK).JSON(apiError(err.Error()))
	}

	app.SaveSettings()
	return c.Status(http.StatusOK).JSON(apiSuccess(true))
}
