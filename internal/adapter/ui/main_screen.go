package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
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

// ==========================================
// Main Update
// ==========================================

// updateMain handles messages when screen == "main".
func (m *Model) updateMain(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	// New log message from event listener
	case queueMessageMsg:
		m.main.messages = append(m.main.messages, msg.message)
		m.main.viewport.SetContent(m.renderLogContent())
		if m.main.autoScroll {
			m.main.viewport.GotoBottom()
		}
		// Re-subscribe to wait for next message
		return m, waitForEventCmd(m.eventChannel)

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
		fmt.Sprintf("↑/↓ scroll • a auto-scroll [%s] • q quit", autoScrollIndicator),
	)

	// Viewport
	viewportView := m.main.viewport.View()

	// Assemble layout
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		help,
		viewportView,
	)
}

// ==========================================
// Log Rendering
// ==========================================

// renderLogContent formats all stored messages as a single string for the viewport.
func (m *Model) renderLogContent() string {
	if len(m.main.messages) == 0 {
		return styles.SubtitleStyle.Render("  Waiting for trace events...")
	}

	var b strings.Builder
	for _, msg := range m.main.messages {
		b.WriteString(formatLogLine(msg))
		b.WriteString("\n")
	}
	return b.String()
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
		lipgloss.NewStyle().Foreground(styles.SecondaryColor).Render(msg.ProcessName()),
		msg.Payload(),
	)
}

// initViewport creates and configures the viewport for the main screen.
// Called when we first receive terminal dimensions or transition to main screen.
func (m *Model) initViewport() {
	vpHeight := m.height - headerHeight
	if vpHeight < 1 {
		vpHeight = 1
	}

	m.main.viewport = viewport.New(
		viewport.WithWidth(m.width),
		viewport.WithHeight(vpHeight),
	)
	m.main.viewport.SetContent(m.renderLogContent())
	m.main.ready = true
}
