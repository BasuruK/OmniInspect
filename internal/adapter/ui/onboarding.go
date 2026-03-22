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
	host := m.onboarding.Host
	portValue := m.onboarding.Port
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

func (m *Model) viewOnboarding() string {
	panelWidth := min(m.width-8, 56)
	panelWidth = max(panelWidth, 40)
	contentWidth := max(panelWidth-4, 10)
	separatorWidth := max(contentWidth-2, 1)

	lines := []string{""}

	for i, field := range onboardingFields {
		isActive := i == m.onboarding.step
		value := m.onboarding.fieldValueValue(i)
		pointer := " "
		labelStyle := styles.OnboardingFieldLabelStyle
		valueStyle := styles.OnboardingFieldValueStyle

		if isActive {
			pointer = styles.OnboardingActiveIndicatorStyle.Render(">")
			labelStyle = styles.OnboardingActiveLabelStyle
			if value != "" {
				valueStyle = styles.OnboardingActiveValueStyle
			}
		} else if value == "" {
			pointer = " "
		}

		lines = append(lines,
			pointer+" "+labelStyle.Render(field.label),
			"  "+valueStyle.Render(renderFieldValue(field, value)),
			"  "+styles.OnboardingSeparatorStyle.Render(strings.Repeat("─", separatorWidth)),
			"",
		)
	}

	if m.onboarding.errMsg != "" {
		lines = append(lines,
			styles.OnboardingErrorStyle.Render(m.onboarding.errMsg),
			"",
		)
	}

	hint := "Use ↑/↓ to edit, Enter to continue"
	if m.onboarding.step < len(onboardingFields)-1 {
		lines = append(lines, styles.OnboardingHintStyle.Width(contentWidth).Align(lipgloss.Center).Render(hint))
	} else {
		lines = append(lines, styles.OnboardingHintStyle.Width(contentWidth).Align(lipgloss.Center).Render("Use ↑/↓ to edit, Enter to save configuration"))
	}

	lines = append(lines, "")

	panelContent := renderOnboardingFrame("Onboarding", panelWidth, lines)

	panel := lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		panelContent,
	)

	return panel
}

func renderOnboardingFrame(title string, width int, lines []string) string {
	innerWidth := max(width-4, 1)
	titleText := "[ " + title + " ]"
	topFill := max(width-lipgloss.Width(titleText)-3, 0)

	var b strings.Builder
	b.WriteString(styles.OnboardingBorderStyle.Render("┌─"))
	b.WriteString(styles.OnboardingTitleStyle.Render(titleText))
	b.WriteString(styles.OnboardingBorderStyle.Render(strings.Repeat("─", topFill) + "┐"))
	b.WriteString("\n")

	for _, line := range lines {
		b.WriteString(renderOnboardingFrameLine(line, innerWidth))
		b.WriteString("\n")
	}

	b.WriteString(styles.OnboardingBorderStyle.Render("└" + strings.Repeat("─", width-2) + "┘"))

	return b.String()
}

func renderOnboardingFrameLine(content string, width int) string {
	padding := max(width-lipgloss.Width(content), 0)
	return styles.OnboardingBorderStyle.Render("│ ") + content + strings.Repeat(" ", padding) + styles.OnboardingBorderStyle.Render(" │")
}

func (state *onboardingState) fieldValue(step int) *string {
	switch step {
	case 0:
		return &state.Host
	case 1:
		return &state.Port
	case 2:
		return &state.ServiceName
	case 3:
		return &state.Username
	case 4:
		return &state.Password
	default:
		panic(fmt.Sprintf("invalid onboarding step: %d", step))
	}
}

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
