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
	cfgLoader := config.NewFileConfigLoader("settings.json")
	appConfig, err := cfgLoader.LoadClientConfigurations()
	if err != nil {
		log.Fatalf("failed to load configurations: %v", err)
	}

	// Initialize Oracle DB Adapter (inject Configurations)
	dbAdapter := oracle.NewOracleAdapter(appConfig.DatabaseSettings)
	if err := dbAdapter.Connect(); err != nil {
		log.Fatalf("failed to connect to Oracle DB: %v", err)
	}
	defer dbAdapter.Close()

	// 2. Services (Inject Adapters)
	permissionService := permissions.NewPermissionService(dbAdapter)

	// 3. Application Bootstrap
	// Run Startup Tasks using Services
	if _, err := permissionService.CheckAndDeploy(); err != nil {
		log.Fatalf("failed to run permission checks: %v", err)
	}

	// 4. Start Application
	omniApp := app.New(appConfig, dbAdapter)
	fmt.Println(omniApp.GetVersion())
	go omniApp.StartServer(done)

	select {
	case <-done:
	case <-signalChan:
	}

	// Cleanup resources before exiting

}
