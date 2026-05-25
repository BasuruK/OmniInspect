package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Help Overlay Tests
// ==========================================

func TestRenderHelpOverlay_ContainsSubscriberProcedureName(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.subscriber = mustNewTestSubscriberWithFunnyName(t, "SUB_TEST", "BARNACLE")

	overlay := m.renderHelpOverlay()

	if !strings.Contains(overlay, "TRACE_MESSAGE_BARNACLE") {
		t.Fatalf("help overlay should contain subscriber procedure name, got: %s", overlay)
	}
}

func TestRenderHelpOverlay_WithoutSubscriberShowsPlaceholder(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	// no subscriber set

	overlay := m.renderHelpOverlay()

	if !strings.Contains(overlay, "TRACE_MESSAGE_<YOUR_NAME>") {
		t.Fatalf("help overlay should contain placeholder when no subscriber, got: %s", overlay)
	}
}

func TestRenderHelpOverlay_ContainsAllSections(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)

	overlay := m.renderHelpOverlay()

	sections := []string{
		"Trace_Message",        // global broadcast method
		"Database Management",  // section 3
		"Webhook",              // section 4
		"Filtering",            // section 5
		"H or Esc",             // close hint
	}
	for _, expected := range sections {
		if !strings.Contains(overlay, expected) {
			t.Fatalf("help overlay should contain %q", expected)
		}
	}
}

func TestMainFooterText_ContainsHelpHint(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)

	footer := m.mainFooterText()

	if !strings.Contains(footer, "H Help") {
		t.Fatalf("footer should contain 'H Help', got: %s", footer)
	}
}

func TestUpdateMain_HKeyOpensHelp(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showHelp = false

	updated, _ := m.updateMain(makeCharPress("h"))

	if !updated.showHelp {
		t.Fatal("pressing H should set showHelp=true")
	}
}

func TestUpdateMain_HKeyClosesHelp(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showHelp = true

	updated, _ := m.updateMain(makeCharPress("h"))

	if updated.showHelp {
		t.Fatal("pressing H when help is open should set showHelp=false")
	}
}

func TestUpdateMain_EscKeyClosesHelp(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showHelp = true

	updated, _ := m.updateMain(tea.KeyPressMsg{Code: tea.KeyEscape, Text: "", Mod: 0})

	if updated.showHelp {
		t.Fatal("pressing Esc when help is open should set showHelp=false")
	}
}

func TestUpdateMain_OtherKeysConsumedWhenHelpOpen(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	m.showHelp = true
	m.main.autoScroll = false

	// Press 'a' (toggle auto-scroll) — should be consumed, not processed
	updated, _ := m.updateMain(makeCharPress("a"))

	if !updated.showHelp {
		t.Fatal("help should remain open when non-dismiss key is pressed")
	}
	if updated.main.autoScroll {
		t.Fatal("auto-scroll should NOT toggle when help is open and 'a' is pressed")
	}
}
