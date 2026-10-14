package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func decodeToolPayload(t *testing.T, result mcp.CallToolResult) map[string]any {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("expected one JSON text result, got %d", len(result.Content))
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected JSON text content")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content.Text), &payload); err != nil {
		t.Fatal(err)
	}
	structuredJSON, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var structured map[string]any
	if err := json.Unmarshal(structuredJSON, &structured); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(payload, structured) {
		t.Fatal("text and structured tool results must agree")
	}
	return payload
}

func TestAddDatabase_ReturnsMissingFieldsWithoutIDEForm(t *testing.T) {
	deps, cleanup := testDeps(t)
	t.Cleanup(cleanup)
	formCalled := false
	session := connectClientServerOpts(t, deps, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			formCalled = true
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	})
	res := callTool(t, session, "add_database", map[string]any{"id": "DEV", "host": "  "})
	out := decodeToolPayload(t, res)
	want := []any{"host", "port", "service", "username", "password"}
	if !res.IsError || out["code"] != "invalid_input" || !reflect.DeepEqual(out["missing_fields"], want) || out["saved"] != false {
		t.Fatalf("expected missing-field feedback, got %+v", out)
	}
	if formCalled {
		t.Fatal("agent-driven setup must not open an IDE form")
	}
	listed := decodeToolPayload(t, callTool(t, session, "list_databases", map[string]any{}))
	if listed["total"] != float64(0) {
		t.Fatal("incomplete input must not save a configuration")
	}
}

func TestAddDatabase_InvalidPortFeedbackAgreesAcrossResultFormats(t *testing.T) {
	deps, cleanup := testDeps(t)
	t.Cleanup(cleanup)
	session := connectClientServer(t, deps)
	res := callTool(t, session, "add_database", map[string]any{
		"id": "DEV", "host": "oracle-dev.local", "port": 65536,
		"service": "DEV_PDB", "username": "DEV_USER", "password": "test-only-password",
	})
	out := decodeToolPayload(t, res)
	if !res.IsError || out["code"] != "invalid_input" || out["saved"] != false || out["connected"] != false {
		t.Fatalf("invalid port must not save: %+v", out)
	}
	if got := out["field_errors"].(map[string]any)["port"]; got != "must be between 1 and 65535" {
		t.Fatalf("missing field-level port feedback: %v", got)
	}
}

func TestAddDatabase_SaveAndConnectInOneCall(t *testing.T) {
	deps, _, cleanup := depsWithFakeDB(t)
	t.Cleanup(cleanup)
	endpoint, _ := startHTTPServer(t, deps, "127.0.0.1:0")
	client := mcp.NewClient(&mcp.Implementation{Name: "setup-client", Version: "test"}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	res := callTool(t, session, "add_database", map[string]any{
		"id": "DEV", "host": "oracle-dev.local", "port": 1521,
		"service": "DEV_PDB", "username": "DEV_USER", "password": "test-only-password", "connect": true,
	})
	if res.IsError {
		t.Fatalf("save/connect failed: %+v", res.Content)
	}
	out := decodeToolPayload(t, res)
	if out["saved"] != true || out["connected"] != true || out["id"] != "DBconfig:DEV" || out["active_database"] != "DBconfig:DEV" {
		t.Fatalf("unexpected save/connect result: %+v", out)
	}
	listed := decodeToolPayload(t, callTool(t, session, "list_databases", map[string]any{}))
	entries := listed["databases"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["is_active"] != true {
		t.Fatalf("saved database must be selected: %+v", listed)
	}
}

func TestAddDatabase_ConnectionFailureKeepsSavedSettingsAndPreviousSelection(t *testing.T) {
	for _, previous := range []bool{false, true} {
		t.Run(map[bool]string{false: "first connection", true: "previous selection"}[previous], func(t *testing.T) {
			deps, fake, cleanup := depsWithFakeDB(t)
			t.Cleanup(cleanup)
			fake.connectErr = errors.New("login failed for test-only-password")
			session := connectClientServer(t, deps)
			if previous {
				res := callTool(t, session, "add_database", map[string]any{
					"id": "OLD", "host": "old.local", "port": 1521,
					"service": "OLD", "username": "OLD_USER", "password": "old-test-password",
				})
				if res.IsError {
					t.Fatal("seed previous selection failed")
				}
			}
			res := callTool(t, session, "add_database", map[string]any{
				"id": "DEV", "host": "oracle-dev.local", "port": 1521,
				"service": "DEV_PDB", "username": "DEV_USER", "password": "test-only-password", "connect": true,
			})
			out := decodeToolPayload(t, res)
			if !res.IsError || out["saved"] != true || out["connected"] != false || out["id"] != "DBconfig:DEV" || out["code"] != "db_unreachable" {
				t.Fatalf("expected saved-but-unconnected result: %+v", out)
			}
			if strings.Contains(res.Content[0].(*mcp.TextContent).Text, "test-only-password") {
				t.Fatal("connection error must not echo the password")
			}
			listed := decodeToolPayload(t, callTool(t, session, "list_databases", map[string]any{}))
			entries := listed["databases"].([]any)
			found := false
			for _, entry := range entries {
				db := entry.(map[string]any)
				if db["database_id"] == "DEV" {
					found = true
					if db["is_active"] != false {
						t.Fatal("failed verification must not select the new configuration")
					}
				}
				if db["database_id"] == "OLD" && db["is_active"] != true {
					t.Fatal("previous selection must remain active")
				}
			}
			if !found {
				t.Fatal("failed connection must retain saved settings for retry")
			}
			fake.connectErr = nil
			retry := callTool(t, session, "connect_database", map[string]any{"id": out["id"]})
			if retry.IsError || decodeToolPayload(t, retry)["active_database"] != "DBconfig:DEV" {
				t.Fatal("saved connection must be usable by connect_database without adding it again")
			}
		})
	}
}

func TestAddDatabase_SaveOnlyAndDuplicateConnectPreserveConfiguration(t *testing.T) {
	deps, fake, cleanup := depsWithFakeDB(t)
	t.Cleanup(cleanup)
	session := connectClientServer(t, deps)
	args := map[string]any{
		"id": "DEV", "host": "original.local", "port": 1521,
		"service": "DEV_PDB", "username": "DEV_USER", "password": "test-only-password",
	}
	res := callTool(t, session, "add_database", args)
	out := decodeToolPayload(t, res)
	if res.IsError || out["saved"] != true || out["connected"] != false || fake.connected {
		t.Fatal("save-only must preserve existing behavior without Oracle verification")
	}
	args["host"] = "replacement.local"
	args["connect"] = true
	duplicate := callTool(t, session, "add_database", args)
	if !duplicate.IsError || decodeToolPayload(t, duplicate)["code"] != "already_exists" {
		t.Fatal("save/connect must reject duplicate IDs")
	}
	listed := decodeToolPayload(t, callTool(t, session, "list_databases", map[string]any{}))
	entries := listed["databases"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["host"] != "original.local" {
		t.Fatal("duplicate save/connect must not overwrite existing settings")
	}
}
