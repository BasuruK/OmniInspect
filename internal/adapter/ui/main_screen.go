package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"log"
	"regexp"
	"slices"
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
const (
	maxMessages         = 1000
	maxProcessNameWidth = 20
	mainGapAfterHeader  = 1
	mainGapAfterStatus  = 0
	mainGapAfterPanel   = 0
)

// ==========================================
// Trace Column Definitions
// ==========================================

const (
	// Column widths for trace line formatting
	colTimestampWidth  = 19 // "2006-01-02 15:04:05"
	colMinLevelWidth   = 7
	colMaxLevelWidth   = 10 // "[CRITICAL]" - max level length with brackets
	colMinAPIWidth     = 10
	colMaxAPIWidth     = 20
	colMinPayloadWidth = 24
	colMinWidth        = colTimestampWidth + colMinLevelWidth + colMinAPIWidth + colMinPayloadWidth + 3

	// Column separator - simple spacing without visible dividers
	colSeparator = " " // Single space between columns
)

// traceLine represents a parsed trace message with columnar data
type traceLine struct {
	timestamp  string
	level      string
	levelStyle lipgloss.Style
	api        string
	payload    string
	raw        *domain.QueueMessage
}

type traceColumnLayout struct {
	timestampWidth int
	levelWidth     int
	apiWidth       int
	payloadWidth   int
}

// ansiEscape matches ANSI escape sequences for sanitization.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[()][AB012]|\x1b[0-9;]*[~^KL]|\x1b[12;[0-9]*[0-9]|[^\x20-\x7E]`)
var wrapTextTokenPattern = regexp.MustCompile(`\s+|\S+`)

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
		if len(m.main.messages) >= maxMessages {
			// Ring-buffer shift — old content is invalid; full rebuild required.
			m.main.messages = m.main.messages[1:]
			m.main.messages = append(m.main.messages, msg.message)
			// Invalidate the cached column widths when the ring buffer evicts a row.
			m.main.cachedLevelWidth = 0
			m.main.cachedAPIWidth = 0
			m.main.cachedWidthKey = 0
			if m.main.ready {
				m.rebuildRenderedContent(m.main.viewport.Width())
			}
		} else {
			// Fast path: append the message, then render only the new line.
			m.main.messages = append(m.main.messages, msg.message)
			if m.main.ready {
				m.appendSingleMessage(msg.message, m.main.viewport.Width())
			}
		}
		if m.main.ready && m.main.autoScroll {
			m.main.viewport.GotoBottom()
		}
		return m, waitForEventCmd(m.ctx, m.eventChannel)

	// Event channel closed (shutdown)
	case eventChannelClosedMsg:
		// Channel is closed — do not re-subscribe; goroutine exits cleanly
		return m, nil

	// Paste events — delegate to database settings overlay when visible
	case tea.PasteMsg:
		if m.dbSettings.visible {
			return m.updateDatabaseSettings(msg)
		}

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
		case "c":
			// Clear all messages
			m.main.messages = nil
			m.main.renderedContent.Reset()
			m.main.viewport.SetContent(m.renderLogContent())
			m.main.viewport.GotoTop()
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

	// Reserve one blank spacer line between each main section so the panel height
	// calculation matches the final rendered layout exactly.
	sectionGapCount := mainGapAfterHeader + mainGapAfterStatus + mainGapAfterPanel
	availableForPanel := contentHeight -
		lipgloss.Height(header) -
		lipgloss.Height(statusBar) -
		lipgloss.Height(footer) -
		sectionGapCount

	// Ensure the panel height doesn't shrink below the minimum usable height, even on very small terminals.
	panelHeight := max(availableForPanel, minPanelHeight, 1)
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

// repeatSectionGaps
func repeatSectionGaps(count int) []string {
	return slices.Repeat([]string{""}, count)
}

// ==========================================
// Main View
// ==========================================

// viewMain renders the main log viewer screen.
func (m *Model) viewMain() string {
	layout := m.computeMainLayout()

	viewportView := m.main.viewport.View()

	logPanelContent := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.SectionTitleStyle.Render("Live Trace Feed"),
		styles.SubtitleStyle.Render("Awaiting Trace Messages..."),
		"",
		viewportView,
	)

	logPanel := applyTotalSize(styles.PrimaryPanelStyle, layout.panelWidth, layout.panelHeight).Render(logPanelContent)

	sections := []string{layout.header}
	sections = append(sections, repeatSectionGaps(mainGapAfterHeader)...)
	sections = append(sections, layout.statusBar)
	sections = append(sections, repeatSectionGaps(mainGapAfterStatus)...)
	sections = append(sections, logPanel)
	sections = append(sections, repeatSectionGaps(mainGapAfterPanel)...)
	sections = append(sections, layout.footer)

	return renderScreen(m.width, m.height, sections...)
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

// formatLogLine applies color styling based on log level and returns a plain
// prefix-plus-payload line.
func (m *Model) formatLogLine(msg *domain.QueueMessage) string {

	timestamp := msg.Timestamp().Format("2006-01-02 15:04:05")

	levelStyle := getLevelStyle(msg.LogLevel())

	renderedTimestamp := styles.LogTimestampStyle.Render(timestamp)
	renderedLevel := levelStyle.Render(fmt.Sprintf("[%-8s]", msg.LogLevel()))
	renderedProcess := styles.LogProcessStyle.Render(truncate(sanitizeLogString(msg.ProcessName()), maxProcessNameWidth))
	prefix := renderedTimestamp + " " + renderedLevel + " " + renderedProcess + " "

	payload := sanitizeLogString(msg.Payload())
	if payload == "" {
		return prefix
	}

	return prefix + payload
}

// parseTraceLine extracts structured data from a QueueMessage
func parseTraceLine(msg *domain.QueueMessage) traceLine {
	return traceLine{
		timestamp:  msg.Timestamp().Format("2006-01-02 15:04:05"),
		level:      formatTraceLevel(msg.LogLevel()),
		levelStyle: getLevelStyle(msg.LogLevel()),
		api:        truncate(sanitizeLogString(msg.ProcessName()), colMaxAPIWidth),
		payload:    sanitizeLogString(msg.Payload()),
		raw:        msg,
	}
}

// getLevelStyle returns the lipgloss style for a given log level
func getLevelStyle(level domain.LogLevel) lipgloss.Style {
	base := styles.LogLevelStyle
	switch level {
	case domain.LogLevelDebug:
		return base.Foreground(styles.DebugColor)
	case domain.LogLevelInfo:
		return base.Foreground(styles.InfoColor)
	case domain.LogLevelWarning:
		return base.Foreground(styles.WarningColor)
	case domain.LogLevelError:
		return base.Foreground(styles.ErrorColor)
	case domain.LogLevelCritical:
		return base.Foreground(styles.CriticalColor)
	default:
		return base.Foreground(styles.MutedColor)
	}
}

func formatTraceLevel(level domain.LogLevel) string {
	return fmt.Sprintf("[%s]", level)
}

// renderTraceColumns renders a traceLine as a fixed-width formatted string with word-wrap.
func renderTraceColumns(line traceLine, layout traceColumnLayout) string {
	if layout.payloadWidth < 5 {
		// Fallback to compact format if too narrow
		return renderCompactLine(line)
	}

	// Build column styles
	tsStyle := styles.LogTimestampStyle.Width(layout.timestampWidth)
	lvlStyle := line.levelStyle.Width(layout.levelWidth)
	apiStyle := styles.LogProcessStyle.Width(layout.apiWidth)
	payStyle := lipgloss.NewStyle().Width(layout.payloadWidth)

	// Word-wrap the payload text to fit within payloadWidth
	wrappedPayload := wrapText(line.payload, layout.payloadWidth)
	payloadLines := strings.Split(wrappedPayload, "\n")

	// Build continuation line indent (spaces for fixed columns + separator)
	indent := strings.Repeat(" ", layout.timestampWidth+layout.levelWidth+layout.apiWidth+(len(colSeparator)*3))

	var result strings.Builder

	// Render first line with columns
	result.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Top,
		tsStyle.Render(line.timestamp),
		colSeparator,
		lvlStyle.Render(line.level),
		colSeparator,
		apiStyle.Render(line.api),
		colSeparator,
		payStyle.Render(payloadLines[0]),
	))

	// Render continuation lines if payload wrapped
	for i := 1; i < len(payloadLines); i++ {
		result.WriteString("\n")
		result.WriteString(indent)
		result.WriteString(payloadLines[i])
	}

	return result.String()
}

// wrapText wraps text to fit within the specified width using simple word-wrapping.
// It preserves original newlines and splits long tokens into width-sized chunks
// rather than truncating with an ellipsis.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	// Split by newlines to preserve original line structure
	lines := strings.Split(text, "\n")
	var result strings.Builder

	for lineIndex, line := range lines {
		if lineIndex > 0 {
			result.WriteString("\n")
		}

		if line == "" {
			continue
		}

		tokens := wrapTextTokenPattern.FindAllString(line, -1)
		if len(tokens) == 0 {
			continue
		}

		var currentLine strings.Builder
		currentWidth := 0
		pendingWhitespace := ""

		appendToken := func(token string) {
			tokenWidth := lipgloss.Width(token)
			if tokenWidth <= width {
				currentLine.WriteString(token)
				currentWidth += tokenWidth
				return
			}

			runes := []rune(token)
			chunkWidth := 0
			chunkStart := 0

			for chunkStart < len(runes) {
				chunkEnd := chunkStart
				for chunkEnd < len(runes) && chunkWidth < width {
					chunkWidth += lipgloss.Width(string(runes[chunkEnd]))
					if chunkWidth > width {
						break
					}
					chunkEnd++
				}

				chunk := string(runes[chunkStart:chunkEnd])
				if chunkEnd == len(runes) {
					currentLine.WriteString(chunk)
					currentWidth = lipgloss.Width(chunk)
				} else {
					result.WriteString(chunk)
					result.WriteString("\n")
				}
				chunkStart = chunkEnd
				chunkWidth = 0
			}
		}

		for _, token := range tokens {
			tokenWidth := lipgloss.Width(token)
			if strings.TrimSpace(token) == "" {
				if currentWidth == 0 {
					appendToken(token)
				} else {
					pendingWhitespace += token
				}
				continue
			}

			separator := pendingWhitespace
			if currentWidth == 0 && separator == " " {
				separator = ""
			}
			combinedWidth := lipgloss.Width(separator) + tokenWidth

			if currentWidth > 0 && currentWidth+combinedWidth > width {
				result.WriteString(currentLine.String())
				result.WriteString("\n")
				currentLine.Reset()
				currentWidth = 0
				separator = pendingWhitespace
				if separator == " " {
					separator = ""
				}
			}

			if separator != "" {
				appendToken(separator)
			}
			appendToken(token)
			pendingWhitespace = ""
		}

		if pendingWhitespace != "" {
			if currentWidth+lipgloss.Width(pendingWhitespace) > width && currentLine.Len() > 0 {
				result.WriteString(currentLine.String())
				result.WriteString("\n")
				currentLine.Reset()
				currentWidth = 0
			}
			appendToken(pendingWhitespace)
		}

		// Write the last line
		if currentLine.Len() > 0 {
			result.WriteString(currentLine.String())
		}
	}

	return result.String()
}

// renderCompactLine is a fallback format for narrow terminals
func renderCompactLine(line traceLine) string {
	tsStyle := styles.LogTimestampStyle
	lvlStyle := line.levelStyle
	apiStyle := styles.LogProcessStyle

	return tsStyle.Render(line.timestamp) + " " +
		lvlStyle.Render(line.level) + " " +
		apiStyle.Render(truncate(line.api, 15)) + " " +
		line.payload
}

// initViewport creates and configures the viewport for the main screen.
// Called when we first receive terminal dimensions or transition to main screen.
func (m *Model) initViewport() {
	layout := m.computeMainLayout()

	m.main.viewport = viewport.New(
		viewport.WithWidth(layout.viewportWidth),
		viewport.WithHeight(layout.viewportHeight),
	)
	m.main.ready = true
	m.rebuildRenderedContent(layout.viewportWidth)
	if m.main.autoScroll {
		m.main.viewport.GotoBottom()
	}
}

// resizeMainViewport keeps the viewport dimensions and rendered content aligned
// with the current terminal size. Width changes require a full content rebuild
// because trace payload wrapping is derived from the viewport width.
func (m *Model) resizeMainViewport() {
	layout := m.computeMainLayout()
	oldWidth := m.main.viewport.Width()
	oldYOffset := m.main.viewport.YOffset()
	wasAtBottom := m.main.viewport.AtBottom()

	m.main.viewport.SetWidth(layout.viewportWidth)
	m.main.viewport.SetHeight(layout.viewportHeight)

	if oldWidth != layout.viewportWidth {
		m.rebuildRenderedContent(layout.viewportWidth)
		if m.main.autoScroll || wasAtBottom {
			m.main.viewport.GotoBottom()
			return
		}

		maxOffset := max(m.main.viewport.TotalLineCount()-m.main.viewport.Height(), 0)
		m.main.viewport.SetYOffset(min(oldYOffset, maxOffset))
		return
	}

	if m.main.autoScroll && wasAtBottom {
		m.main.viewport.GotoBottom()
	}
}

// rebuildRenderedContent regenerates the viewport buffer for the current width
// without changing the trace formatting rules.
func (m *Model) rebuildRenderedContent(viewportWidth int) {
	// Preserve the empty-state content on rebuild.
	if len(m.main.messages) == 0 {
		m.main.renderedContent.Reset()
		m.main.viewport.SetContent(m.renderLogContent())
		return
	}

	useColumns := viewportWidth >= colMinWidth
	layout := m.traceColumnLayout(viewportWidth)

	m.main.renderedContent.Reset()
	for _, queuedMsg := range m.main.messages {
		if useColumns {
			m.main.renderedContent.WriteString(renderTraceColumns(parseTraceLine(queuedMsg), layout))
		} else {
			m.main.renderedContent.WriteString(m.formatLogLine(queuedMsg))
		}
		m.main.renderedContent.WriteString("\n")
	}

	m.main.viewport.SetContent(m.main.renderedContent.String())
}

func (m *Model) traceColumnLayout(availableWidth int) traceColumnLayout {
	separatorWidth := len(colSeparator) * 3
	cacheValid := m.main.cachedWidthKey == availableWidth && len(m.main.messages) > 0

	// Use cached values if availableWidth matches the cached width key
	// (messages were added incrementally and width hasn't changed)
	var levelWidth int
	if cacheValid {
		// Fast path: use cached column widths
		levelWidth = m.main.cachedLevelWidth
	} else {
		// Slow path: full scan needed (width changed, messages removed, or initial build)
		levelWidth = colMinLevelWidth
		for _, queuedMsg := range m.main.messages {
			width := lipgloss.Width(formatTraceLevel(queuedMsg.LogLevel()))
			if width > levelWidth {
				levelWidth = width
			}
		}
		levelWidth = min(max(levelWidth, colMinLevelWidth), colMaxLevelWidth)

		// Update cache
		m.main.cachedLevelWidth = levelWidth
	}

	baseWidth := colTimestampWidth + levelWidth + separatorWidth
	maxAllowedAPIWidth := max(availableWidth-baseWidth-colMinPayloadWidth, colMinAPIWidth)
	apiWidth := min(colMaxAPIWidth, maxAllowedAPIWidth)

	// Use cached API width if availableWidth matches
	var longestAPI int
	if cacheValid {
		longestAPI = m.main.cachedAPIWidth
	} else {
		// Full scan for API width
		longestAPI = 0
		for _, queuedMsg := range m.main.messages {
			width := lipgloss.Width(truncate(sanitizeLogString(queuedMsg.ProcessName()), colMaxAPIWidth))
			if width > longestAPI {
				longestAPI = width
			}
		}

		// Update cache
		m.main.cachedAPIWidth = longestAPI
		m.main.cachedWidthKey = availableWidth
	}

	if longestAPI > 0 {
		apiWidth = min(apiWidth, max(longestAPI, colMinAPIWidth))
	}

	payloadWidth := max(availableWidth-baseWidth-apiWidth, 1)

	return traceColumnLayout{
		timestampWidth: colTimestampWidth,
		levelWidth:     levelWidth,
		apiWidth:       apiWidth,
		payloadWidth:   payloadWidth,
	}
}

// mainViewportDimensions: calculates panel and viewport dimensions accounting for borders and text elements.
func (m *Model) mainViewportDimensions(contentWidth, panelHeight int) (int, int, int) {
	panelWidth := max(contentWidth, 1)
	panelHorizontalFrame, panelVerticalFrame := styles.PrimaryPanelStyle.GetFrameSize()
	panelTextHeight := lipgloss.Height(styles.SectionTitleStyle.Render("Live Trace Feed")) +
		lipgloss.Height(styles.SubtitleStyle.Render("Awaiting Trace Messages...")) +
		1

	viewportWidth := max(panelWidth-panelHorizontalFrame, 1)
	viewportHeight := max(panelHeight-panelVerticalFrame-panelTextHeight, 1)

	return panelWidth, viewportWidth, viewportHeight
}

// mainSubtitle: returns the subtitle for the main screen showing database connection info or a default.
func (m *Model) mainSubtitle() string {
	if m.appConfig == nil {
		return "Live Oracle trace viewer"
	}

	return fmt.Sprintf(
		"%s@%s • %s:%d • %s",
		m.appConfig.Username(),
		m.appConfig.Database(),
		m.appConfig.Host(),
		m.appConfig.Port().Int(),
		m.appConfig.ID(),
	)
}

// mainConnectionMeta: returns connection metadata for display in the status bar (currently unused, reserved for future health details).
func (m *Model) mainConnectionMeta() string {
	return ""
}

// mainStatusText: returns the status bar text showing subscriber name, auto-scroll state, and message count.
func (m *Model) mainStatusText() string {
	autoScroll := styles.WarningColor
	autoScrollText := "manual"
	if m.main.autoScroll {
		autoScroll = styles.SuccessColor
		autoScrollText = "on"
	}

	subscriberName := "pending"
	if m.subscriber != nil {
		subscriberName = m.subscriber.Name()
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		styles.BodyTextStyle.Render("Sub "+subscriberName),
		styles.SubtitleStyle.Render("  •  "),
		lipgloss.NewStyle().Foreground(autoScroll).Bold(true).Render("Auto Scroll ["+autoScrollText+"]"),
		styles.SubtitleStyle.Render("  •  "),
		styles.BodyTextStyle.Render(fmt.Sprintf("Messages %d/%d", len(m.main.messages), maxMessages)),
	)
}

// mainFooterText: returns the footer help text showing available keyboard shortcuts.
func (m *Model) mainFooterText() string {
	return "↑/↓ Scroll  •  A Auto Scroll [on/off]  •  C Clear  •  D Database Settings  •  Q Quit"
}

// appendSingleMessage appends only the newly-arrived message to the rendered buffer.
// If the message widens any column (new max level or API width), it falls back to a
// full rebuild so existing lines stay aligned. This avoids the O(n) re-render in the
// common case where column widths are unchanged.
// Precondition: msg must already be the last element of m.main.messages.
func (m *Model) appendSingleMessage(msg *domain.QueueMessage, viewportWidth int) {
	useColumns := viewportWidth >= colMinWidth

	if useColumns {
		all := m.main.messages
		m.main.messages = all[:len(all)-1]
		prevLayout := m.traceColumnLayout(viewportWidth)
		m.main.messages = all

		// Incrementally update cached column widths for the new message
		newLevelWidth := lipgloss.Width(formatTraceLevel(msg.LogLevel()))
		newAPIWidth := lipgloss.Width(truncate(sanitizeLogString(msg.ProcessName()), colMaxAPIWidth))

		// Clamp new widths to valid range
		newLevelWidth = min(max(newLevelWidth, colMinLevelWidth), colMaxLevelWidth)
		newAPIWidth = min(max(newAPIWidth, colMinAPIWidth), colMaxAPIWidth)

		// Update cached values if new message exceeds current maxima
		cacheUpdated := false
		if newLevelWidth > m.main.cachedLevelWidth {
			m.main.cachedLevelWidth = newLevelWidth
			cacheUpdated = true
		}
		if newAPIWidth > m.main.cachedAPIWidth {
			m.main.cachedAPIWidth = newAPIWidth
			cacheUpdated = true
		}

		// Invalidate cache if viewport width changed
		if m.main.cachedWidthKey != viewportWidth {
			m.main.cachedWidthKey = viewportWidth
			cacheUpdated = true
		}

		// If cache was updated, recompute layout using cached values
		layout := m.traceColumnLayout(viewportWidth)

		// Compare against the pre-append layout.
		if cacheUpdated {
			if prevLayout.levelWidth != layout.levelWidth || prevLayout.apiWidth != layout.apiWidth {
				// Column widths shifted — rebuild all lines for alignment.
				m.rebuildRenderedContent(viewportWidth)
				return
			}
		}

		m.main.renderedContent.WriteString(renderTraceColumns(parseTraceLine(msg), layout))
	} else {
		m.main.renderedContent.WriteString(m.formatLogLine(msg))
	}

	m.main.renderedContent.WriteString("\n")
	m.main.viewport.SetContent(m.main.renderedContent.String())
}
