package mcpserver

import (
	"context"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// buildSDKServer constructs the SDK server with our app identity. Tools are
// registered separately by buildAndRegister.
func buildSDKServer(appName, appVersion string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    appName,
		Version: appVersion,
	}, nil)
}

// ServeWithTransport blocks running the MCP server over the supplied SDK
// transport. Exposed for tests (which use in-memory transports) so the
// handshake test does not need to drive a real stdin/stdout pipe.
func (s *Server) ServeWithTransport(ctx context.Context, t mcp.Transport) error {
	sdk := s.buildAndRegister(time.Now())
	return sdk.Run(ctx, t)
}
