package main

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
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

	// ==========================================
	// Phase 2: Initialize Services (deferred until TUI config is ready)
	// ==========================================

	// Event channel: tracer service → TUI
	eventCh := make(chan *domain.QueueMessage, 100)

	// ── Phase 3: Start TUI ───────────────────────
	// BoltDB is already initialized; TUI handles config loading via onboarding screen

	model, err := ui.NewModel(ui.ModelOpts{
		App:               omniApp,
		BoltAdapter:       boltAdapter,
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

}
