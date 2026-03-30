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
			name:     "multiple spaces preserved",
			text:     "Hello  world",
			width:    80,
			expected: "Hello  world",
		},
		{
			name:     "leading spaces preserved",
			text:     "  Hello",
			width:    80,
			expected: "  Hello",
		},
		{
			name:     "trailing spaces preserved",
			text:     "Hello  ",
			width:    80,
			expected: "Hello  ",
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

func TestWrapTextPreservesWhitespaceRunsAndIndentation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		width    int
		expected string
	}{
		{
			name:     "repeated spaces kept when wrapped",
			text:     "alpha  beta",
			width:    7,
			expected: "alpha\n  beta",
		},
		{
			name:     "indentation kept on wrapped line",
			text:     "    alpha beta",
			width:    10,
			expected: "    alpha\nbeta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			if result != tt.expected {
				t.Fatalf("wrapText(%q, %d) = %q, want %q", tt.text, tt.width, result, tt.expected)
			}

			for i, line := range strings.Split(result, "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Fatalf("line %d exceeds width %d: %q (width %d)", i+1, tt.width, line, lipgloss.Width(line))
				}
			}
		})
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

func TestQueueMessageBeforeViewportReadyBuffersWithoutRendering(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 30)
	msg := newTestQueueMessage(t)

	updated, _ := m.updateMain(queueMessageMsg{message: msg})

	if len(updated.main.messages) != 1 {
		t.Fatalf("expected one buffered message, got %d", len(updated.main.messages))
	}
	if updated.main.messages[0] != msg {
		t.Fatalf("expected buffered message to be retained before viewport init")
	}
	if updated.main.renderedContent.Len() != 0 {
		t.Fatalf("expected no rendered content before viewport init, got %q", updated.main.renderedContent.String())
	}
}

func TestQueueMessageBeforeViewportReadyEvictsWithoutRendering(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 30)
	oldest := newTestQueueMessageWithPayload(t, "msg-old", "oldest payload")
	filler := newTestQueueMessageWithPayload(t, "msg-fill", "filler payload")
	newest := newTestQueueMessageWithPayload(t, "msg-new", "newest payload")

	m.main.messages = append(m.main.messages, oldest)
	for i := 1; i < maxMessages; i++ {
		m.main.messages = append(m.main.messages, filler)
	}
	m.main.cachedLevelWidth = 9
	m.main.cachedAPIWidth = 17
	m.main.cachedWidthKey = 88

	updated, _ := m.updateMain(queueMessageMsg{message: newest})

	if len(updated.main.messages) != maxMessages {
		t.Fatalf("expected ring buffer size %d, got %d", maxMessages, len(updated.main.messages))
	}
	if updated.main.messages[0] == oldest {
		t.Fatalf("expected oldest message to be evicted before viewport init")
	}
	if updated.main.messages[len(updated.main.messages)-1] != newest {
		t.Fatalf("expected newest message to be appended after eviction")
	}
	if updated.main.renderedContent.Len() != 0 {
		t.Fatalf("expected no rendered content before viewport init, got %q", updated.main.renderedContent.String())
	}
	if updated.main.cachedLevelWidth != 0 || updated.main.cachedAPIWidth != 0 || updated.main.cachedWidthKey != 0 {
		t.Fatalf(
			"expected cache invalidation on pre-ready eviction, got level=%d api=%d widthKey=%d",
			updated.main.cachedLevelWidth,
			updated.main.cachedAPIWidth,
			updated.main.cachedWidthKey,
		)
	}
}

func TestInitViewportRebuildsBufferedMessagesAtViewportWidth(t *testing.T) {
	t.Parallel()

	m := newTestMainModel(t, 120, 30)
	msg := newTestQueueMessage(t)

	updated, _ := m.updateMain(queueMessageMsg{message: msg})
	updated.initViewport()

	expected := renderTraceColumns(parseTraceLine(msg), updated.traceColumnLayout(updated.main.viewport.Width())) + "\n"
	if updated.main.renderedContent.String() != expected {
		t.Fatalf("expected viewport init to rebuild buffered messages with actual width\n got: %q\nwant: %q", updated.main.renderedContent.String(), expected)
	}
	if updated.main.viewport.TotalLineCount() == 0 {
		t.Fatalf("expected viewport content to be populated after init")
	}
	if !updated.main.ready {
		t.Fatalf("expected viewport to be marked ready after init")
	}
	if !updated.main.viewport.AtBottom() {
		t.Fatalf("expected auto-scroll viewport to land at bottom after init")
	}
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
		dbFactory: func(_ *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
			return nil, nil // or a mock
		},
		main: mainState{
			autoScroll: true,
		},
	}
}

func newTestQueueMessage(t *testing.T) *domain.QueueMessage {
	t.Helper()

	return newTestQueueMessageWithPayload(
		t,
		"msg-1",
		"Hello world! This is a test string to verify that resizing the tracer viewport changes the panel size without making the frame wrap beyond the terminal width.",
	)
}

func newTestQueueMessageWithPayload(t *testing.T, id, payload string) *domain.QueueMessage {
	t.Helper()

	msg, err := domain.NewQueueMessage(
		id,
		"OMNI_TRACER_API",
		domain.LogLevelError,
		payload,
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
