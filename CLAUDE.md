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

```text
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

# context-mode — MANDATORY routing rules

You have context-mode MCP tools available. These rules are NOT optional — they protect your context window from flooding. A single unrouted command can dump 56 KB into context and waste the entire session.

## BLOCKED commands — do NOT attempt these

### curl / wget — BLOCKED
Any Bash command containing `curl` or `wget` is intercepted and replaced with an error message. Do NOT retry.
Instead use:
- `ctx_fetch_and_index(url, source)` to fetch and index web pages
- `ctx_execute(language: "javascript", code: "const r = await fetch(...)")` to run HTTP calls in sandbox

### Inline HTTP — BLOCKED
Any Bash command containing `fetch('http`, `requests.get(`, `requests.post(`, `http.get(`, or `http.request(` is intercepted and replaced with an error message. Do NOT retry with Bash.
Instead use:
- `ctx_execute(language, code)` to run HTTP calls in sandbox — only stdout enters context

### WebFetch — BLOCKED
WebFetch calls are denied entirely. The URL is extracted and you are told to use `ctx_fetch_and_index` instead.
Instead use:
- `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` to query the indexed content

## REDIRECTED tools — use sandbox equivalents

### Bash (>20 lines output)
Bash is ONLY for: `git`, `mkdir`, `rm`, `mv`, `cd`, `ls`, `npm install`, `pip install`, and other short-output commands.
For everything else, use:
- `ctx_batch_execute(commands, queries)` — run multiple commands + search in ONE call
- `ctx_execute(language: "shell", code: "...")` — run in sandbox, only stdout enters context

### Read (for analysis)
If you are reading a file to **Edit** it → Read is correct (Edit needs content in context).
If you are reading to **analyze, explore, or summarize** → use `ctx_execute_file(path, language, code)` instead. Only your printed summary enters context. The raw file content stays in the sandbox.

### Grep (large results)
Grep results can flood context. Use `ctx_execute(language: "shell", code: "grep ...")` to run searches in sandbox. Only your printed summary enters context.

## Tool selection hierarchy

1. **GATHER**: `ctx_batch_execute(commands, queries)` — Primary tool. Runs all commands, auto-indexes output, returns search results. ONE call replaces 30+ individual calls.
2. **FOLLOW-UP**: `ctx_search(queries: ["q1", "q2", ...])` — Query indexed content. Pass ALL questions as array in ONE call.
3. **PROCESSING**: `ctx_execute(language, code)` | `ctx_execute_file(path, language, code)` — Sandbox execution. Only stdout enters context.
4. **WEB**: `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` — Fetch, chunk, index, query. Raw HTML never enters context.
5. **INDEX**: `ctx_index(content, source)` — Store content in FTS5 knowledge base for later search.

## Subagent routing

When spawning subagents (Agent/Task tool), the routing block is automatically injected into their prompt. Bash-type subagents are upgraded to general-purpose so they have access to MCP tools. You do NOT need to manually instruct subagents about context-mode.

## Output constraints

- Keep responses under 500 words.
- Write artifacts (code, configs, PRDs) to FILES — never return them as inline text. Return only: file path + 1-line description.
- When indexing content, use descriptive source labels so others can `ctx_search(source: "label")` later.

## ctx commands

| Command | Action |
|---------|--------|
| `ctx stats` | Call the `ctx_stats` MCP tool and display the full output verbatim |
| `ctx doctor` | Call the `ctx_doctor` MCP tool, run the returned shell command, display as checklist |
| `ctx upgrade` | Call the `ctx_upgrade` MCP tool, run the returned shell command, display as checklist |
