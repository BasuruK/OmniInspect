package mcpserver

import (
	"context"
	"strings"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Handshake
// ==========================================

func TestServer_HandshakeCompletes(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	session := connectClientServer(t, deps)
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.InitializeResult() == nil {
		t.Fatal("expected InitializeResult, got nil")
	}
}

// ==========================================
// Tool registration
// ==========================================

func TestServer_RegistersExpectedTools(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	session := connectClientServer(t, deps)

	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string]bool{
		"get_status":         false,
		"set_broadcast_mode": false,
	}
	for _, tl := range result.Tools {
		if _, ok := want[tl.Name]; ok {
			want[tl.Name] = true
		}
	}
	missing := make([]string, 0)
	for name, found := range want {
		if !found {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		names := make([]string, 0, len(result.Tools))
		for _, tl := range result.Tools {
			names = append(names, tl.Name)
		}
		t.Fatalf("missing tools %v; registered: %s",
			missing, strings.Join(names, ","))
	}
}

// ==========================================
// Construction guards
// ==========================================

func TestNewServer_RequiresAllDeps(t *testing.T) {
	cases := []struct {
		name string
		mod  func(*Deps)
	}{
		{"App", func(d *Deps) { d.App = nil }},
		{"Bolt", func(d *Deps) { d.Bolt = nil }},
		{"TraceAppender", func(d *Deps) { d.TraceAppender = nil }},
		{"Connector", func(d *Deps) { d.Connector = nil }},
		{"DBSettingsRepo", func(d *Deps) { d.DBSettingsRepo = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, cleanup := testDeps(t)
			defer cleanup()
			tc.mod(&deps)

			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for missing %s", tc.name)
				}
			}()
			_ = NewServer(deps)
		})
	}
}
