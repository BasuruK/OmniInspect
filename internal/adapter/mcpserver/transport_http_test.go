package mcpserver

import (
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

// bodyEchoHandler stands in for the SDK handler: it drains the body and reports how much it got.
func bodyEchoHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestLimitRequest_BoundsBodySize(t *testing.T) {
	const maxBody = 64
	srv := httptest.NewServer(limitRequest(bodyEchoHandler(t), maxBody, time.Minute))
	defer srv.Close()

	post := func(body string) int {
		t.Helper()
		resp, err := http.Post(srv.URL, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("http.Post: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := post(strings.Repeat("a", maxBody)); code != http.StatusOK {
		t.Fatalf("expected a body at the limit to be accepted, got %d", code)
	}
	if code := post(strings.Repeat("a", maxBody+1)); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected a body over the limit to be rejected, got %d", code)
	}
}

func TestLimitRequest_BoundsBodyReadTime(t *testing.T) {
	const (
		readTimeout = 100 * time.Millisecond
		bodyLen     = 64
	)
	readErr := make(chan error, 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		readErr <- err
	})
	srv := httptest.NewServer(limitRequest(next, 1<<20, readTimeout))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: %s\r\nContent-Length: %d\r\n\r\n", addr, bodyLen)
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

	// Finish the body afterwards: it must not unblock a read that already hit the deadline.
	if _, err := conn.Write([]byte(strings.Repeat("a", bodyLen-4))); err != nil {
		t.Fatalf("write rest of body: %v", err)
	}
}
