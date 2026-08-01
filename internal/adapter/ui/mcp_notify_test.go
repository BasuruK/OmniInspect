package ui

import (
	"testing"
	"time"

	"OmniView/internal/core/domain"
)

func TestBindMCPNotify_SafeWithNilProgram(t *testing.T) {
	t.Parallel()

	hooks := BindMCPNotify(nil)
	hooks.OnTracesCleared()
	hooks.OnBroadcastModeChanged(domain.BroadcastModeSubscriber)
	hooks.OnDatabaseConnected("DBconfig:x")
	hooks.OnStopped()
	hooks.MarkReady()
	hooks.OnTracesCleared()
	hooks.OnStopped()
}

func TestBindMCPNotify_CallbacksReturnImmediately(t *testing.T) {
	t.Parallel()

	hooks := BindMCPNotify(NewProgram(&Model{}))
	hooks.MarkReady()

	done := make(chan struct{})
	go func() {
		defer close(done)
		hooks.OnTracesCleared()
		hooks.OnBroadcastModeChanged(domain.BroadcastModeGlobal)
		hooks.OnDatabaseConnected("id")
		hooks.OnStopped()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("MCP notify callbacks blocked the caller")
	}
}

func TestHandleMCPConnectDatabaseMsg_DropsWhenSwitchInProgress(t *testing.T) {
	t.Parallel()

	m := &Model{
		screen: screenLoading,
		loading: loadingState{
			started:  true,
			complete: false,
		},
	}
	next, cmd := m.handleMCPConnectDatabaseMsg(mcpConnectDatabaseMsg{id: "DBconfig:other"})
	if next != m {
		t.Fatal("expected same model")
	}
	if cmd != nil {
		t.Fatal("expected no cmd when switch in progress")
	}
}
