package mcpserver

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// callGetStatus invokes the get_status tool over a live session and
// returns the decoded statusOutput payload.
func callGetStatus(t *testing.T, deps Deps) statusOutput {
	t.Helper()
	session := connectClientServer(t, deps)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(get_status): %v", err)
	}
	if res.IsError {
		t.Fatalf("get_status returned IsError=true: %+v", res.Content)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	var out statusOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode payload %q: %v", tc.Text, err)
	}
	return out
}

// ==========================================
// get_status
// ==========================================

func TestGetStatus_Defaults(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	out := callGetStatus(t, deps)

	if out.Version == "" {
		t.Fatalf("expected non-empty version, got empty")
	}
	if out.ActiveDatabaseID != "" {
		t.Fatalf("expected empty active_database_id, got %q", out.ActiveDatabaseID)
	}
	if out.BroadcastMode != domain.BroadcastModeGlobal.String() {
		t.Fatalf("expected broadcast_mode=Global, got %q", out.BroadcastMode)
	}
	if out.TraceBufferDepth != 0 {
		t.Fatalf("expected trace_buffer_depth=0, got %d", out.TraceBufferDepth)
	}
	if out.UptimeSeconds < 0 {
		t.Fatalf("expected uptime>=0, got %d", out.UptimeSeconds)
	}
}

func TestGetStatus_ReflectsDefaultDatabaseAndMode(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	settings, err := domain.NewDatabaseSettings("prod-42", "FREEPDB1", "db.example.com", domain.Port(1521), "admin", "secret")
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	if err := deps.DBSettingsRepo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := deps.DBSettingsRepo.SetDefault(context.Background(), *settings); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if err := deps.Bolt.SetBroadcastMode(domain.BroadcastModeSubscriber); err != nil {
		t.Fatalf("Bolt.SetBroadcastMode: %v", err)
	}

	out := callGetStatus(t, deps)
	if out.ActiveDatabaseID != "DBconfig:prod-42" {
		t.Fatalf("expected prod-42, got %q", out.ActiveDatabaseID)
	}
	if out.BroadcastMode != domain.BroadcastModeSubscriber.String() {
		t.Fatalf("expected Subscriber, got %q", out.BroadcastMode)
	}
}

// ==========================================
// set_broadcast_mode
// ==========================================

func TestSetBroadcastMode_NotifiesUI(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	var got domain.BroadcastMode
	deps.OnBroadcastModeChanged = func(mode domain.BroadcastMode) { got = mode }

	session := connectClientServer(t, deps)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "set_broadcast_mode",
		Arguments: map[string]any{"mode": "subscriber"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("set_broadcast_mode IsError: %+v", res.Content)
	}
	if got != domain.BroadcastModeSubscriber {
		t.Fatalf("OnBroadcastModeChanged got %v, want subscriber", got)
	}
}

func TestSetBroadcastMode_PersistsAndReturns(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	session := connectClientServer(t, deps)

	for _, mode := range []struct{ in, canonical string }{
		{"global", "Global"},
		{"subscriber", "Only Subscriber"},
		{"broadcast", "Only Broadcast"},
	} {
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "set_broadcast_mode",
			Arguments: map[string]any{"mode": mode.in},
		})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", mode.in, err)
		}
		if res.IsError {
			t.Fatalf("set_broadcast_mode(%s) IsError=true: %+v", mode.in, res.Content)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content item for %s, got %d", mode.in, len(res.Content))
		}
		tc := res.Content[0].(*mcp.TextContent)

		var out setBroadcastModeOutput
		if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
			t.Fatalf("decode payload %q: %v", tc.Text, err)
		}
		if !out.OK {
			t.Fatalf("expected ok=true for %s, got %+v", mode.in, out)
		}
		if out.Mode != mode.canonical {
			t.Fatalf("expected echoed mode %q, got %q", mode.canonical, out.Mode)
		}

		// Persisted state must reflect the change.
		stored, err := deps.Bolt.GetBroadcastMode()
		if err != nil {
			t.Fatalf("GetBroadcastMode: %v", err)
		}
		if stored.String() != mode.canonical {
			t.Fatalf("expected persisted %s, got %s", mode.canonical, stored.String())
		}
	}
}

func TestSetBroadcastMode_RejectsUnknown(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "set_broadcast_mode",
		Arguments: map[string]any{"mode": "garbage"},
	})
	if err != nil {
		// The SDK translates tool-handler errors into JSON-RPC errors.
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for unknown mode, got %+v", res)
	}
}

// ==========================================
// Integration: status reflects mode set via tool
// ==========================================

func TestStatusAfterSetBroadcastMode(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	session := connectClientServer(t, deps)

	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "set_broadcast_mode",
		Arguments: map[string]any{"mode": "broadcast"},
	}); err != nil {
		t.Fatalf("set_broadcast_mode: %v", err)
	}

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("get_status: %v", err)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var out statusOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.BroadcastMode != domain.BroadcastModeBroadcast.String() {
		t.Fatalf("expected broadcast mode reflected, got %q", out.BroadcastMode)
	}
}
