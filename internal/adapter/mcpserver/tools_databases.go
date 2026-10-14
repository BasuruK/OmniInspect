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
	"strings"
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
			res, err := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("list_databases: %v", err), nil)
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
	// Optional schema fields allow structured missing-field feedback to the agent.
	ID       string `json:"id,omitempty" jsonschema:"short user-facing name for this database connection"`
	Host     string `json:"host,omitempty" jsonschema:"Oracle host or IP address"`
	Port     int    `json:"port,omitempty" jsonschema:"Oracle listener port, typically 1521"`
	Service  string `json:"service,omitempty" jsonschema:"Oracle service name or PDB, e.g. FREEPDB1"`
	Username string `json:"username,omitempty" jsonschema:"database login username"`
	Password string `json:"password,omitempty" jsonschema:"Oracle database password collected by the agent. Never echo it in responses."`
	Connect  bool   `json:"connect,omitempty" jsonschema:"When true, save and verify the connection, check permissions and tracer package, then select this database. Defaults to false (save only)."`
}

// addDatabaseOutput is the JSON output shape for add_database.
type addDatabaseOutput struct {
	OK             bool              `json:"ok"`
	ID             string            `json:"id"`
	Saved          bool              `json:"saved"`
	Connected      bool              `json:"connected"`
	ActiveDatabase string            `json:"active_database,omitempty"`
	Code           string            `json:"code,omitempty"`
	Message        string            `json:"message,omitempty"`
	MissingFields  []string          `json:"missing_fields,omitempty"`
	FieldErrors    map[string]string `json:"field_errors,omitempty"`
}

// mcpToolError serializes a structured error response. We piggyback on CallToolResult.IsError + TextContent because MCP does not have a first-class error payload in tool results. `code` is a domain.ErrCode* sentinel so callers can errors.Is against it; the JSON wire field is built from code.Error().
func mcpToolError(code error, message string, extra map[string]any) (*mcp.CallToolResult, error) {
	payload := map[string]any{
		"code":    code.Error(),
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

func missingDatabaseFields(in addDatabaseInput) []string {
	missing := make([]string, 0)
	for _, field := range []struct {
		name    string
		missing bool
	}{
		{"id", strings.TrimSpace(in.ID) == ""},
		{"host", strings.TrimSpace(in.Host) == ""},
		{"port", in.Port == 0},
		{"service", strings.TrimSpace(in.Service) == ""},
		{"username", strings.TrimSpace(in.Username) == ""},
		{"password", strings.TrimSpace(in.Password) == ""},
	} {
		if field.missing {
			missing = append(missing, field.name)
		}
	}
	return missing
}

// addDatabaseFailure lets the SDK serialize the same typed payload into text and structured content.
func addDatabaseFailure(code error, message string, out addDatabaseOutput) (*mcp.CallToolResult, addDatabaseOutput, error) {
	out.OK = false
	out.Code = code.Error()
	out.Message = message
	return &mcp.CallToolResult{IsError: true}, out, nil
}

// addDatabase persists explicit agent-collected fields and optionally verifies/selects the database.
func addDatabase(s *Server) mcp.ToolHandlerFor[addDatabaseInput, addDatabaseOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in addDatabaseInput) (*mcp.CallToolResult, addDatabaseOutput, error) {
		if missing := missingDatabaseFields(in); len(missing) > 0 {
			return addDatabaseFailure(domain.ErrCodeInvalidInput,
				"add_database: gather the missing connection details in IDE conversation and submit all fields",
				addDatabaseOutput{MissingFields: missing})
		}
		if in.Port < int(domain.MinPort) || in.Port > int(domain.MaxPort) {
			return addDatabaseFailure(domain.ErrCodeInvalidInput, "add_database: port must be between 1 and 65535",
				addDatabaseOutput{FieldErrors: map[string]string{"port": "must be between 1 and 65535"}})
		}

		settings, err := domain.NewDatabaseSettings(
			in.ID, in.Service, in.Host,
			domain.Port(in.Port), in.Username, in.Password,
		)
		if err != nil {
			return addDatabaseFailure(domain.ErrCodeInvalidInput, fmt.Sprintf("add_database: validate: %v", err), addDatabaseOutput{})
		}

		becameDefault := false
		if in.Connect {
			err = s.deps.DBSettingsRepo.Create(ctx, *settings)
		} else {
			becameDefault, err = s.deps.DBSettingsRepo.CreateAndSelectIfNone(ctx, *settings)
		}
		switch {
		case errors.Is(err, domain.ErrKeyCollision):
			return addDatabaseFailure(domain.ErrCodeAlreadyExists,
				fmt.Sprintf("add_database: a database with id %q already exists; use a different id", settings.DatabaseID()), addDatabaseOutput{})
		case err != nil:
			return addDatabaseFailure(domain.ErrCodeInternalError,
				redactDatabasePassword(fmt.Sprintf("add_database: persist: %v", err), settings.Password()), addDatabaseOutput{})
		}
		if becameDefault && s.deps.OnDatabaseConnected != nil {
			s.deps.OnDatabaseConnected(settings.StorageKey())
		}

		out := addDatabaseOutput{OK: true, ID: settings.StorageKey(), Saved: true}
		if in.Connect {
			res, connected, err := s.connectSavedDatabase(ctx, settings.StorageKey())
			if res != nil || err != nil {
				out.OK = false
				out.Code = connected.Code
				out.Message = connected.Message
				return res, out, err
			}
			out.Connected = true
			out.ActiveDatabase = connected.ActiveDatabase
		}
		return nil, out, nil
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
	Code           string `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
}

// connectDatabase validates the request, builds the adapter via the injected factory, deploys/checks required permissions and the tracer package against it, and on success sets the database as default via DBSettingsRepo.SetDefault. It deliberately does NOT unregister the previously active database — the spec calls for that only when the new connect succeeds.
func connectDatabase(s *Server) mcp.ToolHandlerFor[connectDatabaseInput, connectDatabaseOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in connectDatabaseInput) (*mcp.CallToolResult, connectDatabaseOutput, error) {
		return s.connectSavedDatabase(ctx, in.ID)
	}
}

// connectSavedDatabase shares verification and selection between add_database and connect_database.
func (s *Server) connectSavedDatabase(ctx context.Context, id string) (*mcp.CallToolResult, connectDatabaseOutput, error) {
	if id == "" {
		return connectDatabaseFailure(domain.ErrCodeInvalidInput, "connect_database: id is required")
	}

	settings, err := s.deps.DBSettingsRepo.GetByID(ctx, id)
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil, connectDatabaseOutput{}, err
	case errors.Is(err, domain.ErrDatabaseSettingsNotFound):
		return connectDatabaseFailure(domain.ErrCodeNotFound, fmt.Sprintf("connect_database: database %q not found", id))
	case err != nil:
		return connectDatabaseFailure(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: lookup database %q: %v", id, err))
	}
	if settings == nil {
		return connectDatabaseFailure(domain.ErrCodeNotFound, fmt.Sprintf("connect_database: database %q not found", id))
	}

	connectionError := func(code error, message string) (*mcp.CallToolResult, connectDatabaseOutput, error) {
		return connectDatabaseFailure(code, redactDatabasePassword(message, settings.Password()))
	}

	if s.deps.DBAdapterFactory == nil {
		return connectionError(domain.ErrCodeInternalError, "connect_database: DBAdapterFactory not configured")
	}
	adapter, err := s.deps.DBAdapterFactory(settings)
	if err != nil {
		return connectionError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: build adapter: %v", err))
	}
	if adapter == nil {
		return connectionError(domain.ErrCodeInternalError, "connect_database: adapter factory returned nil")
	}
	// SetDefault only persists the storage key, not the live adapter, so the handle we just built would otherwise leak. Close it now; the caller's identity (the default storage key) is preserved in BoltDB.
	defer func() {
		if cerr := adapter.Close(ctx); cerr != nil {
			logger.Warn("connect_database: adapter close failed", "id", id, "error", redactDatabasePassword(cerr.Error(), settings.Password()))
		}
	}()

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := adapter.Connect(connectCtx); err != nil {
		return connectionError(domain.ErrCodeDBUnreachable, fmt.Sprintf("connect_database: connect: %v", err))
	}

	deployCtx, deployCancel := context.WithTimeout(ctx, 5*time.Second)
	defer deployCancel()

	if _, err := permissions.NewPermissionService(adapter, s.deps.PermissionsRepo, s.deps.Bolt).DeployAndCheck(deployCtx, settings.Username()); err != nil {
		return connectionError(domain.ErrCodePermissionCheckFailed, fmt.Sprintf("connect_database: permission check: %v", err))
	}

	tracerSvc, err := tracer.NewTracerService(adapter, s.deps.Bolt, nil, tracer.TracerServiceOpts{})
	if err != nil {
		return connectionError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: build tracer service: %v", err))
	}
	if err := tracerSvc.DeployAndCheck(deployCtx); err != nil {
		return connectionError(domain.ErrCodeTracerDeployFailed, fmt.Sprintf("connect_database: tracer deploy: %v", err))
	}

	if _, err := s.deps.DBSettingsRepo.SetDefault(ctx, *settings); err != nil {
		return connectionError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: persist default id: %v", err))
	}

	if s.deps.OnDatabaseConnected != nil {
		s.deps.OnDatabaseConnected(settings.StorageKey())
	}

	return nil, connectDatabaseOutput{
		OK:             true,
		ActiveDatabase: settings.StorageKey(),
	}, nil
}

// redactDatabasePassword prevents adapter error text from echoing submitted credentials.
func redactDatabasePassword(message, password string) string {
	if password == "" {
		return message
	}
	return strings.ReplaceAll(message, password, "<REDACTED>")
}

func connectDatabaseFailure(code error, message string) (*mcp.CallToolResult, connectDatabaseOutput, error) {
	return &mcp.CallToolResult{IsError: true}, connectDatabaseOutput{Code: code.Error(), Message: message}, nil
}
