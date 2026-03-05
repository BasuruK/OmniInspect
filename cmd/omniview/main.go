package main

import (
	"OmniView/internal/adapter/config"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/ui"
	"OmniView/internal/app"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	"OmniView/internal/updater"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Initialize the application
	omniApp := app.New()

	// Start the Bubble Tea UI welcome screen
	uiAdapter := ui.NewUIAdapter(omniApp.GetVersion())
	if err := uiAdapter.StartWelcome(omniApp); err != nil {
		log.Printf("[ui] Welcome screen error: %v\n", err)
		// Fall back to simple ASCII logo
		fmt.Println(omniApp.GetLogoASCII())
	}

	// Clean up leftover binary from a previous update (safe no-op if nothing to clean)
	updater.CleanupOldBinary()

	// Check for updates before anything else (only runs for release builds, skips "dev")
	if err := updater.CheckAndUpdate(app.Version); err != nil {
		log.Printf("[updater] Update failed: %v\n", err)
		// Non-fatal — continue starting the application
	}

	// Listen to system signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Shared cancellable context for startup and runtime
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize BoltDB
	fmt.Println("Initializing BoltDB!")
	boltAdapter := boltdb.NewBoltAdapter("omniview.bolt")
	if err := boltAdapter.Initialize(); err != nil {
		log.Fatalf("failed to initialize BoltDB: %v", err)
	}
	defer boltAdapter.Close()

	// 1. Infrastructure Setup (Logging, Config, etc.)
	// Create repositories
	dbSettingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)

	// Load Configurations
	cfgLoader := config.NewConfigLoader(dbSettingsRepo)
	appConfig, err := cfgLoader.LoadClientConfigurations()
	if err != nil {
		log.Fatalf("failed to load configurations: %v", err)
	}

	// Initialize Oracle DB Adapter (inject Configurations)
	dbAdapter := oracle.NewOracleAdapter(appConfig)
	if err := dbAdapter.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to Oracle DB: %v", err)
	}
	defer dbAdapter.Close(context.Background())

	// 2. Create DDD Repositories
	subscriberRepo := boltdb.NewSubscriberRepository(boltAdapter)
	permissionsRepo := boltdb.NewPermissionsRepository(boltAdapter)

	// 3. Services (Inject Repositories)
	permissionService := permissions.NewPermissionService(dbAdapter, permissionsRepo, boltAdapter)
	tracerService := tracer.NewTracerService(dbAdapter, boltAdapter)
	subscriberService := subscribers.NewSubscriberService(dbAdapter, subscriberRepo)

	// 4. Application Bootstrap
	// Run Startup Tasks using Services
	// 5.1 Ensure Permission Checks Package is Deployed and permissions are granted
	if _, err := permissionService.DeployAndCheck(ctx, appConfig.Username()); err != nil {
		log.Fatalf("failed to run permission checks: %v", err)
	}

	// 5.2. Ensure Tracer Package is Deployed and initialized
	if err := tracerService.DeployAndCheck(ctx); err != nil {
		log.Fatalf("failed to deploy tracer package: %v", err)
	}

	// 5.3. Subscriber Registration
	subscriber, err := subscriberService.RegisterSubscriber(ctx)
	if err != nil {
		log.Fatalf("failed to register subscriber: %v", err)
	}
	fmt.Printf("Registered Subscriber: %s\n", subscriber.Name())

	// 6. Show Application Status
	go omniApp.ShowStatus(done)

	if err := tracerService.StartEventListener(ctx, subscriber, appConfig.Username()); err != nil {
		log.Fatalf("failed to start tracer event listener: %v", err)
	}

	select {
	case <-done:
		cancel()
	case <-signalChan:
		cancel()
	}
}
