# OmniInspect - Project Overview

**Date:** 2026-04-15
**Type:** Single-part Go CLI application with a state-driven terminal UI
**Architecture:** Hexagonal ports-and-adapters with Bubble Tea UI orchestration

## Executive Summary

OmniInspect, branded in the UI as OmniView, is a Go-based terminal application for receiving and displaying Oracle Advanced Queuing trace messages in real time. It combines a Bubble Tea v2 and Lip Gloss v2 text UI with Oracle AQ integration through ODPI-C and PL/SQL assets. The application also persists local configuration in BoltDB, supports webhook forwarding for selected trace messages, and includes self-update and onboarding flows.

## Project Classification

- **Repository Type:** Monolith
- **Project Type:** Best matched as `cli` from the BMAD catalog, with strong desktop/TUI characteristics
- **Primary Languages:** Go, PL/SQL, C
- **Architecture Pattern:** Hexagonal core plus adapter-driven infrastructure and a state-driven terminal UI

## Technology Stack Summary

Version note: dependency versions below are intentionally documented at major.minor granularity to reduce drift in committed docs. See `go.mod` for exact patch and toolchain versions.

| Category | Technology | Version / Notes (major.minor) |
| --- | --- | --- |
| Application language | Go | 1.24 with Go 1.24 toolchain |
| Terminal UI | Bubble Tea | charm.land/bubbletea/v2 2.0 |
| Styling | Lip Gloss | charm.land/lipgloss/v2 2.0 |
| UI components | Bubbles | charm.land/bubbles/v2 2.0 |
| Local persistence | BoltDB | go.etcd.io/bbolt 1.4 |
| Database integration | Oracle AQ | via ODPI-C, CGO, and PL/SQL assets |
| Supporting assets | PL/SQL, C | queue package and dequeue bindings |

## Key Features

- First-run onboarding and persisted database configuration.
- Oracle AQ-based trace listener with subscriber registration and blocking dequeue support.
- Bubble Tea v2 trace console with multiple screens and deterministic state transitions.
- Optional webhook forwarding for messages flagged by `OMNI_TRACER_API.Trace_Message_To_Webhook`.
- Local BoltDB persistence for configuration, permissions, and related runtime state.
- Self-update support and release/platform-specific build packaging.

## Architecture Highlights

- `cmd/omniview/main.go` is the composition root and starts the TUI only after core infrastructure is wired.
- `internal/core` holds domain entities, value objects, sentinel errors, and repository contracts.
- `internal/service` coordinates tracer processing, permissions, subscriber lifecycle, updater behavior, and webhook dispatch.
- `internal/adapter/storage/oracle` encapsulates Oracle-specific AQ, dequeue, and SQL deployment behavior.
- `internal/adapter/storage/boltdb` encapsulates local persisted application state and migration behavior.
- `internal/adapter/ui` contains the Bubble Tea screens, typed messages, layout helpers, and style tokens.

## Development Overview

### Prerequisites

- Go 1.24+
- Oracle Instant Client 23.7+
- A working C toolchain for CGO
- `make` for the supported build and run flow

### Getting Started

Use the Makefile-driven workflow from the repository root. The Makefile supplies required CGO flags and Oracle client linker paths, so direct `go run cmd/omniview/main.go` is intentionally unsupported.

### Key Commands

- **Install dependencies:** `make install`
- **Build:** `make build`
- **Run:** `make run`
- **Test:** `make test`
- **Format:** `make fmt`
- **Lint:** `make lint`

## Repository Structure

The repository is organized as a single executable plus clearly separated internal layers. The runtime data path is Oracle AQ → storage/service coordination → Bubble Tea messages → TUI rendering, with BoltDB used for local persistence and onboarding state.

## Documentation Map

For detailed information, see:

- [index.md](./index.md) - Master documentation index
- [architecture.md](./architecture.md) - Detailed technical architecture
- [source-tree-analysis.md](./source-tree-analysis.md) - Annotated directory structure
- [development-guide.md](./development-guide.md) - Development workflow and environment setup
- [component-inventory.md](./component-inventory.md) - UI screen and component catalog

---

_Generated using BMAD Method `document-project` workflow_
