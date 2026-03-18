package ui

import (
	"fmt"
	"strings"
	"time"

	"OmniView/internal/adapter/ui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Welcome Screen Constants
// ==========================================

const (
	tickInterval  = 80 * time.Millisecond
	versionDelay  = 4  // frames to wait after logo before version
	subtitleDelay = 6  // frames to wait after version
	completeDelay = 10 // frames to wait before completion
)

// Logo frames - each frame reveals more lines
var logoLines = []string{
	`  __  __ __ __  _ _  _   _  _ ___  _   _ `,
	` /__\|  V  |  \| | || \ / || | __|| | | |`,
	`| \/ | \_/ | | ' | |` + "`" + `\ V /'| | _| | 'V' |`,
	` \__/|_| |_|_|\__|_|  \_/  |_|___|!_/ \_!`,
}

// ==========================================
// Welcome Update
// ==========================================

// updateWelcome handles the animation logic for the welcome screen, driven by tickMsg messages.
func (m *Model) updateWelcome(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		if m.welcome.complete {
			// Transition to loading screen after animation is complete
			m.screen = screenLoading
			return m, tea.Batch(
				m.loading.spinner.Tick, // start spinner
				connectDBCmd(m),
			)
		}

		m.welcome.frame++

		// Reveal logo line by line
		if m.welcome.logoRevealed < len(logoLines) && m.welcome.frame%2 == 0 {
			m.welcome.logoRevealed++
		}

		// Show version after logo is fully revealed
		if m.welcome.logoRevealed >= len(logoLines) && m.welcome.frame >= versionDelay {
			m.welcome.showVersion = true
		}

		// Show subtitle after version
		if m.welcome.showVersion && m.welcome.frame >= subtitleDelay {
			m.welcome.showSubtitle = true
		}

		// Complete animation
		if m.welcome.showSubtitle && m.welcome.frame >= completeDelay {
			m.welcome.complete = true
		}

		return m, tea.Tick(tickInterval, func(t time.Time) tea.Msg {
			return tickMsg{time: t}
		})
	}

	return m, nil
}

// viewWelcome renders the welcome screen based on the current animation state.
func (m *Model) viewWelcome() string {
	var b strings.Builder

	// Render logo
	for i := 0; i < m.welcome.logoRevealed && i < len(logoLines); i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styles.LogoStyle.Render(logoLines[i]))
	}

	// Version text
	if m.welcome.showVersion {
		b.WriteString("\n\n")
		versionText := fmt.Sprintf("Version: %s", m.app.GetVersion())
		b.WriteString(styles.VersionStyle.Render(versionText))
	}

	// Subtitle
	if m.welcome.showSubtitle {
		b.WriteString("\n")
		subtitleText := fmt.Sprintf("Created with ❤️ by %s", m.app.GetAuthor())
		b.WriteString(styles.LogoSubtleStyle.Render("\n" + subtitleText))
	}

	// Center in terminal
	content := b.String()
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}
