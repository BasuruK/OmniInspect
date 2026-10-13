package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// startServe runs ServeStreamableHTTP in a goroutine and registers cleanup
// that cancels ctx and joins the goroutine within a bounded timeout, so the
// server never outlives the test.
func startServe(t *testing.T, srv *Server, ctx context.Context, cancel context.CancelFunc, ln net.Listener) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- srv.ServeStreamableHTTP(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("ServeStreamableHTTP: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("ServeStreamableHTTP did not shut down after context cancellation")
		}
	})
	return done
}

func TestServeStreamableHTTP_HandshakeAndShutdown(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	srv := NewServer(deps)
	serverCtx, cancel := context.WithCancel(context.Background())

	startServe(t, srv, serverCtx, cancel, ln)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()

	session, err := client.Connect(dialCtx, &mcp.StreamableClientTransport{
		Endpoint: "http://" + ln.Addr().String(),
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	if session.InitializeResult() == nil {
		t.Fatal("expected InitializeResult, got nil")
	}

	if err := session.Close(); err != nil {
		t.Fatalf("session.Close: %v", err)
	}
}

func TestServeStreamableHTTP_RejectsForeignOrigin(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	srv := NewServer(deps)
	serverCtx, cancel := context.WithCancel(context.Background())

	startServe(t, srv, serverCtx, cancel, ln)

	req, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String(), nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign origin, got %d", resp.StatusCode)
	}
}

func TestLimitRequest_BoundsBodySize(t *testing.T) {
	const maxBody = 64
	// Stands in for the SDK handler: drain the body, 413 if the cap tripped.
	drain := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(limitRequest(drain, maxBody, time.Minute))
	defer srv.Close()

	// GET carries a body too, so the size cap must not be POST-only.
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			send := func(body string) int {
				t.Helper()
				req, err := http.NewRequest(method, srv.URL, strings.NewReader(body))
				if err != nil {
					t.Fatalf("http.NewRequest: %v", err)
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("http.Do: %v", err)
				}
				defer resp.Body.Close()
				return resp.StatusCode
			}

			if code := send(strings.Repeat("a", maxBody)); code != http.StatusOK {
				t.Fatalf("expected a body at the limit to be accepted, got %d", code)
			}
			if code := send(strings.Repeat("a", maxBody+1)); code != http.StatusRequestEntityTooLarge {
				t.Fatalf("expected a body over the limit to be rejected, got %d", code)
			}
		})
	}
}

func TestLimitRequest_BoundsBodyReadTime(t *testing.T) {
	const (
		readTimeout = 100 * time.Millisecond
		bodyLen     = 64
	)

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			readErr := make(chan error, 1)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.Copy(io.Discard, r.Body)
				readErr <- err
			})
			srv := httptest.NewServer(limitRequest(next, 1<<20, readTimeout))
			defer srv.Close()

			conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
			if err != nil {
				t.Fatalf("net.Dial: %v", err)
			}
			defer conn.Close()

			fmt.Fprintf(conn, "%s / HTTP/1.1\r\nHost: %s\r\nContent-Length: %d\r\n\r\n", method, conn.RemoteAddr().String(), bodyLen)
			// Send a few bytes, then stall. With no read deadline the body read cannot finish until the
			// remaining bytes arrive, so a returning read here proves the deadline cut it short.
			if _, err := conn.Write([]byte("1234")); err != nil {
				t.Fatalf("write partial body: %v", err)
			}

			select {
			case err := <-readErr:
				if err == nil {
					t.Fatal("expected a read deadline error, got a complete body read")
				}
			case <-time.After(20 * readTimeout):
				t.Fatal("body read never returned while the client stalled")
			}
		})
	}
}

// startGuardedServer serves handler behind the real guard stack and timeouts, with ReadTimeout
// shrunk so the tests stay fast.
func startGuardedServer(t *testing.T, handler http.Handler, readTimeout time.Duration) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	srv := newHTTPServer(handler)
	srv.ReadTimeout = readTimeout
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// TestReadTimeout_BoundsBodyTheHandlerNeverReads covers the gap limitRequest cannot: the origin guard
// rejects before it runs, so nothing sets a per-request deadline. net/http then drains the unread body
// after the response, bounded in bytes only, so without http.Server.ReadTimeout a dripping client pins
// the connection indefinitely — IdleTimeout does not cover it, since the handler is not idle.
func TestReadTimeout_BoundsBodyTheHandlerNeverReads(t *testing.T) {
	const readTimeout = 500 * time.Millisecond

	addr := startGuardedServer(t, http.NotFoundHandler(), readTimeout)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()

	// Foreign Origin: rejected by requireLoopbackOrigin, which never reaches limitRequest.
	fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: %s\r\nOrigin: https://evil.example\r\nContent-Length: 4096\r\n\r\n", addr)
	if _, err := conn.Write([]byte("x")); err != nil {
		t.Fatalf("write partial body: %v", err)
	}

	// The server owes us a complete exchange and a closed connection well inside a bound it sets itself.
	if err := conn.SetReadDeadline(time.Now().Add(20 * readTimeout)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var got bytes.Buffer
	if _, err := io.Copy(&got, conn); err != nil {
		t.Fatalf("read exchange: %v (server never closed the connection after an early rejection)", err)
	}
	if !strings.Contains(got.String(), "403") {
		t.Fatalf("expected a 403 in the exchange, got %q", got.String())
	}
}

// TestReadTimeout_LeavesLongLivedResponsesAlone is the safety rail on http.Server.ReadTimeout: it is
// read-side only, so an SSE-style response that outlives it must still arrive complete.
func TestReadTimeout_LeavesLongLivedResponsesAlone(t *testing.T) {
	const (
		readTimeout = 200 * time.Millisecond
		chunks      = 10
		chunkDelay  = 50 * time.Millisecond
	)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := range chunks {
			fmt.Fprintf(w, "data: %d\n\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(chunkDelay)
		}
	})
	addr := startGuardedServer(t, handler, readTimeout)

	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if n := strings.Count(string(got), "data: "); n != chunks {
		t.Fatalf("expected all %d stream chunks to arrive, got %d", chunks, n)
	}
}
