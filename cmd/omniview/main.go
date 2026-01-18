package main

import (
	"OmniView/internal/adapter/config"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/app"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	"context"
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
	permissionService := permissions.NewPermissionService(dbAdapter, boltAdapter)
	tracerService := tracer.NewTracerService(dbAdapter, boltAdapter)
	subscriberService := subscribers.NewSubscriberService(dbAdapter, boltAdapter)

	// 3. Application Bootstrap
	// Run Startup Tasks using Services
	// 3.1 Ensure Permission Checks Package is Deployed and permissions are granted
	if _, err := permissionService.DeployAndCheck(appConfig.Username); err != nil {
		log.Fatalf("failed to run permission checks: %v", err)
	}

	// 3.2. Ensure Tracer Package is Deployed and initialized
	if err := tracerService.DeployAndCheck(); err != nil {
		log.Fatalf("failed to deploy tracer package: %v", err)
	}

	// 3.3. Subscriber Registration
	subscriber, err := subscriberService.RegisterSubscriber()
	if err != nil {
		log.Fatalf("failed to register subscriber: %v", err)
	}
	fmt.Printf("Registered Subscriber: %s\n", subscriber.Name)

	// 4. Start Application
	// create cancellable context and tie cancellation to signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// subscriber variable is from RegisterSubscriber(); if it's a value use &subscriber
	if err := tracerService.StartEventListener(ctx, &subscriber, appConfig.Username); err != nil {
		log.Fatalf("failed to start tracer event listener: %v", err)
	}

	omniApp := app.New(boltAdapter, dbAdapter)
	fmt.Println(omniApp.GetVersion())
	go omniApp.StartServer(done)

	select {
	case <-done:
		cancel()
	case <-signalChan:
		cancel()
	}
}
