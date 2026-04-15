# OmniInspect - Architecture

**Date:** 2026-04-15
**Type:** Single-part monolith
**Primary Pattern:** Hexagonal ports-and-adapters with event-driven terminal UI

## Executive Summary

OmniInspect is a Go terminal application that listens to Oracle AQ messages, renders them in a structured TUI, and optionally forwards selected messages to a configured webhook. The repository separates domain and ports from storage and UI adapters, with a composition root in `cmd/omniview/main.go` that wires dependencies and starts the Bubble Tea program.

## High-Level Layers

### Composition Root

- `cmd/omniview/main.go`
- Initializes the application object, BoltDB adapter, updater service, and the TUI model.
- Defers Oracle adapter creation until runtime configuration is available.

### Core Domain

- `internal/core/domain`
- Contains validated entities and value objects such as subscribers, queue messages, database settings, permissions, and webhook configuration.
- Uses sentinel errors to represent business-level failure conditions.

### Core Ports

- `internal/core/ports`
- Defines repository contracts for subscribers, database settings, permissions, Oracle database access, and local config persistence.

### Services

- `internal/service/tracer`
- `internal/service/permissions`
- `internal/service/subscribers`
- `internal/service/webhook`
- `internal/service/updater`
- Coordinate business workflows without owning infrastructure implementation details.

### Adapters

- `internal/adapter/storage/oracle` implements database and AQ access using Go plus C bindings.
- `internal/adapter/storage/boltdb` implements local persistence for config and metadata.
- `internal/adapter/ui` implements the Bubble Tea screens, messages, and layout helpers.
- `internal/adapter/config` handles settings loading concerns.

## Runtime Flow

### Startup Flow

1. The binary starts in `cmd/omniview/main.go`.
2. BoltDB is initialized early because local configuration is needed before runtime database work.
3. The Bubble Tea model starts with the welcome screen.
4. The model checks persisted configuration and either routes the user to onboarding or into the loading flow.
5. Service initialization is deferred until database settings are available.
6. The loading flow connects to Oracle, checks or deploys required SQL assets, registers a subscriber, and starts the event listener.

### Trace Message Flow

1. PL/SQL package `OMNI_TRACER_API` enqueues trace payloads into an Oracle AQ sharded queue.
2. Oracle adapter code dequeues payloads using blocking or batched consumer logic.
3. `TracerService` converts raw payloads into domain `QueueMessage` values.
4. Messages are delivered over a channel into the Bubble Tea model.
5. The UI wraps them into typed `tea.Msg` values and renders them into the main viewport.
6. If `send_to_webhook` is set, the tracer path also queues bounded asynchronous webhook dispatch.

### Webhook Flow

1. Oracle-side `Trace_Message_To_Webhook` adds a marker field in the queue payload.
2. Domain unmarshaling maps `"TRUE"` to the boolean `SendToWebhook` flag.
3. The tracer service looks up persisted webhook configuration from BoltDB.
4. The global bounded dispatcher sends webhook deliveries asynchronously.
5. The webhook service applies SSRF-oriented host and IP restrictions before sending requests.

## Data and Persistence Architecture

### Local State

BoltDB buckets persist:

- Database configurations and default selection
- First-run cycle status
- Webhook configuration
- Subscriber and permissions-related state
- Legacy config migration support for older key formats

### Database Assets

Oracle-side assets under `assets/sql` and `assets/ins` define:

- Queue setup
- PL/SQL package specification and body
- Permission checks
- Initialization and deployment resources

## UI Architecture

The UI uses a single root Bubble Tea model with screen-specific substates:

- Welcome screen
- Loading screen
- Main trace console
- Onboarding form
- Database settings overlay/panel
- Update state tracking

Key traits:

- Typed `tea.Msg` messages for async boundaries
- Pure `View()` rendering from model state
- Screen-specific `updateX` and `viewX` helpers
- Shared Lip Gloss style tokens in `internal/adapter/ui/styles`
- `DESIGN.md` as the target-state UI authority

## Source Tree Highlights

- `cmd/omniview` contains the executable entry point.
- `internal/adapter/ui` contains the highest density of UX logic.
- `internal/adapter/storage/oracle` contains the Oracle integration hotspot and CGO coupling.
- `internal/adapter/storage/boltdb` contains persistence and migration concerns.
- `internal/service/tracer` is the core runtime coordinator for live message handling.
- `assets/sql` and `assets/ins` are effectively part of the deployed runtime contract.

## Development Workflow

- Use `make build`, `make run`, `make test`, `make fmt`, and `make lint`.
- Do not use `go run cmd/omniview/main.go` because the Makefile carries required CGO and Oracle linker settings.
- Keep Oracle Go and C changes aligned across `oracle_adapter.go`, `dequeue_ops.c`, and `dequeue_ops.h`.
- Keep onboarding and persisted settings changes compatible with existing BoltDB state.

## Testing Strategy

- Unit and regression tests are colocated with packages.
- UI tests drive typed Bubble Tea messages and layout behavior directly.
- Repository tests use temporary BoltDB-backed adapters to verify persistence behavior.
- Service tests use narrow stubs for ports and focus on lifecycle and observable side effects.

## Architecture Risks and Constraints

- Oracle Instant Client and ODPI-C are hard environment dependencies.
- The Makefile and packaging flow are part of the executable contract, not optional tooling.
- Webhook forwarding must preserve existing validation and bounded queue behavior.
- Persisted config key formats already have migration logic and should not be broken.

---

_Generated using BMAD Method `document-project` workflow_
