package mcpserver

import (
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Dependencies
// ==========================================

// DBAdapterFactory builds a DatabaseRepository from DatabaseSettings. It is injected so tests can substitute fakes without depending on the real Oracle/ODPI-C adapter.
type DBAdapterFactory func(*domain.DatabaseSettings) (ports.DatabaseRepository, error)

// Deps groups the shared services a Server needs. Every field except DBAdapterFactory is required; NewServer panics if one is missing.
type Deps struct {
	App              *app.App
	Bolt             ports.ConfigRepository
	TraceAppender    ports.TraceAppender
	PermissionsRepo  ports.PermissionsRepository
	DBSettingsRepo   ports.DatabaseSettingsRepository
	DBAdapterFactory DBAdapterFactory // optional — see field doc
}

// ==========================================
// Server
// ==========================================

// Server wraps the underlying MCP SDK server with our application-specific configuration. It is the single composition point for MCP-side tools.
type Server struct {
	deps Deps
}

// NewServer validates deps and returns a Server. The MCP SDK server itself is created lazily on the first ServeStreamableHTTP call so dependency wiring can stay in cmd/.
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
	if deps.PermissionsRepo == nil {
		panic("mcpserver: PermissionsRepo is required")
	}
	if deps.DBSettingsRepo == nil {
		panic("mcpserver: DBSettingsRepo is required")
	}
	return &Server{deps: deps}
}

// boolPtr is a helper for the *bool fields on mcp.ToolAnnotations.
func boolPtr(b bool) *bool { return &b }

// buildAndRegister constructs the SDK server and registers every tool. The startedAt timestamp is captured here so all uptime computations share the same origin.
func (s *Server) buildAndRegister(startedAt time.Time) *mcp.Server {
	sdk := buildSDKServer(s.deps.App.Name, s.deps.App.GetVersion())

	// ── get_status ────────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "get_status",
		Description: "Returns a JSON snapshot of OmniView runtime state: app version, active database id, broadcast mode, current trace buffer depth, and uptime in seconds.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, getStatus(s, startedAt))

	// ── set_broadcast_mode ────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "set_broadcast_mode",
		Description: "Switches the broadcast filter mode (global, subscriber, or broadcast) and persists the choice.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, setBroadcastMode(s))

	// ── list_databases ────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "list_databases",
		Description: "Returns every persisted database configuration. The active database is flagged in the is_active field. Passwords are never included in the response.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, listDatabases(s))

	// ── add_database ──────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "add_database",
		Description: "Persists a new database configuration. Sending plaintext passwords over MCP exposes them in client logs and process " +
			"listings; the first call returns a confirmation request and the second call (with confirm_password_in_plaintext=true) writes the record.",
		// Additive only — a repeat call with the same id fails with already_exists rather than overwriting, so it isn't destructive.
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: false, OpenWorldHint: boolPtr(false)},
	}, addDatabase(s))

	// ── connect_database ──────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name: "connect_database",
		Description: "Verifies the named database is reachable (5s timeout), deploys/checks required permissions and the tracer " +
			"package against it, then sets it as the default database. The verification connection is closed immediately after the checks — no connection is held open by this call.",
		// Talks to an external Oracle instance (open world); repeat calls with the same id converge on the same "connected & default" state.
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(true)},
	}, connectDatabase(s))

	// ── list_traces ──────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "list_traces",
		Description: "Returns the most recent trace messages from the in-memory buffer. Supports since_id, level, and process_name filters applied on the snapshot.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, listTraces(s))

	// ── get_trace ────────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "get_trace",
		Description: "Fetches a single trace message by id. Returns a not_found error code when the id is unknown.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, getTrace(s))

	// ── clear_traces ─────────────────────────
	mcp.AddTool(sdk, &mcp.Tool{
		Name:        "clear_traces",
		Description: "Empties the trace buffer. There is no undo.",
		// No confirmation handshake (unlike add_database's password gate, which
		// exists to stop plaintext-password leakage, not because deletion needs a second step) — DestructiveHint is the SDK-native signal clients use to decide whether to confirm before calling.
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, clearTraces(s))

	return sdk
}
