package ui

import (
	"context"
	"testing"
	"time"

	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"

	tea "charm.land/bubbletea/v2"
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

func TestBindMCPNotify_BuffersBeforeReady(t *testing.T) {
	t.Parallel()

	hooks := BindMCPNotify(NewProgram(&Model{}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		hooks.OnDatabaseConnected("DBconfig:pre")
		hooks.OnTracesCleared()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pre-ready MCP notify blocked the caller")
	}
}

type notifyReplayModel struct {
	markReady func()
	got       chan string
}

func (m *notifyReplayModel) Init() tea.Cmd {
	if m.markReady != nil {
		m.markReady()
	}
	return nil
}

func (m *notifyReplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if c, ok := msg.(mcpConnectDatabaseMsg); ok {
		select {
		case m.got <- c.id:
		default:
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m *notifyReplayModel) View() tea.View {
	return tea.NewView("")
}

func TestBindMCPNotify_ReplaysPendingOnMarkReady(t *testing.T) {
	t.Parallel()

	got := make(chan string, 1)
	m := &notifyReplayModel{got: got}
	p := tea.NewProgram(m,
		tea.WithInput(nil),
		tea.WithoutRenderer(),
		tea.WithoutSignals(),
	)
	hooks := BindMCPNotify(p)
	m.markReady = hooks.MarkReady

	// Arrive before Run/Init — must be replayed once MarkReady fires from Init.
	hooks.OnDatabaseConnected("DBconfig:pre")

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	select {
	case id := <-got:
		if id != "DBconfig:pre" {
			t.Fatalf("got %q, want DBconfig:pre", id)
		}
	case err := <-done:
		t.Fatalf("program exited before replay: %v", err)
	case <-time.After(3 * time.Second):
		p.Quit()
		t.Fatal("pending mcpConnectDatabaseMsg not replayed after MarkReady")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		p.Quit()
		t.Fatal("program did not exit after replay")
	}
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

func TestBindMCPNotify_DropsWhenMailboxFull(t *testing.T) {
	t.Parallel()

	// Program is never Run, so the worker blocks on the first p.Send.
	// Flooding must still return immediately via drop-on-full.
	hooks := BindMCPNotify(NewProgram(&Model{}))
	hooks.MarkReady()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 64 {
			hooks.OnTracesCleared()
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("MCP notify flood blocked the caller")
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

func TestHandleMCPConnectDatabaseMsg_DropsWhenEditInProgress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		set  func(*Model)
	}{
		{
			name: "showAddForm",
			set: func(m *Model) {
				m.dbSettings.showAddForm = true
			},
		},
		{
			name: "editingID",
			set: func(m *Model) {
				m.dbSettings.editingID = "DBconfig:edit"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := &Model{screen: screenMain}
			tc.set(m)
			next, cmd := m.handleMCPConnectDatabaseMsg(mcpConnectDatabaseMsg{id: "DBconfig:other"})
			if next != m {
				t.Fatal("expected same model")
			}
			if cmd != nil {
				t.Fatal("expected no cmd when database settings edit in progress")
			}
			if m.screen != screenMain {
				t.Fatalf("expected screenMain, got %q", m.screen)
			}
		})
	}
}

func TestHandleMCPConnectDatabaseMsg_AdoptsDuringOnboarding(t *testing.T) {
	t.Parallel()

	settings := newTestDatabaseSettings(t, "OTHER")
	mockDB := NewMockDatabaseRepository()
	m := &Model{
		screen:         screenOnboarding,
		ctx:            context.Background(),
		dbSettingsRepo: mcpConnectSettingsRepo{byID: settings},
		dbFactory: func(*domain.DatabaseSettings) (ports.DatabaseRepository, error) {
			return mockDB, nil
		},
	}
	next, cmd := m.handleMCPConnectDatabaseMsg(mcpConnectDatabaseMsg{id: settings.StorageKey()})
	if next != m {
		t.Fatal("expected same model")
	}
	if cmd == nil {
		t.Fatal("expected connect cmd during onboarding adopt")
	}
	if m.screen != screenLoading {
		t.Fatalf("expected screenLoading, got %q", m.screen)
	}
	if m.appConfig == nil || m.appConfig.ID() != settings.ID() {
		t.Fatalf("appConfig not adopted: %#v", m.appConfig)
	}
}

type mcpConnectSettingsRepo struct {
	stubDatabaseSettingsRepository
	byID *domain.DatabaseSettings
}

func (r mcpConnectSettingsRepo) GetByID(context.Context, string) (*domain.DatabaseSettings, error) {
	return r.byID, nil
}

func TestHandleMCPConnectDatabaseMsg_AlreadyConnectedSyncsActiveID(t *testing.T) {
	t.Parallel()

	settings := newTestDatabaseSettings(t, "SAME-DB")
	m := &Model{
		screen:         screenMain,
		appConfig:      settings,
		dbSettingsRepo: mcpConnectSettingsRepo{byID: settings},
	}
	m.dbSettings.activeID = "stale-nav"

	next, cmd := m.handleMCPConnectDatabaseMsg(mcpConnectDatabaseMsg{id: settings.StorageKey()})
	if next != m {
		t.Fatal("expected same model")
	}
	if cmd != nil {
		t.Fatal("expected no cmd when already connected")
	}
	if m.screen != screenMain {
		t.Fatalf("expected screenMain, got %q", m.screen)
	}
	if m.dbSettings.activeID != settings.ID() {
		t.Fatalf("activeID=%q, want resolved ID %q", m.dbSettings.activeID, settings.ID())
	}
}
