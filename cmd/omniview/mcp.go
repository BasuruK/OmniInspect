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
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// mcpListenAddr is where the in-process MCP server listens when the TUI starts it automatically. Loopback-only.
const mcpListenAddr = "127.0.0.1:54332"

// mcpAuthTokenFilename is the file startMCPServer writes the bearer token to.
const mcpAuthTokenFilename = "omniview-mcp.token"

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
func startMCPServer(omniApp *app.App, boltAdapter *boltdb.BoltAdapter, traceAppender ports.TraceAppender, dbSettingsRepo *boltdb.DatabaseSettingsRepository) (stop func(), _ error) {
	// validate before NewServer so a nil dep becomes a clean disabled-MCP log line instead of a panic. mcpserver.NewServer panics on missing required deps; mirror its required set here.
	if omniApp == nil || boltAdapter == nil || traceAppender == nil || dbSettingsRepo == nil {
		return nil, fmt.Errorf("MCP server: missing required dependency (App=%v Bolt=%v TraceAppender=%v DBSettingsRepo=%v)", omniApp, boltAdapter, traceAppender, dbSettingsRepo)
	}

	ln, err := net.Listen("tcp", mcpListenAddr)
	if err != nil {
		_ = os.Remove(mcpAuthTokenFilename) // remove any existing token file so it survive a crash
		return nil, fmt.Errorf("MCP server: listen on %s: %w", mcpListenAddr, err)
	}

	authToken, err := newAuthToken()
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("MCP server: generate auth token: %w", err)
	}

	if err := writeAuthTokenFile(authToken); err != nil {
		_ = ln.Close()
		_ = os.Remove(mcpAuthTokenFilename) // remove any existing token file so it survive a crash
		return nil, fmt.Errorf("MCP server: persist auth token: %w", err)
	}

	srv := mcpserver.NewServer(mcpserver.Deps{
		App:              omniApp,
		Bolt:             boltAdapter,
		TraceAppender:    traceAppender,
		PermissionsRepo:  boltdb.NewPermissionsRepository(boltAdapter),
		DBSettingsRepo:   dbSettingsRepo,
		DBAdapterFactory: oracleDBFactory,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ServeStreamableHTTP(ctx, ln, authToken); err != nil {
			logger.Warn("MCP server stopped", "error", err)
		}
	}()
	logger.Info("MCP server listening", "addr", mcpListenAddr, "token_fingerprint", tokenFingerprint(authToken))

	return func() {
		cancel()
		<-done
		_ = os.Remove(mcpAuthTokenFilename)
	}, nil
}

// newAuthToken returns a per-process random bearer token.
func newAuthToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate MCP auth token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// writeAuthTokenFile persists the bearer token to a 0600 file next to the app's other per-instance secrets (see omniview.key in main.go) so MCP clients can read it directly instead of parsing it out of logs.
func writeAuthTokenFile(token string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(mcpAuthTokenFilename), "."+filepath.Base(mcpAuthTokenFilename)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary MCP auth token file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("chmod MCP auth token file %s: %w", mcpAuthTokenFilename, err)
	}

	if _, err := tmpFile.WriteString(token); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			return fmt.Errorf("write MCP auth token file %s: %w (close: %v)", mcpAuthTokenFilename, err, closeErr)
		}
		return fmt.Errorf("write MCP auth token file %s: %w", mcpAuthTokenFilename, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close MCP auth token file %s: %w", mcpAuthTokenFilename, err)
	}
	if err := os.Rename(tmpPath, mcpAuthTokenFilename); err != nil {
		return fmt.Errorf("replace MCP auth token file %s: %w", mcpAuthTokenFilename, err)
	}
	return nil
}

// tokenFingerprint returns a short, non-reversible identifier for a token so log lines can be correlated to a running instance without ever exposing the bearer value itself.
func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:4])
}
