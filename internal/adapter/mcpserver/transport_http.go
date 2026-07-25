package mcpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServeStreamableHTTP blocks running the MCP server over streamable HTTP on
// ln. Used when stdio is already owned by the TUI, so the MCP server runs
// alongside it in the same process instead of over stdin/stdout. Returns
// nil once ctx is cancelled (graceful shutdown) or the listener is closed.
//
// token must accompany every request as an HTTP Authorization header, scheme Bearer.
// Loopback binding stops remote attackers but not other local accounts on a
// shared/RDP machine, so callers must supply a per-process random token
// (see cmd/omniview/mcp.go) instead of an empty one.
func (s *Server) ServeStreamableHTTP(ctx context.Context, ln net.Listener, token string) error {
	sdk := s.buildAndRegister(time.Now())
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return sdk }, nil)
	// ReadHeaderTimeout and IdleTimeout guard against a slow client (any
	// local process, since this listener is loopback-only) pinning a
	// goroutine indefinitely. No ReadTimeout/WriteTimeout: the streamable
	// HTTP transport can hold a response open for server-initiated
	// messages, and those would cut a legitimate long-lived stream short.
	httpSrv := &http.Server{
		Handler:           requireBearerToken(token, mcpHandler),
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
			// Shutdown only waits; it never forces connections closed. On
			// timeout, force-close so lingering conns/goroutines don't
			// outlive this call returning.
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

const bearerPrefix = "Bearer "

// requireBearerToken rejects any request whose Authorization header doesn't
// match token via constant-time comparison, closing the gap where another
// local account on the same machine could otherwise reach this loopback port.
func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		got := auth
		if len(auth) >= len(bearerPrefix) && strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
			got = auth[len(bearerPrefix):]
		}
		if len(token) == 0 || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
