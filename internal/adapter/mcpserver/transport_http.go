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

// loopbackOrigins is the explicit allowlist of browser Origins permitted to
// call this loopback-only MCP transport. A loopback bind stops remote
// attackers but does nothing against a malicious page in a local browser
// reaching 127.0.0.1 with the user's stored bearer token (it can attach
// arbitrary headers to same-origin fetch-like requests), so we close that
// gap by rejecting any Origin outside this list before doing anything else.
var loopbackOrigins = map[string]struct{}{
	"":                       {},
	"null":                   {}, // sandboxed iframes / opaque-origin fetches
	"http://localhost":       {},
	"http://127.0.0.1":       {},
	"http://localhost.local": {},
	"http://127.0.0.1.local": {},
}

// requireBearerToken rejects any request whose Origin is outside the loopback
// allowlist, then checks the Authorization header uses the Bearer scheme and
// that the extracted token matches the configured token via constant-time
// comparison. Together these close the gaps where another local account, or
// a malicious local browser page, could otherwise reach this loopback port.
func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := loopbackOrigins[r.Header.Get("Origin")]; !ok {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		auth := r.Header.Get("Authorization")
		if len(auth) < len(bearerPrefix) || !strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := auth[len(bearerPrefix):]
		if len(token) == 0 || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
