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
			m.main.renderedContent.Reset()
			for _, msg := range m.main.messages {
				m.main.renderedContent.WriteString(formatLogLine(msg))
				m.main.renderedContent.WriteString("\n")
			}
		}
		m.main.messages = append(m.main.messages, msg.message)
		// Incrementally append the new message to rendered content
		m.main.renderedContent.WriteString(formatLogLine(msg.message))
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
	bodyHeight := m.viewportHeight()

	// Header
	header := styles.HeaderStyle.Render("OmniView — Real-time Traces")

	// Help bar
	autoScrollIndicator := "off"
	if m.main.autoScroll {
		autoScrollIndicator = "on"
	}
	help := styles.HelpStyle.Render(
		fmt.Sprintf("↑/↓ scroll • a auto-scroll [%s] • q quit", autoScrollIndicator),
	)

	// Viewport
	viewportView := lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		Render(m.main.viewport.View())

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

// formatLogLine applies color styling based on log level.
func formatLogLine(msg *domain.QueueMessage) string {
	timestamp := msg.Timestamp().Format("2006-01-02 15:04:05")

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

	return fmt.Sprintf(
		"%s %s %s %s",
		lipgloss.NewStyle().Foreground(styles.MutedColor).Render(timestamp),
		levelStyle.Render(fmt.Sprintf("[%-8s]", msg.LogLevel())),
		lipgloss.NewStyle().Foreground(styles.SecondaryColor).Render(sanitizeLogString(msg.ProcessName())),
		sanitizeLogString(msg.Payload()),
	)
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
