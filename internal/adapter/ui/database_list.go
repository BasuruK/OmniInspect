package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Connection Status
// ==========================================

type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
)

// ==========================================
// Database Entry
// ==========================================

type DatabaseEntry struct {
	Name    string
	Host    string
	Port    string
	Service string
	Status  ConnectionStatus
}

// ==========================================
// Database List
// ==========================================

type DatabaseList struct {
	entries []DatabaseEntry
	cursor  int
	width   int
}

func NewDatabaseList(entries []DatabaseEntry, width int) DatabaseList {
	return DatabaseList{entries: entries, width: width}
}

func (dl DatabaseList) Cursor() int              { return dl.cursor }
func (dl DatabaseList) Entries() []DatabaseEntry { return dl.entries }

func (dl DatabaseList) WithCursor(c int) DatabaseList {
	if c >= 0 && c < len(dl.entries) {
		dl.cursor = c
	}
	return dl
}

func (dl DatabaseList) Update(msg tea.KeyPressMsg) (DatabaseList, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if dl.cursor > 0 {
			dl.cursor--
		}
	case "down", "j":
		if dl.cursor < len(dl.entries)-1 {
			dl.cursor++
		}
	}
	return dl, nil
}

// ─────────────────────────
// Styles
// ─────────────────────────

var (
	listHeaderStyle   = lipgloss.NewStyle().Foreground(styles.SecondaryColor).Bold(true)
	listItemSelected  = lipgloss.NewStyle().Background(styles.SelectionColor).Foreground(styles.BackgroundColor).Bold(true)
	listItemNormal    = lipgloss.NewStyle().Foreground(styles.TextColor)
	listSubtextStyle  = lipgloss.NewStyle().Foreground(styles.MutedColor)
	listCursor        = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Bold(true)
	listDotConnected  = lipgloss.NewStyle().Foreground(styles.SuccessColor)
	listDotError      = lipgloss.NewStyle().Foreground(styles.FailureColor)
	listDotConnecting = lipgloss.NewStyle().Foreground(styles.WarningColor)
	listDotIdle       = lipgloss.NewStyle().Foreground(styles.MutedColor)
	listStateStyle    = lipgloss.NewStyle().Foreground(styles.SecondaryColor).Bold(true)
)

func (dl DatabaseList) Render() string {
	if len(dl.entries) == 0 {
		return styles.EmptyStateStyle.Render("No databases configured yet.")
	}

	lines := []string{listHeaderStyle.Render("Stored database connections")}
	for i, entry := range dl.entries {
		selected := i == dl.cursor

		cursor := "  "
		if selected {
			cursor = listCursor.Render("▸ ")
		}

		var dot string
		switch entry.Status {
		case StatusConnected:
			dot = listDotConnected.Render("●")
		case StatusError:
			dot = listDotError.Render("●")
		case StatusConnecting:
			dot = listDotConnecting.Render("◐")
		default:
			dot = listDotIdle.Render("○")
		}

		state := "SAVED"
		if entry.Status == StatusConnected {
			state = "ACTIVE"
		}

		titleLine := fmt.Sprintf(
			"%s%s %s  %s",
			cursor,
			dot,
			truncate(entry.Name, max(dl.width-16, 8)),
			listStateStyle.Render(state),
		)
		subLine := fmt.Sprintf("   %s", truncate(fmt.Sprintf("%s @ %s", entry.Service, entry.Host), max(dl.width-3, 8)))

		if selected {
			selectedStyle := listItemSelected.Width(dl.width)
			lines = append(
				lines,
				selectedStyle.Render(titleLine),
				selectedStyle.Render(subLine),
				"",
			)
			continue
		}

		lines = append(
			lines,
			listItemNormal.Render(titleLine),
			listSubtextStyle.Render(subLine),
			"",
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
