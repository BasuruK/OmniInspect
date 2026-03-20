package ui

import (
	tea "charm.land/bubbletea/v2"
)

// UIAdapter is the main UI controller
type UIAdapter struct {
	program *tea.Program
}

// NewUIAdapter creates a new UI adapter
func NewUIAdapter(appVersion string) *UIAdapter {
	return &UIAdapter{}
}

// NewProgram creates a configured tea.Program ready to Run().
func NewProgram(model *Model) *tea.Program {
	return tea.NewProgram(model)
}
