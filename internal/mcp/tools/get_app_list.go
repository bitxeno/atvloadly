package tools

import (
	"context"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getAppListInput struct {
	OnlyExpired bool `json:"only_expired,omitempty" jsonschema:"Set to true to return only expired apps"`
}

type appListItem struct {
	ID               uint       `json:"id"`
	IpaName          string     `json:"ipa_name"`
	BundleIdentifier string     `json:"bundle_identifier"`
	Account          string     `json:"account"`
	Device           string     `json:"device"`
	UDID             string     `json:"udid"`
	Version          string     `json:"version"`
	Enabled          bool       `json:"enabled"`
	RefreshedDate    *time.Time `json:"refreshed_date,omitempty"`
	ExpirationDate   *time.Time `json:"expiration_date,omitempty"`
	RefreshedResult  bool       `json:"refreshed_result"`
	IsExpired        bool       `json:"is_expired"`
}

type getAppListOutput struct {
	Total int           `json:"total"`
	Items []appListItem `json:"items"`
}

func registerGetAppList(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "get_app_list",
		Description: "Get all apps. " +
			"Set only_expired=true to return only expired apps. " +
			"Expired means ExpirationDate is before now. " +
			"Call refresh_app with app_id to refresh an app or Call refresh_app without app_id to refresh all apps.",
	}, handleGetAppList)
}

func handleGetAppList(_ context.Context, _ *sdkmcp.CallToolRequest, input getAppListInput) (*sdkmcp.CallToolResult, getAppListOutput, error) {
	apps, err := service.GetEnableAppList()
	if err != nil {
		return nil, getAppListOutput{}, err
	}

	filtered := make([]appListItem, 0)
	for _, app := range apps {
		if input.OnlyExpired && !app.IsExpired() {
			continue
		}
		filtered = append(filtered, toAppListItem(app))
	}

	return nil, getAppListOutput{
		Total: len(filtered),
		Items: filtered,
	}, nil
}

func toAppListItem(app model.InstalledApp) appListItem {
	return appListItem{
		ID:               app.ID,
		IpaName:          app.IpaName,
		BundleIdentifier: app.BundleIdentifier,
		Account:          app.Account,
		Device:           app.Device,
		UDID:             app.UDID,
		Version:          app.Version,
		Enabled:          app.Enabled,
		RefreshedDate:    app.RefreshedDate,
		ExpirationDate:   app.ExpirationDate,
		RefreshedResult:  app.RefreshedResult,
		IsExpired:        app.IsExpired(),
	}
}
