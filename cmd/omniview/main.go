package main

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/app"
	"OmniView/internal/utils"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Listen to system signals for graceful shutdown (omitted for brevity)
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

	// Initialize application
	omniApp := app.New()
	fmt.Println(omniApp.GetVersion())
	go omniApp.StartServer(done)

	select {
	case <-done:
	case <-signalChan:
	}

	// Cleanup resources before exiting
	utils.CleanupResources()
}
