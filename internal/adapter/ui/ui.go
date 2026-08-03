package ui

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/core/domain"
	"fmt"
	"sync"
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
// Notifications before MarkReady (Model.Init) are buffered and replayed so the
// MCP→TUI race window after startMCPServer cannot silently drop state changes.
type MCPNotifyHooks struct {
	OnTracesCleared        func()
	OnBroadcastModeChanged func(domain.BroadcastMode)
	OnDatabaseConnected    func(string)
	OnStopped              func()
	MarkReady              func()
}

// BindMCPNotify builds lifecycle-aware, non-blocking MCP→TUI notification hooks.
// A single worker drains a bounded mailbox so MCP handlers never spawn unbounded goroutines; when the mailbox is full, notifications are dropped. Pre-ready notifications are buffered (bounded) and replayed by MarkReady.
func BindMCPNotify(p *tea.Program) MCPNotifyHooks {
	var ready atomic.Bool
	const mailboxSize = 16
	const maxPending = 16
	mailbox := make(chan tea.Msg, mailboxSize)
	var startOnce sync.Once
	var mu sync.Mutex
	pending := make([]tea.Msg, 0, 4)

	enqueue := func(msg tea.Msg) {
		startOnce.Do(func() {
			go func() {
				for m := range mailbox {
					p.Send(m)
				}
			}()
		})
		// Never block MCP HTTP handlers on the Bubble Tea mailbox.
		select {
		case mailbox <- msg:
		default:
			logger.Warn("mcp notify: TUI mailbox full, dropping", "type", fmt.Sprintf("%T", msg))
		}
	}

	send := func(msg tea.Msg) {
		if p == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if !ready.Load() {
			if len(pending) >= maxPending {
				logger.Warn("mcp notify: pre-ready buffer full, dropping", "type", fmt.Sprintf("%T", msg))
				return
			}
			pending = append(pending, msg)
			return
		}
		enqueue(msg)
	}

	return MCPNotifyHooks{
		OnTracesCleared:        func() { send(tracesClearedMsg{}) },
		OnBroadcastModeChanged: func(mode domain.BroadcastMode) { send(broadcastModeChangedMsg{mode: mode}) },
		OnDatabaseConnected:    func(id string) { send(mcpConnectDatabaseMsg{id: id}) },
		OnStopped:              func() { send(mcpStoppedMsg{}) },
		MarkReady: func() {
			mu.Lock()
			defer mu.Unlock()
			if ready.Load() {
				return
			}
			ready.Store(true)
			toReplay := pending
			pending = nil
			for _, msg := range toReplay {
				enqueue(msg)
			}
		},
	}
}
