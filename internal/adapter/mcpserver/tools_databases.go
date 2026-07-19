package mcpserver

import (
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
			return nil, fmt.Errorf("list_databases: %w", err)
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

// addDatabasePersisted is returned when the caller has confirmed the
// plaintext-password risk and the database settings have been written.
func addDatabasePersisted(id string) addDatabaseOutput {
	return addDatabaseOutput{OK: true, ID: id}
}

// addDatabaseConfirmRequired is returned when the caller must resend with
// confirm_password_in_plaintext=true.
func addDatabaseConfirmRequired() (mcp.CallToolResult, error) {
	payload, _ := mcpToolError("password_in_plaintext",
		"Sending passwords over MCP exposes them in client logs and process listings. "+
			"Confirm to proceed, or use the TUI onboarding form.",
		map[string]any{"confirm_required": true})
	return payload, nil
}

// mcpToolError serializes a structured error response. We piggyback on
// CallToolResult.IsError + TextContent because MCP does not have a
// first-class error payload in tool results.
func mcpToolError(code, message string, extra map[string]any) (mcp.CallToolResult, error) {
	payload := map[string]any{
		"code":    code,
		"message": message,
	}
	for k, v := range extra {
		payload[k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return mcp.CallToolResult{}, err
	}
	return mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}, nil
}

// addDatabase validates, then either asks for confirmation or persists.
func addDatabase(s *Server) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in addDatabaseInput
		if err := decodeArguments(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("add_database: decode arguments: %w", err)
		}

		// First, validate what we can without touching the password.
		if in.ID == "" || in.Host == "" || in.Service == "" || in.Username == "" || in.Password == "" {
			return nil, fmt.Errorf("add_database: id, host, service, username, and password are required")
		}
		if in.Port <= 0 || in.Port > 65535 {
			return nil, fmt.Errorf("add_database: port must be between 1 and 65535")
		}

		if !in.ConfirmPasswordInPlaintext {
			result, err := addDatabaseConfirmRequired()
			if err != nil {
				return nil, err
			}
			return &result, nil
		}

		settings, err := domain.NewDatabaseSettings(
			in.ID, in.Service, in.Host,
			domain.Port(in.Port), in.Username, in.Password,
		)
		if err != nil {
			return nil, fmt.Errorf("add_database: validate: %w", err)
		}

		if err := s.deps.DBSettingsRepo.Save(ctx, *settings); err != nil {
			return nil, fmt.Errorf("add_database: persist: %w", err)
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
			return nil, fmt.Errorf("connect_database: decode arguments: %w", err)
		}
		if in.ID == "" {
			return nil, fmt.Errorf("connect_database: id is required")
		}

		settings, err := s.deps.DBSettingsRepo.GetByID(ctx, in.ID)
		if err != nil {
			return nil, fmt.Errorf("connect_database: load settings: %w", err)
		}
		if settings == nil {
			return nil, fmt.Errorf("connect_database: database %q not found", in.ID)
		}

		adapter, err := s.BuildDBAdapter(settings)
		if err != nil {
			return nil, fmt.Errorf("connect_database: build adapter: %w", err)
		}
		if adapter == nil {
			return nil, fmt.Errorf("connect_database: adapter factory returned nil")
		}

		connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := adapter.Connect(connectCtx); err != nil {
			result, mcpErr := mcpToolError("db_unreachable",
				fmt.Sprintf("connect_database: connect: %v", err), nil)
			if mcpErr != nil {
				return nil, mcpErr
			}
			return &result, nil
		}

		if err := s.deps.Connector.SetActive(ctx, settings.StorageKey()); err != nil {
			// Roll back the connection — we don't want a stale handle left open.
			_ = adapter.Close(ctx)
			return nil, fmt.Errorf("connect_database: persist active id: %w", err)
		}

		return jsonToolResult(connectDatabaseOutput{
			OK:             true,
			ActiveDatabase: settings.StorageKey(),
		}), nil
	}
}
