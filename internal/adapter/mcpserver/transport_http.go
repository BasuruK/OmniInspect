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

// ServeStreamableHTTP blocks running the MCP server over streamable HTTP on ln. Used when stdio is already owned by the TUI, so the MCP server runs
// alongside it in the same process instead of over stdin/stdout. Returns nil once ctx is cancelled (graceful shutdown) or the listener is closed.
//
// The composition root binds ln to loopback. Requests are stateless and do not
// require bearer authentication; Origin and SDK Host validation remain enabled.
func (s *Server) ServeStreamableHTTP(ctx context.Context, ln net.Listener) error {
	sdk := s.buildAndRegister(time.Now())
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return sdk }, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	// ReadHeaderTimeout and IdleTimeout guard against a slow client (any local process, since this listener is loopback-only) pinning a goroutine indefinitely. No ReadTimeout/WriteTimeout: the streamable HTTP transport can hold a response open for server-initiated messages, and those would cut a legitimate long-lived stream short.
	httpSrv := &http.Server{
		Handler:           requireLoopbackOrigin(mcpHandler),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			// Shutdown only waits; it never forces connections closed. On timeout, force-close so lingering conns/goroutines don't outlive this call returning.
			_ = httpSrv.Close()
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

// loopbackOrigins allowlists the browser Origins permitted on this transport. A loopback bind stops remote attackers but not a malicious local page
// reaching 127.0.0.1 (browser fetches can attach arbitrary headers), so any Origin outside this list is rejected before anything else runs.
// "null" (opaque origins: sandboxed iframes, file:// pages) is deliberately excluded — it would admit any page context, not just local ones.
var loopbackOrigins = map[string]struct{}{
	"":                       {},
	"http://localhost":       {},
	"http://127.0.0.1":       {},
	"http://localhost.local": {},
	"http://127.0.0.1.local": {},
}

// requireLoopbackOrigin rejects browser requests from foreign or opaque origins.
func requireLoopbackOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := loopbackOrigins[r.Header.Get("Origin")]; !ok {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
