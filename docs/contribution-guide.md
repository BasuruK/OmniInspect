# OmniInspect - Contribution Guide

**Date:** 2026-04-15

## Core Contribution Principles

- Preserve the hexagonal structure: domain and ports in `internal/core`, coordination in `internal/service`, adapters in `internal/adapter`.
- Keep dependency wiring in `cmd/omniview/main.go` rather than constructing services deep in the codebase.
- Follow existing Go patterns: `New...` constructors, pointer receivers for mutable services and adapters, and validated domain value objects.
- Use Bubble Tea v2 and Lip Gloss v2 imports from `charm.land/...`; do not mix in older module paths.

## Code Style Expectations

- Keep comments short and structural.
- Use the existing section-divider comment style when editing files that already use it.
- Prefer sentinel domain errors and wrapped errors instead of ad hoc error strings.
- Keep domain entities infrastructure-agnostic and expose state through methods where the domain already follows that pattern.

## UI and TUI Changes

- Treat `DESIGN.md` as the target-state UI authority.
- Keep TUI behavior inside `internal/adapter/ui` using the Bubble Tea model/update/view flow.
- Avoid blocking stdin prompts or direct terminal control paths outside the established UI patterns.

## Oracle and Storage Changes

- Keep Oracle Go and C code aligned across `oracle_adapter.go`, `dequeue_ops.c`, and `dequeue_ops.h`.
- Preserve BoltDB compatibility, including legacy migration behavior and persisted first-run state.
- Be careful with webhook and queue-message contracts because the Oracle payload shape is part of the runtime boundary.

## Build and Validation

Use the repo-root Makefile commands:

```bash
make build
make run
make test
make fmt
make lint
```

Do not use `go run cmd/omniview/main.go`.

## Testing Expectations

- Add focused tests close to the changed code.
- Prefer deterministic tests that exercise typed messages, layout behavior, persistence rules, and lifecycle behavior directly.
- Update regression coverage when fixing a bug.

## Reference Documents

- `README.md` for product and setup details
- `AGENTS.md` for contributor and agent guidance
- `.github/copilot-instructions.md` for project-specific coding constraints
- `DESIGN.md` for TUI design authority

---

_Generated using BMAD Method `document-project` workflow_
