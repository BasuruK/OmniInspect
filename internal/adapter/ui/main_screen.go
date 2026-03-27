package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Constants
// ==========================================

// headerHeight is the number of terminal lines reserved for header + help.
const headerHeight = 4

// maxMessages is the maximum number of messages to retain in the ring buffer.
// Oldest messages are dropped when capacity is reached.
// TODO: Make this configurable if users want to adjust memory usage vs. log history depth, consider when implementing main settings flow.
const maxMessages = 1000

// ansiEscape matches ANSI escape sequences for sanitization.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[()][AB012]|\x1b[0-9;]*[~^KL]|\x1b[12;[0-9]*[0-9]|[^\x20-\x7E]`)

// sanitizeLogString removes ANSI escapes and control characters from user-controlled
// log content to prevent terminal injection attacks.
func sanitizeLogString(s string) string {
	// Strip ANSI escape sequences
	s = ansiEscape.ReplaceAllString(s, "")
	// Replace control characters with visible placeholders
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return '·' // dot placeholder for control chars
		}
		if r == 0x7F { // DEL
			return '·'
		}
		return r
	}, s)
	// Collapse leading/trailing whitespace and limit length
	s = strings.TrimSpace(s)
	const maxLen = 10000
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	return s
}

// ==========================================
// Main Update
// ==========================================

// updateMain handles messages when screen == "main".
func (m *Model) updateMain(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	// New log message from event listener
	case queueMessageMsg:
		// Drop oldest message if at capacity to prevent unbounded growth
		if len(m.main.messages) >= maxMessages {
			m.main.messages = m.main.messages[1:]
			// Buffer exceeded — rebuild rendered content from trimmed slice
			m.rebuildRenderedContent()
		}
		m.main.messages = append(m.main.messages, msg.message)
		// Incrementally append the new message to rendered content
		m.main.renderedContent.WriteString(m.formatLogLine(msg.message))
		m.main.renderedContent.WriteString("\n")
		m.main.viewport.SetContent(m.main.renderedContent.String())
		if m.main.autoScroll {
			m.main.viewport.GotoBottom()
		}
		// Re-subscribe to wait for next message
		return m, waitForEventCmd(m.ctx, m.eventChannel)

	// Event channel closed (shutdown)
	case eventChannelClosedMsg:
		// Channel is closed — do not re-subscribe; goroutine exits cleanly
		return m, nil

	// Keyboard input
	case tea.KeyPressMsg:
		switch msg.String() {
		case "a":
			// Toggle auto-scroll
			m.main.autoScroll = !m.main.autoScroll
			if m.main.autoScroll {
				m.main.viewport.GotoBottom()
			}
			return m, nil
		}
	}

	// Forward all other messages to viewport (handles scrolling keys + mouse)
	var cmd tea.Cmd
	m.main.viewport, cmd = m.main.viewport.Update(msg)
	return m, cmd
}

// ==========================================
// Main View
// ==========================================

// viewMain renders the main log viewer screen.
func (m *Model) viewMain() string {
	// Header
	header := styles.HeaderStyle.Render("OmniView — Real-time Traces")

	// Help bar
	autoScrollIndicator := "off"
	if m.main.autoScroll {
		autoScrollIndicator = "on"
	}
	help := styles.HelpStyle.Render(
		fmt.Sprintf("↑/↓ scroll • a auto-scroll [%s] • [q] quit", autoScrollIndicator),
	)

	// Viewport (SoftWrap handles line breaking at configured width)
	viewportView := m.main.viewport.View()

	// Assemble layout
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		help,
		viewportView,
	)

	return content
}

// ==========================================
// Log Rendering
// ==========================================

// renderLogContent returns the current rendered log content.
// Uses the incrementally-built renderedContent when available,
// rebuilding only when the buffer is empty.
func (m *Model) renderLogContent() string {
	if len(m.main.messages) == 0 {
		return styles.SubtitleStyle.Render("  Waiting for trace events...")
	}
	// Return incrementally built content to avoid O(n²) rebuild on every message
	return m.main.renderedContent.String()
}

// formatLogLine applies color styling based on log level and wraps the payload
// column to fit within the terminal width. Continuation lines are indented to
// align with the start of the payload column.
func (m *Model) formatLogLine(msg *domain.QueueMessage) string {
	timestamp := msg.Timestamp().Format("2006-01-02 15:04:05")
	processName := sanitizeLogString(msg.ProcessName())
	payload := sanitizeLogString(msg.Payload())

	// Choose color based on log level
	var levelStyle lipgloss.Style
	switch msg.LogLevel() {
	case domain.LogLevelDebug:
		levelStyle = lipgloss.NewStyle().Foreground(styles.DebugColor)
	case domain.LogLevelInfo:
		levelStyle = lipgloss.NewStyle().Foreground(styles.InfoColor)
	case domain.LogLevelWarning:
		levelStyle = lipgloss.NewStyle().Foreground(styles.WarningColor)
	case domain.LogLevelError:
		levelStyle = lipgloss.NewStyle().Foreground(styles.ErrorColor)
	case domain.LogLevelCritical:
		levelStyle = lipgloss.NewStyle().Foreground(styles.CriticalColor).Bold(true)
	default:
		levelStyle = lipgloss.NewStyle().Foreground(styles.MutedColor)
	}

	// Build styled prefix parts
	styledTimestamp := lipgloss.NewStyle().Foreground(styles.MutedColor).Render(timestamp)
	styledLevel := levelStyle.Render(fmt.Sprintf("[%-8s]", msg.LogLevel()))
	styledProcess := lipgloss.NewStyle().Foreground(styles.SecondaryColor).Render(processName)

	// Calculate prefix display width: "timestamp [level   ] processName "
	// 19 + 1 + 10 + 1 + len(processName) + 1
	prefixWidth := 19 + 1 + 10 + 1 + len(processName) + 1

	// Available width for the payload column
	availWidth := m.width - prefixWidth
	if availWidth < 20 {
		availWidth = 20
	}

	// Wrap payload to fit within the available column width
	payloadLines := wrapText(payload, availWidth)

	// First line: full prefix + first payload segment
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s %s %s %s", styledTimestamp, styledLevel, styledProcess, payloadLines[0]))

	// Continuation lines: indented to align with payload column
	indent := strings.Repeat(" ", prefixWidth)
	for _, line := range payloadLines[1:] {
		b.WriteString("\n")
		b.WriteString(indent)
		b.WriteString(line)
	}
	return b.String()
}

// wrapText splits text into lines that fit within the given width limit.
// Breaks at word boundaries when possible, hard-wraps when a single word exceeds the limit.
func wrapText(text string, limit int) []string {
	if limit <= 0 || len(text) <= limit {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= limit {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
		// Hard-wrap if a single token exceeds the limit
		for len(currentLine) > limit {
			lines = append(lines, currentLine[:limit])
			currentLine = currentLine[limit:]
		}
	}
	lines = append(lines, currentLine)
	return lines
}

// rebuildRenderedContent re-wraps all messages at the current terminal width.
// Called when the terminal is resized.
func (m *Model) rebuildRenderedContent() {
	m.main.renderedContent.Reset()
	for _, msg := range m.main.messages {
		m.main.renderedContent.WriteString(m.formatLogLine(msg))
		m.main.renderedContent.WriteString("\n")
	}
	m.main.viewport.SetContent(m.main.renderedContent.String())
}

// initViewport creates and configures the viewport for the main screen.
// Called when we first receive terminal dimensions or transition to main screen.
func (m *Model) initViewport() {
	vpHeight := m.viewportHeight()

	m.main.viewport = viewport.New(
		viewport.WithWidth(m.width),
		viewport.WithHeight(vpHeight),
	)
	m.main.viewport.SetContent(m.renderLogContent())
	m.main.ready = true
}

// viewportHeight returns the available height for the viewport, accounting for header.
func (m *Model) viewportHeight() int {
	h := m.height - headerHeight
	if h < 1 {
		return 1
	}
	return h
}
