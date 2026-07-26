package mcpserver

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"fmt"
	"strings"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// list_traces
// ==========================================

// listTracesInput is the JSON input shape for list_traces.
type listTracesInput struct {
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of messages to return (default 50, capped at 1000)"`
	SinceID     string `json:"since_id,omitempty" jsonschema:"return only messages whose ID is strictly greater than this"`
	Level       string `json:"level,omitempty" jsonschema:"filter by log level (DEBUG|INFO|WARNING|ERROR|CRITICAL)"`
	ProcessName string `json:"process_name,omitempty" jsonschema:"filter by process name"`
}

// traceDTO is the JSON shape returned for each trace message. Internal domain types are intentionally not exposed — callers see only the fields they can act on.
type traceDTO struct {
	MessageID     string `json:"message_id"`
	ProcessName   string `json:"process_name"`
	LogLevel      string `json:"log_level"`
	Payload       string `json:"payload"`
	Timestamp     string `json:"timestamp"`
	Mode          string `json:"mode"`
	SendToWebhook bool   `json:"send_to_webhook"`
}

// listTracesOutput is the JSON output shape for list_traces.
type listTracesOutput struct {
	Messages  []traceDTO `json:"messages"`
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated"`
}

const (
	defaultListTracesLimit = 50
	maxListTracesLimit     = 1000
)

// listTraces returns the most recent messages from the trace buffer, filtered
// client-side by since_id, level, and process_name.
func listTraces(s *Server) mcp.ToolHandlerFor[listTracesInput, listTracesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listTracesInput) (*mcp.CallToolResult, listTracesOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = defaultListTracesLimit
		}
		if limit > maxListTracesLimit {
			limit = maxListTracesLimit
		}

		var levelFilter domain.LogLevel
		if trimmed := strings.TrimSpace(in.Level); trimmed != "" {
			lvl, err := domain.NewLogLevel(trimmed)
			if err != nil {
				res, mErr := mcpToolError(errCodeInvalidInput, fmt.Sprintf("list_traces: %v", err), nil)
				return res, listTracesOutput{}, mErr
			}
			levelFilter = lvl
		}
		processFilter := strings.TrimSpace(in.ProcessName)

		// Ask the trace buffer for a generous window so client-side filters have something to work with. We over-fetch by a factor that covers
		// realistic filter selectivity; the post-filter list is still capped to `limit` so the response size stays bounded.
		fetch := limit * 4
		if fetch > maxListTracesLimit {
			fetch = maxListTracesLimit
		}

		messages, err := s.deps.TraceAppender.List(ctx, fetch, in.SinceID)
		if err != nil {
			if errors.Is(err, domain.ErrTraceCursorExpired) {
				res, mErr := mcpToolError(errCodeCursorExpired, fmt.Sprintf("list_traces: since_id %q is unknown or has been evicted", in.SinceID), nil)
				return res, listTracesOutput{}, mErr
			}
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("list_traces: %v", err), nil)
			return res, listTracesOutput{}, mErr
		}

		// If the buffer returned exactly as many messages as we asked for, there may be additional matching entries beyond this window that
		// we never examined for level/process_name filtering — the caller cannot otherwise tell "no more matches" from "we only scanned part of the buffer". Surface that explicitly instead of silently under-reporting.
		truncated := len(messages) == fetch

		out := listTracesOutput{Messages: make([]traceDTO, 0, limit), Truncated: truncated}
		for _, msg := range messages {
			if levelFilter != "" && msg.LogLevel() != levelFilter {
				continue
			}
			if processFilter != "" && msg.ProcessName() != processFilter {
				continue
			}
			out.Messages = append(out.Messages, toTraceDTO(msg))
			if len(out.Messages) >= limit {
				break
			}
		}
		out.Total = len(out.Messages)
		return nil, out, nil
	}
}

// toTraceDTO converts a domain QueueMessage into the wire shape. Internal domain types are intentionally not exposed — callers see only the fields they can act on.
func toTraceDTO(msg *domain.QueueMessage) traceDTO {
	return traceDTO{
		MessageID:     msg.MessageID(),
		ProcessName:   msg.ProcessName(),
		LogLevel:      string(msg.LogLevel()),
		Payload:       msg.Payload(),
		Timestamp:     msg.Timestamp().Format("2006-01-02T15:04:05.000Z07:00"),
		Mode:          msg.Mode(),
		SendToWebhook: msg.SendToWebhook(),
	}
}

// ==========================================
// get_trace
// ==========================================

// getTraceInput is the JSON input shape for get_trace.
type getTraceInput struct {
	MessageID string `json:"message_id,omitempty" jsonschema:"the unique id of the trace message to fetch"`
}

// getTrace is the handler for the "get_trace" tool. Returns the message or
// a not_found error code when the id is unknown.
func getTrace(s *Server) mcp.ToolHandlerFor[getTraceInput, traceDTO] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getTraceInput) (*mcp.CallToolResult, traceDTO, error) {
		if in.MessageID == "" {
			res, mErr := mcpToolError(errCodeInvalidInput, "get_trace: message_id is required", nil)
			return res, traceDTO{}, mErr
		}

		msg, err := s.deps.TraceAppender.GetByID(ctx, in.MessageID)
		if err != nil {
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("get_trace: %v", err), nil)
			return res, traceDTO{}, mErr
		}
		if msg == nil {
			res, mErr := mcpToolError(errCodeNotFound, fmt.Sprintf("trace %q not found", in.MessageID), nil)
			return res, traceDTO{}, mErr
		}
		return nil, toTraceDTO(msg), nil
	}
}

// ==========================================
// clear_traces
// ==========================================

// clearTracesOutput is the JSON output shape for clear_traces.
type clearTracesOutput struct {
	OK bool `json:"ok"`
}

// clearTraces empties the trace buffer. There is no undo — callers should confirm before invoking.
func clearTraces(s *Server) mcp.ToolHandlerFor[emptyInput, clearTracesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, clearTracesOutput, error) {
		if err := s.deps.TraceAppender.Clear(ctx); err != nil {
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("clear_traces: %v", err), nil)
			return res, clearTracesOutput{}, mErr
		}
		return nil, clearTracesOutput{OK: true}, nil
	}
}
