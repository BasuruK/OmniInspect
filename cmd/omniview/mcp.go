package main

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/adapter/mcpserver"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"fmt"
	"net"
)

// mcpListenAddr is where the in-process MCP server listens when the TUI starts it automatically. Loopback-only.
const mcpListenAddr = "127.0.0.1:54332"

// MCPCallbacks holds optional TUI notification hooks for in-process MCP mutations.
type MCPCallbacks struct {
	OnTracesCleared        func()
	OnBroadcastModeChanged func(domain.BroadcastMode)
	OnDatabaseConnected    func(string)
	OnStopped              func()
}

// oracleDBFactory builds the real Oracle adapter. It mirrors the closure passed to ui.NewModel in main() so the MCP path uses the same Oracle instantiation rules as the TUI.
func oracleDBFactory(settings *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
	adapter := oracle.NewOracleAdapter(settings)
	if adapter == nil {
		return nil, fmt.Errorf("failed to create oracle adapter: nil settings")
	}
	return adapter, nil
}

// startMCPServer builds the MCP server, wired to the shared BoltAdapter, and serves it over streamable HTTP in a background goroutine. On listen failure,
// MCP is skipped and the TUI still starts (a busy port shouldn't block the whole app). The returned stop func cancels the server context and waits for ServeStreamableHTTP to return.
func startMCPServer(omniApp *app.App, boltAdapter *boltdb.BoltAdapter, traceAppender ports.TraceAppender, dbSettingsRepo *boltdb.DatabaseSettingsRepository, cbs MCPCallbacks) (stop func(), _ error) {
	// validate before NewServer so a nil dep becomes a clean disabled-MCP log line instead of a panic. mcpserver.NewServer panics on missing required deps; mirror its required set here.
	if omniApp == nil || boltAdapter == nil || traceAppender == nil || dbSettingsRepo == nil {
		return nil, fmt.Errorf("MCP server: missing required dependency (App=%v Bolt=%v TraceAppender=%v DBSettingsRepo=%v)", omniApp, boltAdapter, traceAppender, dbSettingsRepo)
	}

	ln, err := net.Listen("tcp", mcpListenAddr)
	if err != nil {
		return nil, fmt.Errorf("MCP server: listen on %s: %w", mcpListenAddr, err)
	}

	srv := mcpserver.NewServer(mcpserver.Deps{
		App:                    omniApp,
		Bolt:                   boltAdapter,
		TraceAppender:          traceAppender,
		PermissionsRepo:        boltdb.NewPermissionsRepository(boltAdapter),
		DBSettingsRepo:         dbSettingsRepo,
		SubscriberRepo:         boltdb.NewSubscriberRepository(boltAdapter),
		DBAdapterFactory:       oracleDBFactory,
		OnTracesCleared:        cbs.OnTracesCleared,
		OnBroadcastModeChanged: cbs.OnBroadcastModeChanged,
		OnDatabaseConnected:    cbs.OnDatabaseConnected,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ServeStreamableHTTP(ctx, ln); err != nil {
			logger.Warn("MCP server stopped", "error", err)
		}
		// Intentional stop (cancel via returned stop func) runs after the TUI exits — skip Send.
		// Unexpected Serve exit while the program is still running must clear the header indicator.
		// Async: never block Serve teardown / done on TUI Program.Send.
		if ctx.Err() == nil && cbs.OnStopped != nil {
			go cbs.OnStopped()
		}
	}()
	logger.Info("MCP server listening", "addr", mcpListenAddr)

	return func() {
		cancel()
		<-done
	}, nil
}
