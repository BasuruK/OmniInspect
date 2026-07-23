package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServeStreamableHTTP blocks running the MCP server over streamable HTTP on
// ln. Used when stdio is already owned by the TUI, so the MCP server runs
// alongside it in the same process instead of over stdin/stdout. Returns
// nil once ctx is cancelled (graceful shutdown) or the listener is closed.
func (s *Server) ServeStreamableHTTP(ctx context.Context, ln net.Listener) error {
	sdk := s.buildAndRegister(time.Now())
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return sdk }, nil)
	httpSrv := &http.Server{Handler: handler}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("MCP server: graceful shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
