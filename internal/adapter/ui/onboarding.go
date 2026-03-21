package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// onboardingField describes each step in the onboarding form.
type onboardingField struct {
	label      string
	placeholder string
	isPassword bool
}

var onboardingFields = []onboardingField{
	{label: "Database Host", placeholder: "e.g., localhost"},
	{label: "Database Port", placeholder: "e.g., 1521"},
	{label: "Service Name / SID", placeholder: "e.g., ORCL"},
	{label: "Username", placeholder: "e.g., SYSTEM"},
	{label: "Password", placeholder: "password", isPassword: true},
}

// ==========================================
// Onboarding Update
// ==========================================

func (m *Model) updateOnboarding(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyPressMsg:
		return m.handleOnboardingKey(msg)

	case onboardingCompleteMsg:
		if msg.err != nil {
			m.onboarding.errMsg = msg.err.Error()
			m.onboarding.submitted = false
			return m, nil
		}
		m.appConfig = msg.config
		m.screen = screenSaved
		m.saved.showPrompt = false
		return m, nil
	}

	return m, nil
}

func (m *Model) handleOnboardingKey(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	step := m.onboarding.step
	value := &m.onboarding.values[step]

	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "enter":
		// Validate current field before advancing
		if errMsg := validateOnboardingField(step, *value); errMsg != "" {
			m.onboarding.errMsg = errMsg
			return m, nil
		}
		m.onboarding.errMsg = ""

		if step < len(onboardingFields)-1 {
			// Advance to next field
			m.onboarding.step++
			return m, nil
		}
		// Last field — submit
		return m, saveOnboardingConfigCmd(m)

	case "backspace":
		if len(*value) > 0 {
			*value = (*value)[:len(*value)-1]
		}
		m.onboarding.errMsg = ""

	case "ctrl+u":
		*value = ""
		m.onboarding.errMsg = ""
	}

	// Character input — accept printable ASCII (letters, digits, punctuation)
	if len(msg.Text) > 0 && !msg.Mod.Contains(tea.ModCtrl) {
		r := []rune(msg.Text)
		if len(r) == 1 && r[0] >= 0x20 && r[0] < 0x7F {
			*value += msg.Text
			m.onboarding.errMsg = ""
		}
	}

	return m, nil
}

// validateOnboardingField validates the value for a given field step.
// Returns an error string if invalid, or empty string if valid.
func validateOnboardingField(step int, value string) string {
	value = strings.TrimSpace(value)
	switch step {
	case 0: // Host
		if value == "" {
			return "Database host cannot be empty"
		}
	case 1: // Port
		if value == "" {
			return "Database port cannot be empty"
		}
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return "Port must be a number between 1 and 65535"
		}
	case 2: // Service Name
		if value == "" {
			return "Service name cannot be empty"
		}
	case 3: // Username
		if value == "" {
			return "Username cannot be empty"
		}
	case 4: // Password
		if value == "" {
			return "Password cannot be empty"
		}
	}
	return ""
}

// saveOnboardingConfigCmd saves the collected form data to BoltDB.
func saveOnboardingConfigCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		values := m.onboarding.values
		port, _ := strconv.Atoi(values[1])

		dbPort, err := domain.NewPort(port)
		if err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("invalid port: %w", err)}
		}

		settings, err := domain.NewDatabaseSettings(
			values[2], // serviceName
			values[0], // host
			dbPort,
			values[3], // username
			values[4], // password
		)
		if err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("failed to create database settings: %w", err)}
		}
		settings.SetAsDefault()

		ctx := context.Background()
		settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
		if err := settingsRepo.Save(ctx, *settings); err != nil {
			return onboardingCompleteMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		return onboardingCompleteMsg{config: settings, err: nil}
	}
}

// ==========================================
// Onboarding View
// ==========================================

func (m *Model) viewOnboarding() string {
	panelWidth := min(m.width-8, 56)
	panelWidth = max(panelWidth, 40)

	var b strings.Builder

	// Panel header
	b.WriteString(styles.OnboardingTitleStyle.Render("Onboarding"))
	b.WriteString("\n\n")

	// Render each field
	for i, field := range onboardingFields {
		isActive := i == m.onboarding.step
		value := m.onboarding.values[i]

		if isActive {
			// Active field
			b.WriteString(styles.OnboardingFieldActiveStyle.Render(
				styles.OnboardingFieldLabelStyle.Render(field.label)+"\n"+
					renderFieldValue(field, value),
			))
		} else if i < m.onboarding.step || value != "" {
			// Completed field (already filled)
			b.WriteString(styles.OnboardingFieldLabelStyle.Render(field.label) + "\n")
			b.WriteString(styles.OnboardingFieldValueStyle.Render(
				renderFieldValue(field, value),
			))
		} else {
			// Future field (not yet visited)
			b.WriteString(styles.OnboardingFieldLabelStyle.Render(field.label) + "\n")
			b.WriteString(styles.OnboardingFieldValueStyle.Render(field.placeholder))
		}
		b.WriteString("\n\n")
	}

	// Error message
	if m.onboarding.errMsg != "" {
		b.WriteString(styles.OnboardingErrorStyle.Render(m.onboarding.errMsg))
		b.WriteString("\n\n")
	}

	// Navigation hint
	if m.onboarding.step < len(onboardingFields)-1 {
		hint := fmt.Sprintf("Press Enter to continue (%s)", onboardingFields[m.onboarding.step].label)
		b.WriteString(styles.OnboardingHintStyle.Render(hint))
	} else {
		b.WriteString(styles.OnboardingHintStyle.Render("Press Enter to save configuration"))
	}

	// Center the panel
	panelContent := b.String()
	panel := lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		styles.OnboardingPanelStyle.Width(panelWidth).Render(panelContent),
	)

	return panel
}

// renderFieldValue renders the display value for a field.
// For password fields, shows bullets; for others shows the actual value.
func renderFieldValue(field onboardingField, value string) string {
	if value == "" {
		return styles.OnboardingFieldValueStyle.Render(field.placeholder)
	}
	if field.isPassword {
		// Show bullets for password
		return lipgloss.NewStyle().
			Foreground(styles.PrimaryColor).
			Render(strings.Repeat("•", min(len(value), 20)))
	}
	return styles.OnboardingFieldValueStyle.Render(value)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
