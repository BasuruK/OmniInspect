package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Welcome Screen Constants
// ==========================================

const (
	tickInterval  = 80 * time.Millisecond
	logoEndFrame  = 8  // frames required for all logo lines to be revealed (4 lines × 2 frames each)
	versionDelay  = 4  // frames to wait after logo finishes before version appears
	subtitleDelay = 6  // frames to wait after version appears before subtitle appears
	completeDelay = 10 // frames to wait after subtitle appears before animation completes
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
			// Check if DB config exists in BoltDB to decide where to go next
			settings, err := m.checkDBConfig()
			if err != nil {
				m.loading.err = fmt.Errorf("failed to load saved database settings: %w", err)
				m.loading.current = ""
				m.screen = screenLoading
				return m, nil
			}
			if settings != nil {
				// Config exists — pre-warm services, then go to loading screen
				m.appConfig = settings
				if err := m.initializeServices(); err != nil {
					m.loading.err = fmt.Errorf("failed to initialize services: %w", err)
					m.loading.current = ""
					m.screen = screenLoading
					return m, nil
				}

				m.loading.err = nil
				m.screen = screenLoading
				return m, tea.Batch(
					m.loading.spinner.Tick,
					checkForUpdateCmd(m),
				)
			}
			// No config — go to onboarding
			m.screen = screenOnboarding
			// Initialize AddDatabaseForm with current terminal dimensions
			m.onboarding.AddDatabaseForm = NewAddDatabaseForm(m.width, m.height)
			return m, nil
		}

		m.welcome.frame++

		// Reveal logo line by line
		if m.welcome.logoRevealed < len(logoLines) && m.welcome.frame%2 == 0 {
			m.welcome.logoRevealed++
		}

		// Cumulative thresholds: each stage is relative to the end of the prior stage
		versionThreshold := logoEndFrame + versionDelay
		subtitleThreshold := versionThreshold + subtitleDelay
		completeThreshold := subtitleThreshold + completeDelay

		// Show version after logo is fully revealed
		if m.welcome.logoRevealed >= len(logoLines) && m.welcome.frame >= versionThreshold {
			m.welcome.showVersion = true
		}

		// Show subtitle after version
		if m.welcome.showVersion && m.welcome.frame >= subtitleThreshold {
			m.welcome.showSubtitle = true
		}

		// Complete animation
		if m.welcome.showSubtitle && m.welcome.frame >= completeThreshold {
			m.welcome.complete = true
		}

		return m, tea.Tick(tickInterval, func(t time.Time) tea.Msg {
			return tickMsg{time: t}
		})
	}

	return m, nil
}

// checkDBConfig returns the database settings if a configuration exists in BoltDB.
func (m *Model) checkDBConfig() (*domain.DatabaseSettings, error) {
	ctx := context.Background()
	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	settings, err := settingsRepo.GetDefault(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrDefaultSettingsNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return settings, nil
}

// viewWelcome renders the welcome screen based on the current animation state.
func (m *Model) viewWelcome() string {
	var logo strings.Builder

	// Render logo
	for i := 0; i < m.welcome.logoRevealed && i < len(logoLines); i++ {
		if i > 0 {
			logo.WriteString("\n")
		}
		logo.WriteString(styles.LogoStyle.Render(logoLines[i]))
	}

	lines := []string{logo.String()}

	if m.welcome.showVersion {
		versionText := fmt.Sprintf("Version %s", m.app.GetVersion())
		lines = append(lines, "", styles.VersionStyle.Render(versionText))
	}

	if m.welcome.showSubtitle {
		lines = append(
			lines,
			styles.LogoSubtleStyle.Render("Real-time Oracle AQ trace console"),
			styles.VersionStyle.Render("Author "+m.app.GetAuthor()),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	content = lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
