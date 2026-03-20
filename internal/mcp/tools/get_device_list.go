package tools

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getDeviceListInput struct{}

type getDeviceListOutput struct {
	Total            int                   `json:"total"`
	AvailableDevices []installDeviceOption `json:"available_devices"`
}

func registerGetDeviceList(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_device_list",
		Description: "Get available devices for app install. Output follows install_app available_devices format.",
	}, handleGetDeviceList)
}

func handleGetDeviceList(_ context.Context, _ *sdkmcp.CallToolRequest, input getDeviceListInput) (*sdkmcp.CallToolResult, getDeviceListOutput, error) {
	_ = input
	_, deviceOptions, _, err := resolveDeviceSelection("")
	if err != nil {
		return nil, getDeviceListOutput{}, err
	}

	return nil, getDeviceListOutput{
		Total:            len(deviceOptions),
		AvailableDevices: deviceOptions,
	}, nil
}
