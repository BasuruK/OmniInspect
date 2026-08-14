package mcpserver

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Helpers
// ==========================================

// fakeDBAdapter is a ports.DatabaseRepository stub used for connect_database
// tests. Connect/Close, plus the PackageExists/FetchWithParams/ExecuteStatement
// calls made by the permission-check and tracer-deploy steps, are exercised.
type fakeDBAdapter struct {
	connectErr error
	closeErr   error
	connected  bool
	closed     bool
}

func (f *fakeDBAdapter) Connect(ctx context.Context) error {
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	return nil
}

func (f *fakeDBAdapter) Close(ctx context.Context) error {
	f.closed = true
	return f.closeErr
}

// Unused interface methods.
func (f *fakeDBAdapter) RegisterNewSubscriber(context.Context, domain.Subscriber) error {
	return nil
}
func (f *fakeDBAdapter) UnregisterSubscriber(context.Context, domain.Subscriber) error {
	return nil
}
func (f *fakeDBAdapter) BulkDequeueTracerMessages(context.Context, domain.Subscriber) ([]string, [][]byte, int, error) {
	return nil, nil, 0, nil
}
func (f *fakeDBAdapter) CheckQueueDepth(context.Context, string, string) (int, error) {
	return 0, nil
}
func (f *fakeDBAdapter) Fetch(context.Context, string) ([]string, error) {
	return nil, nil
}
func (f *fakeDBAdapter) ExecuteStatement(context.Context, string) error { return nil }
func (f *fakeDBAdapter) ExecuteWithParams(context.Context, string, map[string]interface{}) error {
	return nil
}
func (f *fakeDBAdapter) FetchWithParams(context.Context, string, map[string]interface{}) ([]string, error) {
	// Build the "all privileges granted" JSON from the domain type so any
	// privilege added/renamed in domain.PermissionStatus is reflected here
	// automatically.
	status := domain.PermissionStatus{}
	v := reflect.ValueOf(&status).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Bool {
			v.Field(i).SetBool(true)
		}
	}
	allGranted, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	return []string{string(allGranted)}, nil
}
func (f *fakeDBAdapter) PackageExists(context.Context, string) (bool, error) {
	return true, nil
}
func (f *fakeDBAdapter) ProcedureExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *fakeDBAdapter) DeployPackages(context.Context, []string, []string, []string, []string) error {
	return nil
}
func (f *fakeDBAdapter) DeployFile(context.Context, string) error { return nil }

// Compile-time check.
var _ ports.DatabaseRepository = (*fakeDBAdapter)(nil)

// depsWithFakeDB returns Deps whose factory builds a fakeDBAdapter. The
// returned helper lets tests pre-program the adapter's behaviour.
func depsWithFakeDB(t *testing.T) (Deps, *fakeDBAdapter, func()) {
	deps, cleanup := testDeps(t)
	fake := &fakeDBAdapter{}
	deps.DBAdapterFactory = func(*domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return fake, nil
	}
	return deps, fake, cleanup
}

// callTool wraps session.CallTool and decodes a JSON payload from the
// single TextContent.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return *res
}

// ==========================================
// list_databases
// ==========================================

func TestListDatabases_EmptyByDefault(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	res := callTool(t, session, "list_databases", map[string]any{})

	if res.IsError {
		t.Fatalf("expected success, got IsError: %+v", res.Content)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var out databaseListOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Total != 0 || len(out.Databases) != 0 {
		t.Fatalf("expected empty list, got %+v", out)
	}
}

func TestListDatabases_SkipsPasswordAndFlagsActive(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	settings, err := domain.NewDatabaseSettings(
		"prod-1", "FREEPDB1", "db.example.com",
		domain.Port(1521), "admin", "supersecret",
	)
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	if err := deps.DBSettingsRepo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := deps.DBSettingsRepo.SetDefault(context.Background(), *settings); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	session := connectClientServer(t, deps)
	res := callTool(t, session, "list_databases", map[string]any{})
	if res.IsError {
		t.Fatalf("expected success, got IsError: %+v", res.Content)
	}
	tc := res.Content[0].(*mcp.TextContent)
	if got := tc.Text; contains(got, "supersecret") || contains(got, "Password") {
		t.Fatalf("password leaked into response: %s", got)
	}

	var out databaseListOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Total != 1 || len(out.Databases) != 1 {
		t.Fatalf("expected 1 entry, got %+v", out)
	}
	row := out.Databases[0]
	if row.DatabaseID != "prod-1" {
		t.Fatalf("expected database_id=prod-1, got %q", row.DatabaseID)
	}
	if row.Port != 1521 {
		t.Fatalf("expected port=1521, got %d", row.Port)
	}
	if !row.IsActive {
		t.Fatalf("expected is_active=true")
	}
}

// ==========================================
// add_database
// ==========================================

func TestAddDatabase_Persists(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "add_database",
		Arguments: map[string]any{
			"id":       "prod-2",
			"host":     "db.example.com",
			"port":     1521,
			"service":  "FREEPDB1",
			"username": "admin",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		var dump string
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				dump = tc.Text
			}
		}
		t.Fatalf("expected success, got IsError: %s", dump)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var out addDatabaseOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK || out.ID == "" {
		t.Fatalf("expected ok=true and non-empty id, got %+v", out)
	}

	// Verify persisted.
	all, err := deps.DBSettingsRepo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 record, got %d", len(all))
	}
	if all[0].DatabaseID() != "prod-2" {
		t.Fatalf("expected prod-2, got %s", all[0].DatabaseID())
	}
}

func TestAddDatabase_ElicitationPersists(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	session := connectClientServerOpts(t, deps, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			if req.Params == nil || req.Params.RequestedSchema == nil {
				t.Fatalf("expected requested schema")
			}
			return &mcp.ElicitResult{
				Action: "accept",
				Content: map[string]any{
					"id":       "elicit-1",
					"host":     "db.example.com",
					"port":     1521,
					"service":  "FREEPDB1",
					"username": "admin",
					"password": "secret",
				},
			}, nil
		},
	})

	res := callTool(t, session, "add_database", map[string]any{})
	if res.IsError {
		var dump string
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				dump = tc.Text
			}
		}
		t.Fatalf("expected success via elicitation, got IsError: %s", dump)
	}

	all, err := deps.DBSettingsRepo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 1 || all[0].DatabaseID() != "elicit-1" {
		t.Fatalf("expected elicit-1 persisted, got %+v", all)
	}
}

func TestAddDatabase_ElicitationDeclined(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	session := connectClientServerOpts(t, deps, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	})

	res := callTool(t, session, "add_database", map[string]any{})
	if !res.IsError {
		t.Fatalf("expected IsError after decline, got %+v", res)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var payload map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["code"] != "invalid_input" {
		t.Fatalf("expected invalid_input, got %v", payload["code"])
	}

	all, err := deps.DBSettingsRepo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected nothing persisted, got %d", len(all))
	}
}

func TestAddDatabase_RejectsInvalidInput(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	res := callTool(t, session, "add_database", map[string]any{
		"id":       "prod-3",
		"host":     "db.example.com",
		"port":     999999, // out of range
		"service":  "FREEPDB1",
		"username": "admin",
		"password": "secret",
	})
	if !res.IsError {
		t.Fatalf("expected IsError=true for invalid port, got %+v", res)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var payload map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["code"] != "invalid_input" {
		t.Fatalf("expected code=invalid_input, got %v", payload["code"])
	}
}

func TestAddDatabase_RejectsDuplicateID(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	args := map[string]any{
		"id":       "prod-dup",
		"host":     "db.example.com",
		"port":     1521,
		"service":  "FREEPDB1",
		"username": "admin",
		"password": "secret",
	}

	first := callTool(t, session, "add_database", args)
	if first.IsError {
		t.Fatalf("expected first add_database to succeed, got IsError: %+v", first.Content)
	}

	second := callTool(t, session, "add_database", args)
	if !second.IsError {
		t.Fatalf("expected second add_database with same id to fail, got %+v", second)
	}
	tc := second.Content[0].(*mcp.TextContent)
	var payload map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["code"] != "already_exists" {
		t.Fatalf("expected code=already_exists, got %v", payload["code"])
	}

	// Confirm only one record was persisted.
	all, err := deps.DBSettingsRepo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 record persisted, got %d", len(all))
	}
}

func TestAddDatabase_ConcurrentSameID(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	const n = 8
	args := map[string]any{
		"id":       "race-dup",
		"host":     "db.example.com",
		"port":     1521,
		"service":  "FREEPDB1",
		"username": "admin",
		"password": "secret",
	}
	sessions := make([]*mcp.ClientSession, n)
	for i := 0; i < n; i++ {
		sessions[i] = connectClientServer(t, deps)
	}

	var successes atomic.Int32
	var alreadyExists atomic.Int32
	errCh := make(chan string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			res, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "add_database",
				Arguments: args,
			})
			if err != nil {
				errCh <- err.Error()
				return
			}
			if !res.IsError {
				successes.Add(1)
				return
			}
			if len(res.Content) == 0 {
				errCh <- "IsError with empty content"
				return
			}
			tc, ok := res.Content[0].(*mcp.TextContent)
			if !ok {
				errCh <- "IsError content is not TextContent"
				return
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
				errCh <- "decode: " + err.Error()
				return
			}
			code, _ := payload["code"].(string)
			if code != "already_exists" {
				errCh <- "unexpected error: " + tc.Text
				return
			}
			alreadyExists.Add(1)
		}()
	}
	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Fatalf("concurrent add_database: %s", msg)
	}
	if got := successes.Load(); got != 1 {
		t.Fatalf("successes=%d, want 1", got)
	}
	if got := alreadyExists.Load(); got != n-1 {
		t.Fatalf("already_exists=%d, want %d", got, n-1)
	}
	all, err := deps.DBSettingsRepo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("persisted %d records, want 1", len(all))
	}
}

func TestAddDatabaseInputFromElicit_PortSuccess(t *testing.T) {
	cases := []struct {
		name string
		port any
	}{
		{name: "float64", port: float64(1521)},
		{name: "int", port: 1521},
		{name: "int64", port: int64(1521)},
		{name: "json.Number", port: json.Number("1521")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := addDatabaseInputFromElicit(map[string]any{"port": tc.port, "id": "db1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Port != 1521 {
				t.Fatalf("Port=%d, want 1521", got.Port)
			}
		})
	}
}

func TestAddDatabaseInputFromElicit_PortErrorsWrapInvalidPort(t *testing.T) {
	cases := []struct {
		name    string
		content map[string]any
		wantIs  error
	}{
		{name: "missing", content: map[string]any{}},
		{name: "nil", content: map[string]any{"port": nil}},
		{name: "unsupported type", content: map[string]any{"port": true}},
		{name: "json.Number conversion", content: map[string]any{"port": json.Number("not-an-int")}, wantIs: strconv.ErrSyntax},
		{name: "fractional float64", content: map[string]any{"port": 1521.5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := addDatabaseInputFromElicit(tc.content)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, domain.ErrInvalidPort) {
				t.Fatalf("errors.Is(ErrInvalidPort)=false: %v", err)
			}
			if !contains(err.Error(), "add_database") {
				t.Fatalf("missing add_database context: %v", err)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("errors.Is(%v)=false: %v", tc.wantIs, err)
			}
		})
	}
}

// ==========================================
// connect_database
// ==========================================

func TestConnectDatabase_HappyPath(t *testing.T) {
	deps, fake, cleanup := depsWithFakeDB(t)
	defer cleanup()

	settings, err := domain.NewDatabaseSettings(
		"prod-1", "FREEPDB1", "db.example.com",
		domain.Port(1521), "admin", "secret",
	)
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	if err := deps.DBSettingsRepo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	session := connectClientServer(t, deps)
	res := callTool(t, session, "connect_database", map[string]any{
		"id": settings.StorageKey(),
	})
	if res.IsError {
		t.Fatalf("expected success, got IsError: %+v", res.Content)
	}
	if !fake.connected {
		t.Fatal("expected adapter.Connect to be invoked")
	}
	if !fake.closed {
		t.Fatal("expected adapter.Close to be invoked")
	}

	tc := res.Content[0].(*mcp.TextContent)
	var out connectDatabaseOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true, got %+v", out)
	}
	if out.ActiveDatabase != settings.StorageKey() {
		t.Fatalf("expected active=%q, got %q", settings.StorageKey(), out.ActiveDatabase)
	}
	def, err := deps.DBSettingsRepo.GetDefault(context.Background())
	if err != nil || def == nil || def.StorageKey() != settings.StorageKey() {
		t.Fatalf("expected default database=%q, got err=%v def=%v", settings.StorageKey(), err, def)
	}
}

func TestAddDatabase_NotifiesUIOnlyWhenNoDefault(t *testing.T) {
	cases := []struct {
		name        string
		id          string
		seedDefault bool
		wantNotify  bool
	}{
		{name: "first", id: "first-db", seedDefault: false, wantNotify: true},
		{name: "second", id: "second-db", seedDefault: true, wantNotify: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, cleanup := testDeps(t)
			defer cleanup()

			if tc.seedDefault {
				first, err := domain.NewDatabaseSettings(
					"already", "FREEPDB1", "db.example.com",
					domain.Port(1521), "admin", "secret",
				)
				if err != nil {
					t.Fatalf("NewDatabaseSettings: %v", err)
				}
				if err := deps.DBSettingsRepo.Save(context.Background(), *first); err != nil {
					t.Fatalf("Save first: %v", err)
				}
				if _, err := deps.DBSettingsRepo.SetDefault(context.Background(), *first); err != nil {
					t.Fatalf("SetDefault: %v", err)
				}
			}

			var got string
			deps.OnDatabaseConnected = func(id string) { got = id }

			session := connectClientServer(t, deps)
			res := callTool(t, session, "add_database", map[string]any{
				"id":       tc.id,
				"host":     "db.example.com",
				"port":     1521,
				"service":  "FREEPDB1",
				"username": "admin",
				"password": "secret",
			})
			if res.IsError {
				dump := ""
				if len(res.Content) > 0 {
					if text, ok := res.Content[0].(*mcp.TextContent); ok {
						dump = text.Text
					}
				}
				t.Fatalf("expected success, got IsError: %s", dump)
			}
			notified := got != ""
			if notified != tc.wantNotify {
				t.Fatalf("notified=%v, want %v (got %q)", notified, tc.wantNotify, got)
			}
			if !tc.wantNotify {
				return
			}
			def, err := deps.DBSettingsRepo.GetDefault(context.Background())
			if err != nil {
				t.Fatalf("GetDefault: %v", err)
			}
			if def.StorageKey() != got {
				t.Fatalf("default %q != notified %q", def.StorageKey(), got)
			}
		})
	}
}

func TestConnectDatabase_NotifiesUI(t *testing.T) {
	deps, _, cleanup := depsWithFakeDB(t)
	defer cleanup()

	settings, err := domain.NewDatabaseSettings(
		"notify-1", "FREEPDB1", "db.example.com",
		domain.Port(1521), "admin", "secret",
	)
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	if err := deps.DBSettingsRepo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var got string
	deps.OnDatabaseConnected = func(id string) { got = id }

	session := connectClientServer(t, deps)
	res := callTool(t, session, "connect_database", map[string]any{"id": settings.StorageKey()})
	if res.IsError {
		t.Fatalf("expected success, got IsError: %+v", res.Content)
	}
	if got != settings.StorageKey() {
		t.Fatalf("OnDatabaseConnected got %q, want %q", got, settings.StorageKey())
	}
}

func TestConnectDatabase_Unreachable(t *testing.T) {
	deps, fake, cleanup := depsWithFakeDB(t)
	defer cleanup()
	fake.connectErr = errors.New("oracle unreachable")

	settings, err := domain.NewDatabaseSettings(
		"prod-1", "FREEPDB1", "db.example.com",
		domain.Port(1521), "admin", "secret",
	)
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	if err := deps.DBSettingsRepo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	session := connectClientServer(t, deps)
	res := callTool(t, session, "connect_database", map[string]any{
		"id": settings.StorageKey(),
	})
	if !res.IsError {
		t.Fatalf("expected IsError=true on unreachable, got %+v", res)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var payload map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["code"] != "db_unreachable" {
		t.Fatalf("expected code=db_unreachable, got %v", payload["code"])
	}
	// The default database must NOT have changed.
	if _, err := deps.DBSettingsRepo.GetDefault(context.Background()); !errors.Is(err, domain.ErrDefaultSettingsNotFound) {
		t.Fatalf("expected no default database to be set, got err=%v", err)
	}
}

func TestConnectDatabase_UnknownID(t *testing.T) {
	deps, _, cleanup := depsWithFakeDB(t)
	defer cleanup()
	session := connectClientServer(t, deps)

	res := callTool(t, session, "connect_database", map[string]any{"id": "DBconfig:nope"})
	if !res.IsError {
		t.Fatalf("expected IsError=true for unknown id, got %+v", res)
	}
	tc := res.Content[0].(*mcp.TextContent)
	var payload map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["code"] != "not_found" {
		t.Fatalf("expected code=not_found, got %v", payload["code"])
	}
}

// ==========================================
// Helpers
// ==========================================

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
