package main

import (
	"OmniView/internal/app"
	"OmniView/internal/utils"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Listen to system signals for graceful shutdown (omitted for brevity)
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

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
