package logger

// ==========================================
// Application Logger Adapter
// ==========================================
// Backed by the stdlib log/slog package (Go 1.21+).
//
// Call Init() once from the composition root (cmd/omniview/main.go).
// Every package in the codebase can then call the package-level
// functions Debug / Info / Warn / Error directly — no parameters,
// no injected dependencies. The calling file, function name, and
// line number are captured automatically via runtime.Callers at the
// correct stack depth, so each log line knows exactly where it came from.
//
// Log format (text, human-readable):
//
//	time=2026-05-13T14:00:00Z level=INFO source=oracle_adapter.go:112 msg="dequeue started" subscriber=OMNI_SUB
//
// Before Init() is called the logger falls back to stderr, so early
// startup errors are never silently lost.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

// ─────────────────────────
// Internal state
// ─────────────────────────

// handle holds the active slog.Handler.
// atomic.Value allows storage of the interface type directly (vs atomic.Pointer
// which would require *slog.Handler). Swaps are safe when Init() is called on the
// main goroutine while other goroutines may already be logging.
var handle atomic.Value

func init() {
	// Bootstrap fallback: stderr until Init() configures the real file.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
	handle.Store(h)
}

// ─────────────────────────
// Initializer
// ─────────────────────────

// Init opens (or creates/appends to) logPath and installs a file-backed
// text handler as the active logger. Returns a cleanup func that must
// be deferred in main() to flush and close the log file cleanly.
//
// Call once — only from cmd/omniview/main.go (composition root).
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

// ─────────────────────────
// Internal dispatch
// ─────────────────────────

// emit creates a slog.Record with the program counter of the actual
// call site (not of this wrapper), so the source file/line in each
// log entry reflects the package that called Debug/Info/Warn/Error.
//
// Stack depth for runtime.Callers:
//
//	0 = runtime.Callers itself
//	1 = emit
//	2 = Debug / Info / Warn / Error (exported wrapper)
//	3 = actual call site  ← we want this
func emit(level slog.Level, msg string, args ...any) {
	h := handle.Load().(slog.Handler)
	ctx := context.Background()
	if !h.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = h.Handle(ctx, r)
}

// ─────────────────────────
// Public API
// ─────────────────────────

// Debug logs a message at DEBUG level.
// Accepts alternating key-value pairs or slog.Attr values as args.
func Debug(msg string, args ...any) { emit(slog.LevelDebug, msg, args...) }

// Info logs a message at INFO level.
func Info(msg string, args ...any) { emit(slog.LevelInfo, msg, args...) }

// Warn logs a message at WARN level.
func Warn(msg string, args ...any) { emit(slog.LevelWarn, msg, args...) }

// Error logs a message at ERROR level.
func Error(msg string, args ...any) { emit(slog.LevelError, msg, args...) }

// With returns a *slog.Logger pre-loaded with the given attributes.
// Useful for long-lived components that want to attach a fixed context
// to every log call without repeating keys:
//
//	log := logger.With("subscriber", name)
//	log.Info("dequeue started")
func With(args ...any) *slog.Logger {
	return slog.New(handle.Load().(slog.Handler)).With(args...)
}
