package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"log"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Database Settings Sub-State
// ==========================================

type databaseSettingsState struct {
	databaseList              DatabaseList
	databases                 []domain.DatabaseSettings
	activeID                  string
	visible                   bool
	showAddForm               bool
	editingID                 string
	editingOriginalStorageKey string // storage key captured when edit form opens
	deleteConfirmID           string
	showDeleteConfirm         bool
	addForm                   AddDatabaseForm
	dialogMsg                 string
	showDialog                bool
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

// settingsPanelWidth returns the base responsive settings panel width for
// `settingsPanelWidth()`, bounded to 60-92 columns and typically landing near
// ~70% of the terminal. The final rendered width is also capped by
// `screenContentSize()`, so the panel will not exceed the available content
// area.
func settingsPanelWidth(termWidth int) int {
	contentWidth, _ := screenContentSize(termWidth, 1)
	preferredWidth := max(min(termWidth-10, 92), 60)
	return min(preferredWidth, max(contentWidth, 1))
}

// initDatabaseSettings: initializes the database settings panel with the list of databases and marks it visible.
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

// resizeDatabaseSettings: updates the database settings panel dimensions when the terminal is resized.
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

// closeDatabaseSettings: hides the database settings panel and resets all overlay states.
func (m *Model) closeDatabaseSettings() {
	m.dbSettings.visible = false
	m.dbSettings.showAddForm = false
	m.dbSettings.showDialog = false
	m.dbSettings.dialogMsg = ""
	m.dbSettings.editingID = ""
	m.dbSettings.addForm.editingDB = nil
	m.dbSettings.editingOriginalStorageKey = ""
}

// ==========================================
// Update
// ==========================================

// updateDatabaseSettings: handles messages for the database settings panel including navigation, selection, and add form overlay.
func (m *Model) updateDatabaseSettings(msg tea.Msg) (*Model, tea.Cmd) {
	// Delegate to add-form overlay when open
	if m.dbSettings.showAddForm {
		// Handle window resize to adjust form dimensions
		if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
			m.resizeDatabaseSettings(sizeMsg.Width, sizeMsg.Height)
		}

		// Handle ctrl+c specially to quit
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}
		// Forward the original msg to allow paste and other message types
		var cmd tea.Cmd
		m.dbSettings.addForm, cmd = m.dbSettings.addForm.Update(msg)
		if m.dbSettings.addForm.IsCancelled() {
			m.dbSettings.showAddForm = false
			m.dbSettings.editingID = ""
			m.dbSettings.editingOriginalStorageKey = ""
			m.dbSettings.addForm.editingDB = nil
			return m, nil
		}
		if m.dbSettings.addForm.IsSubmitted() {
			cmd := m.saveAddFormCmd()
			m.dbSettings.showAddForm = false
			m.dbSettings.editingID = ""
			m.dbSettings.editingOriginalStorageKey = ""
			m.dbSettings.addForm.editingDB = nil
			return m, cmd
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// While the delete confirm modal is open, only allow confirm/cancel keys.
		if m.dbSettings.showDeleteConfirm {
			switch msg.String() {
			case "ctrl+c":
				m.cancel()
				return m, tea.Quit
			case "y":
				return m, func() tea.Msg {
					return deleteConfirmedMsg{id: m.dbSettings.deleteConfirmID}
				}
			case "n", "esc", "q":
				m.dbSettings.showDeleteConfirm = false
				m.dbSettings.deleteConfirmID = ""
			}
			return m, nil
		}

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
		case "e":
			cursor := m.dbSettings.databaseList.Cursor()
			if cursor >= 0 && cursor < len(m.dbSettings.databases) {
				selected := m.dbSettings.databases[cursor]
				return m, func() tea.Msg {
					return editDatabaseMsg{id: selected.ID()}
				}
			}
			return m, nil
		case "x":
			cursor := m.dbSettings.databaseList.Cursor()
			if cursor >= 0 && cursor < len(m.dbSettings.databases) {
				selected := m.dbSettings.databases[cursor]
				m.dbSettings.deleteConfirmID = selected.ID()
				m.dbSettings.showDeleteConfirm = true
			}
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

	case editDatabaseMsg:
		for i := range m.dbSettings.databases {
			if m.dbSettings.databases[i].ID() == msg.id {
				db := &m.dbSettings.databases[i]
				m.dbSettings.addForm = NewAddDatabaseForm(m.width, m.height)
				m.dbSettings.addForm.SetFieldValue(formFieldDatabaseID, db.DatabaseID())
				m.dbSettings.addForm.SetFieldValue(formFieldHost, db.Host())
				m.dbSettings.addForm.SetFieldValue(formFieldPort, fmt.Sprintf("%d", db.Port().Int()))
				m.dbSettings.addForm.SetFieldValue(formFieldService, db.Database())
				m.dbSettings.addForm.SetFieldValue(formFieldUser, db.Username())
				m.dbSettings.addForm.SetFieldValue(formFieldPass, db.Password())
				m.dbSettings.addForm.editingDB = db
				m.dbSettings.editingID = db.ID()
				m.dbSettings.editingOriginalStorageKey = db.PersistedKey()
				m.dbSettings.showAddForm = true
				break
			}
		}
		return m, nil

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

	case deleteConfirmedMsg:
		// Prevent deletion of active connection
		if msg.id == m.dbSettings.activeID {
			m.dbSettings.dialogMsg = "Cannot delete the currently active database connection."
			m.dbSettings.showDialog = true
			m.dbSettings.showDeleteConfirm = false
			m.dbSettings.deleteConfirmID = ""
			return m, nil
		}
		// Proceed with deletion
		settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
		if err := settingsRepo.Delete(m.ctx, msg.id); err != nil {
			m.dbSettings.showDeleteConfirm = false
			m.dbSettings.deleteConfirmID = ""
			m.dbSettings.dialogMsg = err.Error()
			m.dbSettings.showDialog = true
			return m, nil
		}
		m.dbSettings.showDeleteConfirm = false
		m.dbSettings.deleteConfirmID = ""
		m.reloadDatabaseList()
		return m, nil

	case tea.WindowSizeMsg:
		m.resizeDatabaseSettings(msg.Width, msg.Height)
		return m, nil
	}

	return m, nil
}

// viewDatabaseSettings: renders the database settings panel showing current connection and stored databases list.
func (m *Model) viewDatabaseSettings() string {
	panelWidth := settingsPanelWidth(m.width)
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
		styles.OnboardingHintStyle.Width(innerWidth).Render("↑/↓ Select  •  Enter Connect  •  A Add  •  E Edit  •  X Delete  •  Esc Back"),
	)

	panel := renderFramedPanel("Connections", panelWidth, panelTypeInfo, lipgloss.JoinVertical(lipgloss.Left, parts...))
	return panel
}

// viewDeleteConfirmModal: renders a standalone warning dialog for confirming database deletion.
func (m *Model) viewDeleteConfirmModal() string {
	var dbName string
	for _, db := range m.dbSettings.databases {
		if db.ID() == m.dbSettings.deleteConfirmID {
			dbName = db.DatabaseID()
			break
		}
	}
	// Fallback in case the database was not found (should not happen). Prevention incase of a race condition.
	if dbName == "" {
		dbName = "(unknown)"
	}

	modalWidth := max(min(m.width-20, 60), 44)
	innerWidth := max(modalWidth-4, 24)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.BodyTextStyle.Width(innerWidth).Render("Are you sure you want to delete:"),
		"",
		lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true).Width(innerWidth).Render("  "+dbName),
		"",
		styles.SubtitleStyle.Width(innerWidth).Render("This action cannot be undone."),
		"",
		lipgloss.NewStyle().Foreground(styles.SuccessColor).Bold(true).Render("Y")+" "+
			styles.BodyTextStyle.Render("Confirm")+"   "+
			styles.SubtitleStyle.Render("N / Esc  Cancel"),
	)

	return renderFramedPanel("Confirm Delete", modalWidth, panelTypeWarning, content)
}

// ==========================================
// Database Switching
// ==========================================

func (m *Model) showDatabaseSwitchError(err error) (*Model, tea.Cmd) {
	log.Printf("[UI] Database switch failed: %v", err)
	m.loading.err = err
	m.dbSettings.visible = true
	m.dbSettings.dialogMsg = err.Error()
	m.dbSettings.showDialog = true
	return m, nil
}

func (m *Model) markActiveConnectionPermissionsValidated() error {
	if m.appConfig == nil {
		return fmt.Errorf("active database configuration is required")
	}

	m.appConfig.MarkPermissionsValidated()
	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	if err := settingsRepo.Save(m.ctx, *m.appConfig); err != nil {
		return fmt.Errorf("save validated connection %q: %w", m.appConfig.DatabaseID(), err)
	}

	for index := range m.dbSettings.databases {
		if m.dbSettings.databases[index].ID() == m.appConfig.ID() {
			m.dbSettings.databases[index].MarkPermissionsValidated()
			break
		}
	}

	return nil
}

// databaseSettingsWithDefaultState: returns a copy of the given settings with the default flag set as specified, preserving permission validation state.
func databaseSettingsWithDefaultState(settings domain.DatabaseSettings, isDefault bool) (domain.DatabaseSettings, error) {
	updated, err := domain.NewDatabaseSettings(
		settings.DatabaseID(),
		settings.Database(),
		settings.Host(),
		settings.Port(),
		settings.Username(),
		settings.Password(),
	)
	if err != nil {
		return domain.DatabaseSettings{}, fmt.Errorf("clone database settings %q: %w", settings.DatabaseID(), err)
	}
	if settings.PermissionsValidated() {
		updated.MarkPermissionsValidated()
	}
	if isDefault {
		updated.SetAsDefault()
	}
	return *updated, nil
}

// syncDatabaseSettingsDefaults: updates the default state of the database settings in the UI list based on the selected database.
func (m *Model) syncDatabaseSettingsDefaults(selectedDb domain.DatabaseSettings) {
	for index := range m.dbSettings.databases {
		updated, err := databaseSettingsWithDefaultState(m.dbSettings.databases[index], m.dbSettings.databases[index].ID() == selectedDb.ID())
		if err != nil {
			log.Printf("[UI] Failed to sync database default state for %s: %v", m.dbSettings.databases[index].DatabaseID(), err)
			continue
		}
		m.dbSettings.databases[index] = updated
	}
}

// persistDefaultDatabaseSelection: updates the default database selection in the repository.
func (m *Model) persistDefaultDatabaseSelection(previousDefault *domain.DatabaseSettings, selectedDb domain.DatabaseSettings) error {
	if m.boltAdapter == nil {
		return nil
	}

	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	var updatedPrevious *domain.DatabaseSettings

	if previousDefault != nil && previousDefault.ID() != selectedDb.ID() && previousDefault.IsDefault() {
		clearedPrevious, err := databaseSettingsWithDefaultState(*previousDefault, false)
		if err != nil {
			return fmt.Errorf("clear previous default %q: %w", previousDefault.DatabaseID(), err)
		}
		updatedPrevious = &clearedPrevious
	}

	if err := settingsRepo.SwitchDefault(m.ctx, updatedPrevious, selectedDb); err != nil {
		return fmt.Errorf("persist default database selection %q: %w", selectedDb.DatabaseID(), err)
	}

	return nil
}

// handleSettingsSetAsMain: updates the active database configuration and reinitializes dependent services.
func (m *Model) handleSettingsSetAsMain(selectedDb domain.DatabaseSettings) (*Model, tea.Cmd) {
	newAdapter, err := m.dbFactory(&selectedDb)
	if err != nil {
		return m.showDatabaseSwitchError(fmt.Errorf("failed to initialize database %q: %w", selectedDb.DatabaseID(), err))
	}
	if newAdapter == nil {
		return m.showDatabaseSwitchError(fmt.Errorf("failed to initialize database %q: adapter is nil", selectedDb.DatabaseID()))
	}

	if err := newAdapter.Connect(m.ctx); err != nil {
		_ = newAdapter.Close(m.ctx)
		return m.showDatabaseSwitchError(fmt.Errorf("failed to connect to database %q: %w", selectedDb.DatabaseID(), err))
	}
	if err := newAdapter.Close(m.ctx); err != nil {
		log.Printf("[UI] Failed to close validated database adapter for %s: %v", selectedDb.DatabaseID(), err)
	}

	previousConfig := m.appConfig
	updatedSelected, err := databaseSettingsWithDefaultState(selectedDb, true)
	if err != nil {
		return m.showDatabaseSwitchError(fmt.Errorf("failed to prepare database %q as default: %w", selectedDb.DatabaseID(), err))
	}
	if err := m.persistDefaultDatabaseSelection(previousConfig, updatedSelected); err != nil {
		return m.showDatabaseSwitchError(fmt.Errorf("failed to persist database %q as default: %w", selectedDb.DatabaseID(), err))
	}

	m.resetConnectionEventStream()
	if m.tracerService != nil {
		m.tracerService.CancelConnectionListener()
	}
	if m.dbAdapter != nil {
		if err := m.dbAdapter.Close(m.ctx); err != nil {
			log.Printf("[UI] Failed to close current database adapter: %v", err)
		}
	}

	m.appConfig = &updatedSelected
	m.dbAdapter = newAdapter
	m.syncDatabaseSettingsDefaults(*m.appConfig)

	// Reset dependent services to be reinit with new adapter
	m.permissionService = nil
	m.tracerService = nil
	m.subscriberService = nil
	m.main.messages = nil
	m.main.renderedContent.Reset()
	m.main.ready = false
	m.closeDatabaseSettings()
	m.stopLoadingRetryTimer()
	m.screen = screenLoading
	m.loading.steps = nil
	m.loading.err = nil
	m.loading.retryCount = 0
	m.loading.current = "Connecting..."
	return m, connectDBCmd(m, true)
}

// reloadDatabaseList: reloads the database list from BoltDB storage and updates the UI list.
func (m *Model) reloadDatabaseList() {
	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	databases, err := settingsRepo.GetAll(m.ctx)
	if err != nil {
		log.Printf("[UI] Failed to reload database settings: %v", err)
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

// saveAddFormCmd: async command to validate and save a new or edited database configuration.
// originalStorageKey is non-empty when editing; it is the bolt key of the record being edited.
func (m *Model) saveAddFormCmd() tea.Cmd {
	databaseID, host, portStr, service, username, password := m.dbSettings.addForm.FieldValues()
	originalStorageKey := m.dbSettings.editingOriginalStorageKey
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

		settingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)

		if originalStorageKey != "" {
			// Edit path: build new settings, delete old key if ID changed, save new.
			existing, err := settingsRepo.GetByID(ctx, originalStorageKey)
			if err != nil {
				return dbValidationResultMsg{err: fmt.Errorf("failed to load existing record: %w", err)}
			}
			if err := existing.Update(databaseID, service, host, dbPort, username, password); err != nil {
				return dbValidationResultMsg{err: fmt.Errorf("failed to update settings: %w", err)}
			}
			if err := settingsRepo.Replace(ctx, originalStorageKey, *existing); err != nil {
				return dbValidationResultMsg{err: fmt.Errorf("failed to replace entry: %w", err)}
			}
			return dbValidationResultMsg{settings: existing}
		}

		// Add path: create a brand-new record.
		settings, err := domain.NewDatabaseSettings(databaseID, service, host, dbPort, username, password)
		if err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("failed to create settings: %w", err)}
		}
		if err := settingsRepo.Save(ctx, *settings); err != nil {
			return dbValidationResultMsg{err: fmt.Errorf("failed to save: %w", err)}
		}
		return dbValidationResultMsg{settings: settings}
	}
}
