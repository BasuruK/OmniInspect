package ui

import (
	"OmniView/internal/core/domain"

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

// BindMCPNotify sends MCP notifications to the TUI program.
func BindMCPNotify(p *tea.Program) (onClear func(), onMode func(domain.BroadcastMode), onConnect func(string), onStopped func()) {
	return func() {
			if p != nil {
				p.Send(tracesClearedMsg{})
			}
		}, func(mode domain.BroadcastMode) {
			if p != nil {
				p.Send(broadcastModeChangedMsg{mode: mode})
			}
		}, func(id string) {
			if p != nil {
				p.Send(mcpConnectDatabaseMsg{id: id})
			}
		}, func() {
			if p != nil {
				p.Send(mcpStoppedMsg{})
			}
		}
}
