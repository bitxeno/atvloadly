package mcp

import (
	"net/http"
	"sync"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/mcp/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	handlerOnce sync.Once
	handler     http.Handler
)

// NewHTTPHandler creates a singleton MCP streamable HTTP handler.
func NewHTTPHandler() http.Handler {
	handlerOnce.Do(func() {
		server := sdkmcp.NewServer(&sdkmcp.Implementation{
			Name:    "atvloadly-mcp",
			Version: app.Version.Version,
		}, nil)
		tools.Register(server)

		handler = sdkmcp.NewStreamableHTTPHandler(func(_ *http.Request) *sdkmcp.Server {
			return server
		}, nil)

		log.Infof("MCP endpoint enabled on /mcp")
	})

	return handler
}
