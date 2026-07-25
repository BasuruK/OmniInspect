package mcpserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// headerRoundTripper injects a static header on every outgoing request, standing in for
// an MCP client configured with the server's bearer token.
type headerRoundTripper struct {
	header string
	value  string
}

func (h headerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(h.header, h.value)
	return http.DefaultTransport.RoundTrip(r)
}

// startServe runs ServeStreamableHTTP in a goroutine and registers cleanup
// that cancels ctx and joins the goroutine within a bounded timeout, so the
// server never outlives the test.
func startServe(t *testing.T, srv *Server, ctx context.Context, cancel context.CancelFunc, ln net.Listener, token string) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- srv.ServeStreamableHTTP(ctx, ln, token) }()
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

	const testToken = "test-secret"

	startServe(t, srv, serverCtx, cancel, ln, testToken)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()

	session, err := client.Connect(dialCtx, &mcp.StreamableClientTransport{
		Endpoint:   "http://" + ln.Addr().String(),
		HTTPClient: &http.Client{Transport: headerRoundTripper{header: "Authorization", value: "Bearer " + testToken}},
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

func TestServeStreamableHTTP_RejectsWrongToken(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	srv := NewServer(deps)
	serverCtx, cancel := context.WithCancel(context.Background())

	startServe(t, srv, serverCtx, cancel, ln, "right-token")

	req, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String(), nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", resp.StatusCode)
	}
}
