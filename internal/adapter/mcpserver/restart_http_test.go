package mcpserver

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// startHTTPServer uses a real loopback listener so restart tests cover the wire protocol.
func startHTTPServer(t *testing.T, deps Deps, addr string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- NewServer(deps).ServeStreamableHTTP(ctx, ln) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("ServeStreamableHTTP: %v", err)
				}
			case <-time.After(6 * time.Second):
				t.Error("HTTP server shutdown timed out")
			}
		})
	}
	t.Cleanup(stop)
	return "http://" + ln.Addr().String() + "/mcp", stop
}

func TestServeStreamableHTTP_ClientContinuesAfterRestartWithoutToken(t *testing.T) {
	deps, cleanup := testDeps(t)
	t.Cleanup(cleanup)
	endpoint, stop := startHTTPServer(t, deps, "127.0.0.1:0")
	client := mcp.NewClient(&mcp.Implementation{Name: "restart-client", Version: "test"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("connect without bearer: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if res := callTool(t, session, "get_status", map[string]any{}); res.IsError {
		t.Fatal("get_status before restart failed")
	}
	stop()
	startHTTPServer(t, deps, strings.TrimSuffix(strings.TrimPrefix(endpoint, "http://"), "/mcp"))
	if res := callTool(t, session, "get_status", map[string]any{}); res.IsError {
		t.Fatal("same IDE session failed after restart")
	}
}

func TestServeStreamableHTTP_IgnoresStaleSessionAndAuthorization(t *testing.T) {
	deps, cleanup := testDeps(t)
	t.Cleanup(cleanup)
	endpoint, _ := startHTTPServer(t, deps, "127.0.0.1:0")
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_status","arguments":{}}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", "session-from-before-restart")
	req.Header.Set("Authorization", "Bearer obsolete-ide-config")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("old IDE headers: HTTP %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Mcp-Session-Id"); got != "" {
		t.Fatal("stateless HTTP must not issue a session ID")
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type=%q, want application/json", got)
	}
}

func TestServeStreamableHTTP_RejectsForeignHost(t *testing.T) {
	deps, cleanup := testDeps(t)
	t.Cleanup(cleanup)
	endpoint, _ := startHTTPServer(t, deps, "127.0.0.1:0")
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("HTTP %d, want 403", resp.StatusCode)
	}
}
