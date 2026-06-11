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

// helpOverlaySepMaxWidth caps the header separator line so it does not span the
// full inner width on wide terminals. Chosen to match the typical title line length.
const helpOverlaySepMaxWidth = 52

// logLevelLabel renders the five valid log_level_ values in their canonical
// colors so users can match what they type with the colors in the live log.
// Reuses the same color tokens as getLevelStyle() in main_screen.go.
func logLevelLabel() string {
	base := styles.LogLevelStyle
	return styles.SubtitleStyle.Render("log_level_: ") +
		base.Foreground(styles.InfoColor).Render("'INFO'") +
		styles.SubtitleStyle.Render(", ") +
		base.Foreground(styles.DebugColor).Render("'DEBUG'") +
		styles.SubtitleStyle.Render(", ") +
		base.Foreground(styles.WarningColor).Render("'WARNING'") +
		styles.SubtitleStyle.Render(", ") +
		base.Foreground(styles.ErrorColor).Render("'ERROR'") +
		styles.SubtitleStyle.Render(", ") +
		base.Foreground(styles.CriticalColor).Render("'CRITICAL'") +
		styles.SubtitleStyle.Render("  (default: INFO)")
}

// renderHelpOverlay renders the in-app help overlay panel.
// It is displayed centered over the main screen when m.showHelp is true.
// The subscriber's live procedure name is injected into section 1 when available.
func (m *Model) renderHelpOverlay() string {
	contentWidth, _ := screenContentSize(m.width, m.height)
	width := min(contentWidth-4, helpOverlayMaxWidth)
	// Prevent negative width if terminal is extremely narrow.
	if width < 0 {
		width = 0
	}
	// border=2, Padding(1,3) → horizontal overhead = 2 + 3*2 = 8
	innerWidth := max(width-8, 1)

	// closeHintStyle is the base style for the close-hint line. Width is applied
	// dynamically below because it depends on the runtime innerWidth value.
	closeHintStyle := lipgloss.NewStyle().
		Foreground(styles.MutedColor).
		Align(lipgloss.Center)
	centerLineStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Align(lipgloss.Center)

	// Derive subscriber procedure name — show placeholder when not yet assigned.
	subscriberProc := "Omni_Tracer_API.Trace_Message<YOUR_NAME>('msg', optional [log_level_])"
	if m.subscriber != nil {
		funnyName := m.subscriber.FunnyName()
		if funnyName != "" {
			// Convert single-word funnyName from ALL-CAPS to PascalCase (e.g., "Chester")
			pascalName := strings.ToUpper(funnyName[:1]) + strings.ToLower(funnyName[1:])
			subscriberProc = fmt.Sprintf("Omni_Tracer_API.Trace_Message_%s('msg', optional [log_level_])", pascalName)
		}
	}

	sep := centerLineStyle.Render(styles.SubtitleStyle.Render(strings.Repeat("─", min(innerWidth, helpOverlaySepMaxWidth))))

	lines := []string{
		centerLineStyle.Render(styles.HeaderTitleStyle.Render("OmniView Help")),
		sep,
		"",
		styles.SectionTitleStyle.Render("1. Subscriber-Specific Method"),
		styles.ProcedureCallStyle.Render(subscriberProc),
		logLevelLabel(),
		styles.SubtitleStyle.Render("Routes the message ONLY to your OmniView instance."),
		"",
		styles.SectionTitleStyle.Render("2. Global Broadcast Method"),
		// Trace_Message is the actual mixed-case PL/SQL identifier in OMNI_TRACER_API;
		// subscriber-specific procedures are generated all-caps (TRACE_MESSAGE_<NAME>).
		styles.ProcedureCallStyle.Render("Omni_Tracer_API.Trace_Message('msg', optional [log_level_])"),
		logLevelLabel(),
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
		styles.BodyTextStyle.Render("Cycle: Global → Subscriber Only → Broadcast Only → Global"),
		styles.SubtitleStyle.Render("Global: all messages  •  Subscriber: yours only  •  Broadcast: broadcast only"),
		"",
		centerLineStyle.Render(styles.SubtitleStyle.Render(strings.Repeat("─", min(innerWidth, helpOverlaySepMaxWidth)))),
		centerLineStyle.Render(styles.SubtitleStyle.Render("Made With Love 💖 by Basuru Balasuriya")),
		"",
		closeHintStyle.Width(innerWidth).Render("[ H or Esc — Close ]"),
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.SurfaceColor).
		Padding(1, 3).
		Width(width).
		Render(content)
}
