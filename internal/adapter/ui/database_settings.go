package ui

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/spinner"
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
	dialog                    settingsDialog
	// Danger zone fields
	showDropProcedureConfirm bool
	dropProcedureConfirmMsg  string
	dropProcedureTarget      string
	dropProcedureDeleting    bool
	dropProcedureResultMsg   string
	dropProcedureResultIsErr bool
	spinner                  spinner.Model
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

func keyMatchesRune(msg tea.KeyPressMsg, want rune) bool {
	if unicode.ToLower(msg.Code) == want {
		return true
	}
	text := []rune(msg.Text)
	return len(text) == 1 && unicode.ToLower(text[0]) == want
}

func isConfirmKey(msg tea.KeyPressMsg) bool {
	return keyMatchesRune(msg, 'y')
}

func isCancelKey(msg tea.KeyPressMsg) bool {
	return keyMatchesRune(msg, 'n') || keyMatchesRune(msg, 'q') || msg.String() == "esc"
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
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(styles.WarningColor)),
		),
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
	m.dbSettings.dialog.clear()
	m.dbSettings.editingID = ""
	m.dbSettings.addForm.editingDB = nil
	m.dbSettings.editingOriginalStorageKey = ""
	m.dbSettings.showDropProcedureConfirm = false
	m.dbSettings.dropProcedureConfirmMsg = ""
	m.dbSettings.dropProcedureTarget = ""
	m.dbSettings.dropProcedureDeleting = false
	m.dbSettings.dropProcedureResultMsg = ""
	m.dbSettings.dropProcedureResultIsErr = false
}

// ==========================================
// Update
// ==========================================

// clearDatabaseSettingsDialog dismisses the error/info dialog in the database settings panel.
func (m *Model) clearDatabaseSettingsDialog() {
	m.dbSettings.dialog.clear()
}

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
			}
			if isConfirmKey(msg) {
				return m, func() tea.Msg {
					return deleteConfirmedMsg{id: m.dbSettings.deleteConfirmID}
				}
			}
			if isCancelKey(msg) {
				m.dbSettings.showDeleteConfirm = false
				m.dbSettings.deleteConfirmID = ""
			}
			return m, nil
		}

		// While the drop procedure confirm modal is open, only allow confirm/cancel keys.
		if m.dbSettings.showDropProcedureConfirm {
			switch msg.String() {
			case "ctrl+c":
				m.cancel()
				return m, tea.Quit
			}
			if isConfirmKey(msg) {
				m.dbSettings.showDropProcedureConfirm = false
				m.dbSettings.dropProcedureConfirmMsg = ""
				m.dbSettings.dropProcedureDeleting = true
				target := m.dbSettings.dropProcedureTarget
				return m, tea.Batch(
					m.dbSettings.spinner.Tick,
					func() tea.Msg {
						return dropSubscriberProcedureMsg{funnyName: target}
					},
				)
			}
			if isCancelKey(msg) {
				m.dbSettings.showDropProcedureConfirm = false
				m.dbSettings.dropProcedureConfirmMsg = ""
				m.dbSettings.dropProcedureTarget = ""
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "q", "esc":
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
			if m.dbSettings.dialog.visible {
				m.dbSettings.dialog.clear()
				return m, nil
			}
			if m.dbSettings.dropProcedureResultMsg != "" {
				m.dbSettings.dropProcedureResultMsg = ""
				m.dbSettings.dropProcedureResultIsErr = false
				return m, nil
			}
			m.closeDatabaseSettings()
			return m, nil
		case "a":
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
			m.dbSettings.addForm = NewAddDatabaseForm(m.width, m.height)
			m.dbSettings.showAddForm = true
			return m, nil
		case "e":
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
			cursor := m.dbSettings.databaseList.Cursor()
			if cursor >= 0 && cursor < len(m.dbSettings.databases) {
				selected := m.dbSettings.databases[cursor]
				return m, func() tea.Msg {
					return editDatabaseMsg{id: selected.ID()}
				}
			}
			return m, nil
		case "x":
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
			cursor := m.dbSettings.databaseList.Cursor()
			if cursor >= 0 && cursor < len(m.dbSettings.databases) {
				selected := m.dbSettings.databases[cursor]
				m.dbSettings.deleteConfirmID = selected.ID()
				m.dbSettings.showDeleteConfirm = true
			}
			return m, nil
		case "p":
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
			if m.subscriber != nil && m.subscriber.FunnyName() != "" {
				m.dbSettings.dropProcedureTarget = m.subscriber.FunnyName()
				m.dbSettings.dropProcedureConfirmMsg = fmt.Sprintf("This will delete your procedure TRACE_MESSAGE_%s. You can regenerate it by restarting OmniView.", m.dbSettings.dropProcedureTarget)
				m.dbSettings.showDropProcedureConfirm = true
			}
			return m, nil
		case "enter":
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
			cursor := m.dbSettings.databaseList.Cursor()
			if cursor >= 0 && cursor < len(m.dbSettings.databases) {
				selected := m.dbSettings.databases[cursor]
				if selected.ID() != m.dbSettings.activeID {
					return m.handleSettingsSetAsMain(selected)
				}
			}
			return m, nil
		default:
			if m.dbSettings.dropProcedureDeleting {
				return m, nil
			}
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
			m.dbSettings.dialog.set(msg.err.Error(), true)
			return m, nil
		}
		m.reloadDatabaseList()
		return m, nil

	case dbSwitchResultMsg:
		if msg.err != nil {
			m.dbSettings.dialog.set(msg.err.Error(), true)
			return m, nil
		}
		return m, nil

	case deleteConfirmedMsg:
		// Prevent deletion of active connection
		if msg.id == m.dbSettings.activeID {
			m.dbSettings.dialog.set("Cannot delete the currently active database connection.", true)
			m.dbSettings.showDeleteConfirm = false
			m.dbSettings.deleteConfirmID = ""
			return m, nil
		}
		// Proceed with deletion
		settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
		if err := settingsRepo.Delete(m.ctx, msg.id); err != nil {
			m.dbSettings.showDeleteConfirm = false
			m.dbSettings.deleteConfirmID = ""
			m.dbSettings.dialog.set(err.Error(), true)
			return m, nil
		}
		m.dbSettings.showDeleteConfirm = false
		m.dbSettings.deleteConfirmID = ""
		m.reloadDatabaseList()
		return m, nil

	case dropSubscriberProcedureMsg:
		funnyName := msg.funnyName
		if funnyName == "" {
			m.dbSettings.dialog.set("No subscriber procedure to delete.", true)
			m.dbSettings.dropProcedureDeleting = false
			return m, nil
		}
		return m, func() tea.Msg {
			var dropErr error
			if m.subscriberService == nil {
				dropErr = fmt.Errorf("drop subscriber procedure: %w", domain.ErrProcedureGeneration)
			} else {
				dropErr = m.subscriberService.DropSubscriberProcedure(m.ctx, funnyName)
			}
			return dropSubscriberProcedureResultMsg{err: dropErr}
		}
	case spinner.TickMsg:
		if m.dbSettings.dropProcedureDeleting {
			var cmd tea.Cmd
			m.dbSettings.spinner, cmd = m.dbSettings.spinner.Update(msg)
			return m, cmd
		}

	case dropSubscriberProcedureResultMsg:
		m.dbSettings.dropProcedureDeleting = false
		if msg.err != nil {
			m.dbSettings.dropProcedureResultMsg = fmt.Sprintf("Failed to delete procedure: %v", msg.err)
			m.dbSettings.dropProcedureResultIsErr = true
			return m, nil
		}
		m.dbSettings.dropProcedureResultMsg = "Procedure deleted successfully. Restart OmniView to regenerate."
		m.dbSettings.dropProcedureResultIsErr = false
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
			BorderColor: styles.ConnectionBorderColor,
		}),
		"",
		m.dbSettings.databaseList.Render(),
	}

	parts = append(parts, renderSettingsDialogLines(m.dbSettings.dialog, innerWidth)...)

	// Danger Zone for subscriber procedure deletion
	if m.subscriber != nil && m.subscriber.FunnyName() != "" {
		var dangerContent string
		if m.dbSettings.dropProcedureDeleting {
			dangerContent = lipgloss.JoinHorizontal(
				lipgloss.Left,
				m.dbSettings.spinner.View(),
				"  ",
				styles.SubtitleStyle.Render("Deleting procedure, please wait a moment..."),
			)
		} else if m.dbSettings.dropProcedureResultMsg != "" {
			resultStyle := styles.OnboardingSavedStyle
			if m.dbSettings.dropProcedureResultIsErr {
				resultStyle = styles.OnboardingErrorStyle
			}
			dangerContent = lipgloss.JoinVertical(
				lipgloss.Left,
				resultStyle.Render(m.dbSettings.dropProcedureResultMsg),
				"",
				styles.SubtitleStyle.Render("Press Esc to dismiss."),
			)
		} else {
			dangerContent = styles.SubtitleStyle.Render("Press P to delete your subscriber procedure.")
		}
		parts = append(parts,
			"",
			renderEmbeddedField(embeddedFieldOptions{
				Label:       "Danger Zone",
				Value:       dangerContent,
				Width:       innerWidth,
				BorderColor: styles.ErrorColor,
			}),
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

// viewDropProcedureConfirmModal: renders a standalone warning dialog for confirming procedure deletion.
func (m *Model) viewDropProcedureConfirmModal() string {
	funnyName := m.dbSettings.dropProcedureTarget

	modalWidth := max(min(m.width-20, 60), 44)
	innerWidth := max(modalWidth-4, 24)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.DangerZoneStyle.Width(innerWidth).Render("Delete Subscriber Procedure"),
		"",
		styles.BodyTextStyle.Width(innerWidth).Render("Are you sure you want to delete your procedure?"),
		"",
		lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true).Width(innerWidth).Render("  TRACE_MESSAGE_"+funnyName),
		"",
		styles.SubtitleStyle.Width(innerWidth).Render("You can regenerate it by restarting OmniView."),
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
	logger.Error("database switch failed", "error", err)
	m.loading.err = err
	m.dbSettings.visible = true
	m.dbSettings.dialog.set(err.Error(), true)
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
			logger.Warn("failed to sync database default state", "databaseID", m.dbSettings.databases[index].DatabaseID(), "error", err)
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
		logger.Warn("failed to close validated database adapter", "databaseID", selectedDb.DatabaseID(), "error", err)
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
			dbID := ""
			if m.appConfig != nil {
				dbID = m.appConfig.DatabaseID()
			}
			logger.Warn("failed to close current database adapter", "databaseID", dbID, "error", err)
		}
	}

	m.appConfig = &updatedSelected
	m.dbAdapter = newAdapter
	m.syncDatabaseSettingsDefaults(*m.appConfig)

	// Reset dependent services to be reinit with new adapter
	m.permissionService = nil
	m.tracerService = nil
	m.subscriberService = nil
	m.resetMainLogState()
	m.main.ready = false
	m.closeDatabaseSettings()
	m.stopLoadingRetryTimer()
	m.screen = screenLoading
	m.loading.steps = nil
	m.loading.err = nil
	m.loading.started = true
	m.loading.complete = false
	m.loading.retryCount = 0
	m.loading.current = "Connecting..."
	return m, connectDBCmd(m, true)
}

// reloadDatabaseList: reloads the database list from BoltDB storage and updates the UI list.
func (m *Model) reloadDatabaseList() {
	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	databases, err := settingsRepo.GetAll(m.ctx)
	if err != nil {
		logger.Error("failed to reload database settings", "error", err)
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
