package mcpserver

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Helpers
// ==========================================

// seedMessages appends the given messages to the trace store so list_traces
// and friends have something to return.
func seedMessages(t *testing.T, store ports.TraceAppender, ids ...string) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	for i, id := range ids {
		ts := base.Add(time.Duration(i) * time.Second)
		msg, err := domain.NewQueueMessage(id, "proc-A", domain.LogLevelInfo, "payload-"+id, ts)
		if err != nil {
			t.Fatalf("NewQueueMessage(%q): %v", id, err)
		}
		if err := store.Append(ctx, msg); err != nil {
			t.Fatalf("Append(%q): %v", id, err)
		}
	}
}

// decodeListTraces calls the tool and decodes the standard envelope.
func decodeListTraces(t *testing.T, session *mcp.ClientSession, args map[string]any) listTracesOutput {
	t.Helper()
	res := callTool(t, session, "list_traces", args)
	if res.IsError {
		t.Fatalf("list_traces returned IsError=true: %+v", res.Content)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var out listTracesOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// ==========================================
// list_traces
// ==========================================

func TestListTraces_EmptyByDefault(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	out := decodeListTraces(t, session, map[string]any{})
	if out.Total != 0 || len(out.Messages) != 0 {
		t.Fatalf("expected empty, got %+v", out)
	}
}

func TestListTraces_DefaultLimit(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	seedMessages(t, deps.TraceAppender, "a", "b", "c")

	session := connectClientServer(t, deps)
	out := decodeListTraces(t, session, map[string]any{})
	if out.Total != 3 {
		t.Fatalf("expected 3 messages, got %d", out.Total)
	}
	// newest first
	if out.Messages[0].Cursor != "c" || out.Messages[2].Cursor != "a" {
		t.Fatalf("expected newest-first ordering, got %+v", out.Messages)
	}
}

func TestListTraces_RespectsLimit(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	seedMessages(t, deps.TraceAppender, "a", "b", "c", "d", "e")

	session := connectClientServer(t, deps)
	out := decodeListTraces(t, session, map[string]any{"limit": 2})
	if out.Total != 2 {
		t.Fatalf("expected 2 messages, got %d", out.Total)
	}
	if out.Messages[0].Cursor != "e" || out.Messages[1].Cursor != "d" {
		t.Fatalf("expected newest 2, got %+v", out.Messages)
	}
}

func TestListTraces_CapsLimit(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	// Seed one more than the configured cap so the cap, not the buffer,
	// is what bounds the response. Use literals so a regression in the
	// constant is caught, not silently re-asserted.
	const seeded = 1001
	const want = 1000
	ids := make([]string, seeded)
	for i := range ids {
		ids[i] = fmt.Sprintf("m-%04d", i)
	}
	seedMessages(t, deps.TraceAppender, ids...)

	session := connectClientServer(t, deps)
	out := decodeListTraces(t, session, map[string]any{"limit": 99999})
	if out.Total != want {
		t.Fatalf("expected cap at %d, got %d", want, out.Total)
	}
	if len(out.Messages) != want {
		t.Fatalf("expected %d messages, got %d", want, len(out.Messages))
	}
}

func TestListTraces_SinceIDFilter(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	seedMessages(t, deps.TraceAppender, "a", "b", "c", "d")

	session := connectClientServer(t, deps)
	out := decodeListTraces(t, session, map[string]any{"since_cursor": "b"})
	if out.Total != 2 {
		t.Fatalf("expected 2 (>b), got %d (%v)", out.Total, out.Messages)
	}
	ids := []string{out.Messages[0].Cursor, out.Messages[1].Cursor}
	if ids[0] != "d" || ids[1] != "c" {
		t.Fatalf("expected [d c], got %v", ids)
	}
}

func TestListTraces_TruncatedWhenFetchWindowExhausted(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	// limit=1 -> fetch window = 4. Seed more matching entries than the
	// fetch window can cover so the handler can't tell whether additional
	// matches exist beyond what it examined.
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = fmt.Sprintf("m-%02d", i)
	}
	seedMessages(t, deps.TraceAppender, ids...)

	session := connectClientServer(t, deps)
	out := decodeListTraces(t, session, map[string]any{"limit": 1})
	if !out.Truncated {
		t.Fatalf("expected truncated=true when fetch window is exhausted, got %+v", out)
	}
}

func TestListTraces_NotTruncatedWhenBufferExhausted(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	seedMessages(t, deps.TraceAppender, "a", "b", "c")

	session := connectClientServer(t, deps)
	out := decodeListTraces(t, session, map[string]any{"limit": 50})
	if out.Truncated {
		t.Fatalf("expected truncated=false when the whole buffer was scanned, got %+v", out)
	}
}

// ==========================================
// list_traces broadcast-mode filtering
// ==========================================

// modeSeed pairs a message id with the wire mode it should carry.
type modeSeed struct{ id, mode string }

// seedMessagesWithMode appends messages carrying explicit modes, built through the Oracle JSON wire shape because QueueMessage.mode has no exported setter.
func seedMessagesWithMode(t *testing.T, store ports.TraceAppender, seeds ...modeSeed) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	for i, s := range seeds {
		raw := fmt.Sprintf(`{"message_id":%q,"process_name":"proc-A","log_level":"INFO","payload":"payload-%s","timestamp":%d,"send_to_webhook":"false","mode":%q}`,
			s.id, s.id, base.Add(time.Duration(i)*time.Second).Unix(), s.mode)
		var msg domain.QueueMessage
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			t.Fatalf("unmarshal %q: %v", s.id, err)
		}
		if err := store.Append(ctx, &msg); err != nil {
			t.Fatalf("Append(%q): %v", s.id, err)
		}
	}
}

func TestListTraces_FiltersByActiveBroadcastMode(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	// "Global" on the wire = broadcast-to-all message; "Subscriber" = subscriber-targeted.
	seedMessagesWithMode(t, deps.TraceAppender,
		modeSeed{"bcast-1", "Global"},
		modeSeed{"sub-1", "Subscriber"},
	)

	session := connectClientServer(t, deps)

	if err := deps.Bolt.SetBroadcastMode(domain.BroadcastModeSubscriber); err != nil {
		t.Fatalf("SetBroadcastMode: %v", err)
	}
	out := decodeListTraces(t, session, map[string]any{})
	if out.Total != 1 || out.Messages[0].Cursor != "sub-1" {
		t.Fatalf("subscriber mode: expected only sub-1, got %+v", out.Messages)
	}

	// Switching modes must be reflected on the very next call.
	if err := deps.Bolt.SetBroadcastMode(domain.BroadcastModeBroadcast); err != nil {
		t.Fatalf("SetBroadcastMode: %v", err)
	}
	out = decodeListTraces(t, session, map[string]any{})
	if out.Total != 1 || out.Messages[0].Cursor != "bcast-1" {
		t.Fatalf("broadcast mode: expected only bcast-1, got %+v", out.Messages)
	}
}

// ==========================================
// clear_traces
// ==========================================

func TestClearTraces_EmptiesBuffer(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	seedMessages(t, deps.TraceAppender, "a", "b", "c")
	if got := deps.TraceAppender.Len(context.Background()); got != 3 {
		t.Fatalf("setup: expected len=3, got %d", got)
	}

	session := connectClientServer(t, deps)
	res := callTool(t, session, "clear_traces", map[string]any{})
	if res.IsError {
		t.Fatalf("expected success, got IsError")
	}
	tc := res.Content[0].(*mcp.TextContent)
	var out clearTracesOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true")
	}
	if got := deps.TraceAppender.Len(context.Background()); got != 0 {
		t.Fatalf("expected len=0 after clear, got %d", got)
	}
}
