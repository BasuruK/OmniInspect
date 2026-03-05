package ui

import (
	"OmniView/internal/adapter/ui/welcome"
	"OmniView/internal/app"

	"charm.land/bubbletea/v2"
)

// UIAdapter is the main UI controller that manages the different screens
type UIAdapter struct {
	program    *tea.Program
	welcome    *welcome.Model
	currentView viewType
	quit       bool
}

type viewType int

const (
	viewWelcome viewType = iota
	viewMain
	viewSettings
)

// NewUIAdapter creates a new UI adapter
func NewUIAdapter(appVersion string) *UIAdapter {
	return &UIAdapter{
		currentView: viewWelcome,
		quit:        false,
	}
}

// StartWelcome starts the welcome screen animation
func (u *UIAdapter) StartWelcome(omniApp *app.App) error {
	welcomeModel := welcome.New(omniApp)

	p := tea.NewProgram(welcomeModel)

	u.program = p
	u.welcome = welcomeModel

	_, err := p.Run()
	return err
}

// Msg represents UI messages
type Msg struct {
	Type    string
	Payload interface{}
}

// Constants for message types
const (
	MsgWelcomeComplete = "welcome_complete"
	MsgNavigateMain   = "navigate_main"
	MsgQuit           = "quit"
)
