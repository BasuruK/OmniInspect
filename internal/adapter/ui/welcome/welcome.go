package welcome

import (
	"fmt"
	"strings"
	"time"

	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/app"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Animation speed constants
const (
	TickInterval   = 80 * time.Millisecond
	LogoLines      = 6
	VersionDelay   = 4  // frames to wait after logo before version
	SubtitleDelay  = 6  // frames to wait after version
	CompleteDelay  = 10 // frames to wait before completion
)

// Logo frames - each frame reveals more lines
var logoLines = []string{
	`  __  __ __ __  _ _  _   _  _ ___  _   _ `,
	` /__\|  V  |  \| | || \ / || | __|| | | |`,
	`| \/ | \_/ | | ' | |` + "`" + `\ V /'| | _| | 'V' |`,
	` \__/|_| |_|_|\__|_|  \_/  |_|___|!_/ \_!`,
}

// Model holds the state for the welcome screen
type Model struct {
	frame        int
	logoRevealed int
	showVersion  bool
	showSubtitle bool
	isComplete   bool
	app          *app.App
	width        int
	height       int
}

// New creates a new welcome screen model
func New(omniApp *app.App) *Model {
	return &Model{
		app:          omniApp,
		frame:        0,
		logoRevealed: 0,
		showVersion:  false,
		showSubtitle: false,
		isComplete:   false,
		width:        80,
		height:       24,
	}
}

// Init implements the tea.Model interface
func (m *Model) Init() tea.Cmd {
	return tea.Tick(TickInterval, func(t time.Time) tea.Msg {
		return tickMsg{t}
	})
}

// tickMsg is sent on each animation frame
type tickMsg struct {
	time.Time
}

// Update implements the tea.Model interface
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.isComplete {
			// Animation done - return quit command to exit the program
			return m, tea.Quit
		}

		m.frame++

		// Reveal logo line by line
		if m.logoRevealed < len(logoLines) && m.frame%2 == 0 {
			m.logoRevealed++
		}

		// Show version after logo is fully revealed
		if m.logoRevealed >= len(logoLines) && m.frame >= VersionDelay {
			m.showVersion = true
		}

		// Show subtitle after version
		if m.showVersion && m.frame >= SubtitleDelay {
			m.showSubtitle = true
		}

		// Complete animation
		if m.showSubtitle && m.frame >= CompleteDelay {
			m.isComplete = true
		}

		return m, tea.Tick(TickInterval, func(t time.Time) tea.Msg {
			return tickMsg{t}
		})

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	return m, nil
}

// View implements the tea.Model interface
func (m *Model) View() tea.View {
	// Build the logo content
	var logoContent string
	for i := 0; i < m.logoRevealed && i < len(logoLines); i++ {
		if i > 0 {
			logoContent += "\n"
		}
		logoContent += styles.LogoStyle.Render(logoLines[i])
	}

	// Build the full content
	var content string

	if logoContent != "" {
		content += logoContent
	}

	if m.showVersion {
		if content != "" {
			content += "\n\n"
		}
		versionText := fmt.Sprintf("Version: %s", m.app.GetVersion())
		content += styles.VersionStyle.Render(versionText)
	}

	if m.showSubtitle {
		if content != "" {
			content += "\n"
		}
		subtitleText := fmt.Sprintf("Created with \u2764\ufe0f by %s", m.app.GetAuthor())
		content += styles.LogoSubtleStyle.Render("\n" + subtitleText)
	}

	// Center the content vertically and horizontally
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	verticalPadding := (m.height - lineCount) / 2
	if verticalPadding < 1 {
		verticalPadding = 1
	}

	horizontalPadding := (m.width - 44) / 2 // Logo is ~44 chars wide
	if horizontalPadding < 1 {
		horizontalPadding = 1
	}

	// Create padded content
	paddedContent := ""
	for i := 0; i < verticalPadding; i++ {
		paddedContent += "\n"
	}

	for _, line := range lines {
		spaces := ""
		for i := 0; i < horizontalPadding; i++ {
			spaces += " "
		}
		paddedContent += spaces + line + "\n"
	}

	// Apply styling and create the view
	styledContent := lipgloss.NewStyle().
		Background(styles.BackgroundColor).
		Foreground(styles.TextColor).
		Render(paddedContent)

	return tea.NewView(styledContent)
}

// IsComplete returns whether the welcome animation is finished
func (m *Model) IsComplete() bool {
	return m.isComplete
}
