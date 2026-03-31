package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/core/domain"
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Onboarding Update
// ==========================================

// updateOnboarding: handles messages for the onboarding screen, routing keyboard input and processing form submission.
func (m *Model) updateOnboarding(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyPressMsg:
		// Handle ctrl+c globally to quit
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}
		// Delegate to AddDatabaseForm
		m.onboarding.AddDatabaseForm, _ = m.onboarding.AddDatabaseForm.Update(msg)
		// Check if form was cancelled
		if m.onboarding.AddDatabaseForm.IsCancelled() {
			m.cancel()
			return m, tea.Quit
		}
		// Check if form was submitted
		if m.onboarding.AddDatabaseForm.IsSubmitted() {
			return m, saveOnboardingConfigCmd(m)
		}
		return m, nil

	case tea.PasteMsg:
		// Delegate paste to AddDatabaseForm
		m.onboarding.AddDatabaseForm, _ = m.onboarding.AddDatabaseForm.Update(msg)
		return m, nil

	case onboardingCompleteMsg:
		if msg.err != nil {
			m.onboarding.errMsg = msg.err.Error()
			m.onboarding.submitted = false
			// Reset form state to allow retry with proper dimensions
			m.onboarding.AddDatabaseForm = NewAddDatabaseForm(m.width, m.height)
			return m, nil
		}
		m.onboarding.submitted = false
		m.appConfig = msg.config
		if err := m.initializeServices(); err != nil {
			m.loading.err = err
			m.loading.current = ""
			m.screen = screenLoading
			return m, nil
		}

		m.loading.err = nil
		m.loading.steps = nil
		m.loading.current = ""
		m.screen = screenLoading
		return m, tea.Batch(
			m.loading.spinner.Tick,
			connectDBCmd(m),
		)
	}

	return m, nil
}

// saveOnboardingConfigCmd saves the collected form data to BoltDB.
func saveOnboardingConfigCmd(m *Model) tea.Cmd {
	databaseID, host, portStr, serviceName, username, password := m.onboarding.AddDatabaseForm.FieldValues()
	ctx := m.ctx
	boltAdapter := m.boltAdapter

	return func() tea.Msg {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("invalid port: %w", err)}
		}

		dbPort, err := domain.NewPort(port)
		if err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("invalid port: %w", err)}
		}

		settings, err := domain.NewDatabaseSettings(
			databaseID,
			serviceName,
			host,
			dbPort,
			username,
			password,
		)
		if err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("failed to create database settings: %w", err)}
		}
		settings.SetAsDefault()

		settingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)
		if err := settingsRepo.Save(ctx, *settings); err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		return onboardingCompleteMsg{config: settings, err: nil}
	}
}

// ==========================================
// Onboarding View
// ==========================================

// viewOnboarding: renders the database configuration onboarding form using AddDatabaseForm.
func (m *Model) viewOnboarding() string {
	// Use AddDatabaseForm.Modal() directly for rendering
	return m.onboarding.AddDatabaseForm.Render()
}
