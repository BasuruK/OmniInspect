package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/storage/oracle"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
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
			// Create services now that we have appConfig
			m.dbAdapter = oracle.NewOracleAdapter(m.appConfig)
			subscriberRepo := boltdb.NewSubscriberRepository(m.boltAdapter)
			permissionsRepo := boltdb.NewPermissionsRepository(m.boltAdapter)

			m.permissionService = permissions.NewPermissionService(m.dbAdapter, permissionsRepo, m.boltAdapter)
			m.tracerService = tracer.NewTracerService(m.dbAdapter, m.boltAdapter, m.eventChannel)
			m.subscriberService = subscribers.NewSubscriberService(m.dbAdapter, subscriberRepo)

			// Transition to loading screen
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
