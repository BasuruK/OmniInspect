# OmniInspect - Source Tree Analysis

**Date:** 2026-04-15

## Overview

OmniInspect is organized as a single Go application with a clear split between executable bootstrapping, internal layers, runtime assets, and documentation. The most important structural pattern is the separation of domain and ports from adapters and services, plus a dedicated UI adapter for the Bubble Tea terminal interface.

## Complete Directory Structure

```text
OmniInspect/
├── cmd/
│   └── omniview/                 # Main executable entry point
├── internal/
│   ├── adapter/
│   │   ├── config/              # Settings loading helpers
│   │   ├── storage/
│   │   │   ├── boltdb/          # Local persistence adapter
│   │   │   └── oracle/          # Oracle AQ and SQL deployment adapter
│   │   └── ui/                  # Bubble Tea screens, messages, layouts, styles
│   ├── app/                     # Application version and app-level helpers
│   ├── core/
│   │   ├── domain/              # Entities, value objects, sentinel errors
│   │   └── ports/               # Repository contracts
│   ├── service/                 # Business logic coordinators
│   └── updater/                 # Update orchestration helpers
├── assets/
│   ├── ins/                     # Oracle initialization scripts
│   ├── sql/                     # PL/SQL package and related SQL assets
│   └── embed_files.go           # Embedded asset wiring
├── docs/                        # Existing design and implementation notes
├── resources/                   # Images and icons used by the application
├── scripts/                     # Setup, debug, and operational helper scripts
├── third_party/
│   └── odpi/                    # ODPI-C library and headers
├── AGENTS.md                    # Engineering guidance for contributors and agents
├── DESIGN.md                    # Target-state TUI design authority
├── README.md                    # Product, setup, and architecture overview
├── Makefile                     # Canonical build and run workflow
├── go.mod                       # Go module and dependency manifest
├── settings.json                # Local/editor support settings
└── omniview.bolt               # Local runtime state database
```

## Critical Directories

### `cmd/omniview`

**Purpose:** Composition root and executable bootstrap.
**Contains:** `main.go`, runtime wiring, initial infrastructure creation, TUI startup.
**Entry Points:** `cmd/omniview/main.go`

### `internal/adapter/ui`

**Purpose:** Terminal UI layer built on Bubble Tea v2 and Lip Gloss v2.
**Contains:** root model, typed messages, per-screen rendering, forms, styles, animations, overlays.
**Entry Points:** `model.go`, `messages.go`, `welcome.go`, `loading.go`, `main_screen.go`, `onboarding.go`, `database_settings.go`

### `internal/adapter/storage/oracle`

**Purpose:** Oracle AQ integration and SQL deployment adapter.
**Contains:** Oracle adapter, queue and subscription logic, SQL parsing, C dequeue bindings.
**Integration:** Connects runtime services to Oracle AQ and deployed SQL assets.

### `internal/adapter/storage/boltdb`

**Purpose:** Local persistence adapter for runtime config and metadata.
**Contains:** Bolt adapter, database settings repository, permissions repository, subscriber repository.
**Integration:** Supports first-run flows, persisted connection state, migrations, and webhook config.

### `internal/core/domain`

**Purpose:** Business entities and validated value objects.
**Contains:** database settings, subscriber, queue message, permissions, webhook, sentinel errors.

### `internal/core/ports`

**Purpose:** Infrastructure-agnostic repository contracts.
**Contains:** interfaces for subscribers, settings, permissions, Oracle database access, and config persistence.

### `internal/service`

**Purpose:** Business workflow coordination.
**Contains:** tracer handling, permissions checks, subscriber management, webhook dispatch, updater behavior.

### `assets/sql`

**Purpose:** Runtime Oracle-side contract.
**Contains:** `Omni_Tracer.sql`, permission checks, and deployable SQL resources.

### `assets/ins`

**Purpose:** Oracle initialization support.
**Contains:** initialization script assets used during setup or deployment flows.

### `docs`

**Purpose:** Existing architectural and implementation knowledge base.
**Contains:** multi-subscriber planning, blocking dequeue design, self-updater notes, subscriber isolation analysis.

### `scripts`

**Purpose:** Operational and developer utilities.
**Contains:** ODPI setup helper, SQL cleanup, listener restart script, SQL notebook for debugging.

### `resources`

**Purpose:** User-facing binary assets.
**Contains:** icons and branding images used for packaging and display.

### `third_party/odpi`

**Purpose:** Vendored Oracle database integration dependency.
**Contains:** ODPI-C includes, library artifacts, and supporting sources.

## Entry Points

- **Primary entry point:** `cmd/omniview/main.go`
- **Database package runtime contract:** `assets/sql/Omni_Tracer.sql`
- **UI design authority:** `DESIGN.md`
- **Operational workflow authority:** `Makefile`

## File Organization Patterns

- Hexagonal layering under `internal/`
- UI split by screen and helper responsibilities
- Storage adapters grouped by backend technology
- Assets separated from runtime Go code
- Long-lived architecture decisions captured in `docs/`

## Key File Types

### Go source

- **Pattern:** `*.go`
- **Purpose:** Application, domain, services, adapters, tests
- **Examples:** `internal/adapter/ui/model.go`, `internal/service/tracer/tracer_service.go`

### SQL assets

- **Pattern:** `assets/sql/*.sql`
- **Purpose:** Oracle package deployment and permission-related SQL
- **Examples:** `assets/sql/Omni_Tracer.sql`, `assets/sql/Permission_Checks.sql`

### C bindings

- **Pattern:** `*.c`, `*.h`
- **Purpose:** Dequeue and Oracle interop support for CGO-backed storage code
- **Examples:** `internal/adapter/storage/oracle/dequeue_ops.c`, `internal/adapter/storage/oracle/dequeue_ops.h`

### Markdown docs

- **Pattern:** `*.md`
- **Purpose:** Product, design, contribution, and architecture documentation
- **Examples:** `README.md`, `DESIGN.md`, `docs/SELF_UPDATER_IMPLEMENTATION.md`

## Asset Locations

- **Images and branding:** `resources/`
- **Deployable SQL and init assets:** `assets/sql`, `assets/ins`
- **Vendored Oracle dependency:** `third_party/odpi/`

## Configuration Files

- `go.mod`: module definition and dependency versions
- `Makefile`: canonical build, run, test, and packaging workflow
- `settings.json`: local project settings
- `.github/copilot-instructions.md`: project-specific AI coding guidance

## Notes for Development

The most sensitive integration boundaries are the Oracle adapter, the BoltDB persistence layer, and the Bubble Tea root model. Cross-cutting changes should preserve the current separation between composition root, domain contracts, services, adapters, and deployed SQL assets.

---

_Generated using BMAD Method `document-project` workflow_
