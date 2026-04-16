# OmniInspect - Development Guide

**Date:** 2026-04-15

## Prerequisites

- Go 1.24+
- Oracle Instant Client 23.7+
- A recent C toolchain for CGO
- `make`
- Access to an Oracle database if you need full runtime validation

### Platform Notes

#### macOS ARM64

- Default Oracle Instant Client path in the Makefile: `/opt/oracle/instantclient_23_7`

#### Windows

- Default Oracle Instant Client path in the Makefile: `C:\oracle_inst\instantclient_23_7`

## Repository Setup

```bash
make install
```

This downloads Go dependencies and tidies the module.

## Build Workflow

Use the Makefile from the repository root.

```bash
make build
```

For a versioned build:

```bash
make build VERSION=v1.0.0
```

### Important Constraint

Do not use `go run cmd/omniview/main.go`. The Makefile carries required CGO flags and Oracle linker path setup.

## Run Locally

```bash
make run
```

This builds and starts the application using the supported runtime configuration.

## Test Workflow

Run the full suite:

```bash
make test
```

Narrow package-level iteration is also common during development:

```bash
go test -v ./internal/core/domain
go test -v ./internal/adapter/ui
go test -v ./internal/service/tracer
```

## Formatting and Linting

```bash
make fmt
make lint
```

## First-Run and Configuration Notes

- The application uses `omniview.bolt` for local persisted state.
- First run routes through onboarding when no usable configuration exists.
- Database settings, default selection, and webhook configuration must remain compatible with persisted state.

## Oracle and Asset Workflow

- SQL assets live under `assets/sql`
- Initialization assets live under `assets/ins`
- ODPI-C support assets live under `third_party/odpi`
- Setup helper: `scripts/setup_odpi.py`

When changing dequeue behavior, keep these files aligned:

- `internal/adapter/storage/oracle/oracle_adapter.go`
- `internal/adapter/storage/oracle/dequeue_ops.c`
- `internal/adapter/storage/oracle/dequeue_ops.h`

## Common Tasks

### Clean artifacts

```bash
make clean
```

### Build ODPI-C only

```bash
make odpi
```

### Debug Oracle-related build behavior

Review the Makefile targets and environment exports rather than introducing alternate local scripts.

## Recommended Development Flow

1. Install dependencies and verify Oracle prerequisites.
2. Build with `make build`.
3. Run with `make run`.
4. Add or update focused tests for changed packages.
5. Run `make test`, `make fmt`, and `make lint` before finalizing changes.

---

_Generated using BMAD Method `document-project` workflow_
