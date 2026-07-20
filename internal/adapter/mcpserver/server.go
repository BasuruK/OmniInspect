package mcpserver

import (
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/connector"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Dependencies
// ==========================================

// DBAdapterFactory builds a DatabaseRepository from DatabaseSettings. It is
// injected so tests can substitute fakes without depending on the real
// Oracle/ODPI-C adapter.
type DBAdapterFactory func(*domain.DatabaseSettings) (ports.DatabaseRepository, error)

// Deps groups the shared services a Server needs. Every field except
// DBAdapterFactory is required; NewServer panics when a critical
// dependency is missing. DBAdapterFactory is optional: when nil, calls to
// it from later tool handlers return an error (it must be set by the
// composition root in cmd/).
type Deps struct {
	App              *app.App
	Bolt             ports.ConfigRepository
	TraceAppender    ports.TraceAppender
	Connector        *connector.Connector
	DBSettingsRepo   ports.DatabaseSettingsRepository
	DBAdapterFactory DBAdapterFactory // optional — see field doc
}

// ==========================================
// Server
// ==========================================

// Server wraps the underlying MCP SDK server with our application-specific
// configuration. It is the single composition point for MCP-side tools.
type Server struct {
	deps Deps

	mcpMu sync.Mutex // guards mcp; Serve* may only run once per Server, but the
	// field write and any future concurrent readers must still be race-free.
	mcp *mcp.Server
}

// setMCP records the built SDK server under lock. Serve* calls this once at
// startup; guarding it costs nothing and closes off a data race for any
// future code path that reads s.mcp from another goroutine.
func (s *Server) setMCP(sdk *mcp.Server) {
	s.mcpMu.Lock()
	defer s.mcpMu.Unlock()
	s.mcp = sdk
}

// NewServer builds a Server with an empty tool list. Tools are added by
// later stories (M3.4–M3.6). The MCP SDK server itself is created lazily
// on the first ServeStdio call so dependency wiring can stay in cmd/.
func NewServer(deps Deps) *Server {
	if deps.App == nil {
		panic("mcpserver: App is required")
	}
	if deps.Bolt == nil {
		panic("mcpserver: Bolt is required")
	}
	if deps.TraceAppender == nil {
		panic("mcpserver: TraceAppender is required")
	}
	if deps.Connector == nil {
		panic("mcpserver: Connector is required")
	}
	if deps.DBSettingsRepo == nil {
		panic("mcpserver: DBSettingsRepo is required")
	}
	return &Server{deps: deps}
}

// Deps returns the dependency bag (for tool handlers that need access to
// individual services).
func (s *Server) Deps() Deps { return s.deps }

// BuildDBAdapter is a convenience helper for tool handlers that need a
// live DatabaseRepository. When no factory was injected at construction
// time, it returns an explicit error so the tool can surface a useful
// message rather than panicking.
func (s *Server) BuildDBAdapter(settings *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
	if s == nil || s.deps.DBAdapterFactory == nil {
		return nil, fmt.Errorf("mcpserver: DBAdapterFactory not configured")
	}
	return s.deps.DBAdapterFactory(settings)
}

// buildAndRegister constructs the SDK server and registers every tool that
// has been implemented so far. New tools are added here as later stories
// land (M3.5, M3.6). The startedAt timestamp is captured here so all
// uptime computations share the same origin.
func (s *Server) buildAndRegister(startedAt time.Time) *mcp.Server {
	sdk := buildSDKServer(s.deps.App.Name, s.deps.App.GetVersion())

	// ── get_status ────────────────────────────
	sdk.AddTool(&mcp.Tool{
		Name: "get_status",
		Description: "Returns a JSON snapshot of OmniView runtime state: " +
			"app version, active database id, broadcast mode, current trace " +
			"buffer depth, and uptime in seconds.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, getStatus(s, startedAt))

	// ── set_broadcast_mode ────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "set_broadcast_mode",
		Description: "Switches the broadcast filter mode (global, subscriber, or broadcast) and persists the choice.",
	}, setBroadcastMode(s))

	// ── list_databases ────────────────────────
	sdk.AddTool(&mcp.Tool{
		Name: "list_databases",
		Description: "Returns every persisted database configuration. The active " +
			"database is flagged in the is_active field. Passwords are never " +
			"included in the response.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, listDatabases(s))

	// ── add_database ──────────────────────────
	sdk.AddTool(&mcp.Tool{
		Name: "add_database",
		Description: "Persists a new database configuration. Sending plaintext " +
			"passwords over MCP exposes them in client logs and process " +
			"listings; the first call returns a confirmation request and the " +
			"second call (with confirm_password_in_plaintext=true) writes the " +
			"record.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["id","host","port","service","username","password"],
			"properties":{
				"id":{"type":"string"},
				"host":{"type":"string"},
				"port":{"type":"integer"},
				"service":{"type":"string"},
				"username":{"type":"string"},
				"password":{"type":"string"},
				"confirm_password_in_plaintext":{"type":"boolean"}
			}
		}`),
	}, addDatabase(s))

	// ── connect_database ──────────────────────
	sdk.AddTool(&mcp.Tool{
		Name: "connect_database",
		Description: "Verifies the named database is reachable (5s timeout), marks it active, and " +
			"persists the choice. The verification connection itself is closed immediately after " +
			"the check — no connection is held open by this call.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["id"],
			"properties":{"id":{"type":"string"}}
		}`),
	}, connectDatabase(s))

	// ── list_traces ──────────────────────────
	sdk.AddTool(&mcp.Tool{
		Name: "list_traces",
		Description: "Returns the most recent trace messages from the in-memory " +
			"buffer. Supports since_id, level, and process_name filters applied " +
			"on the snapshot.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"limit":{"type":"integer"},
				"since_id":{"type":"string"},
				"level":{"type":"string"},
				"process_name":{"type":"string"}
			}
		}`),
	}, listTraces(s))

	// ── get_trace ────────────────────────────
	sdk.AddTool(&mcp.Tool{
		Name:        "get_trace",
		Description: "Fetches a single trace message by id. Returns a not_found error code when the id is unknown.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["message_id"],
			"properties":{"message_id":{"type":"string"}}
		}`),
	}, getTrace(s))

	// ── clear_traces ─────────────────────────
	sdk.AddTool(&mcp.Tool{
		Name:        "clear_traces",
		Description: "Empties the trace buffer. There is no undo.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, clearTraces(s))

	return sdk
}
