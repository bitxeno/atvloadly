package tools

import (
	"context"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchAppInput struct {
	Keyword     string `json:"keyword,omitempty" jsonschema:"Keyword to search in app name, bundle identifier, account or device"`
	UDID        string `json:"udid,omitempty" jsonschema:"Optional device UDID filter"`
	OnlyExpired bool   `json:"only_expired,omitempty" jsonschema:"Set to true to return only expired apps"`
	Offset      int    `json:"offset,omitempty" jsonschema:"Result offset for pagination (default: 0)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Result limit for pagination (default: no limit)"`
}

type searchAppItem struct {
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

type searchAppOutput struct {
	HowToGetExpiredApps string          `json:"how_to_get_expired_apps"`
	Total               int             `json:"total"`
	Offset              int             `json:"offset"`
	Limit               int             `json:"limit"`
	Items               []searchAppItem `json:"items"`
}

func registerSearchApp(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "search_app",
		Description: "Search enabled apps by keyword or UDID. " +
			"To get expired apps, set only_expired=true. " +
			"Expired means ExpirationDate is before now. " +
			"Call refresh_app with app_id to refresh an app or Call refresh_app without app_id to refresh all apps.",
	}, handleSearchApp)
}

func handleSearchApp(_ context.Context, _ *sdkmcp.CallToolRequest, input searchAppInput) (*sdkmcp.CallToolResult, searchAppOutput, error) {
	var (
		apps []model.InstalledApp
		err  error
	)

	if strings.TrimSpace(input.UDID) != "" {
		apps, err = service.GetEnableAppListByUDID(strings.TrimSpace(input.UDID))
	} else {
		apps, err = service.GetEnableAppList()
	}
	if err != nil {
		return nil, searchAppOutput{}, err
	}

	keyword := strings.ToLower(strings.TrimSpace(input.Keyword))
	filtered := make([]searchAppItem, 0)
	for _, app := range apps {
		if input.OnlyExpired && !app.IsExpired() {
			continue
		}
		if keyword != "" && !matchAppKeyword(app, keyword) {
			continue
		}
		filtered = append(filtered, toSearchAppItem(app))
	}

	total := len(filtered)
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}

	limit := input.Limit
	if limit <= 0 {
		limit = total - offset
	}
	end := offset + limit
	if end > total {
		end = total
	}

	items := filtered[offset:end]
	return nil, searchAppOutput{
		HowToGetExpiredApps: "Set only_expired=true. Expired apps are determined by ExpirationDate < now.",
		Total:               total,
		Offset:              offset,
		Limit:               limit,
		Items:               items,
	}, nil
}

func matchAppKeyword(app model.InstalledApp, keyword string) bool {
	fields := []string{
		app.IpaName,
		app.BundleIdentifier,
		app.Account,
		app.Device,
	}

	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

func toSearchAppItem(app model.InstalledApp) searchAppItem {
	return searchAppItem{
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
