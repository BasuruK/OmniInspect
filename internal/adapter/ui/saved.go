package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Saved Update
// ==========================================

func (m *Model) updateSaved(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "enter":
			if err := m.initializeServices(); err != nil {
				m.loading.err = err
				m.loading.current = ""
				m.screen = screenLoading
				return m, nil
			}

			// Transition to loading screen
			m.loading.err = nil
			m.screen = screenLoading
			return m, tea.Batch(
				m.loading.spinner.Tick,
				connectDBCmd(m),
			)
		}
	}
	return m, nil
}

// ==========================================
// Saved View
// ==========================================

func (m *Model) viewSaved() string {
	var b strings.Builder

	b.WriteString(styles.OnboardingSavedStyle.Render("✓ Configuration saved!"))
	b.WriteString("\n\n")
	b.WriteString(styles.OnboardingHintStyle.Render("Press Enter to continue..."))

	content := b.String()
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}
