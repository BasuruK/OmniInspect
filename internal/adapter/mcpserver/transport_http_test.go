package mcpserver

import (
	"context"
	"net"
	"net/http"
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
