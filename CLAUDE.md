# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OmniView (also known as OmniInspect) is a Message Passing TUI application that connects to Oracle Database and displays real-time trace messages via Oracle Advanced Queuing (AQ). It consists of:

1. **Go TUI Application** - Built with Bubble Tea v2, listens for trace messages
2. **PL/SQL Package (OMNI_TRACER_API)** - Deployed to Oracle to enqueue trace messages

## Common Commands

```bash
# Build the application (use THIS, not 'go run')
make build

# Build with version
make build VERSION=v1.0.0

# Build and run
make run

# Run tests
make test

# Clean build artifacts
make clean

# Build ODPI-C library only
make odpi
```

**Important**: Always use `make run` or `make build` instead of `go run cmd/omniview/main.go`. The Makefile sets required CGO environment variables for the Oracle ODPI-C driver.

## Architecture

Hexagonal (Ports and Adapters) architecture:

```
┌─────────────────────────────────────────┐
│  cmd/omniview/main.go (Bootstrap)       │
└─────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  Services (internal/service/)           │
│  - tracer/tracer_service.go             │
│  - permissions/permissions_service.go   │
│  - subscribers/subscriber_service.go    │
└─────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  Adapters (internal/adapter/)           │
│  - storage/oracle/ (ODPI-C driver)      │
│  - storage/boltdb/ (local persistence)  │
│  - config/ (settings loader)            │
│  - ui/ (Bubble Tea TUI)                 │
└─────────────────────────────────────────┘
```

## Key Dependencies

- **Go**: 1.24+
- **Oracle Instant Client**: 23.7+ (platform-specific)
- **ODPI-C**: Oracle Database driver for C (built via Makefile)
- **Bubble Tea v2**: TUI framework (charm.land/bubbletea/v2)
- **Lipgloss**: Terminal styling (charm.land/lipgloss/v2)
- **BoltDB**: Local storage (go.etcd.io/bbolt)

## Platform Notes

- **macOS ARM64**: Requires Oracle Instant Client for ARM64 at `/opt/oracle/instantclient_23_7`
- **Windows x64**: Requires Oracle Instant Client at `C:\oracle_inst\instantclient_23_7`
- **Linux x64**: Requires Oracle Instant Client at `/opt/oracle/instantclient_23_7`

The ODPI-C library setup is handled by `python scripts/setup_odpi.py`.

## Database Usage

End users call from Oracle:

```sql
OMNI_TRACER_API.Trace_Message(
    message_    => '{"order_id": 12345, "status": "completed"}',
    log_level_  => 'INFO'
);
```

## UI Layer

The TUI uses Bubble Tea v2 with Elm Architecture pattern:
- **Model**: [model.go](internal/adapter/ui/model.go) - State management
- **Update**: [messages.go](internal/adapter/ui/messages.go) - Message handlers
- **View**: [main_screen.go](internal/adapter/ui/main_screen.go), [welcome.go](internal/adapter/ui/welcome.go) - UI components
- **Styles**: [styles.go](internal/adapter/ui/styles/styles.go) - Lipgloss styling

## Local Storage

On first run, the app creates `omniview.bolt` (BoltDB) to store:
- Database connection settings
- Subscriber configuration
- Permissions

To switch databases: delete `omniview.bolt` and restart.