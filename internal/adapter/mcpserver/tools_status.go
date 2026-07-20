package mcpserver

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Result helpers
// ==========================================

// jsonToolResult encodes the value as JSON and wraps it in a single
// TextContent. MCP tool results must carry Content, so this is the
// canonical "return a JSON payload" wrapper for the untyped handler.
func jsonToolResult(v any) *mcp.CallToolResult {
	payload, err := json.Marshal(v)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("encode error: %v", err)}},
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
	}
}

// decodeArguments unmarshals the raw arguments blob into the given typed
// struct. Both the SDK and tests pass raw JSON here.
func decodeArguments(raw json.RawMessage, into any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, into)
}

// ==========================================
// Status Tool
// ==========================================

// statusOutput is the JSON shape returned by the get_status tool.
type statusOutput struct {
	Version            string `json:"version"`
	ActiveDatabaseID   string `json:"active_database_id"`
	BroadcastMode      string `json:"broadcast_mode"`
	TraceBufferDepth   int    `json:"trace_buffer_depth"`
	TraceBufferEvicted int    `json:"trace_buffer_evicted,omitempty"`
	UptimeSeconds      int64  `json:"uptime_seconds"`
}

// traceEvictionCounter is implemented by trace appenders that track how many
// entries have been overwritten by FIFO eviction (e.g. tracebuffer.RingBuffer).
// It's checked via type assertion rather than added to ports.TraceAppender so
// implementations that don't track eviction aren't forced to grow a no-op
// method.
type traceEvictionCounter interface {
	Evicted(ctx context.Context) int
}

// getStatus is the handler for the "get_status" tool. It composes the
// response from app version, Connector, BoltDB broadcast mode, trace buffer
// length, and process uptime. trace_buffer_depth reports the in-memory
// trace ring buffer's current size — NOT the Oracle AQ queue depth, which
// requires an active database connection and is not surfaced by this tool.
func getStatus(s *Server, startedAt time.Time) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mode, err := s.deps.Bolt.GetBroadcastMode()
		if err != nil {
			return mcpToolError("internal_error", fmt.Sprintf("get_status: read broadcast mode: %v", err), nil)
		}
		out := statusOutput{
			Version:          s.deps.App.GetVersion(),
			ActiveDatabaseID: s.deps.Connector.Active(),
			BroadcastMode:    mode.String(),
			TraceBufferDepth: s.deps.TraceAppender.Len(ctx),
			UptimeSeconds:    int64(time.Since(startedAt).Seconds()),
		}
		if ev, ok := s.deps.TraceAppender.(traceEvictionCounter); ok {
			out.TraceBufferEvicted = ev.Evicted(ctx)
		}
		return jsonToolResult(out), nil
	}
}

// ==========================================
// Broadcast Mode Tool
// ==========================================

// setBroadcastModeInput is the JSON input shape for set_broadcast_mode.
type setBroadcastModeInput struct {
	Mode string `json:"mode" jsonschema:"the broadcast mode to activate: global, subscriber, or broadcast"`
}

// setBroadcastModeOutput is the JSON output shape for set_broadcast_mode.
type setBroadcastModeOutput struct {
	OK   bool   `json:"ok"`
	Mode string `json:"mode"`
}

// broadcastModes maps the MCP-facing string identifier to its domain value.
// Unknown inputs are rejected loudly so callers cannot accidentally reset
// their configuration via an unknown alias.
var broadcastModes = map[string]domain.BroadcastMode{
	"global":     domain.BroadcastModeGlobal,
	"subscriber": domain.BroadcastModeSubscriber,
	"broadcast":  domain.BroadcastModeBroadcast,
}

// parseBroadcastMode validates the input string and returns the
// corresponding domain value. domain.NewBroadcastMode silently defaults to
// Global for unknown inputs; the MCP surface must reject unknowns loudly.
func parseBroadcastMode(raw string) (domain.BroadcastMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if mode, ok := broadcastModes[normalized]; ok {
		return mode, nil
	}
	return 0, fmt.Errorf("unknown broadcast mode %q (expected global|subscriber|broadcast)", raw)
}

// setBroadcastMode is the handler for the "set_broadcast_mode" tool. It
// validates the new mode (rejecting unknown values), persists the choice
// through the BoltDB repository, and returns the resolved mode.
func setBroadcastMode(s *Server) mcp.ToolHandlerFor[setBroadcastModeInput, setBroadcastModeOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setBroadcastModeInput) (*mcp.CallToolResult, setBroadcastModeOutput, error) {
		mode, err := parseBroadcastMode(in.Mode)
		if err != nil {
			return nil, setBroadcastModeOutput{}, err
		}
		if err := s.deps.Bolt.SetBroadcastMode(mode); err != nil {
			return nil, setBroadcastModeOutput{}, fmt.Errorf("set_broadcast_mode: persist: %w", err)
		}
		out := setBroadcastModeOutput{OK: true, Mode: mode.String()}
		return jsonToolResult(out), out, nil
	}
}
