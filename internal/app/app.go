package app

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

// GetAuthor returns the application author's name
func (a *App) GetAuthor() string {
	return a.Author
}

// GetName returns the application name
func (a *App) GetName() string {
	return a.Name
}
