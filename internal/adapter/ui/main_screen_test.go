package ui

import (
	"strings"
	"testing"
	"time"

	"OmniView/internal/core/domain"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Main Screen Layout Tests
// ==========================================

func TestScreenContentSizeDoesNotOverflowSmallTerminal(t *testing.T) {
	t.Parallel()

	width, height := screenContentSize(10, 6)
	if width > 10 {
		t.Fatalf("content width overflowed terminal: got %d, terminal %d", width, 10)
	}
	if height > 6 {
		t.Fatalf("content height overflowed terminal: got %d, terminal %d", height, 6)
	}
	if width < 1 || height < 1 {
		t.Fatalf("content size must stay positive, got %dx%d", width, height)
	}
}

func TestComputeMainLayoutFitsAvailableHeight(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 36)
	layout := m.computeMainLayout()
	contentWidth, contentHeight := screenContentSize(m.width, m.height)

	if layout.panelWidth != contentWidth {
		t.Fatalf("panel width mismatch: got %d want %d", layout.panelWidth, contentWidth)
	}

	totalHeight := lipgloss.Height(layout.header) +
		lipgloss.Height(layout.statusBar) +
		lipgloss.Height(layout.footer) +
		layout.panelHeight +
		mainGapAfterHeader +
		mainGapAfterStatus +
		mainGapAfterPanel

	if totalHeight != contentHeight {
		t.Fatalf("layout height mismatch: got %d want %d", totalHeight, contentHeight)
	}
}

func TestMainViewStaysWithinTerminalAfterResize(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 64, 24)
	m.initViewport()
	m.main.messages = []*domain.QueueMessage{newTestQueueMessage(t)}
	m.rebuildRenderedContent(m.main.viewport.Width())

	initial := m.viewMain()
	assertRenderedWithinTerminal(t, initial, m.width, m.height)

	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	updated, ok := updatedModel.(*Model)
	if !ok {
		t.Fatalf("update returned unexpected model type %T", updatedModel)
	}

	layout := updated.computeMainLayout()
	if updated.main.viewport.Width() != layout.viewportWidth {
		t.Fatalf("viewport width mismatch after resize: got %d want %d", updated.main.viewport.Width(), layout.viewportWidth)
	}
	if updated.main.viewport.Height() != layout.viewportHeight {
		t.Fatalf("viewport height mismatch after resize: got %d want %d", updated.main.viewport.Height(), layout.viewportHeight)
	}

	rendered := updated.viewMain()
	assertRenderedWithinTerminal(t, rendered, updated.width, updated.height)
}

func TestTraceColumnLayoutShrinksAPIColumnForShortNames(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 30)
	m.main.messages = []*domain.QueueMessage{
		newTestQueueMessage(t),
	}

	layout := m.traceColumnLayout(100)
	if layout.apiWidth >= colMaxAPIWidth {
		t.Fatalf("expected API column to shrink below max width, got %d", layout.apiWidth)
	}
	if layout.apiWidth < lipgloss.Width("OMNI_TRACER_API") {
		t.Fatalf("expected API column to fit process name, got %d", layout.apiWidth)
	}
	if layout.payloadWidth <= 100-(colTimestampWidth+colLevelWidth+colMaxAPIWidth+3) {
		t.Fatalf("expected payload width to grow when API column shrinks, got %d", layout.payloadWidth)
	}
}

func newTestMainModel(t *testing.T, width, height int) *Model {
	t.Helper()

	return &Model{
		screen: screenMain,
		width:  width,
		height: height,
		main: mainState{
			autoScroll: true,
		},
	}
}

func newTestQueueMessage(t *testing.T) *domain.QueueMessage {
	t.Helper()

	msg, err := domain.NewQueueMessage(
		"msg-1",
		"OMNI_TRACER_API",
		domain.LogLevelError,
		"Hello world! This is a test string to verify that resizing the tracer viewport changes the panel size without making the frame wrap beyond the terminal width.",
		time.Date(2026, time.March, 30, 12, 37, 28, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("failed to create queue message: %v", err)
	}

	return msg
}

func assertRenderedWithinTerminal(t *testing.T, rendered string, width, height int) {
	t.Helper()

	if got := lipgloss.Width(rendered); got != width {
		t.Fatalf("rendered width mismatch: got %d want %d", got, width)
	}
	if got := lipgloss.Height(rendered); got != height {
		t.Fatalf("rendered height mismatch: got %d want %d", got, height)
	}

	for i, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d exceeded terminal width: got %d want <= %d", i+1, got, width)
		}
	}
}
