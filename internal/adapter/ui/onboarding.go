package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// onboardingField describes each step in the onboarding form.
type onboardingField struct {
	label       string
	placeholder string
	isPassword  bool
}

var onboardingFields = []onboardingField{
	{label: "Database ID", placeholder: "e.g., PROD-EU ... name to identify this database in the app"},
	{label: "Database Host", placeholder: "e.g., localhost"},
	{label: "Database Port", placeholder: "e.g., 1521"},
	{label: "Service Name / SID", placeholder: "e.g., ORCL"},
	{label: "Username", placeholder: "e.g., SYSTEM"},
	{label: "Password", placeholder: "password", isPassword: true},
}

// ==========================================
// Onboarding Update
// ==========================================

// updateOnboarding: handles messages for the onboarding screen, routing keyboard input and processing form submission.
func (m *Model) updateOnboarding(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyPressMsg:
		return m.handleOnboardingKey(msg)

	case tea.PasteMsg:
		step := m.onboarding.step
		value := m.onboarding.fieldValue(step)
		for _, r := range msg.Content {
			if r >= 0x20 && r < 0x7F {
				*value += string(r)
			}
		}
		m.onboarding.errMsg = ""
		return m, nil

	case onboardingCompleteMsg:
		if msg.err != nil {
			m.onboarding.errMsg = msg.err.Error()
			m.onboarding.submitted = false
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

// handleOnboardingKey: processes keyboard input for the onboarding form including navigation, validation, and character input.
func (m *Model) handleOnboardingKey(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	if m.onboarding.submitted && msg.String() != "ctrl+c" {
		return m, nil
	}

	step := m.onboarding.step
	value := m.onboarding.fieldValue(step)

	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "shift+tab", "up":
		if step > 0 {
			m.onboarding.step--
		}
		m.onboarding.errMsg = ""
		return m, nil

	case "tab", "down":
		if step < len(onboardingFields)-1 {
			m.onboarding.step++
		}
		m.onboarding.errMsg = ""
		return m, nil

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
		m.onboarding.submitted = true
		return m, saveOnboardingConfigCmd(m)

	case "backspace":
		if len(*value) == 0 {
			if step > 0 {
				m.onboarding.step--
			}
			m.onboarding.errMsg = ""
			return m, nil
		}
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
	case 0: // Database ID
		if value == "" {
			return "Database ID cannot be empty"
		}
	case 1: // Host
		if value == "" {
			return "Database host cannot be empty"
		}
	case 2: // Port
		if value == "" {
			return "Database port cannot be empty"
		}
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return "Port must be a number between 1 and 65535"
		}
	case 3: // Service Name
		if value == "" {
			return "Service name cannot be empty"
		}
	case 4: // Username
		if value == "" {
			return "Username cannot be empty"
		}
	case 5: // Password
		if value == "" {
			return "Password cannot be empty"
		}
	}
	return ""
}

// saveOnboardingConfigCmd saves the collected form data to BoltDB.
func saveOnboardingConfigCmd(m *Model) tea.Cmd {
	databaseID := m.onboarding.DatabaseID
	host := m.onboarding.Host
	portValue := strings.TrimSpace(m.onboarding.Port)
	serviceName := m.onboarding.ServiceName
	username := m.onboarding.Username
	password := m.onboarding.Password
	ctx := m.ctx
	boltAdapter := m.boltAdapter

	return func() tea.Msg {
		port, err := strconv.Atoi(portValue)
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

// viewOnboarding: renders the database configuration onboarding form with all fields and navigation hints.
func (m *Model) viewOnboarding() string {
	available := m.width - 10
	panelWidth := min(available, 74)
	if available >= 52 {
		panelWidth = max(panelWidth, 52)
	}
	fieldWidth := max(panelWidth-4, 24)
	lines := []string{
		styles.OnboardingTitleStyle.Render("Database Onboarding"),
		styles.OnboardingBannerStyle.Render("Important: fields marked with (*) are required."),
		"",
	}

	for i, field := range onboardingFields {
		isActive := i == m.onboarding.step
		value := m.onboarding.fieldValueValue(i)

		lines = append(
			lines,
			renderEmbeddedField(embeddedFieldOptions{
				Label:    field.label,
				Value:    renderOnboardingFieldValue(field, value, isActive),
				Width:    fieldWidth,
				Focused:  isActive,
				Required: true,
			}),
			"",
		)
	}

	if m.onboarding.errMsg != "" {
		lines = append(lines,
			styles.OnboardingErrorStyle.Render(m.onboarding.errMsg),
			"",
		)
	}

	primaryAction := "Enter Continue"
	if m.onboarding.step == len(onboardingFields)-1 {
		primaryAction = "Enter Save"
	}

	actionBar := renderCenteredActionButtons(
		fieldWidth,
		primaryAction,
		false,
		"Ctrl+C Exit",
		false,
	)

	lines = append(
		lines,
		actionBar,
		"",
		styles.OnboardingHintStyle.Render("↑/↓ Cycle Fields  •  Ctrl+U clear field"),
	)

	panel := renderFramedPanel("Configuration", panelWidth, lipgloss.JoinVertical(lipgloss.Left, lines...))
	return placeCentered(m.width, m.height, panel)
}

// fieldValue: returns a pointer to the field value for the given step (0-5), enabling direct modification.
func (state *onboardingState) fieldValue(step int) *string {
	switch step {
	case 0:
		return &state.DatabaseID
	case 1:
		return &state.Host
	case 2:
		return &state.Port
	case 3:
		return &state.ServiceName
	case 4:
		return &state.Username
	case 5:
		return &state.Password
	default:
		panic(fmt.Sprintf("invalid onboarding step: %d", step))
	}
}

// fieldValueValue: returns the string value of the field at the given step without pointer access.
func (state *onboardingState) fieldValueValue(step int) string {
	return *state.fieldValue(step)
}

// renderFieldValue renders the display value for a field.
// For password fields, shows bullets; for others shows the actual value.
func renderFieldValue(field onboardingField, value string) string {
	if value == "" {
		return field.placeholder
	}
	if field.isPassword {
		return strings.Repeat("•", min(len(value), 20))
	}
	return value
}

// renderOnboardingFieldValue: renders the display value for an onboarding field with appropriate styling based on focus state.
func renderOnboardingFieldValue(field onboardingField, value string, focused bool) string {
	displayValue := renderFieldValue(field, value)

	if value == "" {
		displayValue = styles.FieldPlaceholderStyle.Render(displayValue)
	} else {
		displayValue = styles.OnboardingFieldValueStyle.Render(displayValue)
	}

	if focused {
		return displayValue + styles.FieldCursorStyle.Render("_")
	}

	return displayValue
}
