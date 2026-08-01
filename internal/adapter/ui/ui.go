package ui

import (
	"OmniView/internal/core/domain"
	"sync/atomic"

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

// MCPNotifyHooks are fire-and-forget callbacks for in-process MCP → TUI sync.
// Send is gated on MarkReady (call from Model.Init) so MCP cannot block on an unstarted Program.
type MCPNotifyHooks struct {
	OnTracesCleared        func()
	OnBroadcastModeChanged func(domain.BroadcastMode)
	OnDatabaseConnected    func(string)
	OnStopped              func()
	MarkReady              func()
}

// BindMCPNotify builds lifecycle-aware, non-blocking MCP→TUI notification hooks.
func BindMCPNotify(p *tea.Program) MCPNotifyHooks {
	var ready atomic.Bool
	send := func(msg tea.Msg) {
		if p == nil || !ready.Load() {
			return
		}
		// Never block MCP HTTP handlers on the Bubble Tea mailbox.
		go p.Send(msg)
	}
	return MCPNotifyHooks{
		OnTracesCleared:        func() { send(tracesClearedMsg{}) },
		OnBroadcastModeChanged: func(mode domain.BroadcastMode) { send(broadcastModeChangedMsg{mode: mode}) },
		OnDatabaseConnected:    func(id string) { send(mcpConnectDatabaseMsg{id: id}) },
		OnStopped:              func() { send(mcpStoppedMsg{}) },
		MarkReady:              func() { ready.Store(true) },
	}
}
