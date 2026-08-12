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

	"github.com/google/jsonschema-go/jsonschema"
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
	// omitempty so clients can call with empty args and let form elicitation collect values.
	ID       string `json:"id,omitempty" jsonschema:"short user-facing name for this database connection"`
	Host     string `json:"host,omitempty" jsonschema:"Oracle host or IP address"`
	Port     int    `json:"port,omitempty" jsonschema:"Oracle listener port, typically 1521"`
	Service  string `json:"service,omitempty" jsonschema:"Oracle service name or PDB, e.g. FREEPDB1"`
	Username string `json:"username,omitempty" jsonschema:"database login username"`
	Password string `json:"password,omitempty" jsonschema:"Oracle database password. Required. Safe to pass: this MCP server is locally hosted."`
}

// addDatabaseOutput is the JSON output shape for add_database.
type addDatabaseOutput struct {
	OK bool   `json:"ok"`
	ID string `json:"id"`
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

func addDatabaseInputComplete(in addDatabaseInput) bool {
	return in.ID != "" && in.Host != "" && in.Service != "" && in.Username != "" && in.Password != "" &&
		in.Port > 0 && in.Port <= 65535
}

func sessionSupportsFormElicitation(ss *mcp.ServerSession) bool {
	if ss == nil {
		return false
	}
	ip := ss.InitializeParams()
	if ip == nil || ip.Capabilities == nil || ip.Capabilities.Elicitation == nil {
		return false
	}
	caps := ip.Capabilities.Elicitation
	// Empty elicitation capability object means form mode (backward compatible).
	if caps.Form == nil && caps.URL != nil {
		return false
	}
	return true
}

func schemaDefault(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// addDatabaseElicitSchema builds a flat form schema. Password is included because this
// tool already transports plaintext credentials as MCP args; URL-mode elicitation would
// need a separate credential UI we do not host yet.
func addDatabaseElicitSchema(in addDatabaseInput) *jsonschema.Schema {
	minPort, maxPort := 1.0, 65535.0
	portSchema := &jsonschema.Schema{
		Type:        "integer",
		Title:       "Port",
		Description: "Oracle listener port",
		Minimum:     &minPort,
		Maximum:     &maxPort,
	}
	if in.Port > 0 {
		portSchema.Default = schemaDefault(in.Port)
	} else {
		portSchema.Default = schemaDefault(1521)
	}

	stringField := func(title, desc, value string) *jsonschema.Schema {
		s := &jsonschema.Schema{Type: "string", Title: title, Description: desc}
		if value != "" {
			s.Default = schemaDefault(value)
		}
		return s
	}

	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"id":       stringField("Database ID", "Short user-facing name for this connection", in.ID),
			"host":     stringField("Host", "Oracle host or IP address", in.Host),
			"port":     portSchema,
			"service":  stringField("Service", "Oracle service name or PDB", in.Service),
			"username": stringField("Username", "Database login username", in.Username),
			"password": stringField("Password", "Oracle database password. Required. Safe to pass: this MCP server is locally hosted.", in.Password),
		},
		Required: []string{"id", "host", "port", "service", "username", "password"},
	}
}

func elicitContentString(content map[string]any, key string) string {
	v, ok := content[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func elicitContentInt(content map[string]any, key string) (int, error) {
	v, ok := content[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case json.Number:
		n, err := t.Int64()
		return int(n), err
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func addDatabaseInputFromElicit(content map[string]any) (addDatabaseInput, error) {
	port, err := elicitContentInt(content, "port")
	if err != nil {
		return addDatabaseInput{}, err
	}
	return addDatabaseInput{
		ID:       elicitContentString(content, "id"),
		Host:     elicitContentString(content, "host"),
		Port:     port,
		Service:  elicitContentString(content, "service"),
		Username: elicitContentString(content, "username"),
		Password: elicitContentString(content, "password"),
	}, nil
}

// tryElicitAddDatabase asks the host for a connection form when supported.
// handled=false means the caller should use the supplied args (or return invalid_input).
func tryElicitAddDatabase(ctx context.Context, req *mcp.CallToolRequest, in addDatabaseInput) (out addDatabaseInput, handled bool, res *mcp.CallToolResult, err error) {
	if req == nil || !sessionSupportsFormElicitation(req.Session) {
		return in, false, nil, nil
	}

	elicitRes, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Mode:    "form",
		Message: "Enter Oracle database connection details. Password is required. This MCP server is locally hosted, so passing the password is acceptable.",
		RequestedSchema: addDatabaseElicitSchema(in),
	})
	if err != nil {
		// Capability races / older clients: fall back instead of failing the tool.
		if strings.Contains(err.Error(), "does not support") {
			return in, false, nil, nil
		}
		res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("add_database: elicitation failed: %v", err), nil)
		return in, true, res, mErr
	}

	switch elicitRes.Action {
	case "accept":
		out, err = addDatabaseInputFromElicit(elicitRes.Content)
		if err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInvalidInput, fmt.Sprintf("add_database: elicitation content: %v", err), nil)
			return in, true, res, mErr
		}
		return out, true, nil, nil
	case "decline", "cancel":
		res, mErr := mcpToolError(domain.ErrCodeInvalidInput,
			fmt.Sprintf("add_database: user %s elicitation", elicitRes.Action), nil)
		return in, true, res, mErr
	default:
		res, mErr := mcpToolError(domain.ErrCodeInternalError,
			fmt.Sprintf("add_database: unexpected elicitation action %q", elicitRes.Action), nil)
		return in, true, res, mErr
	}
}

// addDatabase elicits missing fields when the client supports it, then persists.
func addDatabase(s *Server) mcp.ToolHandlerFor[addDatabaseInput, addDatabaseOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in addDatabaseInput) (*mcp.CallToolResult, addDatabaseOutput, error) {
		if !addDatabaseInputComplete(in) {
			elicited, handled, elicitRes, elicitErr := tryElicitAddDatabase(ctx, req, in)
			if handled {
				if elicitRes != nil || elicitErr != nil {
					return elicitRes, addDatabaseOutput{}, elicitErr
				}
				in = elicited
			}
		}

		if !addDatabaseInputComplete(in) {
			if in.ID == "" || in.Host == "" || in.Service == "" || in.Username == "" || in.Password == "" {
				res, err := mcpToolError(domain.ErrCodeInvalidInput,
					"add_database: id, host, service, username, and password are required", nil)
				return res, addDatabaseOutput{}, err
			}
			res, err := mcpToolError(domain.ErrCodeInvalidInput, "add_database: port must be between 1 and 65535", nil)
			return res, addDatabaseOutput{}, err
		}

		// Reject silent overwrite of an existing ID; propagate anything other than "not found" as an internal error instead of silently falling through to create, which would mask genuine backend failures.
		existing, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID)
		switch {
		case err == nil && existing != nil:
			res, mErr := mcpToolError(domain.ErrCodeAlreadyExists,
				fmt.Sprintf("add_database: a database with id %q already exists; use a different id", in.ID), nil)
			return res, addDatabaseOutput{}, mErr
		case err != nil && !errors.Is(err, domain.ErrDatabaseSettingsNotFound):
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("add_database: lookup existing: %v", err), nil)
			return res, addDatabaseOutput{}, mErr
		}

		settings, err := domain.NewDatabaseSettings(
			in.ID, in.Service, in.Host,
			domain.Port(in.Port), in.Username, in.Password,
		)
		if err != nil {
			res, err := mcpToolError(domain.ErrCodeInvalidInput, fmt.Sprintf("add_database: validate: %v", err), nil)
			return res, addDatabaseOutput{}, err
		}

		if err := s.deps.DBSettingsRepo.Save(ctx, *settings); err != nil {
			res, err := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("add_database: persist: %v", err), nil)
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
			res, err := mcpToolError(domain.ErrCodeInvalidInput, "connect_database: id is required", nil)
			return res, connectDatabaseOutput{}, err
		}

		settings, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID)
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, connectDatabaseOutput{}, err
		case errors.Is(err, domain.ErrDatabaseSettingsNotFound):
			res, mErr := mcpToolError(domain.ErrCodeNotFound, fmt.Sprintf("connect_database: database %q not found", in.ID), nil)
			return res, connectDatabaseOutput{}, mErr
		case err != nil:
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: lookup database %q: %v", in.ID, err), nil)
			return res, connectDatabaseOutput{}, mErr
		}
		if settings == nil {
			res, err := mcpToolError(domain.ErrCodeNotFound, fmt.Sprintf("connect_database: database %q not found", in.ID), nil)
			return res, connectDatabaseOutput{}, err
		}

		if s.deps.DBAdapterFactory == nil {
			res, err := mcpToolError(domain.ErrCodeInternalError, "connect_database: DBAdapterFactory not configured", nil)
			return res, connectDatabaseOutput{}, err
		}
		adapter, err := s.deps.DBAdapterFactory(settings)
		if err != nil {
			res, err := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: build adapter: %v", err), nil)
			return res, connectDatabaseOutput{}, err
		}
		if adapter == nil {
			res, err := mcpToolError(domain.ErrCodeInternalError, "connect_database: adapter factory returned nil", nil)
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
			res, err := mcpToolError(domain.ErrCodeDBUnreachable, fmt.Sprintf("connect_database: connect: %v", err), nil)
			return res, connectDatabaseOutput{}, err
		}

		deployCtx, deployCancel := context.WithTimeout(ctx, 5*time.Second)
		defer deployCancel()

		if _, err := permissions.NewPermissionService(adapter, s.deps.PermissionsRepo, s.deps.Bolt).DeployAndCheck(deployCtx, settings.Username()); err != nil {
			res, mErr := mcpToolError(domain.ErrCodePermissionCheckFailed, fmt.Sprintf("connect_database: permission check: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}

		tracerSvc, err := tracer.NewTracerService(adapter, s.deps.Bolt, nil, tracer.TracerServiceOpts{})
		if err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: build tracer service: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}
		if err := tracerSvc.DeployAndCheck(deployCtx); err != nil {
			res, mErr := mcpToolError(domain.ErrCodeTracerDeployFailed, fmt.Sprintf("connect_database: tracer deploy: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}

		if _, err := s.deps.DBSettingsRepo.SetDefault(ctx, *settings); err != nil {
			res, mErr := mcpToolError(domain.ErrCodeInternalError, fmt.Sprintf("connect_database: persist default id: %v", err), nil)
			return res, connectDatabaseOutput{}, mErr
		}

		if s.deps.OnDatabaseConnected != nil {
			s.deps.OnDatabaseConnected(settings.StorageKey())
		}

		return nil, connectDatabaseOutput{
			OK:             true,
			ActiveDatabase: settings.StorageKey(),
		}, nil
	}
}
