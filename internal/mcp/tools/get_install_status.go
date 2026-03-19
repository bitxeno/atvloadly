package tools

import (
	"context"
	"fmt"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getInstallStatusInput struct {
	AppID uint `json:"app_id" jsonschema:"Required app_id returned by install_app"`
}

type getInstallStatusOutput struct {
	AppID             uint                 `json:"app_id"`
	InstallState      string               `json:"install_state"`
	InstallInProgress bool                 `json:"install_in_progress"`
	IpaName           string               `json:"ipa_name"`
	BundleIdentifier  string               `json:"bundle_identifier"`
	LastRefreshAt     *string              `json:"last_refresh_at,omitempty"`
	LastSuccess       bool                 `json:"last_success"`
	LastErrorCode     model.RefreshedError `json:"last_error_code"`
}

func registerGetInstallStatus(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "get_install_status",
		Description: "Get install status by app_id. " +
			"Install state is derived from task.GetCurrentInstallingApps() and saved DB refresh result.",
	}, handleGetInstallStatus)
}

func handleGetInstallStatus(_ context.Context, _ *sdkmcp.CallToolRequest, input getInstallStatusInput) (*sdkmcp.CallToolResult, getInstallStatusOutput, error) {
	if input.AppID == 0 {
		return nil, getInstallStatusOutput{}, fmt.Errorf("app_id is required")
	}

	app, err := service.GetApp(input.AppID)
	if err != nil {
		return nil, getInstallStatusOutput{}, err
	}

	inProgress := false
	for _, v := range task.GetCurrentInstallingApps() {
		if v.ID == app.ID {
			inProgress = true
			break
		}
	}

	state := resolveInstallState(*app, inProgress)
	var refreshedAt *string
	if app.RefreshedDate != nil {
		formatted := app.RefreshedDate.Format("2006-01-02 15:04:05")
		refreshedAt = &formatted
	}

	return nil, getInstallStatusOutput{
		AppID:             app.ID,
		InstallState:      state,
		InstallInProgress: inProgress,
		IpaName:           app.IpaName,
		BundleIdentifier:  app.BundleIdentifier,
		LastRefreshAt:     refreshedAt,
		LastSuccess:       app.RefreshedResult,
		LastErrorCode:     app.RefreshedError,
	}, nil
}

func resolveInstallState(app model.InstalledApp, inProgress bool) string {
	if inProgress {
		return "in_progress"
	}
	if app.RefreshedDate == nil {
		return "queued_or_unknown"
	}
	if app.RefreshedResult {
		return "completed_success"
	}
	return "completed_failed"
}
