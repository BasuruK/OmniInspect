package main

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/ui"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	updaterSvc "OmniView/internal/service/updater"
	"OmniView/internal/updater"
	"fmt"
	"os"
)

func main() {
	// ==========================================
	// Phase 1: Application Initialization - Pre-TUI setup
	// ==========================================
	omniApp := app.New()

	// Self-update cleanup (remove .old binary leftovers from previous update)
	updater.CleanupOldBinary()

	// Initialize BoltDB (fast, no UI needed)
	boltAdapter := boltdb.NewBoltAdapter("omniview.bolt")
	if err := boltAdapter.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize BoltDB: %v\n", err)
		os.Exit(1)
	}
	defer boltAdapter.Close()

	// ==========================================
	// Phase 2: Initialize Services (deferred until TUI config is ready)
	// ==========================================

	// Event channel: tracer service → TUI
	eventCh := make(chan *domain.QueueMessage, 100)

	// Updater service for update checking within TUI
	updaterService := updaterSvc.NewUpdaterService(omniApp.GetVersion())

	// ── Phase 3: Start TUI ───────────────────────
	// BoltDB is already initialized; TUI handles config loading via onboarding screen

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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	p := ui.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

}
