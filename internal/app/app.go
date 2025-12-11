package app

import (
	"OmniView/internal/utils"
	"bufio"
	"fmt"
	"os"
)

// App represents the main application structure
type App struct {
	Name    string // Name of the application
	Author  string // Author of the program
	Version string // Version of the application
}

// New creates a new instance of the application
func New() *App {
	return &App{
		Version: "0.1.0",
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

func (a *App) StartServer() {
	// Start the server
	fmt.Println("Server started")

	// Connect to the database
	utils.ExecuteStatement("SELECT 'HELLO WORLD' FROM DUAL")
	//utils.FetchData()

	fmt.Println("Press Enter to Continue...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
