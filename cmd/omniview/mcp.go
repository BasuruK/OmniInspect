package main

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/adapter/mcpserver"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/tracebuffer"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/connector"
	"context"
	"fmt"
	"net"
)

// mcpListenAddr is where the in-process MCP server listens when the TUI starts it automatically. Loopback-only.
const mcpListenAddr = "127.0.0.1:7337"

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

// startMCPServer builds the MCP server on its own BoltAdapter-backed
// Connector and serves it over streamable HTTP in a background goroutine. On listen failure, MCP is skipped and the TUI still starts (a busy port shouldn't block the whole app).
// The returned stop func cancels the server context and waits for ServeStreamableHTTP to return.
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
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ServeStreamableHTTP(ctx, ln); err != nil {
			logger.Warn("MCP server stopped", "error", err)
		}
	}()
	logger.Info("MCP server listening", "addr", mcpListenAddr)

	return func() {
		cancel()
		<-done
	}, nil
}
