---
workflowType: 'mcp-epics-and-stories'
project_name: OmniInspect
user_name: Basuruk
date: '2026-07-18'
parent: _bmad-output/planning-artifacts/epics.md
companion: _bmad-output/planning-artifacts/mcp-architecture-spec.md
---

# MCP Server — Epic & Story Breakdown

## Epic M1: Shared Trace Buffer

**Goal:** Lift trace storage behind a port so TUI and MCP share one source of truth.

### Story M1.1 — Define `TraceStore` port

Create `internal/core/ports/trace_store.go`.

Interface:

```go
type TraceStore interface {
    Append(ctx context.Context, msg *domain.QueueMessage) error
    List(ctx context.Context, limit int, sinceID string) ([]*domain.QueueMessage, error)
    GetByID(ctx context.Context, id string) (*domain.QueueMessage, error)
    Clear(ctx context.Context) error
    Len(ctx context.Context) int
}
```

**AC:**
- File compiles, no behavior change yet.
- Doc comments per method (one line + thread-safety note).

### Story M1.2 — Implement ringbuffer

Create `internal/adapter/tracebuffer/ringbuffer.go`.

- Bounded slice, cap 10 000, FIFO eviction.
- `sync.RWMutex`.
- Constructor: `New(capacity int) *RingBuffer`.
- All methods ctx-aware (ctx used only for cancellation, not I/O).

**Tests:** concurrent append/read/clear, eviction at cap, GetByID hit + miss, sinceID filter.

### Story M1.3 — Wire tracer → buffer

Modify `internal/service/tracer/tracer_service.go` constructor: accept `traceStore ports.TraceStore`. One new call in `handleTracerMessage`:

```go
if ts.traceStore != nil {
    _ = ts.traceStore.Append(ctx, msg)
}
```

Add `traceStore` to `TracerService` struct + constructor param. Update `cmd/omniview/main.go` and `internal/adapter/ui/model.go::initializeServices` to pass the buffer (new shared instance).

**AC:**
- TUI shows messages exactly as before.
- Concurrent reader sees same messages as UI.
- `go test -race` clean.

---

## Epic M2: Connector Service

**Goal:** Single source of truth for "active database" outside the UI.

### Story M2.1 — `Connector` service

Create `internal/service/connector/connector.go`.

```go
type Connector struct {
    bolt ports.ConfigRepository
    mu   sync.RWMutex
    id   string
}

func New(b ports.ConfigRepository) *Connector
func (c *Connector) Active() string
func (c *Connector) SetActive(ctx context.Context, id string) error
```

`Active()` reads in-memory cached id. `SetActive` persists to BoltDB (`SaveDatabaseConfig` with a tiny new marker key, or reuse `GetDefaultDatabaseConfig` swap — choose in impl).

**Tests:** in-memory fake bolt, concurrent SetActive race-free.

---

## Epic M3: MCP Server Foundation

**Goal:** Single binary with `mcp` subcommand, stdio transport, 6 tools, integration test.

### Story M3.1 — Add SDK dependency

`go get github.com/modelcontextprotocol/go-sdk`. Verify `go build ./...` still passes.

### Story M3.2 — `mcp` subcommand skeleton

Create `cmd/omniview/mcp.go`. Routing in `main.go`:

```go
if len(os.Args) > 1 && os.Args[1] == "mcp" {
    return runMCP(omniApp)
}
```

`runMCP` flow:
1. Init logger → `omniview.log`.
2. Init BoltDB + credential cipher (same code path as TUI, extracted).
3. Init `TraceStore` + `Connector`.
4. Init `dbSettingsRepo`.
5. Build server.
6. `server.ServeStdio(ctx)`.
7. On return: defer-close BoltDB.

**AC:** `./omniview mcp --help` prints usage. Server handshake succeeds with empty tool list (no tools yet).

### Story M3.3 — Server core + transport

Create `internal/adapter/mcpserver/server.go` + `transport_stdio.go`.

- `Server` struct holds deps + `*mcp.Server`.
- `NewServer(deps Deps) *Server` — registers empty tool set for now.
- `ServeStdio(ctx context.Context) error` — uses SDK's stdio transport.

**Tests:** SDK in-memory transport, assert handshake + `tools/list` returns empty list.

### Story M3.4 — `get_status` + `set_broadcast_mode` tools

`internal/adapter/mcpserver/tools_status.go`.

- `get_status` — Input: none. Output:
```json
{
  "version": "1.0.0",
  "active_database_id": "prod-1",
  "broadcast_mode": "global",
  "queue_depth": 0,
  "uptime_seconds": 1234
}
```
Sources: `app.Version`, `Connector.Active()`, `bolt.GetBroadcastMode()`, `db.CheckQueueDepth` (nil-safe if `db == nil`).

- `set_broadcast_mode` — Input: `{mode: "global"|"subscriber"|"broadcast"}`. Validates via `domain.NewBroadcastMode`. Persists via `bolt.SetBroadcastMode`. Updates in-memory cache (live effect for new messages). Returns `{ok, mode}`.

**Tests:** unit test with fakes (valid + invalid mode), integration test via in-memory transport.

### Story M3.5 — `list_databases` + `add_database` + `connect_database`

`tools_databases.go`.

- `list_databases` — calls `dbSettingsRepo.GetAll`. DTO strips `Password()`. Adds `is_active` from `Connector.Active()`.
- `add_database` — accepts `{id, host, port, service, username, password, confirm_password_in_plaintext?}`. Two-step:
  - Without confirm flag → returns `{code: "password_in_plaintext", message: "Sending passwords over MCP exposes them in client logs and process listings. Confirm to proceed, or use the TUI onboarding form.", confirm_required: true}`. Tool description states the same.
  - With `confirm_password_in_plaintext=true` → builds `domain.DatabaseSettings` via existing constructor (validates), persists via `dbSettingsRepo.Save`. Returns `{ok, id}`.
- `connect_database` — accepts `{id}`. Looks up settings, builds adapter via `dbFactory`, calls `Connect` with 5s timeout. On success, calls `Connector.SetActive(ctx, id)` and persists. On failure, returns `{code: "db_unreachable", message}`. Does not unregister existing active DB unless new connect succeeds.

**Tests:** unit (validation, password confirm gate, connect timeout), integration (round-trip via MCP).

### Story M3.6 — Trace tools

`tools_traces.go`.

- `list_traces` — `{limit?=50, since_id?=null, level?=null, process_name?=null}` → `{messages: [...], total: int}`. Filters applied in-memory on the ringbuffer snapshot.
- `get_trace` — `{message_id}` → trace or `{code: "not_found"}`.
- `clear_traces` — calls `TraceStore.Clear`. Returns `{ok}`.

**Tests:** unit (filter combos, paging), integration (full MCP call).

### Story M3.7 — Stdout discipline guard

Add `internal/adapter/mcpserver/stdout_guard_test.go`. Test pattern:

1. Start server via SDK stdio transport on a pipe.
2. Trigger one tool call.
3. Assert: every stdout byte is either empty or valid MCP JSON framing (newline-delimited).

If any logger writes slip through, this test fails. Fix by ensuring logger routes to file in `mcp` subcommand (already true — `omniview.log`).

### Story M3.8 — README + Claude Desktop config snippet

Add to README:

```json
{
  "mcpServers": {
    "omniview": {
      "command": "/path/to/omniview",
      "args": ["mcp"]
    }
  }
}
```

Plus: registration with Claude Code (`claude mcp add omniview -- /path/to/omniview mcp`).

---

## Epic Order & Dependencies

```
M1.1 → M1.2 → M1.3
              ↓
        M2.1 (independent, parallelizable)
              ↓
M3.1 → M3.2 → M3.3 → {M3.4, M3.5, M3.6} (parallel) → M3.7 → M3.8
```

## Estimates

| Epic | Stories | Days |
|------|---------|------|
| M1   | 3       | 2–3  |
| M2   | 1       | 1    |
| M3   | 8       | 5–6  |
| **Total v1** | **12** | **~9 days** |

## Test Strategy

- Unit: per-package, no SDK needed.
- Integration: SDK in-memory transport for full tool round-trips.
- Race: `go test -race ./internal/adapter/mcpserver/... ./internal/adapter/tracebuffer/... ./internal/service/tracer/...`.
- Manual: register with Claude Code, ask "list databases" + "show last 5 traces".

## Out of Scope (v1)

`delete_database` (TUI-only). HTTP transport, auth, subscriber/webhook tools, live-tail resources, multi-client fan-out, cross-platform service install.
