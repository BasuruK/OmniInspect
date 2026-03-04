package app

import (
	"bufio"
	"fmt"
	"os"
)

// Version is set at build time via -ldflags "-X OmniView/internal/app.Version=vX.Y.Z"
// When not set (e.g. during local development), it defaults to "dev".
var Version = "dev"

// App represents the main application structure
type App struct {
	Name    string // Name of the application
	Author  string // Author of the program
	Version string // Version of the application
}

// New creates a new instance of the application
func New() *App {
	return &App{
		Version: Version,
		Author:  "Basuru Balasuriya",
		Name:    "OmniView",
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

func (a *App) GetLogoASCII() string {
	return `
  __  __ __ __  _ _  _   _  _ ___  _   _ 
 /__\|  V  |  \| | || \ / || | __|| | | |
| \/ | \_/ | | ' | |` + "`" + `\ V /'| | _| | 'V' |
 \__/|_| |_|_|\__|_|  \_/  |_|___|!_/ \_!
Created with ❤️  by ` + a.GetAuthor() + `
Version: ` + a.GetVersion() + `
`
}

func (a *App) ShowStatus(done chan struct{}) {
	// Start the server
	fmt.Println("Tracer started")

	fmt.Println("Press Enter to Continue...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	close(done)
}
