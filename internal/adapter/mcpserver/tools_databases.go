package mcpserver

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/core/domain"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/tracer"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// list_databases
// ==========================================

// databaseListEntry is one row of the list_databases response. Password is never included; only metadata safe for an LLM client to read.
type databaseListEntry struct {
	DatabaseID string `json:"database_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Service    string `json:"service"`
	Username   string `json:"username"`
	IsActive   bool   `json:"is_active"`
}

// databaseListOutput is the response payload of list_databases.
type databaseListOutput struct {
	Databases []databaseListEntry `json:"databases"`
	Total     int                 `json:"total"`
}

// listDatabases returns every persisted database configuration, flagging the one currently marked default.
func listDatabases(s *Server) mcp.ToolHandlerFor[emptyInput, databaseListOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, databaseListOutput, error) {
		all, err := s.deps.DBSettingsRepo.GetAll(ctx)
		if err != nil {
			res, err := mcpToolError(errCodeInternalError, fmt.Sprintf("list_databases: %v", err), nil)
			return res, databaseListOutput{}, err
		}
		out := databaseListOutput{Databases: make([]databaseListEntry, 0, len(all))}
		for _, d := range all {
			out.Databases = append(out.Databases, databaseListEntry{
				DatabaseID: d.DatabaseID(),
				Host:       d.Host(),
				Port:       int(d.Port()),
				Service:    d.Database(),
				Username:   d.Username(),
				IsActive:   d.IsDefault(),
			})
		}
		out.Total = len(out.Databases)
		return nil, out, nil
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
	return mcpToolError(errCodePasswordInPlaintext,
		"Sending passwords over MCP exposes them in client logs and process listings. Confirm to proceed, or use the TUI onboarding form.",
		map[string]any{"confirm_required": true})
}

// Stable error codes returned in mcpToolError's "code" field.
const (
	errCodeInvalidInput          = "invalid_input"           // bad/missing arguments
	errCodeNotFound              = "not_found"               // referenced entity does not exist
	errCodeAlreadyExists         = "already_exists"          // create would collide with an existing entity
	errCodeInternalError         = "internal_error"          // storage/backend failure not caused by caller input
	errCodeDBUnreachable         = "db_unreachable"          // connect_database's connect attempt failed
	errCodePermissionCheckFailed = "permission_check_failed" // connect_database's permission deploy/check step failed
	errCodeTracerDeployFailed    = "tracer_deploy_failed"    // connect_database's tracer package deploy step failed
	errCodePasswordInPlaintext   = "password_in_plaintext"   // add_database confirmation gate
	errCodeCursorExpired         = "cursor_expired"          // list_traces' since_id is unknown or evicted
)

// mcpToolError serializes a structured error response. We piggyback on CallToolResult.IsError + TextContent because MCP does not have a first-class error payload in tool results.
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
func addDatabase(s *Server) mcp.ToolHandlerFor[addDatabaseInput, addDatabaseOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in addDatabaseInput) (*mcp.CallToolResult, addDatabaseOutput, error) {
		// First, validate what we can without touching the password.
		if in.ID == "" || in.Host == "" || in.Service == "" || in.Username == "" || in.Password == "" {
			res, err := mcpToolError(errCodeInvalidInput,
				"add_database: id, host, service, username, and password are required", nil)
			return res, addDatabaseOutput{}, err
		}
		if in.Port <= 0 || in.Port > 65535 {
			res, err := mcpToolError(errCodeInvalidInput, "add_database: port must be between 1 and 65535", nil)
			return res, addDatabaseOutput{}, err
		}

		// Reject silent overwrite of an existing ID; propagate anything other than "not found" as an internal error instead of silently falling through to create, which would mask genuine backend failures.
		existing, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID)
		switch {
		case err == nil && existing != nil:
			res, mErr := mcpToolError(errCodeAlreadyExists,
				fmt.Sprintf("add_database: a database with id %q already exists; use a different id", in.ID), nil)
			return res, addDatabaseOutput{}, mErr
		case err != nil && !errors.Is(err, domain.ErrDatabaseSettingsNotFound):
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("add_database: lookup existing: %v", err), nil)
			return res, addDatabaseOutput{}, mErr
		}

		if !in.ConfirmPasswordInPlaintext {
			res, err := addDatabaseConfirmRequired()
			return res, addDatabaseOutput{}, err
		}

		settings, err := domain.NewDatabaseSettings(
			in.ID, in.Service, in.Host,
			domain.Port(in.Port), in.Username, in.Password,
		)
		if err != nil {
			res, err := mcpToolError(errCodeInvalidInput, fmt.Sprintf("add_database: validate: %v", err), nil)
			return res, addDatabaseOutput{}, err
		}

		if err := s.deps.DBSettingsRepo.Save(ctx, *settings); err != nil {
			res, err := mcpToolError(errCodeInternalError, fmt.Sprintf("add_database: persist: %v", err), nil)
			return res, addDatabaseOutput{}, err
		}
		return nil, addDatabaseOutput{OK: true, ID: settings.StorageKey()}, nil
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

// connectDatabase validates the request, builds the adapter via the injected factory, deploys/checks required permissions and the tracer package against it, and on success sets the database as default via DBSettingsRepo.SetDefault. It deliberately does NOT unregister the previously active database — the spec calls for that only when the new connect succeeds.
func connectDatabase(s *Server) mcp.ToolHandlerFor[connectDatabaseInput, connectDatabaseOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in connectDatabaseInput) (*mcp.CallToolResult, connectDatabaseOutput, error) {
		if in.ID == "" {
			res, err := mcpToolError(errCodeInvalidInput, "connect_database: id is required", nil)
			return res, connectDatabaseOutput{}, err
		}

		settings, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID)
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, connectDatabaseOutput{}, err
		case errors.Is(err, domain.ErrDatabaseSettingsNotFound):
			res, mErr := mcpToolError(errCodeNotFound, fmt.Sprintf("connect_database: database %q not found", in.ID), nil)
			return res, connectDatabaseOutput{}, mErr
		case err != nil:
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("connect_database: lookup database %q: %v", in.ID, err), nil)
			return res, connectDatabaseOutput{}, mErr
		}
		if settings == nil {
			res, err := mcpToolError(errCodeNotFound, fmt.Sprintf("connect_database: database %q not found", in.ID), nil)
			return res, connectDatabaseOutput{}, err
		}

		if s.deps.DBAdapterFactory == nil {
			res, err := mcpToolError(errCodeInternalError, "connect_database: DBAdapterFactory not configured", nil)
			return res, connectDatabaseOutput{}, err
		}
		adapter, err := s.deps.DBAdapterFactory(settings)
		if err != nil {
			res, err := mcpToolError(errCodeInternalError, fmt.Sprintf("connect_database: build adapter: %v", err), nil)
			return res, connectDatabaseOutput{}, err
		}
		if adapter == nil {
			res, err := mcpToolError(errCodeInternalError, "connect_database: adapter factory returned nil", nil)
			return res, connectDatabaseOutput{}, err
		}
		// SetDefault only persists the storage key, not the live adapter, so the handle we just built would otherwise leak. Close it now; the caller's identity (the default storage key) is preserved in BoltDB.
		defer func() {
			if cerr := adapter.Close(ctx); cerr != nil {
				logger.Warn("connect_database: adapter close failed", "id", in.ID, "error", cerr)
			}
		}()

		connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := adapter.Connect(connectCtx); err != nil {
			res, err := mcpToolError(errCodeDBUnreachable, fmt.Sprintf("connect_database: connect: %v", err), nil)
			return res, connectDatabaseOutput{}, err
		}

		deployCtx, deployCancel := context.WithTimeout(ctx, 5*time.Second)
		defer deployCancel()

		if _, err := permissions.NewPermissionService(adapter, s.deps.PermissionsRepo, s.deps.Bolt).DeployAndCheck(deployCtx, settings.Username()); err != nil {
			res, mErr := mcpToolError(errCodePermissionCheckFailed, fmt.Sprintf("connect_database: permission check: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}

		tracerSvc, err := tracer.NewTracerService(adapter, s.deps.Bolt, nil, tracer.TracerServiceOpts{})
		if err != nil {
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("connect_database: build tracer service: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}
		if err := tracerSvc.DeployAndCheck(deployCtx); err != nil {
			res, mErr := mcpToolError(errCodeTracerDeployFailed, fmt.Sprintf("connect_database: tracer deploy: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}

		if _, err := s.deps.DBSettingsRepo.SetDefault(ctx, *settings); err != nil {
			res, mErr := mcpToolError(errCodeInternalError, fmt.Sprintf("connect_database: persist default id: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}

		return nil, connectDatabaseOutput{
			OK:             true,
			ActiveDatabase: settings.StorageKey(),
		}, nil
	}
}
