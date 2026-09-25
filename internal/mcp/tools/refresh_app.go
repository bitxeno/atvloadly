package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/signing"
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

// expiryLayout formats the dates reported to the agent.
const expiryLayout = "2006-01-02 15:04:05 MST"

// externalRefreshNote explains what refreshing an app signed with an external
// certificate does to its validity.
const externalRefreshNote = "Refreshing an app signed with an external certificate reinstalls it with the current certificate and provisioning profile of its signing identity: " +
	"the app then stays valid until the identity expires (the earlier of the certificate and profile expiry). " +
	"With an unchanged certificate and profile, a reinstall does not extend its validity. " +
	"To extend it, replace the provisioning profile of the identity and refresh the app, or import a new signing identity and install the app with it."

func registerRefreshApp(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "refresh_app",
		Description: "Queue app refresh tasks. " +
			"If app_id is provided, refresh that app. " +
			"If app_id is omitted, refresh all expired enabled apps signed with an Apple account; expired apps signed with an external certificate are skipped. " +
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

		message := "App refresh task queued."
		if app.IsExternalSigning() {
			message, err = externalReinstallMessage(*app, time.Now())
			if err != nil {
				return nil, refreshAppOutput{}, err
			}
		}
		task.RefreshApp(*app)
		log.Infof("MCP refresh_app queued app id=%d name=%s", app.ID, app.IpaName)
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
		message += fmt.Sprintf(" %d expired app(s) signed with an external certificate were skipped: they expire with their signing identity, "+
			"so reinstalling one succeeds only once that identity is valid again. "+
			"Replace the provisioning profile of the identity and then refresh the app with app_id, "+
			"or import a new signing identity and install the app with it.", skippedExternal)
	}
	return nil, refreshAppOutput{
		Mode:         "expired_all",
		QueuedCount:  queued,
		SkippedCount: skipped,
		Message:      message,
	}, nil
}

// externalReinstallMessage describes the queued reinstall of an app signed
// with an external certificate. The reinstall signs with the current
// certificate and profile of the identity, so the app gets the current expiry
// of the identity; it fails when the identity is missing or expired at now.
func externalReinstallMessage(app model.InstalledApp, now time.Time) (string, error) {
	identity, err := service.GetSigningIdentity(app.SigningIdentityID)
	if signing.CodeOf(err) == signing.CodeIdentityNotFound {
		return fmt.Sprintf("App reinstall task queued, but it will fail: signing identity %d of the app no longer exists. "+
			"Import a new signing identity and install the app with it.", app.SigningIdentityID), nil
	}
	if err != nil {
		return "", fmt.Errorf("load signing identity %d of app %d: %w", app.SigningIdentityID, app.ID, err)
	}

	if now.After(identity.CertificateNotAfter) {
		return fmt.Sprintf("App reinstall task queued, but it will fail: the certificate of its signing identity expired on %s. "+
			"Import a new signing identity and install the app with it.", identity.CertificateNotAfter.Format(expiryLayout)), nil
	}
	if now.After(identity.ProfileExpirationDate) {
		return fmt.Sprintf("App reinstall task queued, but it will fail: the provisioning profile of its signing identity expired on %s. "+
			"Replace the provisioning profile of the identity and then refresh the app again, "+
			"or import a new signing identity and install the app with it.", identity.ProfileExpirationDate.Format(expiryLayout)), nil
	}

	expiry := identity.ExpiresAt()
	message := fmt.Sprintf("App reinstall task queued. It signs the app with the current certificate and provisioning profile of its signing identity, "+
		"so the app then stays valid until %s.", expiry.Format(expiryLayout))
	if app.ExpirationDate != nil && !expiry.After(*app.ExpirationDate) {
		message += " This does not extend its validity: replace the provisioning profile of the identity, " +
			"or import a new signing identity and install the app with it, to extend it."
	}
	return message, nil
}
