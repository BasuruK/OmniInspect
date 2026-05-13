# Application Logger Adapter - Deep Dive Documentation

**Generated:** 2026-05-13
**Scope:** `/internal/adapter/logger`
**Files Analyzed:** 1
**Lines of Code:** 131
**Workflow Mode:** Exhaustive Deep-Dive

## Overview

**Purpose:** Centralized logging adapter providing structured, file-backed logging for diagnostic event capture across the entire OmniInspect application.

**Key Responsibilities:**
- Provide package-level `Debug / Info / Warn / Error` functions requiring no injected dependencies
- Auto-capture source file/line/function from call site using `runtime.Callers`
- Route log output to file with `os.O_CREATE|os.O_APPEND|os.O_WRONLY` (append mode, never truncate)
- Maintain stderr fallback before `Init()` is called (ensures early startup errors are never lost)
- Return a cleanup function for deferred调用 in composition root

**Integration Points:**
- Composition root: `cmd/omniview/main.go` — must call `logger.Init()` once at startup
- Any package in codebase: calls `logger.Info(...)`, `logger.Debug(...)`, etc. directly
- No other packages import this — logger uses package-level functions, not injected interface

---

## Complete File Inventory

### `/Users/basuruk/Dev/OmniInspect/internal/adapter/logger/logger.go`

**Purpose:** Application-wide structured logger backed by Go 1.21+ `log/slog` package. Provides no-dependency logging API where callers get automatic source location capture (file, function, line).

**Lines of Code:** 131
**File Type:** Go source file (adapter)

**What Future Contributors Must Know:**
- Call `Init(logPath)` once from `cmd/omniview/main.go` (composition root). Never call from other packages.
- The returned cleanup func must be deferred: `defer logger.Init(...)`
- Before `Init()` is called, logs go to stderr. After `Init()`, logs go to the file.
- Any package can call `logger.Debug/Info/Warn/Error` directly — no receiver, no parameters beyond message and key-value pairs
- Source location in logs reflects the calling package, not this file, thanks to `runtime.Callers(3, pcs[:])` at stack depth 3
- Uses `atomic.Value` for thread-safe handler swapping
- **Logging convention for new code**: Use structured key-value pairs instead of string interpolation. Example: `logger.Error("batch processing failed", "subscriber", subscriber.Name(), "error", err)` — not `log.Printf("failed: %v", err)`

**Exports:**

- `Init(logPath string) (func(), error)` — Opens file, installs file-backed slog.Handler, returns cleanup
- `Debug(msg string, args ...any)` — Logs at DEBUG level
- `Info(msg string, args ...any)` — Logs at INFO level
- `Warn(msg string, args ...any)` — Logs at WARN level
- `Error(msg string, args ...any)` — Logs at ERROR level
- `With(args ...any) *slog.Logger` — Returns a logger pre-loaded with attributes for long-lived components

**Dependencies:**
- `context` — standard library
- `fmt` — standard library (`fmt.Errorf` only)
- `log/slog` — Go 1.21+ stdlib structured logging
- `os` — standard library (file open)
- `runtime` — standard library (call stack capture)
- `sync/atomic` — standard library (thread-safe handler storage)
- `time` — standard library (timestamp for log records)

**Used By:**
- `cmd/omniview/main.go` — calls `logger.Info("OmniInspect starting", "version", omniApp.GetVersion())`

**Key Implementation Details:**

```go
// Internal state: thread-safe handler storage via atomic.Value
var handle atomic.Value

func init() {
    // Bootstrap fallback: stderr until Init() configures the real file.
    h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        AddSource: true,
        Level:     slog.LevelDebug,
    })
    handle.Store(h)
}

// Init opens (or creates/appends to) logPath and installs a file-backed text handler
func Init(logPath string) (func(), error) {
    f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil {
        return nil, fmt.Errorf("logger.Init: open %q: %w", logPath, err)
    }

    h := slog.NewTextHandler(f, &slog.HandlerOptions{
        AddSource: true,
        Level:     slog.LevelDebug,
    })
    handle.Store(h)

    return func() { _ = f.Close() }, nil
}

// emit captures the actual call site PC so source file/line reflects the caller, not this wrapper
func emit(level slog.Level, msg string, args ...any) {
    h := handle.Load().(slog.Handler)
    ctx := context.Background()
    if !h.Enabled(ctx, level) {
        return
    }
    var pcs [1]uintptr
    runtime.Callers(3, pcs[:])  // 0=runtime.Callers, 1=emit, 2=Debug/Info/Warn/Error wrapper, 3=actual call site
    r := slog.NewRecord(time.Now(), level, msg, pcs[0])
    r.Add(args...)
    _ = h.Handle(ctx, r)
}
```

**Log format (text, human-readable):**
```
time=2026-05-13T14:00:00Z level=INFO source=oracle_adapter.go:112 msg="dequeue started" subscriber=OMNI_SUB
```

**Patterns Used:**
- Singleton pattern via package-level `var handle atomic.Value`
- Initializer pattern returning cleanup func for defer-based lifecycle management
- Stack depth capture for accurate source location in logs

**State Management:** No mutable state beyond the atomic handler pointer. All log calls are stateless.

**Side Effects:**
- File I/O: writes to log file on every log call (buffered by slog)
- No external service calls, no database access

**Error Handling:**
- `Init()` returns error if file cannot be opened (caller must handle)
- `emit()` silently drops errors from `h.Handle()` — logging must never panic

**Testing:**
- No test file exists. Recommended: `logger_test.go` with temp file, verify log output format and source location accuracy.

**Comments/TODOs:**
- None

---

## Contributor Checklist

- **Risks & Gotchas:**
  - Calling `Init()` multiple times will replace the handler but not close the previous file — cleanup func must be called for each Init
  - Before `Init()` is called, all logs go to stderr (useful for early startup diagnostics)
  - `atomic.Value` requires the stored type to be consistent — handler is always `slog.Handler`

- **Pre-change Verification Steps:**
  - Run `go build ./...` to ensure no compilation errors
  - Run `go vet ./internal/adapter/logger/...`
  - Verify log file is created and contains expected entries after running the application

- **Suggested Tests Before PR:**
  - Unit test for `Init()` with valid path (file created), invalid path (error returned), cleanup func called
  - Unit test for `Debug/Info/Warn/Error` output format
  - Unit test verifying source file/line capture accuracy

## Architecture & Design Patterns

### Code Organization

Package-level functions with internal state. No struct, no interface — zero dependency injection required. Calling packages use the logger as a utility, not a service.

### Design Patterns

- **Initializer Pattern:** `Init()` returns cleanup func to be deferred. Ensures file handle is closed cleanly on shutdown.
- **Atomic Swap Pattern:** `atomic.Value` for lock-free handler replacement while other goroutines may be logging.
- **Stack Depth Capture:** `runtime.Callers(3, ...)` navigates call stack to record the true call site, not intermediate wrapper functions.

### State Management Strategy

Stateless — the only mutable state is the `atomic.Value` storing the current `slog.Handler`. Log calls are independent and idempotent.

### Error Handling Philosophy

Logging is never allowed to fail silently in a way that panics the application. `emit()` swallows `h.Handle()` errors. `Init()` errors are propagated to caller.

### Testing Strategy

Direct output validation against known format strings. Use temporary files for `Init()` testing. No mocks needed — slog's text handler output is deterministic.

## Data Flow

### Data Entry Points

- **`logger.Info/Warn/Error/Debug(...)`** — Any package in the codebase can call these. Message and key-value args flow into `emit()`.

### Data Transformations

- **`emit()`** — Calls `slog.Handler.Handle()` with a record built from current time, level, message, and captured program counter.
- **`slog.TextHandler`** — Formats the record as human-readable text line and writes to the file handle.

### Data Exit Points

- **File** (`logPath` passed to `Init()`) — Appended log lines in text format.
- **stderr** (before `Init()` is called) — Fallback output when file not yet configured.

## Integration Points

### Shared State

- **`handle atomic.Value`** — Global logger handler. Thread-safe read/write via atomic swap. All log calls read from this.

### Events

- None. This is a fire-and-forget diagnostic output adapter.

### Database Access

- None.

## Dependency Graph

### Entry Points (Not Imported by Others in Scope)

- `cmd/omniview/main.go` — Imports and calls `logger.Info(...)` at startup

### Files This Depends On

- Standard library only (`context`, `fmt`, `log/slog`, `os`, `runtime`, `sync/atomic`, `time`)

### Files That Depend On This

- `cmd/omniview/main.go` — Calls `logger.Info("OmniInspect starting", ...)` at startup
- `internal/service/tracer/tracer_service.go` — Migrated from `log.Printf` to `logger.Debug/Info/Warn/Error` for all diagnostic events:
  - `logger.Error("webhook send failed", ...)` — webhook delivery failure
  - `logger.Warn("webhook dispatcher stopped, dropping message")` — dispatcher stopped
  - `logger.Warn("webhook queue full, dropping message")` — queue backpressure
  - `logger.Error("initial batch processing failed", ...)` — batch init failure
  - `logger.Error("batch processing failed", ...)` — dequeue error
  - `logger.Error("failed to unmarshal message", ...)` — JSON parse error
  - `logger.Warn("event channel full, dropping message")` — TUI channel backpressure
  - `logger.Error("failed to marshal message for webhook", ...)` — marshalling error
  - `logger.Debug("Omni_Tracer.sql content unchanged", ...)` — package hash match
  - `logger.Warn("failed to store updated package hash", ...)` — BoltDB write warning
- `internal/adapter/storage/boltdb/bolt_adapter.go` — Migration warnings for legacy DB config migration:
  - `logger.Warn("skipping legacy database setting: failed to unmarshal JSON", ...)`
  - `logger.Warn("skipping legacy database setting: failed to re-marshal JSON", ...)`
- `internal/adapter/storage/boltdb/database_settings_repository.go` — BoltDB iteration warning:
  - `logger.Warn("failed to unmarshal database settings", "key", ..., "error", ...)`
- `internal/adapter/ui/database_settings.go` — UI-layer diagnostics:
  - `logger.Error("database switch failed", ...)` — DB switch error
  - `logger.Warn("failed to sync database default state", ...)` — default sync failure
  - `logger.Warn("failed to close validated database adapter", ...)` — adapter cleanup warning
  - `logger.Warn("failed to close current database adapter", ...)` — current adapter close warning
  - `logger.Error("failed to reload database settings", ...)` — settings reload error
- `internal/adapter/ui/main_screen.go` — Main screen data load failures:
  - `logger.Error("failed to load database settings", ...)` — DB settings load failure
  - `logger.Error("failed to load webhook config", ...)` — webhook config load failure
- `internal/adapter/ui/loading.go` — Startup loading diagnostics:
  - `logger.Warn("update check failed", ...)` — non-fatal updater check failure
  - `logger.Error("failed to load database settings", ...)` — settings load failure