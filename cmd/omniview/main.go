package main

import (
	"OmniView/internal/adapter/config"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/ui"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	"OmniView/internal/updater"
	"fmt"
	"log"
	"os"
)

func main() {
	// ==========================================
	// Phase 1: Application Initialization - Pre-TUI setup
	// ==========================================
	omniApp := app.New()

	// Self-update check (may replace binary and restart)
	updater.CleanupOldBinary()
	if err := updater.CheckAndUpdate(app.Version); err != nil {
		log.Printf("[updater] Update failed: %v\n", err)
	}

	// Initialize BoltDB (fast, no UI needed)
	boltAdapter := boltdb.NewBoltAdapter("omniview.bolt")
	if err := boltAdapter.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize BoltDB: %v\n", err)
		os.Exit(1)
	}
	defer boltAdapter.Close()

	// Load configuration (may prompt user via stdin if first run)
	dbSettingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)
	cfgLoader := config.NewConfigLoader(dbSettingsRepo, boltAdapter)
	appConfig, err := cfgLoader.LoadClientConfigurations()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ==========================================
	// Phase 2: Initialize and Inject Services
	// ==========================================

	// Event channel: tracer service → TUI
	eventCh := make(chan *domain.QueueMessage, 100)

	// Create adapters and services
	dbAdapter := oracle.NewOracleAdapter(appConfig)
	subscriberRepo := boltdb.NewSubscriberRepository(boltAdapter)
	permissionsRepo := boltdb.NewPermissionsRepository(boltAdapter)

	permissionService := permissions.NewPermissionService(dbAdapter, permissionsRepo, boltAdapter)
	tracerService := tracer.NewTracerService(dbAdapter, boltAdapter, eventCh)
	subscriberService := subscribers.NewSubscriberService(dbAdapter, subscriberRepo)

	// ── Phase 3: Start TUI ───────────────────────

	model, err := ui.NewModel(ui.ModelOpts{
		App:               omniApp,
		DBAdapter:         dbAdapter,
		PermissionService: permissionService,
		TracerService:     tracerService,
		SubscriberService: subscriberService,
		AppConfig:         appConfig,
		EventChannel:      eventCh,
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

	// Graceful shutdown: stop webhook dispatcher and event listeners, then wait for completion
	tracer.StopAll(tracerService)
}
