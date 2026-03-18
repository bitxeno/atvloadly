package tools

import (
	"context"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getRefreshStatusInput struct {
	AppID          uint   `json:"app_id,omitempty" jsonschema:"Optional app ID. If set, returns status for a single app"`
	UDID           string `json:"udid,omitempty" jsonschema:"Optional device UDID filter when app_id is not provided"`
	OnlyInProgress bool   `json:"only_in_progress,omitempty" jsonschema:"If true, return only apps that are currently refreshing"`
}

type getRefreshStatusItem struct {
	AppID             uint                 `json:"app_id"`
	IpaName           string               `json:"ipa_name"`
	BundleIdentifier  string               `json:"bundle_identifier"`
	UDID              string               `json:"udid"`
	RefreshState      string               `json:"refresh_state"`
	RefreshInProgress bool                 `json:"refresh_in_progress"`
	LastRefreshAt     *time.Time           `json:"last_refresh_at,omitempty"`
	LastSuccess       bool                 `json:"last_success"`
	LastErrorCode     model.RefreshedError `json:"last_error_code"`
	IsExpired         bool                 `json:"is_expired"`
}

type getRefreshStatusOutput struct {
	Summary struct {
		InProgressCount int `json:"in_progress_count"`
		SuccessCount    int `json:"success_count"`
		FailedCount     int `json:"failed_count"`
		UnknownCount    int `json:"unknown_count"`
	} `json:"summary"`
	Items []getRefreshStatusItem `json:"items"`
}

func registerGetRefreshStatus(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "get_refresh_status",
		Description: "Get real-time app refresh status. " +
			"refresh_state is one of: in_progress, completed_success, completed_failed, unknown. " +
			"Use this tool after refresh_app to let AI know current progress and final result.",
	}, handleGetRefreshStatus)
}

func handleGetRefreshStatus(_ context.Context, _ *sdkmcp.CallToolRequest, input getRefreshStatusInput) (*sdkmcp.CallToolResult, getRefreshStatusOutput, error) {
	apps, err := loadAppsForRefreshStatus(input)
	if err != nil {
		return nil, getRefreshStatusOutput{}, err
	}

	inProgressMap := getInstallingAppIDMap()
	items := make([]getRefreshStatusItem, 0, len(apps))
	var output getRefreshStatusOutput

	for _, app := range apps {
		inProgress := inProgressMap[app.ID]
		state := resolveRefreshState(app, inProgress)

		if input.OnlyInProgress && !inProgress {
			continue
		}

		item := getRefreshStatusItem{
			AppID:             app.ID,
			IpaName:           app.IpaName,
			BundleIdentifier:  app.BundleIdentifier,
			UDID:              app.UDID,
			RefreshState:      state,
			RefreshInProgress: inProgress,
			LastRefreshAt:     app.RefreshedDate,
			LastSuccess:       app.RefreshedResult,
			LastErrorCode:     app.RefreshedError,
			IsExpired:         app.IsExpired(),
		}

		switch state {
		case "in_progress":
			output.Summary.InProgressCount++
		case "completed_success":
			output.Summary.SuccessCount++
		case "completed_failed":
			output.Summary.FailedCount++
		default:
			output.Summary.UnknownCount++
		}

		items = append(items, item)
	}

	output.Items = items
	return nil, output, nil
}

func loadAppsForRefreshStatus(input getRefreshStatusInput) ([]model.InstalledApp, error) {
	if input.AppID > 0 {
		app, err := service.GetApp(input.AppID)
		if err != nil {
			return nil, err
		}
		return []model.InstalledApp{*app}, nil
	}

	if strings.TrimSpace(input.UDID) != "" {
		return service.GetEnableAppListByUDID(strings.TrimSpace(input.UDID))
	}

	return service.GetEnableAppList()
}

func getInstallingAppIDMap() map[uint]bool {
	apps := task.GetCurrentInstallingApps()
	m := make(map[uint]bool, len(apps))
	for _, app := range apps {
		m[app.ID] = true
	}
	return m
}

func resolveRefreshState(app model.InstalledApp, inProgress bool) string {
	if inProgress {
		return "in_progress"
	}
	if app.RefreshedDate == nil {
		return "unknown"
	}
	if app.RefreshedResult {
		return "completed_success"
	}
	return "completed_failed"
}
