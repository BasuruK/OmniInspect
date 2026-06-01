package main

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/adapter/security/credcipher"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/ui"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/tracer"
	updaterSvc "OmniView/internal/service/updater"
	"OmniView/internal/updater"
	"errors"
	"fmt"
	"os"
	"runtime"
)

func main() {
	omniApp := app.New()
	if err := run(omniApp); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(omniApp *app.App) error {
	// ==========================================
	// Phase 1: Application Initialization - Pre-TUI setup
	// ==========================================

	// ── Logger ───────────────────────────────
	closeLog, err := logger.Init("omniview.log")
	if err != nil {
		return fmt.Errorf("failed to initialise logger: %w", err)
	}
	defer closeLog()
	logger.Info("OmniInspect starting", "version", omniApp.GetVersion())

	updater.CleanupOldBinary()

	// Enable at-rest encryption for credentials persisted in BoltDB. The master key
	// is stored in a 0600 file alongside the database and generated on first run.
	const boltDBPath = "omniview.bolt"
	const keyPath = "omniview.key"
	if _, err := os.Stat(boltDBPath); err == nil {
		if _, keyErr := os.Stat(keyPath); keyErr != nil {
			if errors.Is(keyErr, os.ErrNotExist) {
				return fmt.Errorf("credential key %q is missing while %q already exists; restore the original key file or remove %q and reconfigure, otherwise stored credentials cannot be decrypted", keyPath, boltDBPath, boltDBPath)
			}
			return fmt.Errorf("failed to stat credential key %s: %w", keyPath, keyErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat BoltDB file %s: %w", boltDBPath, err)
	}

	credCipher, err := credcipher.New(credcipher.NewFileKeyProvider(keyPath))
	if err != nil {
		return fmt.Errorf("failed to initialise credential cipher: %w", err)
	}
	domain.SetCredentialCipher(credCipher)

	// Initialize BoltDB
	boltAdapter, err := boltdb.NewBoltAdapter(boltDBPath)
	if err != nil {
		return fmt.Errorf("failed to create BoltDB adapter: %w", err)
	}
	if err := boltAdapter.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize BoltDB: %w", err)
	}
	defer boltAdapter.Close()

	// ==========================================
	// Phase 2: Initialize Services
	// ==========================================

	eventCh := make(chan *domain.QueueMessage, 100)
	updaterService := updaterSvc.NewUpdaterService(omniApp.GetVersion())

	// ── Phase 3: Start TUI ───────────────────────

	dbSettingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)

	model, err := ui.NewModel(ui.ModelOpts{
		App:         omniApp,
		BoltAdapter: boltAdapter,
		DBFactory: func(settings *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
			adapter := oracle.NewOracleAdapter(settings)
			if adapter == nil {
				return nil, fmt.Errorf("failed to create oracle adapter: nil settings")
			}
			return adapter, nil
		},
		DBSettingsRepo: dbSettingsRepo,
		EventChannel:   eventCh,
		UpdaterService: updaterService,
	})
	if err != nil {
		return fmt.Errorf("failed to create UI model: %w", err)
	}

	const minRows = 36
	const minCols = 130
	if fi, err := os.Stdout.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		term := os.Getenv("TERM")
		emit := runtime.GOOS == "windows" ||
			(term != "" && term != "dumb") ||
			os.Getenv("WT_SESSION") != ""
		if emit {
			fmt.Printf("\033[8;%d;%dt", minRows, minCols)
		}
	}

	p := ui.NewProgram(model)
	if _, err := p.Run(); err != nil {
		tracer.StopWebhookDispatcher()
		return fmt.Errorf("TUI error: %w", err)
	}

	tracer.StopWebhookDispatcher()
	return nil
}
