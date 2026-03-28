package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"OmniView/internal/service/tracer"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Database Settings Sub-State
// ==========================================

type databaseSettingsState struct {
	databaseList DatabaseList
	databases    []domain.DatabaseSettings
	activeID     string
	visible      bool
	showAddForm  bool
	addForm      AddDatabaseForm
	dialogMsg    string
	showDialog   bool
}

// ==========================================
// Helpers
// ==========================================

// buildDatabaseEntries converts persisted settings into list entries.
func buildDatabaseEntries(databases []domain.DatabaseSettings, activeID string) []DatabaseEntry {
	entries := make([]DatabaseEntry, 0, len(databases))
	for _, db := range databases {
		status := StatusDisconnected
		if db.ID() == activeID {
			status = StatusConnected
		}
		entries = append(entries, DatabaseEntry{
			Name:    db.DatabaseID(),
			Host:    db.Host(),
			Port:    fmt.Sprintf("%d", db.Port().Int()),
			Service: db.Database(),
			Status:  status,
		})
	}
	return entries
}

// settingsPanelWidth returns a responsive panel width (50–80 cols, ~70% of terminal).
func settingsPanelWidth(termWidth int) int {
	return max(min(termWidth-10, 92), 60)
}

func (m *Model) initDatabaseSettings(databases []domain.DatabaseSettings, activeID string) {
	pw := settingsPanelWidth(m.width)
	innerW := pw - 4
	entries := buildDatabaseEntries(databases, activeID)
	m.dbSettings = databaseSettingsState{
		databaseList: NewDatabaseList(entries, innerW),
		databases:    databases,
		activeID:     activeID,
		visible:      true,
	}
}

func (m *Model) resizeDatabaseSettings(width, height int) {
	if !m.dbSettings.visible {
		return
	}

	pw := settingsPanelWidth(width)
	innerW := pw - 4
	entries := m.dbSettings.databaseList.Entries()
	cursor := m.dbSettings.databaseList.Cursor()
	m.dbSettings.databaseList = NewDatabaseList(entries, innerW).WithCursor(cursor)

	if m.dbSettings.showAddForm {
		m.dbSettings.addForm.width = width
		m.dbSettings.addForm.height = height
	}
}

func (m *Model) closeDatabaseSettings() {
	m.dbSettings.visible = false
	m.dbSettings.showAddForm = false
	m.dbSettings.showDialog = false
	m.dbSettings.dialogMsg = ""
}

// ==========================================
// Update
// ==========================================

func (m *Model) updateDatabaseSettings(msg tea.Msg) (*Model, tea.Cmd) {
	// Delegate to add-form overlay when open
	if m.dbSettings.showAddForm {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() == "ctrl+c" {
				m.cancel()
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.dbSettings.addForm, cmd = m.dbSettings.addForm.Update(keyMsg)
			if m.dbSettings.addForm.IsCancelled() {
				m.dbSettings.showAddForm = false
				return m, nil
			}
			if m.dbSettings.addForm.IsSubmitted() {
				m.dbSettings.showAddForm = false
				return m, m.saveAddFormCmd()
			}
			return m, cmd
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
			if m.dbSettings.showDialog {
				m.dbSettings.showDialog = false
				return m, nil
			}
			m.closeDatabaseSettings()
			return m, nil
		case "a":
			m.dbSettings.addForm = NewAddDatabaseForm(m.width, m.height)
			m.dbSettings.showAddForm = true
			return m, nil
		case "enter":
			cursor := m.dbSettings.databaseList.Cursor()
			if cursor >= 0 && cursor < len(m.dbSettings.databases) {
				selected := m.dbSettings.databases[cursor]
				if selected.ID() != m.dbSettings.activeID {
					return m.handleSettingsSetAsMain(selected)
				}
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.dbSettings.databaseList, cmd = m.dbSettings.databaseList.Update(msg)
			return m, cmd
		}

	case dbValidationResultMsg:
		if msg.err != nil {
			m.dbSettings.dialogMsg = msg.err.Error()
			m.dbSettings.showDialog = true
			return m, nil
		}
		m.reloadDatabaseList()
		return m, nil

	case dbSwitchResultMsg:
		if msg.err != nil {
			m.dbSettings.dialogMsg = msg.err.Error()
			m.dbSettings.showDialog = true
			return m, nil
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.resizeDatabaseSettings(msg.Width, msg.Height)
		return m, nil
	}

	return m, nil
}

func (m *Model) viewDatabaseSettings() string {
	panelWidth := max(min(m.width-10, 92), 60)
	innerWidth := max(panelWidth-4, 24)
	activeSummary := styles.EmptyStateStyle.Render("No active connection configured.")
	if m.appConfig != nil {
		activeSummary = lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.JoinHorizontal(
				lipgloss.Center,
				lipgloss.NewStyle().Foreground(styles.SuccessColor).Bold(true).Render("●"),
				" ",
				styles.BodyTextStyle.Bold(true).Render(m.appConfig.DatabaseID()),
			),
			styles.SubtitleStyle.Render(m.appConfig.DisplayTarget()),
		)
	}

	parts := []string{
		styles.OnboardingTitleStyle.Render("Database Settings"),
		styles.SubtitleStyle.Render("Manage stored Oracle connections and switch the active configuration."),
		"",
		renderEmbeddedField(embeddedFieldOptions{
			Label:       "Current Connection",
			Value:       activeSummary,
			Width:       innerWidth,
			BorderColor: "#F0C802",
		}),
		"",
		styles.SectionTitleStyle.Render("Stored Connections"),
		styles.SubtitleStyle.Render("Use Enter to switch to the selected database connection."),
		"",
		m.dbSettings.databaseList.Render(),
	}

	if m.dbSettings.showDialog && m.dbSettings.dialogMsg != "" {
		parts = append(
			parts,
			"",
			styles.OnboardingErrorStyle.Render("Error: "+m.dbSettings.dialogMsg),
			styles.SubtitleStyle.Width(innerWidth).Render("Press Esc to dismiss the message and stay on this screen."),
		)
	}

	parts = append(
		parts,
		"",
		styles.OnboardingHintStyle.Width(innerWidth).Render("↑/↓ Select  •  Enter Connect  •  a Add Database  •  Esc Back"),
	)

	panel := renderFramedPanel("Connections", panelWidth, lipgloss.JoinVertical(lipgloss.Left, parts...))
	return panel
}

// ==========================================
// Database Switching
// ==========================================

func (m *Model) handleSettingsSetAsMain(selected domain.DatabaseSettings) (*Model, tea.Cmd) {
	if m.tracerService != nil {
		tracer.StopAll(m.tracerService)
	}
	if m.dbAdapter != nil {
		m.dbAdapter.Close(m.ctx)
	}
	m.appConfig = &selected
	m.main.messages = nil
	m.main.renderedContent.Reset()
	m.main.ready = false
	m.closeDatabaseSettings()
	m.screen = screenLoading
	m.loading.steps = nil
	m.loading.err = nil
	m.loading.current = "Connecting..."
	return m, connectDBCmd(m)
}

func (m *Model) reloadDatabaseList() {
	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	databases, err := settingsRepo.GetAll(m.ctx)
	if err != nil {
		m.dbSettings.databases = nil
	} else {
		m.dbSettings.databases = databases
	}
	pw := settingsPanelWidth(m.width)
	innerW := pw - 4
	entries := buildDatabaseEntries(m.dbSettings.databases, m.dbSettings.activeID)
	m.dbSettings.databaseList = NewDatabaseList(entries, innerW)
}

// ==========================================
// Async Commands
// ==========================================

func (m *Model) saveAddFormCmd() tea.Cmd {
	databaseID, host, portStr, service, username, password := m.dbSettings.addForm.FieldValues()
	ctx := m.ctx
	boltAdapter := m.boltAdapter

	return func() tea.Msg {
		port, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("invalid port: %w", err)}
		}
		dbPort, err := domain.NewPort(port)
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("invalid port: %w", err)}
		}
		settings, err := domain.NewDatabaseSettings(databaseID, service, host, dbPort, username, password)
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("failed to create settings: %w", err)}
		}
		settingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)
		if err := settingsRepo.Save(ctx, *settings); err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("failed to save: %w", err)}
		}
		return dbValidationResultMsg{settings: settings}
	}
}
