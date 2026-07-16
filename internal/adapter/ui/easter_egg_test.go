package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Helper Functions
// ==========================================

// makeAltMKeyPress creates a tea.KeyPressMsg for the alt+m combination used to
// toggle the Easter Egg overlay.
func makeAltMKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'm', Text: "", Mod: tea.ModAlt}
}

// ==========================================
// Easter Egg Overlay Open/Close Tests
// ==========================================

func TestUpdateMain_AltMOpensEasterEggAndBumpsSession(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = false
	m.easterEggSession = 5

	updated, cmd := m.updateMain(makeAltMKeyPress())

	if !updated.showEasterEgg {
		t.Fatal("pressing alt+m should set showEasterEgg=true")
	}
	if updated.easterEggSession != 6 {
		t.Fatalf("alt+m should increment easterEggSession, got %d, want 6", updated.easterEggSession)
	}
	if cmd == nil {
		t.Fatal("alt+m should schedule an easterEggTickCmd, got nil cmd")
	}
	msg := cmd()
	tick, ok := msg.(easterEggTickMsg)
	if !ok {
		t.Fatalf("alt+m command should produce easterEggTickMsg, got %T", msg)
	}
	if tick.session != 6 {
		t.Fatalf("scheduled tick should carry the new session id, got %d, want 6", tick.session)
	}
}

func TestUpdateMain_AltMClosesEasterEgg(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = true

	updated, _ := m.updateMain(makeAltMKeyPress())

	if updated.showEasterEgg {
		t.Fatal("pressing alt+m when Easter Egg is open should set showEasterEgg=false")
	}
}

func TestUpdateMain_EscClosesEasterEgg(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = true

	updated, _ := m.updateMain(tea.KeyPressMsg{Code: tea.KeyEscape, Text: "", Mod: 0})

	if updated.showEasterEgg {
		t.Fatal("pressing Esc when Easter Egg is open should set showEasterEgg=false")
	}
}

func TestUpdateMain_OtherKeysConsumedWhenEasterEggOpen(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = true
	m.main.autoScroll = false

	// Press 'a' (toggle auto-scroll) — should be consumed, not processed.
	updated, _ := m.updateMain(makeCharPress("a"))

	if !updated.showEasterEgg {
		t.Fatal("Easter Egg should remain open when non-dismiss key is pressed")
	}
	if updated.main.autoScroll {
		t.Fatal("auto-scroll should NOT toggle when Easter Egg is open and 'a' is pressed")
	}

	// Regression: Press 'q' — should be consumed, not quit.
	final, cmd := updated.updateMain(makeCharPress("q"))
	if !final.showEasterEgg {
		t.Fatal("Easter Egg should remain open when 'q' is pressed")
	}
	if cmd != nil {
		t.Fatal("updateMain should return nil cmd for 'q' when Easter Egg is open — quit is handled at the root Update() level")
	}
}

// ==========================================
// Easter Egg Tick / Session Tests
// ==========================================

func TestUpdateMain_EasterEggTick_StepsPhysicsWhenSessionMatches(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = true
	m.easterEggSession = 3
	m.resetEasterEggPhysics()
	traceLenBefore := len(m.easterEggTrace)

	updated, cmd := m.updateMain(easterEggTickMsg{session: 3})

	if len(updated.easterEggTrace) != traceLenBefore+1 {
		t.Fatalf("tick with matching session should step physics and append one trace point, got len %d, want %d",
			len(updated.easterEggTrace), traceLenBefore+1)
	}
	if cmd == nil {
		t.Fatal("tick with matching session should reschedule the next tick, got nil cmd")
	}
	msg := cmd()
	next, ok := msg.(easterEggTickMsg)
	if !ok {
		t.Fatalf("rescheduled command should produce easterEggTickMsg, got %T", msg)
	}
	if next.session != 3 {
		t.Fatalf("rescheduled tick should carry the same session id, got %d, want 3", next.session)
	}
}

func TestUpdateMain_EasterEggTick_DiscardedWhenSessionStale(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = true
	m.easterEggSession = 3
	m.resetEasterEggPhysics()
	traceLenBefore := len(m.easterEggTrace)
	theta1Before := m.easterEggTheta1

	// A stale tick from a previous (session 2) animation loop, still in
	// flight after the overlay was reopened at session 3.
	updated, cmd := m.updateMain(easterEggTickMsg{session: 2})

	if len(updated.easterEggTrace) != traceLenBefore {
		t.Fatalf("stale tick must not step physics: trace len changed from %d to %d", traceLenBefore, len(updated.easterEggTrace))
	}
	if updated.easterEggTheta1 != theta1Before {
		t.Fatal("stale tick must not step physics: theta1 changed")
	}
	if cmd != nil {
		t.Fatal("stale tick must not reschedule another tick")
	}
}

func TestUpdateMain_EasterEggTick_DiscardedWhenOverlayClosed(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showEasterEgg = false
	m.easterEggSession = 1
	m.resetEasterEggPhysics()
	traceLenBefore := len(m.easterEggTrace)

	updated, cmd := m.updateMain(easterEggTickMsg{session: 1})

	if len(updated.easterEggTrace) != traceLenBefore {
		t.Fatal("tick must not step physics when the overlay is closed")
	}
	if cmd != nil {
		t.Fatal("tick must not reschedule another tick when the overlay is closed")
	}
}

// ==========================================
// Root Update() Quit-Blocking Tests
// ==========================================

// TestUpdate_QuitBlockedWhenEasterEggOpen verifies that the root Update() handler does
// NOT issue tea.Quit when the Easter Egg overlay is open and 'q' is pressed.
func TestUpdate_QuitBlockedWhenEasterEggOpen(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.screen = screenMain
	m.main.ready = true
	m.showEasterEgg = true

	_, cmd := m.Update(makeCharPress("q"))
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("Update() must not issue tea.Quit when the Easter Egg overlay is open")
		}
	}
}

// TestUpdate_CtrlCBlockedWhenEasterEggOpen verifies that the root Update() handler does
// NOT issue tea.Quit when the Easter Egg overlay is open and ctrl+c is pressed.
func TestUpdate_CtrlCBlockedWhenEasterEggOpen(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.screen = screenMain
	m.main.ready = true
	m.showEasterEgg = true

	_, cmd := m.Update(makeCtrlKeyPress('c'))
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("Update() must not issue tea.Quit when the Easter Egg overlay is open and ctrl+c is pressed")
		}
	}
}
