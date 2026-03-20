package tools

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getAccountListInput struct{}

type getAccountListOutput struct {
	Total             int                    `json:"total"`
	AvailableAccounts []installAccountOption `json:"available_accounts"`
}

func registerGetAccountList(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_account_list",
		Description: "Get available Apple accounts for app install. Output follows install_app available_accounts format.",
	}, handleGetAccountList)
}

func handleGetAccountList(_ context.Context, _ *sdkmcp.CallToolRequest, input getAccountListInput) (*sdkmcp.CallToolResult, getAccountListOutput, error) {
	_ = input
	_, accountOptions, _, err := resolveAccountSelection("")
	if err != nil {
		return nil, getAccountListOutput{}, err
	}

	return nil, getAccountListOutput{
		Total:             len(accountOptions),
		AvailableAccounts: accountOptions,
	}, nil
}
