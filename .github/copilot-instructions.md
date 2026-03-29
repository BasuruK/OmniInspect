# Project Guidelines

## Code Style
- Follow the existing Go patterns: constructor functions named `New...`, dependency injection through constructor args or option structs, and pointer receivers for services and adapters.
- Keep domain entities and value objects in `internal/core/domain` validated and infrastructure-agnostic. Expose state through methods instead of public struct fields when the domain already follows that pattern.
- Prefer sentinel domain errors and wrap returned errors with context instead of introducing ad hoc error strings.
- Keep comments short and structural. Match the existing section-divider comment style when touching files that already use it.
- Bubble Tea code in this repo uses the v2 `charm.land/...` imports already declared in `go.mod`; do not mix in the older `github.com/charmbracelet/...` module paths.

## Architecture
- Preserve the hexagonal structure: `internal/core/domain` and `internal/core/ports` define the core model and contracts, `internal/service` coordinates business logic, and `internal/adapter` contains Oracle, BoltDB, config, and UI implementations.
- `cmd/omniview/main.go` is the composition root. Wire new dependencies there instead of letting adapters or services construct each other internally.
- Keep TUI behavior inside `internal/adapter/ui` using Bubble Tea's model/update/view flow. Avoid adding blocking stdin prompts or direct terminal control paths inside the TUI layer.
- When changing Oracle dequeueing or CGO code, keep the Go and C sides aligned across `internal/adapter/storage/oracle/oracle_adapter.go`, `dequeue_ops.c`, and `dequeue_ops.h`.

## Build And Test
- Use `make build`, `make run`, `make test`, `make fmt`, and `make lint` from the repo root.
- Do not use `go run cmd/omniview/main.go`; the Makefile sets the CGO flags and Oracle client linker paths required for this project.
- Oracle development depends on Instant Client and ODPI-C artifacts. On macOS ARM64, the default Instant Client path in the Makefile is `/opt/oracle/instantclient_23_7`.
- For setup and packaging details, use `scripts/setup_odpi.py`, the `Makefile`, and `README.md` rather than duplicating release steps in code changes.

## Conventions
- Repository interfaces live under `internal/core/ports`; concrete storage implementations belong under `internal/adapter/storage`.
- First-run and persisted connection state live in `omniview.bolt`, with initial config flow coordinated through the config loader and `settings.json`.
- When working on the TUI, follow DESIGN.md guidelines and existing patterns in `internal/adapter/ui`. Avoid mixing TUI logic into services or domain code.
- Prefer representative project patterns from `cmd/omniview/main.go`, `internal/core/domain/subscriber.go`, `internal/core/ports/repository.go`, and `internal/adapter/ui/model.go` when adding new code.