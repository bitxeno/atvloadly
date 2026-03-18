package tools

import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

// Register registers all MCP tools used by atvloadly.
func Register(server *sdkmcp.Server) {
	if server == nil {
		return
	}

	registerSearchApp(server)
	registerRefreshApp(server)
	registerGetRefreshStatus(server)
	registerInstallApp(server)
	registerGetInstallStatus(server)
}
