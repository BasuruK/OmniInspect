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
// newHTTPServer wraps handler in the transport's guard stack and timeouts. Tests build their server
// from this too, so a timeout change here cannot drift away from what the tests exercise.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler: requireLoopbackOrigin(limitRequest(handler, maxRequestBodyBytes, requestReadTimeout)),
		// ReadHeaderTimeout and IdleTimeout guard against a slow client (any local process, since this listener is loopback-only) pinning a goroutine indefinitely. Body size and body read time are bounded per request by limitRequest, and again by ReadTimeout: a handler that rejects before reading a body (the origin guard, an SDK-side rejection) never reaches limitRequest, and net/http's post-handler body drain is bounded in bytes only, not in time, so a dripping client would otherwise hold that connection past IdleTimeout. No WriteTimeout: the streamable HTTP transport can hold a response open for server-initiated messages, and that would cut a legitimate long-lived stream short. ReadTimeout is read-side only, so it cannot cut one.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       requestReadTimeout,
		IdleTimeout:       120 * time.Second,
	}
}

func (s *Server) ServeStreamableHTTP(ctx context.Context, ln net.Listener) error {
	sdk := s.buildAndRegister(time.Now())
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return sdk }, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	httpSrv := newHTTPServer(mcpHandler)

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

const (
	// maxRequestBodyBytes caps one client request. Requests are small JSON-RPC envelopes, so this is far above any legitimate call; it exists so a client streaming
	// an endless body cannot grow the process. Responses are unaffected — MaxBytesReader wraps the request body only.
	maxRequestBodyBytes = 4 << 20 // 4 MiB
	// requestReadTimeout bounds how long a client may take to send one request. ReadHeaderTimeout covers headers only, so without this a client that dribbles body
	// bytes holds a connection and its goroutine open indefinitely.
	requestReadTimeout = 30 * time.Second
)

// limitRequest bounds request bodies before the SDK handler sees them: by size, and by how long the body read may take.
//
// The read bound is a per-connection read deadline rather than http.Server.ReadTimeout, so a request whose body is read inside the handler fails on its own deadline
// instead of on the server-wide one, and the deadline can be cleared on the way out. The deadline is cleared so keep-alive reuse works.
func limitRequest(next http.Handler, maxBodyBytes int64, readTimeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Every method can carry a body — a dribbled GET body pins a connection exactly like a POST. But net/http always hands a handler a non-nil Body, even
		// when there is nothing to read, and a bodyless GET is the long-lived SSE stream: deadline that one and every keep-alive connection outliving the timeout
		// is dropped instead of reused. ContentLength != 0 is the real "may carry a body" test (-1 means chunked or unknown length).
		if r.Body == nil || r.ContentLength == 0 {
			next.ServeHTTP(w, r)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		// Best effort: transports without deadline support (h2 in net/http) simply keep the size bound and the server's own read timeouts.
		rc := http.NewResponseController(w)
		_ = rc.SetReadDeadline(time.Now().Add(readTimeout))
		defer func() { _ = rc.SetReadDeadline(time.Time{}) }()

		next.ServeHTTP(w, r)
	})
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
