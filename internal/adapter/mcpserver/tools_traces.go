package mcpserver

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/service/subscribers"
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
	SinceCursor string `json:"since_cursor,omitempty" jsonschema:"return only messages after this opaque cursor (taken from a previous list_traces response)"`
	Level       string `json:"level,omitempty" jsonschema:"filter by log level (DEBUG|INFO|WARNING|ERROR|CRITICAL)"`
	ProcessName string `json:"process_name,omitempty" jsonschema:"filter by process name"`
}

// traceDTO is the JSON shape returned for each trace message. Internal domain types are intentionally not exposed — callers see only the fields they can act on.
type traceDTO struct {
	// Cursor is the opaque pagination token for since_cursor. It happens to be the Oracle AQ message id underneath, but that id varies per
	// broadcast mode and means nothing to consumers — it only names this buffer entry, so it must be treated as an uninterpreted string.
	Cursor      string `json:"cursor"`
	ProcessName string `json:"process_name"`
	LogLevel    string `json:"log_level"`
	Payload     string `json:"payload"`
	Timestamp   string `json:"timestamp"`
	Mode        string `json:"mode"`
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

// listTraces returns the most recent messages from the trace buffer, filtered client-side by since_cursor, level, and process_name.
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
				res, mErr := mcpToolError(domain.ErrCodeInvalidInput, fmt.Sprintf("list_traces: %v", err), nil)
				return res, listTracesOutput{}, mErr
			}
			levelFilter = lvl
		}
		processFilter := strings.TrimSpace(in.ProcessName)

		// Mirror the TUI viewport: the active broadcast mode decides which messages are visible. Read it per call so set_broadcast_mode takes
		// effect immediately rather than on some future restart.
		mode, err := s.deps.Bolt.GetBroadcastMode()
		if err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("list_traces: broadcast mode: %v", err), nil)
			return res, listTracesOutput{}, mErr
		}

		// Ask the trace buffer for a generous window so client-side filters have something to work with. We over-fetch by a factor that covers realistic filter selectivity; the post-filter list is still capped to `limit` so the response size stays bounded.
		fetch := limit * 4
		if fetch > maxListTracesLimit {
			fetch = maxListTracesLimit
		}

		messages, err := s.deps.TraceAppender.List(ctx, fetch, in.SinceCursor)
		if err != nil {
			if errors.Is(err, domain.ErrTraceCursorExpired) {
				res, mErr := mcpToolError(domain.ErrCodeCursorExpired, fmt.Sprintf("list_traces: since_cursor %q is unknown or has been evicted", in.SinceCursor), nil)
				return res, listTracesOutput{}, mErr
			}
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("list_traces: %v", err), nil)
			return res, listTracesOutput{}, mErr
		}

		// If the buffer returned exactly as many messages as we asked for, there may be additional matching entries beyond this window that
		// we never examined for level/process_name filtering — the caller cannot otherwise tell "no more matches" from "we only scanned part of the buffer". Surface that explicitly instead of silently under-reporting.
		truncated := len(messages) == fetch

		out := listTracesOutput{Messages: make([]traceDTO, 0, limit), Truncated: truncated}
		for _, msg := range messages {
			if !mode.Includes(msg) {
				continue
			}
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
		Cursor:      msg.MessageID(),
		ProcessName: msg.ProcessName(),
		LogLevel:    string(msg.LogLevel()),
		Payload:     msg.Payload(),
		Timestamp:   msg.Timestamp().Format("2006-01-02T15:04:05.000Z07:00"),
		Mode:        msg.Mode(),
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
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("clear_traces: %v", err), nil)
			return res, clearTracesOutput{}, mErr
		}
		if s.deps.OnTracesCleared != nil {
			s.deps.OnTracesCleared()
		}
		return nil, clearTracesOutput{OK: true}, nil
	}
}

// ==========================================
// get_trace_method
// ==========================================

// getTraceMethodInput is the JSON input shape for get_trace_method.
type getTraceMethodInput struct {
	Message     string `json:"message_" jsonschema:"message text to embed in the generated call"`
	LogLevel    string `json:"log_level_,omitempty" jsonschema:"optional log level (default INFO; DEBUG|INFO|WARNING|ERROR|CRITICAL)"`
	ProcessName string `json:"process_name_,omitempty" jsonschema:"optional process name argument"`
}

// getTraceMethodOutput is the JSON output shape for get_trace_method.
type getTraceMethodOutput struct {
	Call string `json:"call"`
}

// sqlStringLiteral wraps s as an Oracle single-quoted literal, doubling embedded quotes.
func sqlStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// getTraceMethod returns a ready-to-use subscriber-specific Omni_Tracer_API.Trace_Message_<FunnyName> call.
func getTraceMethod(s *Server) mcp.ToolHandlerFor[getTraceMethodInput, getTraceMethodOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getTraceMethodInput) (*mcp.CallToolResult, getTraceMethodOutput, error) {
		message := strings.TrimSpace(in.Message)
		if message == "" {
			res, mErr := mcpToolError(domain.ErrCodeInvalidInput, "get_trace_method: message_ is required", nil)
			return res, getTraceMethodOutput{}, mErr
		}

		level := domain.LogLevelInfo
		if trimmed := strings.TrimSpace(in.LogLevel); trimmed != "" {
			lvl, err := domain.NewLogLevel(trimmed)
			if err != nil {
				res, mErr := mcpToolError(domain.ErrCodeInvalidInput, fmt.Sprintf("get_trace_method: %v", err), nil)
				return res, getTraceMethodOutput{}, mErr
			}
			level = lvl
		}

		sub, err := subscribers.LoadSoleSubscriber(ctx, s.deps.SubscriberRepo)
		if err != nil {
			if errors.Is(err, domain.ErrSubscriberNotFound) {
				res, mErr := mcpToolError(domain.ErrCodeNotFound, "get_trace_method: no subscriber assigned", nil)
				return res, getTraceMethodOutput{}, mErr
			}
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("get_trace_method: %v", err), nil)
			return res, getTraceMethodOutput{}, mErr
		}

		funnyName := strings.TrimSpace(sub.FunnyName())
		if funnyName == "" {
			res, mErr := mcpToolError(domain.ErrCodeNotFound, "get_trace_method: subscriber has no funny name assigned", nil)
			return res, getTraceMethodOutput{}, mErr
		}
		if err := domain.ValidateFunnyNameForSQLInjection(funnyName); err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInvalidInput, fmt.Sprintf("get_trace_method: %v", err), nil)
			return res, getTraceMethodOutput{}, mErr
		}

		call := fmt.Sprintf("Omni_Tracer_API.Trace_Message_%s(%s, %s",
			domain.PascalFunnyName(funnyName), sqlStringLiteral(message), sqlStringLiteral(level.String()))
		if processName := strings.TrimSpace(in.ProcessName); processName != "" {
			call += ", " + sqlStringLiteral(processName)
		}
		call += ")"

		return nil, getTraceMethodOutput{Call: call}, nil
	}
}
