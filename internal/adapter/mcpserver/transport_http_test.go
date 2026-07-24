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

	done := make(chan error, 1)
	go func() { done <- srv.ServeStreamableHTTP(serverCtx, ln, testToken) }()

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
	_ = session.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeStreamableHTTP: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeStreamableHTTP did not shut down after context cancellation")
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
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.ServeStreamableHTTP(serverCtx, ln, "right-token") }()

	resp, err := http.Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", resp.StatusCode)
	}
}
