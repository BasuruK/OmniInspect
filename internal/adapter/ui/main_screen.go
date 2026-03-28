package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"log"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Constants
// ==========================================

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
			return '?' // ASCII placeholder for control chars
		}
		if r == 0x7F { // DEL
			return '?' // ASCII placeholder for DEL
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
	case dbValidationResultMsg, dbSwitchResultMsg:
		if m.dbSettings.visible {
			return m.updateDatabaseSettings(msg)
		}
		return m, nil

	// New log message from event listener
	case queueMessageMsg:
		// Drop oldest message if at capacity to prevent unbounded growth
		if len(m.main.messages) >= maxMessages {
			m.main.messages = m.main.messages[1:]
			// Buffer exceeded — rebuild rendered content from trimmed slice
			m.main.renderedContent.Reset()
			for _, queuedMsg := range m.main.messages {
				m.main.renderedContent.WriteString(m.formatLogLine(queuedMsg))
				m.main.renderedContent.WriteString("\n")
			}
		} else {
			// Incrementally append the new message to rendered content
			m.main.renderedContent.WriteString(m.formatLogLine(msg.message))
			m.main.renderedContent.WriteString("\n")
		}
		m.main.messages = append(m.main.messages, msg.message)
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
		if m.dbSettings.visible {
			return m.updateDatabaseSettings(msg)
		}
		switch msg.String() {
		case "a":
			// Toggle auto-scroll
			m.main.autoScroll = !m.main.autoScroll
			if m.main.autoScroll {
				m.main.viewport.GotoBottom()
			}
			return m, nil
		case "d":
			// Open database settings
			activeID := ""
			if m.appConfig != nil {
				activeID = m.appConfig.ID()
			}
			databases, err := m.dbSettingsRepo.GetAll(m.ctx)
			if err != nil {
				log.Printf("[UI] Failed to load database settings: %v", err)
				databases = []domain.DatabaseSettings{}
			}
			m.initDatabaseSettings(databases, activeID)
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

// mainLayoutParts holds the computed layout pieces for the main screen.
type mainLayoutParts struct {
	header         string
	statusBar      string
	footer         string
	panelHeight    int
	panelWidth     int
	viewportWidth  int
	viewportHeight int
}

// computeMainLayout computes all layout pieces for the main screen.
// Centralizes the height calculations and layout logic in one place.
func (m *Model) computeMainLayout() mainLayoutParts {
	contentWidth, contentHeight := screenContentSize(m.width, m.height)
	header := renderScreenHeader(
		contentWidth,
		"OmniView Trace Console",
		m.mainSubtitle(),
		m.mainConnectionMeta(),
	)
	statusBar := renderInfoBar(contentWidth, m.mainStatusText())
	footer := renderFooterBar(contentWidth, m.mainFooterText())

	panelHeight := max(
		contentHeight-lipgloss.Height(header)-lipgloss.Height(statusBar)-lipgloss.Height(footer)-panelHeightCompensation,
		minPanelHeight,
	)
	panelWidth, viewportWidth, viewportHeight := m.mainViewportDimensions(contentWidth, panelHeight)

	return mainLayoutParts{
		header:         header,
		statusBar:      statusBar,
		footer:         footer,
		panelHeight:    panelHeight,
		panelWidth:     panelWidth,
		viewportWidth:  viewportWidth,
		viewportHeight: viewportHeight,
	}
}

// ==========================================
// Main View
// ==========================================

// viewMain renders the main log viewer screen.
func (m *Model) viewMain() string {
	layout := m.computeMainLayout()

	viewportView := styles.ViewportStyle.
		Width(layout.viewportWidth).
		Height(layout.viewportHeight).
		Render(m.main.viewport.View())

	logPanel := applyTotalSize(styles.PrimaryPanelStyle, layout.panelWidth, layout.panelHeight).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			styles.SectionTitleStyle.Render("Live Trace Feed"),
			styles.SubtitleStyle.Render("Awaiting Trace Messages..."),
			"",
			viewportView,
		))

	return renderScreen(
		m.width,
		m.height,
		layout.header,
		"",
		layout.statusBar,
		"",
		logPanel,
		"",
		layout.footer,
	)
}

// ==========================================
// Log Rendering
// ==========================================

// renderLogContent returns the current rendered log content.
// Uses the incrementally-built renderedContent when available,
// rebuilding only when the buffer is empty.
func (m *Model) renderLogContent() string {
	if len(m.main.messages) == 0 {
		return styles.EmptyStateStyle.Render("Waiting for trace events from Oracle AQ...")
	}
	// Return incrementally built content to avoid O(n²) rebuild on every message
	return m.main.renderedContent.String()
}

// formatLogLine applies color styling based on log level and wraps the payload
// column to fit within the terminal width. Continuation lines are indented to
// align with the start of the payload column.
func (m *Model) formatLogLine(msg *domain.QueueMessage) string {
	timestamp := msg.Timestamp().Format("2006-01-02 15:04:05")

	// Choose color based on log level
	var levelStyle lipgloss.Style
	switch msg.LogLevel() {
	case domain.LogLevelDebug:
		levelStyle = styles.LogLevelStyle.Foreground(styles.DebugColor)
	case domain.LogLevelInfo:
		levelStyle = styles.LogLevelStyle.Foreground(styles.InfoColor)
	case domain.LogLevelWarning:
		levelStyle = styles.LogLevelStyle.Foreground(styles.WarningColor)
	case domain.LogLevelError:
		levelStyle = styles.LogLevelStyle.Foreground(styles.ErrorColor)
	case domain.LogLevelCritical:
		levelStyle = styles.LogLevelStyle.Foreground(styles.CriticalColor)
	default:
		levelStyle = styles.LogLevelStyle.Foreground(styles.MutedColor)
	}

	return fmt.Sprintf(
		"%s %s %s %s",
		styles.LogTimestampStyle.Render(timestamp),
		levelStyle.Render(fmt.Sprintf("[%-8s]", msg.LogLevel())),
		styles.LogProcessStyle.Render(sanitizeLogString(msg.ProcessName())),
		sanitizeLogString(msg.Payload()),
	)
}

// initViewport creates and configures the viewport for the main screen.
// Called when we first receive terminal dimensions or transition to main screen.
func (m *Model) initViewport() {
	layout := m.computeMainLayout()

	m.main.viewport = viewport.New(
		viewport.WithWidth(layout.viewportWidth),
		viewport.WithHeight(layout.viewportHeight),
	)
	m.main.viewport.SetContent(m.renderLogContent())
	m.main.ready = true

	// Rebuild rendered content with real viewport width in case messages were
	// buffered before the viewport was initialized (formatted with the fallback
	// column width). No-op when there are no messages.
	if len(m.main.messages) > 0 {
		m.main.renderedContent.Reset()
		for _, queuedMsg := range m.main.messages {
			m.main.renderedContent.WriteString(m.formatLogLine(queuedMsg))
			m.main.renderedContent.WriteString("\n")
		}
		m.main.viewport.SetContent(m.main.renderedContent.String())
	}
}

func (m *Model) mainViewportDimensions(contentWidth, panelHeight int) (int, int, int) {
	panelWidth := max(contentWidth, 20)
	panelHorizontalFrame, panelVerticalFrame := styles.PrimaryPanelStyle.GetFrameSize()
	panelTextHeight := lipgloss.Height(styles.SectionTitleStyle.Render("Live Trace Feed")) +
		lipgloss.Height(styles.SubtitleStyle.Render("Awaiting Trace Messages...")) +
		1

	viewportWidth := max(panelWidth-panelHorizontalFrame, 10)
	viewportHeight := max(panelHeight-panelVerticalFrame-panelTextHeight, 1)

	return panelWidth, viewportWidth, viewportHeight
}

func (m *Model) mainSubtitle() string {
	if m.appConfig == nil {
		return "Live Oracle trace viewer"
	}

	return fmt.Sprintf(
		"%s@%s • %s:%d",
		m.appConfig.Username(),
		m.appConfig.Database(),
		m.appConfig.Host(),
		m.appConfig.Port().Int(),
	)
}

// mainConnectionMeta returns a string with connection details for the status bar. TODO: Add connection health details here in the future.
func (m *Model) mainConnectionMeta() string {
	return ""
}

func (m *Model) mainStatusText() string {
	autoScroll := styles.WarningColor
	autoScrollText := "manual"
	if m.main.autoScroll {
		autoScroll = styles.SuccessColor
		autoScrollText = "on"
	}

	subscriberName := "pending"
	if m.subscriber != nil {
		subscriberName = truncate(m.subscriber.Name(), 20)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		styles.BodyTextStyle.Render("Sub "+subscriberName),
		styles.SubtitleStyle.Render("  •  "),
		lipgloss.NewStyle().Foreground(autoScroll).Bold(true).Render("Auto Scroll "+autoScrollText),
		styles.SubtitleStyle.Render("  •  "),
		styles.BodyTextStyle.Render(fmt.Sprintf("Messages %d/%d", len(m.main.messages), maxMessages)),
	)
}

func (m *Model) mainFooterText() string {
	return "↑/↓ Scroll  •  A Auto Scroll [on/off]  •  D Database Settings  •  Q Quit"
}
