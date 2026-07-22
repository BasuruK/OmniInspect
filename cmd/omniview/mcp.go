package main

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/adapter/mcpserver"
	"OmniView/internal/adapter/security/credcipher"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/tracebuffer"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/connector"
	"context"
	"errors"
	"fmt"
	"net"
)

// mcpListenAddr is where the in-process MCP server listens when the TUI
// starts it automatically (see startMCPServer). Loopback-only.
//
// ponytail: fixed port, add a flag/env override if a user ever needs two
// instances running side by side.
const mcpListenAddr = "127.0.0.1:7337"

// mcpUsage is printed by `omniview mcp --help`.
const mcpUsage = `omniview mcp — start the OmniView Model Context Protocol server.

The server communicates over stdin/stdout using the MCP framing protocol.
All log output is redirected to omniview.log so the stdio stream remains
clean for MCP messages.

Usage:
  omniview mcp [--help]

Environment:
  None. Configuration is read from omniview.bolt in the working directory.

Exit codes:
  0  clean shutdown
  1  initialization or runtime error
`

// oracleDBFactory builds the real Oracle adapter. It mirrors the closure
// passed to ui.NewModel in main() so the MCP path uses the same Oracle
// instantiation rules as the TUI.
func oracleDBFactory(settings *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
	adapter := oracle.NewOracleAdapter(settings)
	if adapter == nil {
		return nil, fmt.Errorf("failed to create oracle adapter: nil settings")
	}
	return adapter, nil
}

// runMCP is the entry point for the `mcp` subcommand. It performs the same
// pre-TUI bootstrap as the main TUI (logger, credential cipher, BoltDB),
// constructs the shared services (TraceAppender, Connector, dbSettingsRepo),
// then hands off to the MCP server. On return, BoltDB is closed.
func runMCP(omniApp *app.App) error {
	// ── Logger ───────────────────────────────
	closeLog, err := logger.Init("omniview.log")
	if err != nil {
		return fmt.Errorf("failed to initialise logger: %w", err)
	}
	defer closeLog()
	logger.Info("OmniInspect MCP server starting", "version", omniApp.GetVersion())

	// ── Credential cipher ────────────────────
	const keyPath = "omniview.key"
	credCipher, err := credcipher.New(credcipher.NewFileKeyProvider(keyPath))
	if err != nil {
		return fmt.Errorf("failed to initialise credential cipher: %w", err)
	}
	domain.SetCredentialCipher(credCipher)

	// ── BoltDB ───────────────────────────────
	const boltDBPath = "omniview.bolt"
	boltAdapter, err := boltdb.NewBoltAdapter(boltDBPath)
	if err != nil {
		return fmt.Errorf("failed to open BoltDB at %q: %w", boltDBPath, err)
	}
	if err := boltAdapter.Initialize(); err != nil {
		_ = boltAdapter.Close()
		return fmt.Errorf("failed to initialise BoltDB: %w", err)
	}
	defer func() {
		if cerr := boltAdapter.Close(); cerr != nil {
			logger.Warn("failed to close BoltDB", "error", cerr)
		}
	}()

	// ── Shared services ──────────────────────
	traceAppender := tracebuffer.New(10000)
	conn := connector.New(boltAdapter)
	dbSettingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)

	// ── Server ───────────────────────────────
	srv := mcpserver.NewServer(mcpserver.Deps{
		App:              omniApp,
		Bolt:             boltAdapter,
		TraceAppender:    traceAppender,
		Connector:        conn,
		DBSettingsRepo:   dbSettingsRepo,
		DBAdapterFactory: oracleDBFactory,
	})

	if err := srv.ServeStdio(context.Background()); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("MCP server: %w", err)
	}
	return nil
}

// printMCPUsage writes the mcp subcommand help text to stdout. Used by the
// `--help` short-circuit in main().
func printMCPUsage() {
	fmt.Print(mcpUsage)
}

// startMCPServer builds the MCP server on its own BoltAdapter-backed
// Connector (the TUI never touches the active-database-id key, so this
// doesn't race with anything) and serves it over streamable HTTP in a
// background goroutine. The MCP subcommand's stdio path is unaffected —
// this is only for the "TUI auto-starts MCP" case. Returns a stop func
// that shuts the server down; on listen failure, MCP is skipped and the
// TUI still starts (a busy port shouldn't block the whole app).
func startMCPServer(
	omniApp *app.App,
	boltAdapter *boltdb.BoltAdapter,
	traceAppender *tracebuffer.RingBuffer,
	dbSettingsRepo *boltdb.DatabaseSettingsRepository,
) (stop func(), _ error) {
	ln, err := net.Listen("tcp", mcpListenAddr)
	if err != nil {
		return func() {}, fmt.Errorf("MCP server: listen on %s: %w", mcpListenAddr, err)
	}

	srv := mcpserver.NewServer(mcpserver.Deps{
		App:              omniApp,
		Bolt:             boltAdapter,
		TraceAppender:    traceAppender,
		Connector:        connector.New(boltAdapter),
		DBSettingsRepo:   dbSettingsRepo,
		DBAdapterFactory: oracleDBFactory,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := srv.ServeStreamableHTTP(ctx, ln); err != nil {
			logger.Warn("MCP server stopped", "error", err)
		}
	}()
	logger.Info("MCP server listening", "addr", mcpListenAddr)

	return cancel, nil
}
