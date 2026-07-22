package mcpserver

import (
	"context"
	"net"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServeStreamableHTTP_HandshakeAndShutdown(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	srv := NewServer(deps)
	serverCtx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- srv.ServeStreamableHTTP(serverCtx, ln) }()

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
