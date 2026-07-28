package mcpserver

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Shared types
// ==========================================

// emptyInput is the input type for tools that take no arguments.
type emptyInput struct{}

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

// getStatus is the handler for the "get_status" tool. It reports app version, active database, broadcast mode, trace buffer depth, and process uptime.
func getStatus(s *Server, startedAt time.Time) mcp.ToolHandlerFor[emptyInput, statusOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, statusOutput, error) {
		mode, err := s.deps.Bolt.GetBroadcastMode()
		if err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("get_status: read broadcast mode: %v", err), nil)
			return res, statusOutput{}, mErr
		}
		activeDatabaseID := ""
		if def, err := s.deps.DBSettingsRepo.GetDefault(ctx); err != nil {
			if !errors.Is(err, domain.ErrDefaultSettingsNotFound) {
				res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("get_status: read default database: %v", err), nil)
				return res, statusOutput{}, mErr
			}
		} else if def != nil {
			activeDatabaseID = def.StorageKey()
		}
		out := statusOutput{
			Version:            s.deps.App.GetVersion(),
			ActiveDatabaseID:   activeDatabaseID,
			BroadcastMode:      mode.String(),
			TraceBufferDepth:   s.deps.TraceAppender.Len(ctx),
			TraceBufferEvicted: s.deps.TraceAppender.Evicted(ctx),
			UptimeSeconds:      int64(time.Since(startedAt).Seconds()),
		}
		return nil, out, nil
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

// broadcastModes maps the MCP-facing string identifier to its domain value. Unknown inputs are rejected loudly so callers cannot accidentally reset their configuration via an unknown alias.
var broadcastModes = map[string]domain.BroadcastMode{
	"global":     domain.BroadcastModeGlobal,
	"subscriber": domain.BroadcastModeSubscriber,
	"broadcast":  domain.BroadcastModeBroadcast,
}

// parseBroadcastMode validates the input string and returns the corresponding domain value. domain.NewBroadcastMode silently defaults to Global for unknown inputs; the MCP surface must reject unknowns loudly.
func parseBroadcastMode(raw string) (domain.BroadcastMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if mode, ok := broadcastModes[normalized]; ok {
		return mode, nil
	}
	return 0, fmt.Errorf("unknown broadcast mode %q (expected global|subscriber|broadcast)", raw)
}

// setBroadcastMode is the handler for the "set_broadcast_mode" tool. It validates the new mode (rejecting unknown values), persists the choice through the BoltDB repository, and returns the resolved mode.
func setBroadcastMode(s *Server) mcp.ToolHandlerFor[setBroadcastModeInput, setBroadcastModeOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setBroadcastModeInput) (*mcp.CallToolResult, setBroadcastModeOutput, error) {
		mode, err := parseBroadcastMode(in.Mode)
		if err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInvalidInput, fmt.Sprintf("set_broadcast_mode: %v", err), nil)
			return res, setBroadcastModeOutput{}, mErr
		}
		if err := s.deps.Bolt.SetBroadcastMode(mode); err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("set_broadcast_mode: persist: %v", err), nil)
			return res, setBroadcastModeOutput{}, mErr
		}
		out := setBroadcastModeOutput{OK: true, Mode: mode.String()}
		return nil, out, nil
	}
}
