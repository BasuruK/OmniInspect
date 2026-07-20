package mcpserver

import (
	"OmniView/internal/adapter/tracebuffer"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/connector"
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
	TraceAppender    *tracebuffer.RingBuffer
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

// buildAndRegister constructs the SDK server and registers every tool that
// has been implemented so far. New tools are added here as later stories
// land (M3.5, M3.6). The startedAt timestamp is captured here so all
// uptime computations share the same origin.
func (s *Server) buildAndRegister(startedAt time.Time) *mcp.Server {
	sdk := buildSDKServer(s.deps.App.Name, s.deps.App.GetVersion())

	// ── get_status ────────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "get_status",
		Description: "Returns a JSON snapshot of OmniView runtime state: " +
			"app version, active database id, broadcast mode, current trace " +
			"buffer depth, and uptime in seconds.",
	}, getStatus(s, startedAt))

	// ── set_broadcast_mode ────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "set_broadcast_mode",
		Description: "Switches the broadcast filter mode (global, subscriber, or broadcast) and persists the choice.",
	}, setBroadcastMode(s))

	// ── list_databases ────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "list_databases",
		Description: "Returns every persisted database configuration. The active " +
			"database is flagged in the is_active field. Passwords are never " +
			"included in the response.",
	}, listDatabases(s))

	// ── add_database ──────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "add_database",
		Description: "Persists a new database configuration. Sending plaintext " +
			"passwords over MCP exposes them in client logs and process " +
			"listings; the first call returns a confirmation request and the " +
			"second call (with confirm_password_in_plaintext=true) writes the " +
			"record.",
	}, addDatabase(s))

	// ── connect_database ──────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "connect_database",
		Description: "Verifies the named database is reachable (5s timeout), marks it active, and " +
			"persists the choice. The verification connection itself is closed immediately after " +
			"the check — no connection is held open by this call.",
	}, connectDatabase(s))

	// ── list_traces ──────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "list_traces",
		Description: "Returns the most recent trace messages from the in-memory " +
			"buffer. Supports since_id, level, and process_name filters applied " +
			"on the snapshot.",
	}, listTraces(s))

	// ── get_trace ────────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "get_trace",
		Description: "Fetches a single trace message by id. Returns a not_found error code when the id is unknown.",
	}, getTrace(s))

	// ── clear_traces ─────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "clear_traces",
		Description: "Empties the trace buffer. There is no undo.",
	}, clearTraces(s))

	return sdk
}
