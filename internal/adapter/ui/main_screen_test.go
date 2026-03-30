package ui

import (
	"strings"
	"testing"
	"time"

	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// wrapText Tests
// ==========================================

func TestWrapTextBasic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		width    int
		expected string
	}{
		{
			name:     "empty string",
			text:     "",
			width:    80,
			expected: "",
		},
		{
			name:     "width zero returns original",
			text:     "Hello world",
			width:    0,
			expected: "Hello world",
		},
		{
			name:     "width negative returns original",
			text:     "Hello world",
			width:    -1,
			expected: "Hello world",
		},
		{
			name:     "single word fits",
			text:     "Hello",
			width:    80,
			expected: "Hello",
		},
		{
			name:     "multiple words fit on one line",
			text:     "Hello world",
			width:    80,
			expected: "Hello world",
		},
		{
			name:     "simple word wrap",
			text:     "Hello world this is a test",
			width:    15,
			expected: "Hello world\nthis is a test",
		},
		{
			name:     "preserves newlines",
			text:     "Hello\nworld",
			width:    80,
			expected: "Hello\nworld",
		},
		{
			name:     "newlines with wrapping",
			text:     "Hello world this\nis a test",
			width:    15,
			expected: "Hello world\nthis\nis a test",
		},
		{
			name:     "empty lines preserved",
			text:     "Hello\n\nworld",
			width:    80,
			expected: "Hello\n\nworld",
		},
		{
			name:     "multiple spaces collapsed to single",
			text:     "Hello  world",
			width:    80,
			expected: "Hello world",
		},
		{
			name:     "leading spaces collapsed",
			text:     "  Hello",
			width:    80,
			expected: "Hello",
		},
		{
			name:     "trailing spaces collapsed",
			text:     "Hello  ",
			width:    80,
			expected: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			if result != tt.expected {
				t.Errorf("wrapText(%q, %d) = %q, want %q", tt.text, tt.width, result, tt.expected)
			}
		})
	}
}

func TestWrapTextLongWords(t *testing.T) {
	t.Parallel()

	// Test that long words are split rather than truncated
	result := wrapText("superlongword", 10)
	// Should NOT contain ellipsis
	if strings.Contains(result, "…") {
		t.Errorf("wrapText should split long words, not truncate with ellipsis; got %q", result)
	}
	// Should preserve full word content
	fullWord := strings.ReplaceAll(result, "\n", "")
	if fullWord != "superlongword" {
		t.Errorf("wrapText should preserve entire word; got %q which concatenates to %q", result, fullWord)
	}
	// No line should exceed width
	for i, line := range strings.Split(result, "\n") {
		if lipgloss.Width(line) > 10 {
			t.Errorf("line %d exceeds width 10: %q (width %d)", i+1, line, lipgloss.Width(line))
		}
	}
}

func TestWrapTextPreservesSingleSpaces(t *testing.T) {
	t.Parallel()

	// Input has single spaces, output should have single spaces
	result := wrapText("Hello world this is a test", 80)
	expected := "Hello world this is a test"
	if result != expected {
		t.Errorf("expected single spaces preserved, got %q want %q", result, expected)
	}
}

func TestWrapTextNoEllipsis(t *testing.T) {
	t.Parallel()

	// Long words should be split, not truncated with ellipsis
	result := wrapText("superlongword", 10)
	// Should NOT contain ellipsis character
	if strings.Contains(result, "…") {
		t.Errorf("wrapText should split long words, not truncate with ellipsis; got %q", result)
	}
	// The word should be fully preserved (just split into chunks)
	fullWord := strings.ReplaceAll(result, "\n", "")
	if fullWord != "superlongword" {
		t.Errorf("wrapText should preserve entire word content; got %q", result)
	}
}

func TestWrapTextRealWorldPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		width int
	}{
		{
			name:  "SQL statement",
			text:  "SELECT user_id, username, email FROM users WHERE created_at > '2024-01-01'",
			width: 50,
		},
		{
			name:  "stack trace line",
			text:  "at com.example.app.service.UserService.getUser(UserService.java:42)",
			width: 60,
		},
		{
			name:  "long UUID",
			text:  "550e8400-e29b-41d4-a716-446655440000",
			width: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)

			// Verify no line exceeds width
			for i, line := range strings.Split(result, "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Errorf("line %d exceeds width %d: %q (width %d)", i+1, tt.width, line, lipgloss.Width(line))
				}
			}

			// Verify no ellipsis
			if strings.Contains(result, "…") {
				t.Errorf("result should not contain ellipsis: %q", result)
			}
		})
	}
}

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
	if layout.levelWidth != lipgloss.Width("[ERROR]") {
		t.Fatalf("expected level column to shrink to visible content width, got %d", layout.levelWidth)
	}
	if layout.apiWidth >= colMaxAPIWidth {
		t.Fatalf("expected API column to shrink below max width, got %d", layout.apiWidth)
	}
	if layout.apiWidth < lipgloss.Width("OMNI_TRACER_API") {
		t.Fatalf("expected API column to fit process name, got %d", layout.apiWidth)
	}
	if layout.payloadWidth <= 100-(colTimestampWidth+colMaxLevelWidth+colMaxAPIWidth+3) {
		t.Fatalf("expected payload width to grow when API column shrinks, got %d", layout.payloadWidth)
	}
}

func newTestMainModel(t *testing.T, width, height int) *Model {
	t.Helper()

	return &Model{
		screen: screenMain,
		width:  width,
		height: height,
		dbFactory: func(_ *domain.DatabaseSettings) ports.DatabaseRepository {
			return nil // or a mock
		},
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
