package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"strconv"
	"strings"

	"OmniView/internal/service/tracer"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// dbManagerField describes each step in the database registration form.
type dbManagerField struct {
	label       string
	placeholder string
	isPassword  bool
}

var dbManagerFields = []dbManagerField{
	{label: "Database Host", placeholder: "e.g., localhost"},
	{label: "Database Port", placeholder: "e.g., 1521"},
	{label: "Service Name / SID", placeholder: "e.g., ORCL"},
	{label: "Username", placeholder: "e.g., SYSTEM"},
	{label: "Password", placeholder: "password", isPassword: true},
}

// ==========================================
// Update
// ==========================================

func (m *Model) updateDatabaseManager(msg tea.Msg) (*Model, tea.Cmd) {
	// Handle dialog dismissal first
	if m.databaseManager.showDialog {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "q", "enter", "ctrl+c":
				m.databaseManager.showDialog = false
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "q", "esc":
			m.screen = screenMain
			return m, nil
		case "tab":
			if m.databaseManager.focus == "list" {
				m.databaseManager.focus = "form"
			} else {
				m.databaseManager.focus = "list"
			}
			return m, nil
		case "shift+tab":
			if m.databaseManager.focus == "form" {
				m.databaseManager.focus = "list"
			} else {
				m.databaseManager.focus = "form"
			}
			return m, nil
		}

		if m.databaseManager.focus == "list" {
			return m.handleDBListKey(msg)
		}
		return m.handleDBFormKey(msg)

	case dbValidationResultMsg:
		m.databaseManager.submitted = false
		if msg.err != nil {
			m.databaseManager.dialogMsg = msg.err.Error()
			m.databaseManager.showDialog = true
			return m, nil
		}
		// Save succeeded — reload list and select new entry
		settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
		databases, err := settingsRepo.GetAll(m.ctx)
		if err != nil {
			m.databaseManager.databases = nil
		} else {
			m.databaseManager.databases = databases
		}
		// Select the newly saved entry
		for i, db := range m.databaseManager.databases {
			if db.ID() == msg.settings.ID() {
				m.databaseManager.selectedIndex = i
				break
			}
		}
		// Clear form
		m.databaseManager.host = ""
		m.databaseManager.port = ""
		m.databaseManager.serviceName = ""
		m.databaseManager.username = ""
		m.databaseManager.password = ""
		m.databaseManager.step = 0
		m.databaseManager.focus = "list"
		return m, nil

	case dbSwitchResultMsg:
		if msg.err != nil {
			m.databaseManager.dialogMsg = msg.err.Error()
			m.databaseManager.showDialog = true
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleDBListKey(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	switch msg.String() {
	case "up", "shift+tab":
		if m.databaseManager.selectedIndex > 0 {
			m.databaseManager.selectedIndex--
		}
	case "down", "tab":
		if m.databaseManager.selectedIndex < len(m.databaseManager.databases)-1 {
			m.databaseManager.selectedIndex++
		}
	case "n":
		// Switch to form with blank entry for new database
		m.databaseManager.focus = "form"
		m.databaseManager.step = 0
		m.databaseManager.host = ""
		m.databaseManager.port = ""
		m.databaseManager.serviceName = ""
		m.databaseManager.username = ""
		m.databaseManager.password = ""
		m.databaseManager.formErr = ""
	case "enter":
		if len(m.databaseManager.databases) == 0 {
			return m, nil
		}
		selected := m.databaseManager.databases[m.databaseManager.selectedIndex]
		if selected.ID() == m.databaseManager.activeID {
			return m, nil // already active
		}
		return m.handleSetAsMain(selected)
	}
	return m, nil
}

func (m *Model) handleDBFormKey(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	if m.databaseManager.submitted {
		return m, nil
	}

	step := m.databaseManager.step
	value := m.dbManagerFieldValue(step)

	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "up", "shift+tab":
		if step > 0 {
			m.databaseManager.step--
		}
		m.databaseManager.formErr = ""
		return m, nil

	case "down", "tab":
		if step < len(dbManagerFields)-1 {
			m.databaseManager.step++
		}
		m.databaseManager.formErr = ""
		return m, nil

	case "enter":
		if errMsg := validateDBFormField(step, *value); errMsg != "" {
			m.databaseManager.formErr = errMsg
			return m, nil
		}
		m.databaseManager.formErr = ""

		if step < len(dbManagerFields)-1 {
			m.databaseManager.step++
			return m, nil
		}
		// Last field — submit
		m.databaseManager.submitted = true
		return m, saveDBConfigCmd(m)

	case "backspace":
		if len(*value) == 0 {
			if step > 0 {
				m.databaseManager.step--
			}
			m.databaseManager.formErr = ""
			return m, nil
		}
		if len(*value) > 0 {
			*value = (*value)[:len(*value)-1]
		}
		m.databaseManager.formErr = ""

	case "ctrl+u":
		*value = ""
		m.databaseManager.formErr = ""

	case "n":
		// Clear form for new entry
		m.databaseManager.step = 0
		m.databaseManager.host = ""
		m.databaseManager.port = ""
		m.databaseManager.serviceName = ""
		m.databaseManager.username = ""
		m.databaseManager.password = ""
		m.databaseManager.formErr = ""
		return m, nil

	case "c":
		// Cancel — clear form and return to list
		m.databaseManager.step = 0
		m.databaseManager.host = ""
		m.databaseManager.port = ""
		m.databaseManager.serviceName = ""
		m.databaseManager.username = ""
		m.databaseManager.password = ""
		m.databaseManager.formErr = ""
		m.databaseManager.focus = "list"
		return m, nil
	}

	// Character input
	if len(msg.Text) > 0 && !msg.Mod.Contains(tea.ModCtrl) {
		r := []rune(msg.Text)
		if len(r) == 1 && r[0] >= 0x20 && r[0] < 0x7F {
			*value += msg.Text
			m.databaseManager.formErr = ""
		}
	}

	return m, nil
}

func (m *Model) handleSetAsMain(selected domain.DatabaseSettings) (*Model, tea.Cmd) {
	// Stop event listener and webhook dispatcher
	if m.tracerService != nil {
		tracer.StopAll(m.tracerService)
	}
	// Close existing connection
	if m.dbAdapter != nil {
		m.dbAdapter.Close(m.ctx)
	}
	// Swap config
	m.appConfig = &selected
	// Reset mainState
	m.main.messages = nil
	m.main.renderedContent.Reset()
	m.main.ready = false
	// Transition to loading
	m.screen = screenLoading
	m.loading.steps = nil
	m.loading.err = nil
	m.loading.current = "Connecting..."
	return m, connectDBCmd(m)
}

// ==========================================
// View
// ==========================================

func (m *Model) viewDatabaseManager() string {
	totalWidth := min(m.width-4, 100)
	leftW := totalWidth / 2
	rightW := totalWidth - leftW - 1

	leftPane := m.viewDatabaseList(leftW)
	rightPane := m.viewDatabaseForm(rightW)
	divider := styles.DBPaneBorderStyle.Render("│")

	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, divider, rightPane)
	frame := renderTwoPaneFrame("Database Manager", totalWidth, layout)

	var content string
	if m.databaseManager.showDialog {
		dialog := m.viewErrorDialog()
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame) +
			"\n" + dialog
	} else {
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame)
	}

	return content
}

func (m *Model) viewDatabaseList(width int) string {
	listFocus := m.databaseManager.focus == "list"

	var b strings.Builder

	// Title
	title := styles.DBPaneTitleStyle.Render("Registered Databases")
	b.WriteString(title)
	b.WriteString("\n\n")

	if len(m.databaseManager.databases) == 0 {
		empty := styles.DBPaneHintStyle.Render("No databases registered")
		b.WriteString(empty)
		b.WriteString("\n")
	} else {
		for i, db := range m.databaseManager.databases {
			isActive := db.ID() == m.databaseManager.activeID
			isSelected := i == m.databaseManager.selectedIndex

			var itemStyle lipgloss.Style
			var prefix string
			if isActive {
				itemStyle = styles.DBListActiveStyle
				prefix = "● "
			} else if isSelected && listFocus {
				itemStyle = styles.DBListSelectedStyle
				prefix = "> "
			} else if isSelected {
				prefix = "  "
				itemStyle = styles.DBListNormalStyle
			} else {
				prefix = "  "
				itemStyle = styles.DBListNormalStyle
			}

			label := fmt.Sprintf("%s@%s:%d/%s", db.Username(), db.Host(), db.Port().Int(), db.Database())
			if db.IsDefault() {
				label += " [default]"
			}
			b.WriteString(prefix)
			b.WriteString(itemStyle.Render(label))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Hints
	if listFocus {
		hint := "↑/↓ navigate  Enter set as main  Tab ↔ form  n new"
		b.WriteString(styles.DBPaneHintStyle.Render(hint))
	} else {
		hint := "Tab ↔ switch pane  |  n new  |  c cancel"
		b.WriteString(styles.DBPaneHintStyle.Render(hint))
	}

	return lipgloss.NewStyle().Width(width).Height(max(m.height-6, 10)).Render(b.String())
}

func (m *Model) viewDatabaseForm(width int) string {
	formFocus := m.databaseManager.focus == "form"
	separatorWidth := max(width-4, 1)

	var b strings.Builder

	title := styles.DBPaneTitleStyle.Render("Register Database")
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, field := range dbManagerFields {
		isActive := i == m.databaseManager.step
		value := m.dbManagerFieldValueValue(i)
		pointer := " "
		labelStyle := styles.OnboardingFieldLabelStyle
		valueStyle := styles.OnboardingFieldValueStyle

		if isActive && formFocus {
			pointer = styles.DBPaneActiveIndicatorStyle.Render(">")
			labelStyle = styles.OnboardingActiveLabelStyle
			if value != "" {
				valueStyle = styles.OnboardingActiveValueStyle
			}
		} else if value == "" {
			pointer = " "
		}

		b.WriteString(pointer + " " + labelStyle.Render(field.label) + "\n")
		b.WriteString("  " + valueStyle.Render(renderDBFormFieldValue(field, value)) + "\n")
		b.WriteString(styles.OnboardingSeparatorStyle.Render(strings.Repeat("─", separatorWidth)) + "\n")
	}

	if m.databaseManager.formErr != "" {
		b.WriteString(styles.OnboardingErrorStyle.Render(m.databaseManager.formErr) + "\n")
	}

	b.WriteString("\n")

	// Buttons
	if formFocus {
		saveStyle := styles.DBPaneButtonActiveStyle
		cancelStyle := styles.DBPaneButtonStyle
		b.WriteString(fmt.Sprintf("  %s  %s",
			saveStyle.Render("[ Save ]"),
			cancelStyle.Render("[ Cancel ]")))
	} else {
		b.WriteString(fmt.Sprintf("  %s  %s",
			styles.DBPaneButtonStyle.Render("[ Save ]"),
			styles.DBPaneButtonStyle.Render("[ Cancel ]")))
	}

	return lipgloss.NewStyle().Width(width).Height(max(m.height-6, 10)).Render(b.String())
}

func (m *Model) viewErrorDialog() string {
	dialogWidth := 60
	innerWidth := dialogWidth - 4

	top := styles.OnboardingBorderStyle.Render("┌─ Error ─" + strings.Repeat("─", innerWidth-10) + "─┐")
	middle := styles.OnboardingBorderStyle.Render("│ ") +
		styles.DBDialogStyle.Render(m.databaseManager.dialogMsg) +
		strings.Repeat(" ", innerWidth-lipgloss.Width(m.databaseManager.dialogMsg)) +
		styles.OnboardingBorderStyle.Render(" │")
	bottom := styles.OnboardingBorderStyle.Render("└" + strings.Repeat("─", dialogWidth-2) + "─┘")
	hint := styles.OnboardingHintStyle.Render(strings.Repeat(" ", (dialogWidth-lipgloss.Width("[ Cancel ]"))/2) + "[ Cancel ]")

	dialog := lipgloss.JoinVertical(lipgloss.Left, top, middle, bottom, hint)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

func renderTwoPaneFrame(title string, width int, content string) string {
	innerWidth := max(width-4, 1)
	titleText := "[ " + title + " ]"
	topFill := max(width-lipgloss.Width(titleText)-3, 0)

	var b strings.Builder
	b.WriteString(styles.OnboardingBorderStyle.Render("┌─"))
	b.WriteString(styles.OnboardingTitleStyle.Render(titleText))
	b.WriteString(styles.OnboardingBorderStyle.Render(strings.Repeat("─", topFill) + "┐"))
	b.WriteString("\n")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		padding := max(innerWidth-lipgloss.Width(line), 0)
		b.WriteString(styles.OnboardingBorderStyle.Render("│ "))
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(styles.OnboardingBorderStyle.Render(" │"))
		b.WriteString("\n")
	}

	b.WriteString(styles.OnboardingBorderStyle.Render("└" + strings.Repeat("─", width-2) + "┘"))

	return b.String()
}

func (m *Model) dbManagerFieldValue(step int) *string {
	switch step {
	case 0:
		return &m.databaseManager.host
	case 1:
		return &m.databaseManager.port
	case 2:
		return &m.databaseManager.serviceName
	case 3:
		return &m.databaseManager.username
	case 4:
		return &m.databaseManager.password
	default:
		panic(fmt.Sprintf("invalid dbManager step: %d", step))
	}
}

func (m *Model) dbManagerFieldValueValue(step int) string {
	return *m.dbManagerFieldValue(step)
}

func renderDBFormFieldValue(field dbManagerField, value string) string {
	if value == "" {
		return field.placeholder
	}
	if field.isPassword {
		return strings.Repeat("•", min(len(value), 20))
	}
	return value
}

func validateDBFormField(step int, value string) string {
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

// ==========================================
// Async Commands
// ==========================================

func saveDBConfigCmd(m *Model) tea.Cmd {
	host := m.databaseManager.host
	portValue := strings.TrimSpace(m.databaseManager.port)
	serviceName := m.databaseManager.serviceName
	username := m.databaseManager.username
	password := m.databaseManager.password
	ctx := m.ctx
	boltAdapter := m.boltAdapter

	return func() tea.Msg {
		port, err := strconv.Atoi(portValue)
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("invalid port: %w", err)}
		}

		dbPort, err := domain.NewPort(port)
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("invalid port: %w", err)}
		}

		settings, err := domain.NewDatabaseSettings(serviceName, host, dbPort, username, password)
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("failed to create database settings: %w", err)}
		}

		settingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)
		if err := settingsRepo.Save(ctx, *settings); err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		return dbValidationResultMsg{settings: settings, err: nil}
	}
}
