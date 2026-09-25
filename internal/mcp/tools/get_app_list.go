package tools

import (
	"context"
	"time"

	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/utils"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getAppListInput struct {
	OnlyExpired bool `json:"only_expired,omitempty" jsonschema:"Set to true to return only expired apps"`
}

type appListItem struct {
	ID               uint              `json:"id"`
	IpaName          string            `json:"ipa_name"`
	BundleIdentifier string            `json:"bundle_identifier"`
	Account          string            `json:"account"`
	Device           string            `json:"device"`
	Version          string            `json:"version"`
	Enabled          bool              `json:"enabled"`
	ExpirationDate   *time.Time        `json:"expiration_date,omitempty"`
	IsExpired        bool              `json:"is_expired"`
	SigningMode      model.SigningMode `json:"signing_mode"`
	// SigningIdentityID is the external signing identity of the app.
	SigningIdentityID uint `json:"signing_identity_id,omitempty"`
	// AutoRefresh reports whether the scheduled refresh renews the app; apps
	// signed with an external certificate expire with their signing identity.
	AutoRefresh bool `json:"auto_refresh"`
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
			"signing_mode is apple_id or external_certificate; auto_refresh=false apps (external_certificate) are not refreshed automatically and a refresh reinstalls them without extending their validity. " +
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
	deviceName := "Unpaired"
	if dev, found := manager.GetDeviceByUDID(app.UDID); found && dev != nil {
		deviceName = dev.Name
	}

	return appListItem{
		ID:                app.ID,
		IpaName:           app.IpaName,
		BundleIdentifier:  app.BundleIdentifier,
		Account:           utils.MaskEmail(app.Account),
		Device:            deviceName,
		Version:           app.Version,
		Enabled:           app.Enabled,
		ExpirationDate:    app.ExpirationDate,
		IsExpired:         app.IsExpired(),
		SigningMode:       app.EffectiveSigningMode(),
		SigningIdentityID: app.SigningIdentityID,
		AutoRefresh:       !app.IsExternalSigning(),
	}
}
