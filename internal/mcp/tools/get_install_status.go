package tools

import (
	"context"

	"github.com/bitxeno/atvloadly/internal/task"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getInstallStatusInput struct{}

type getInstallStatusOutput struct {
	InstallState      string `json:"install_state"`
	InstallInProgress bool   `json:"install_in_progress"`
}

func registerGetInstallStatus(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "get_install_status",
		Description: "Get install app status. " +
			"Returns in_progress when there is any active install app task, otherwise completed.",
	}, handleGetInstallStatus)
}

func handleGetInstallStatus(_ context.Context, _ *sdkmcp.CallToolRequest, input getInstallStatusInput) (*sdkmcp.CallToolResult, getInstallStatusOutput, error) {
	_ = input
	if len(task.GetCurrentInstallingApps()) == 0 {
		return nil, getInstallStatusOutput{
			InstallState:      "completed",
			InstallInProgress: false,
		}, nil
	}

	return nil, getInstallStatusOutput{
		InstallState:      "in_progress",
		InstallInProgress: true,
	}, nil
}
