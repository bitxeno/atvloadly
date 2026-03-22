package tools

import (
	"context"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type refreshAppInput struct {
	AppID uint `json:"app_id,omitempty" jsonschema:"Optional app id. If omitted, all expired enabled apps will be queued"`
}

type refreshAppOutput struct {
	Mode         string `json:"mode"`
	QueuedCount  int    `json:"queued_count"`
	SkippedCount int    `json:"skipped_count"`
	Message      string `json:"message"`
}

func registerRefreshApp(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "refresh_app",
		Description: "Queue app refresh tasks. " +
			"If app_id is provided, refresh that app. " +
			"If app_id is omitted, refresh all expired enabled apps. " +
			"Call get_refresh_status to check in_progress/completed results.",
	}, handleRefreshApp)
}

func handleRefreshApp(_ context.Context, _ *sdkmcp.CallToolRequest, input refreshAppInput) (*sdkmcp.CallToolResult, refreshAppOutput, error) {
	if input.AppID > 0 {
		app, err := service.GetApp(input.AppID)
		if err != nil {
			return nil, refreshAppOutput{}, err
		}

		task.RefreshApp(*app)
		log.Infof("MCP refresh_app queued app id=%d name=%s", app.ID, app.IpaName)
		return nil, refreshAppOutput{
			Mode:         "single",
			QueuedCount:  1,
			SkippedCount: 0,
			Message:      "App refresh task queued.",
		}, nil
	}

	apps, err := service.GetEnableAppList()
	if err != nil {
		return nil, refreshAppOutput{}, err
	}

	queued := 0
	skipped := 0
	for _, app := range apps {
		if !app.IsExpired() {
			skipped++
			continue
		}

		task.RefreshApp(app)
		queued++
	}

	log.Infof("MCP refresh_app queued=%d skipped=%d", queued, skipped)
	return nil, refreshAppOutput{
		Mode:         "expired_all",
		QueuedCount:  queued,
		SkippedCount: skipped,
		Message:      "Expired app refresh tasks queued.",
	}, nil
}
