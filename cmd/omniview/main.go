package main

import (
	"OmniView/internal/adapter/config"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/app"
	"OmniView/internal/service/permissions"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Listen to system signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Initialize BoltDB
	fmt.Println("Initializing BoltDB!")
	boltAdapter := boltdb.NewBoltAdapter("omniview.bolt")
	if err := boltAdapter.Initialize(); err != nil {
		log.Fatalf("failed to initialize BoltDB: %v", err)
	}
	defer boltAdapter.Close()

	// 1. Infrastructure Setup (Logging, Config, etc.)
	// Load Configurations
	cfgLoader := config.NewConfigLoader(boltAdapter)
	appConfig, err := cfgLoader.LoadClientConfigurations()
	if err != nil {
		log.Fatalf("failed to load configurations: %v", err)
	}

	// Initialize Oracle DB Adapter (inject Configurations)
	dbAdapter := oracle.NewOracleAdapter(appConfig)
	if err := dbAdapter.Connect(); err != nil {
		log.Fatalf("failed to connect to Oracle DB: %v", err)
	}
	defer dbAdapter.Close()

	// 2. Services (Inject Adapters)
	permissionService := permissions.NewPermissionService(dbAdapter)

	// 3. Application Bootstrap
	// Run Startup Tasks using Services
	// 3.1 Ensure Permission Checks Package is Deployed
	if _, err := permissionService.Check(appConfig.Username); err != nil {
		log.Fatalf("failed to run permission checks: %v", err)
	}
	// 3.2 Check for Permissions
	// 3.2.1 Check if permission check has already been performed.
	// 3.2.2 If not, run permission checks against the database.

	// 4. Start Application
	omniApp := app.New(boltAdapter, dbAdapter)
	fmt.Println(omniApp.GetVersion())
	go omniApp.StartServer(done)

	select {
	case <-done:
	case <-signalChan:
	}
}
