package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// ==========================================
// Help Overlay
// ==========================================

// helpOverlayMaxWidth is the maximum rendered width of the help overlay panel.
const helpOverlayMaxWidth = 90

// renderHelpOverlay renders the in-app help overlay panel.
// It is displayed centered over the main screen when m.showHelp is true.
// The subscriber's live procedure name is injected into section 1 when available.
func (m *Model) renderHelpOverlay() string {
	contentWidth, _ := screenContentSize(m.width, m.height)
	width := min(contentWidth-4, helpOverlayMaxWidth)
	if width < 40 {
		width = 40
	}
	// border=2, Padding(1,3) → horizontal overhead = 2 + 3*2 = 8
	innerWidth := max(width-8, 1)

	// Derive subscriber procedure name — show placeholder when not yet assigned.
	subscriberProc := "OMNI_TRACER_API.TRACE_MESSAGE_<YOUR_NAME>('msg', log_level_)"
	if m.subscriber != nil {
		funnyName := m.subscriber.FunnyName()
		if funnyName != "" {
			subscriberProc = fmt.Sprintf("OMNI_TRACER_API.TRACE_MESSAGE_%s('msg', log_level_ [optional])", funnyName)
		}
	}

	sep := styles.SubtitleStyle.Render(strings.Repeat("─", min(innerWidth, 52)))

	lines := []string{
		styles.HeaderTitleStyle.Render("OmniView Help"),
		sep,
		"",
		styles.SectionTitleStyle.Render("1. Subscriber-Specific Method"),
		styles.ProcedureCallStyle.Render(subscriberProc),
		styles.SubtitleStyle.Render("Routes the message ONLY to your OmniView instance."),
		"",
		styles.SectionTitleStyle.Render("2. Global Broadcast Method"),
		styles.ProcedureCallStyle.Render("OMNI_TRACER_API.Trace_Message('msg', log_level_ [optional])"),
		styles.SubtitleStyle.Render("Sends to ALL connected OmniView subscribers."),
		"",
		styles.SectionTitleStyle.Render("3. Database Management  [D]"),
		styles.BodyTextStyle.Render("Add, switch, or edit database connections."),
		styles.SubtitleStyle.Render("N = New  •  E = Edit  •  Enter = Switch to selected"),
		"",
		styles.SectionTitleStyle.Render("4. Webhook Configuration  [S]"),
		styles.BodyTextStyle.Render("Open Settings → Webhook section."),
		styles.SubtitleStyle.Render("Configure endpoint URL and toggle delivery on/off."),
		"",
		styles.SectionTitleStyle.Render("5. Message Filtering  [B]"),
		styles.BodyTextStyle.Render("Cycle: Global → Subscriber Only → Broadcast Only"),
		styles.SubtitleStyle.Render("Global: all messages  •  Subscriber: yours only  •  Broadcast: broadcast only"),
		"",
		lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Width(innerWidth).
			Align(lipgloss.Center).
			Render("[ H or Esc — Close ]"),
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.SurfaceColor).
		Padding(1, 3).
		Width(width).
		Render(content)
}
