package mcpserver

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/tracebuffer"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Test Helpers (white-box, shared across _test.go files)
// ==========================================

// testDeps wires a fully-usable Deps with the smallest possible real
// dependencies: a tmp BoltDB, in-memory trace buffer, real repositories.
func testDeps(t *testing.T) (Deps, func()) {
	t.Helper()

	dir := t.TempDir()
	boltPath := filepath.Join(dir, "test.bolt")

	bolt, err := boltdb.NewBoltAdapter(boltPath)
	if err != nil {
		t.Fatalf("NewBoltAdapter: %v", err)
	}
	if err := bolt.Initialize(); err != nil {
		_ = bolt.Close()
		t.Fatalf("bolt.Initialize: %v", err)
	}

	return Deps{
			App:             &app.App{Name: "omniview-test", Version: "test"},
			Bolt:            bolt,
			TraceAppender:   tracebuffer.New(maxListTracesLimit + 1),
			PermissionsRepo: boltdb.NewPermissionsRepository(bolt),
			DBSettingsRepo:  boltdb.NewDatabaseSettingsRepository(bolt),
			DBAdapterFactory: func(*domain.DatabaseSettings) (ports.DatabaseRepository, error) {
				return nil, nil // unused in this test
			},
		}, func() {
			_ = bolt.Close()
		}
}

// connectClientServer brings up a server + client pair over in-memory
// transports and asserts the handshake completes. Returns the client session
// (caller closes it).
func connectClientServer(t *testing.T, deps Deps) *mcp.ClientSession {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	srv := NewServer(deps)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	t.Cleanup(serverCancel)

	// Run the server in a goroutine. Run blocks until the transport closes.
	go func() {
		_ = srv.ServeWithTransport(serverCtx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})
	return session
}
