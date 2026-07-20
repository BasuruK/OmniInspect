package mcpserver

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// list_databases
// ==========================================

// databaseListEntry is one row of the list_databases response. Password is
// never included; only metadata safe for an LLM client to read.
type databaseListEntry struct {
	StorageKey           string `json:"storage_key"`
	DatabaseID           string `json:"database_id"`
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	Service              string `json:"service"`
	Username             string `json:"username"`
	IsActive             bool   `json:"is_active"`
	PermissionsValidated bool   `json:"permissions_validated"`
}

// databaseListOutput is the response payload of list_databases.
type databaseListOutput struct {
	Databases []databaseListEntry `json:"databases"`
	Total     int                 `json:"total"`
}

// listDatabases returns every persisted database configuration, flagging
// the one currently marked active by the Connector.
func listDatabases(s *Server) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		all, err := s.deps.DBSettingsRepo.GetAll(ctx)
		if err != nil {
			return mcpToolError("internal_error", fmt.Sprintf("list_databases: %v", err), nil)
		}
		active := s.deps.Connector.Active()
		out := databaseListOutput{Databases: make([]databaseListEntry, 0, len(all))}
		for _, d := range all {
			out.Databases = append(out.Databases, databaseListEntry{
				StorageKey:           d.StorageKey(),
				DatabaseID:           d.DatabaseID(),
				Host:                 d.Host(),
				Port:                 int(d.Port()),
				Service:              d.Database(),
				Username:             d.Username(),
				IsActive:             d.StorageKey() == active,
				PermissionsValidated: d.PermissionsValidated(),
			})
		}
		out.Total = len(out.Databases)
		return jsonToolResult(out), nil
	}
}

// ==========================================
// add_database
// ==========================================

// addDatabaseInput is the JSON input shape for add_database.
type addDatabaseInput struct {
	ID                         string `json:"id" jsonschema:"user-facing identifier for this database"`
	Host                       string `json:"host"`
	Port                       int    `json:"port"`
	Service                    string `json:"service"`
	Username                   string `json:"username"`
	Password                   string `json:"password"`
	ConfirmPasswordInPlaintext bool   `json:"confirm_password_in_plaintext,omitempty"`
}

// addDatabaseOutput is the JSON output shape for add_database.
type addDatabaseOutput struct {
	OK bool   `json:"ok"`
	ID string `json:"id"`
}

// addDatabaseConfirmRequired is returned when the caller must resend with
// confirm_password_in_plaintext=true.
func addDatabaseConfirmRequired() (*mcp.CallToolResult, error) {
	return mcpToolError("password_in_plaintext",
		"Sending passwords over MCP exposes them in client logs and process listings. "+
			"Confirm to proceed, or use the TUI onboarding form.",
		map[string]any{"confirm_required": true})
}

// mcpToolError serializes a structured error response. We piggyback on
// CallToolResult.IsError + TextContent because MCP does not have a
// first-class error payload in tool results.
//
// Stable codes used across this package: "invalid_input" (bad/missing
// arguments), "not_found" (referenced entity does not exist),
// "already_exists" (create would collide with an existing entity),
// "internal_error" (storage/backend failure not caused by caller input),
// "db_unreachable" (connect attempt failed), "password_in_plaintext"
// (add_database confirmation gate). "no_active_db" and "db_locked" are
// reserved by the architecture spec for tools that gate on connection
// state; no current v1 tool has that precondition.
func mcpToolError(code, message string, extra map[string]any) (*mcp.CallToolResult, error) {
	payload := map[string]any{
		"code":    code,
		"message": message,
	}
	for k, v := range extra {
		payload[k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}, nil
}

// addDatabase validates, then either asks for confirmation or persists.
func addDatabase(s *Server) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in addDatabaseInput
		if err := decodeArguments(req.Params.Arguments, &in); err != nil {
			return mcpToolError("invalid_input", fmt.Sprintf("add_database: decode arguments: %v", err), nil)
		}

		// First, validate what we can without touching the password.
		if in.ID == "" || in.Host == "" || in.Service == "" || in.Username == "" || in.Password == "" {
			return mcpToolError("invalid_input",
				"add_database: id, host, service, username, and password are required", nil)
		}
		if in.Port <= 0 || in.Port > 65535 {
			return mcpToolError("invalid_input", "add_database: port must be between 1 and 65535", nil)
		}

		// Reject silent overwrite of an existing ID. GetByID returns a
		// non-nil error for "not found" as well as genuine backend failures
		// (see its doc comment); we can't distinguish those cases from the
		// error alone, so we treat "no error, settings found" as the only
		// definitive already-exists signal and let any error fall through to
		// the create path, matching this repository's existing convention
		// (see connect_database).
		if existing, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID); err == nil && existing != nil {
			return mcpToolError("already_exists",
				fmt.Sprintf("add_database: a database with id %q already exists; use a different id", in.ID), nil)
		}

		if !in.ConfirmPasswordInPlaintext {
			return addDatabaseConfirmRequired()
		}

		settings, err := domain.NewDatabaseSettings(
			in.ID, in.Service, in.Host,
			domain.Port(in.Port), in.Username, in.Password,
		)
		if err != nil {
			return mcpToolError("invalid_input", fmt.Sprintf("add_database: validate: %v", err), nil)
		}

		if err := s.deps.DBSettingsRepo.Save(ctx, *settings); err != nil {
			return mcpToolError("internal_error", fmt.Sprintf("add_database: persist: %v", err), nil)
		}
		return jsonToolResult(addDatabaseOutput{OK: true, ID: settings.StorageKey()}), nil
	}
}

// ==========================================
// connect_database
// ==========================================

// connectDatabaseInput is the JSON input shape for connect_database.
type connectDatabaseInput struct {
	ID string `json:"id" jsonschema:"storage_key of the database to connect to"`
}

// connectDatabaseOutput is the JSON output shape for connect_database.
type connectDatabaseOutput struct {
	OK             bool   `json:"ok"`
	ActiveDatabase string `json:"active_database"`
}

// connectDatabase validates the request, builds the adapter via the
// injected factory, and on success marks the database active via Connector.
// It deliberately does NOT unregister the previously active database — the
// spec calls for that only when the new connect succeeds.
func connectDatabase(s *Server) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in connectDatabaseInput
		if err := decodeArguments(req.Params.Arguments, &in); err != nil {
			return mcpToolError("invalid_input", fmt.Sprintf("connect_database: decode arguments: %v", err), nil)
		}
		if in.ID == "" {
			return mcpToolError("invalid_input", "connect_database: id is required", nil)
		}

		settings, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID)
		if err != nil {
			return mcpToolError("not_found", fmt.Sprintf("connect_database: database %q not found: %v", in.ID, err), nil)
		}
		if settings == nil {
			return mcpToolError("not_found", fmt.Sprintf("connect_database: database %q not found", in.ID), nil)
		}

		adapter, err := s.BuildDBAdapter(settings)
		if err != nil {
			return mcpToolError("internal_error", fmt.Sprintf("connect_database: build adapter: %v", err), nil)
		}
		if adapter == nil {
			return mcpToolError("internal_error", "connect_database: adapter factory returned nil", nil)
		}
		// Connector only persists the storage key, not the live adapter, so
		// the handle we just built would otherwise leak. Close it now; the
		// caller's identity (the active storage key) is preserved in BoltDB
		// and the Connector's in-memory cache.
		defer func() {
			if cerr := adapter.Close(ctx); cerr != nil {
				logger.Warn("connect_database: adapter close failed", "id", in.ID, "error", cerr)
			}
		}()

		connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := adapter.Connect(connectCtx); err != nil {
			return mcpToolError("db_unreachable", fmt.Sprintf("connect_database: connect: %v", err), nil)
		}

		if err := s.deps.Connector.SetActive(ctx, settings.StorageKey()); err != nil {
			return mcpToolError("internal_error", fmt.Sprintf("connect_database: persist active id: %v", err), nil)
		}

		return jsonToolResult(connectDatabaseOutput{
			OK:             true,
			ActiveDatabase: settings.StorageKey(),
		}), nil
	}
}
