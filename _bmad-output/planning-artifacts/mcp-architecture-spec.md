---
workflowType: 'mcp-architecture-addendum'
project_name: OmniInspect
user_name: Basuruk
date: '2026-07-18'
parent: _bmad-output/planning-artifacts/architecture.md
companion: _bmad-output/planning-artifacts/mcp-feasibility-study.md
---

# MCP Server — Architecture Addendum

## Scope

Add an MCP server to OmniInspect. Stdout transport only in v1. Single binary. No HTTP, no auth, no TUI changes.

## Locked Decisions

| Decision              | Value                                          |
|-----------------------|------------------------------------------------|
| Transport             | stdio only (v1)                                |
| Binary                | single `omniview` with `mcp` subcommand        |
| Trace cap             | 10 000, not configurable                       |
| HTTP transport        | deferred (v1.1+)                               |
| Auth                  | none — process-level OS auth                   |
| SDK                   | `github.com/modelcontextprotocol/go-sdk`       |

## Components

### New ports

- `internal/core/ports/trace_store.go` — `TraceStore` interface (Append, List, GetByID, Clear, Len). Shared by TUI and MCP.

### New adapters

- `internal/adapter/tracebuffer/` — in-memory ring buffer, cap 10 000, mutex-guarded. Implements `TraceStore`.
- `internal/adapter/mcpserver/` — MCP server.
  - `server.go` — `NewServer(deps) *Server`, registers tools, exposes `ServeStdio(ctx)`.
  - `tools_traces.go` — `list_traces`, `get_trace`, `clear_traces`.
  - `tools_databases.go` — `list_databases`, `add_database`.
  - `tools_status.go` — `get_status`.
  - `transport_stdio.go` — wires SDK stdio transport, asserts stdout cleanliness.

### New service

- `internal/service/connector/` — `Connector` tracks the active DB ID (read/written via BoltDB). `SetActive(ctx, id)` is idempotent.

## Wiring (composition root)

`cmd/omniview/main.go` gets a `mcp` subcommand path:

```
mcp subcommand
  → load BoltDB
  → load credential cipher
  → init TraceStore
  → init dbSettingsRepo
  → init Connector (loads active ID from BoltDB)
  → build mcpserver.Server(deps)
  → server.ServeStdio(ctx)  // blocks
  → defer close BoltDB
```

TUI path stays untouched. Tracer path gets **one** extra call: `traceStore.Append(msg)` inside `handleTracerMessage`, after the existing `eventChannel <- msg` send.

## Data Flow

```
Oracle AQ → BulkDequeue → TracerService.handleTracerMessage
                                ├─→ eventChannel → Bubble Tea Model (UI)
                                └─→ traceStore.Append → ringbuffer → MCP list_traces
```

`list_traces` reads from the ringbuffer. No re-dequeue. No Oracle roundtrip. No staleness vs UI.

## Tool Surface (v1)

| Tool             | Input                                              | Output                              | Port                          |
|------------------|----------------------------------------------------|-------------------------------------|-------------------------------|
| `list_databases` | —                                                  | `[{id, host, port, service, user, is_active}]` (no pwd) | `DatabaseSettingsRepository.GetAll` + active-id from `Connector` |
| `add_database`   | `{id, host, port, service, username, password, confirm_password_in_plaintext?}` | `{ok, id}` or `{code: "password_in_plaintext", message, confirm_required: true}` | `DatabaseSettingsRepository.Save` |
| `connect_database`| `{id}`                                            | `{ok}` or `{code: "db_unreachable", message}` | `Connector.SetActive` + `dbFactory.Connect` |
| `set_broadcast_mode` | `{mode: "global"|"subscriber"|"broadcast"}`   | `{ok, mode}`                        | `bolt.SetBroadcastMode`       |
| `list_traces`    | `{limit?, since_id?, level?, process_name?}`       | `{messages, total}`                 | `TraceStore.List`             |
| `get_trace`      | `{message_id}`                                     | trace or not-found                  | `TraceStore.GetByID`          |
| `clear_traces`   | —                                                  | `{ok}`                              | `TraceStore.Clear`            |
| `get_status`     | —                                                  | `{version, active_db, broadcast_mode, queue_depth, uptime_s}` | `app.Version`, `Connector`, `bolt`, `db.CheckQueueDepth` |

**Excluded:** `delete_database` — TUI-only (per user decision).

### `add_database` password flow

Two-step to make the security risk explicit:

1. Tool called without `confirm_password_in_plaintext=true` → returns `{code: "password_in_plaintext", message: "Sending passwords over MCP exposes them in client logs and process listings. Confirm to proceed, or use the TUI onboarding form.", confirm_required: true}`.
2. Tool called with `confirm_password_in_plaintext=true` → proceeds, saves, returns `{ok, id}`.

MCP tool description states this risk verbatim.

Errors returned as MCP `isError: true` with structured payload `{code, message}`. Domain sentinel errors mapped to stable codes (`not_found`, `invalid_input`, `db_locked`, `no_active_db`).

## Lifecycle

- Server runs for full process lifetime.
- No TUI in `mcp` subcommand mode — pure server.
- Shutdown: SIGINT/SIGTERM → cancel ctx → `ServeStdio` returns → close BoltDB.
- Logger writes to `omniview.log` (existing behavior). Stdout untouched by app code. **CI guard:** integration test asserts stdout contains only valid MCP JSON frames.

## Concurrency

- Ringbuffer: `sync.RWMutex` (read-heavy).
- Connector: `sync.RWMutex` (active-id reads dominate).
- Tool handlers: stateless, short-lived, ctx-bounded (2s default timeout, overridable per-tool).

## Security

- Passwords never leave process. `add_database` accepts a password, encrypts via existing `domain.SetCredentialCipher`, persists to BoltDB, never returns it.
- `list_databases` DTO omits password field.
- Stdout discipline enforced by integration test.
- Doc: log file may contain message payloads — existing behavior, no change.

## Failure Modes

| Failure                          | Behavior                                  |
|----------------------------------|-------------------------------------------|
| BoltDB open fails                | exit non-zero, stderr message             |
| Tool before listener starts      | `list_traces` returns `[]`, `get_status.queue_depth` = 0 |
| Tool needs DB, no active DB      | `{code: "no_active_db", message: ...}`    |
| Oracle unreachable               | `add_database` saves without verifying (matches UI); `connect_database` returns `db_unreachable` (v1.1 tool) |
| Ringbuffer full                  | oldest evicted (FIFO)                      |
| MCP client misbehaves            | SDK handles framing errors; server logs and continues |

## Files Changed

**New:**
- `internal/core/ports/trace_store.go`
- `internal/adapter/tracebuffer/ringbuffer.go` + `_test.go`
- `internal/adapter/mcpserver/server.go`
- `internal/adapter/mcpserver/tools_traces.go`
- `internal/adapter/mcpserver/tools_databases.go`
- `internal/adapter/mcpserver/tools_status.go`
- `internal/adapter/mcpserver/transport_stdio.go`
- `internal/adapter/mcpserver/integration_test.go`
- `internal/service/connector/connector.go` + `_test.go`
- `cmd/omniview/mcp.go` (subcommand entry)

**Modified (minimal):**
- `cmd/omniview/main.go` — route `mcp` subcommand
- `internal/service/tracer/tracer_service.go` — one extra `traceStore.Append` call
- `go.mod` — add MCP SDK

## Non-Goals (v1)

HTTP transport, auth, webhook tools, subscriber tools, live-tail resource subscriptions, cross-platform daemon mode, multi-client fan-out.

## Open Questions (carry forward)

- Resource subscriptions for live-tail (`tail://traces`) — defer to v1.1
- Whether `add_database` should verify connectivity before save — keep matching UI (save-then-connect)
