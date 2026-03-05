package ui

import (
	"OmniView/internal/adapter/ui/welcome"
	"OmniView/internal/app"

	"charm.land/bubbletea/v2"
)

// UIAdapter is the main UI controller
type UIAdapter struct {
	program *tea.Program
}

// NewUIAdapter creates a new UI adapter
func NewUIAdapter(appVersion string) *UIAdapter {
	return &UIAdapter{}
}

// StartWelcome starts the welcome screen animation
// After animation completes (about 3 seconds), it returns
// and the caller can continue with main application logic
func (u *UIAdapter) StartWelcome(omniApp *app.App) error {
	welcomeModel := welcome.New(omniApp)

	p := tea.NewProgram(welcomeModel)

	u.program = p

	// Run() blocks until the program exits (when we call tea.Quit)
	// This happens after the animation completes (~3 seconds)
	_, err := p.Run()

	// When we get here, the welcome screen has exited
	// Continue with main application logic
	return err
}
