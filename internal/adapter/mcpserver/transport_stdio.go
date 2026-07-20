package mcpserver

import (
	"context"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// buildSDKServer constructs the SDK server with our app identity. The tool
// set is empty for now; M3.4–M3.6 add handlers.
func buildSDKServer(appName, appVersion string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    appName,
		Version: appVersion,
	}, nil)
}

// ServeStdio blocks running the MCP server over stdin/stdout using the SDK's
// stdio transport. The caller is responsible for installing the logger (this
// package's logger writes to omniview.log, never to stdout).
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.ServeWithTransport(ctx, &mcp.StdioTransport{})
}

// ServeWithTransport blocks running the MCP server over the supplied SDK
// transport. Exposed for tests (which use in-memory transports) so the
// handshake test does not need to drive a real stdin/stdout pipe.
func (s *Server) ServeWithTransport(ctx context.Context, t mcp.Transport) error {
	sdk := s.buildAndRegister(time.Now())
	return sdk.Run(ctx, t)
}
