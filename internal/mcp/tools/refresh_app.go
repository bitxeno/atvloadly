package tools

import (
	"context"
	"fmt"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type refreshAppInput struct {
	AppID uint `json:"app_id,omitempty" jsonschema:"Optional app id. If omitted, all expired enabled Apple ID apps will be queued"`
}

type refreshAppOutput struct {
	Mode         string `json:"mode"`
	QueuedCount  int    `json:"queued_count"`
	SkippedCount int    `json:"skipped_count"`
	Message      string `json:"message"`
}

// externalRefreshNote explains what refreshing an app signed with an external
// certificate does.
const externalRefreshNote = "Apps signed with an external certificate are reinstalled with the same signing identity: " +
	"this does not extend their validity, which ends with the certificate or provisioning profile. " +
	"Replace the provisioning profile or import a new signing identity to extend it."

func registerRefreshApp(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "refresh_app",
		Description: "Queue app refresh tasks. " +
			"If app_id is provided, refresh that app. " +
			"If app_id is omitted, refresh all expired enabled apps signed with an Apple account. " +
			externalRefreshNote + " " +
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
		message := "App refresh task queued."
		if app.IsExternalSigning() {
			message = "App reinstall task queued. " + externalRefreshNote
			if app.ExpirationDate != nil {
				message += fmt.Sprintf(" The app stays valid until %s.", app.ExpirationDate.Format("2006-01-02 15:04:05 MST"))
			}
		}
		return nil, refreshAppOutput{
			Mode:         "single",
			QueuedCount:  1,
			SkippedCount: 0,
			Message:      message,
		}, nil
	}

	apps, err := service.GetEnableAppList()
	if err != nil {
		return nil, refreshAppOutput{}, err
	}

	queued := 0
	skipped := 0
	skippedExternal := 0
	for _, app := range apps {
		if !app.IsExpired() {
			skipped++
			continue
		}
		if app.IsExternalSigning() {
			skipped++
			skippedExternal++
			continue
		}

		task.RefreshApp(app)
		queued++
	}

	log.Infof("MCP refresh_app queued=%d skipped=%d", queued, skipped)
	message := "Expired app refresh tasks queued."
	if skippedExternal > 0 {
		message += fmt.Sprintf(" %d expired app(s) signed with an external certificate were skipped. %s Refresh them with app_id to reinstall them.", skippedExternal, externalRefreshNote)
	}
	return nil, refreshAppOutput{
		Mode:         "expired_all",
		QueuedCount:  queued,
		SkippedCount: skipped,
		Message:      message,
	}, nil
}
