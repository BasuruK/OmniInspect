package app

import (
	"OmniView/internal/core/ports"
	"bufio"
	"fmt"
	"os"
)

// App represents the main application structure
type App struct {
	Name    string // Name of the application
	Author  string // Author of the program
	Version string // Version of the application
	db      ports.DatabaseRepository
	config  ports.ConfigRepository
}

// New creates a new instance of the application
func New(config ports.ConfigRepository, db ports.DatabaseRepository) *App {
	return &App{
		Version: "0.1.0",
		Author:  "Basuru Balasuriya",
		Name:    "OmniView",
		db:      db,
		config:  config,
	}
}

// GetVersion returns the current application version
func (a *App) GetVersion() string {
	return a.Version
}

// GetAuthor returns the application authors name“
func (a *App) GetAuthor() string {
	return a.Author
}

// GetName returns the application name
func (a *App) GetName() string {
	return a.Name
}

func (a *App) StartServer(done chan struct{}) {
	// Start the server
	fmt.Println("Tracer started")

	fmt.Println("Press Enter to Continue...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	close(done)
}
