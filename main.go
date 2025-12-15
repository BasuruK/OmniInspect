package main

import (
	"OmniView/internal/app"
	"OmniView/internal/utils"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Listen to system signals for graceful shutdown (omitted for brevity)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Initialize application
	omniApp := app.New()
	println(omniApp.GetVersion())
	omniApp.StartServer()

	<-signalChan // Wait for termination signal

	// Cleanup resources before exiting
	utils.CleanupResources()

	os.Exit(0)
}
