package ui

import (
	"OmniView/internal/adapter/logger"
	"OmniView/internal/core/domain"
	"OmniView/internal/updater"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Welcome screen messages
// ==========================================

// dbReadyMsg signals that database config check is complete.
type dbReadyMsg struct {
	settings *domain.DatabaseSettings
	err      error
}

// welcomeCompleteMsg signals the welcome animation is done.
type welcomeCompleteMsg struct{}

// welcomeResizeMsg carries window size events for the welcome screen.
type welcomeResizeMsg struct {
	Width, Height int
}

// ==========================================
// Loading Screen messages
// ==========================================

// startLoadingMsg tells Update() to begin the loading sequence.
type startLoadingMsg struct{}

// dbConnectedMsg is returned after Oracle DB connection attempt.
// isSwitch indicates whether this connection attempt is part of a database switch.
type dbConnectedMsg struct {
	err      error
	isSwitch bool
}

// permissionsCheckedMsg is returned after permission deploy/check.
type permissionsCheckedMsg struct {
	err error
}

// tracerDeployedMsg is returned after tracer deploy/check.
type tracerDeployedMsg struct {
	err error
}

// subscriberRegisteredMsg is returned after subscriber registration.
type subscriberRegisteredMsg struct {
	subscriber *domain.Subscriber
	err        error
}

// loadingCompleteMsg signals all loading steps succeeded.
type loadingCompleteMsg struct{}

// ==========================================
// Main Screen messages
// ==========================================

// queueMessageMsg wraps a single log message from the event listener.
type queueMessageMsg struct {
	message *domain.QueueMessage
}

// tracesClearedMsg is sent when MCP (or another non-UI consumer) empties the shared trace buffer.
type tracesClearedMsg struct{}

// broadcastModeChangedMsg is sent when MCP persists a new broadcast mode.
type broadcastModeChangedMsg struct {
	mode domain.BroadcastMode
}

// mcpStoppedMsg is sent when the in-process MCP HTTP server exits unexpectedly.
type mcpStoppedMsg struct{}

// mcpConnectDatabaseMsg asks the TUI to adopt a database MCP already set as default.
type mcpConnectDatabaseMsg struct {
	id string // storage key
}

// ==========================================
// Onboarding Screen messages
// ==========================================

// onboardingCompleteMsg is sent after the user submits the onboarding form
// and the config has been saved to BoltDB.
type onboardingCompleteMsg struct {
	config *domain.DatabaseSettings
	err    error
}

// ==========================================
// Database Settings Screen messages
// ==========================================

// dbValidationResultMsg is returned after testing a new DB connection.
type dbValidationResultMsg struct {
	settings *domain.DatabaseSettings
	err      error
}

// dbSwitchResultMsg is returned after attempting to switch the active DB.
type dbSwitchResultMsg struct {
	err error
}

// editDatabaseMsg triggers edit mode for a database entry.
type editDatabaseMsg struct {
	id string
}

// confirmDeleteMsg shows the delete confirmation dialog for a database.
type confirmDeleteMsg struct {
	id string
}

// deleteConfirmedMsg confirms deletion of a database.
type deleteConfirmedMsg struct {
	id string
}

// dropSubscriberProcedureMsg requests dropping the current subscriber's procedure.
type dropSubscriberProcedureMsg struct {
	funnyName string
}

// dropSubscriberProcedureResultMsg returns the result of a drop procedure attempt.
type dropSubscriberProcedureResultMsg struct {
	err error
}

// webhookConfigSavedMsg is returned after attempting to save or clear the webhook configuration.
type webhookConfigSavedMsg struct {
	config  *domain.WebhookConfig
	deleted bool
	err     error
}

// ==========================================
// Updater messages
// ==========================================

// updateCheckResultMsg is returned after checking for updates.
type updateCheckResultMsg struct {
	info *updater.UpdateInfo
	err  error
}

// updateUserResponseMsg is returned after the user responds to an update prompt.
type updateUserResponseMsg struct {
	accepted bool
}

// updateProgressMsg reports the current stage of the update process.
type updateProgressMsg struct {
	stage string
}

// updateCompleteMsg signals the update was successfully applied.
type updateCompleteMsg struct{}

// updateErrorMsg signals an update-related error.
type updateErrorMsg struct {
	err error
}

// ==========================================
// MCP → TUI message handlers
// ==========================================

func (m *Model) handleMCPStoppedMsg(_ mcpStoppedMsg) (*Model, tea.Cmd) {
	m.mcpActive = false
	return m, nil
}

// handleMCPConnectDatabaseMsg adopts a database MCP already probed + SetDefault'd.
// Skips handleSettingsSetAsMain's blocking Connect probe; loading reconnect uses connectDBCmd.
func (m *Model) handleMCPConnectDatabaseMsg(msg mcpConnectDatabaseMsg) (*Model, tea.Cmd) {
	if msg.id == "" {
		return m, nil
	}
	if m.screen == screenLoading && m.loading.started && !m.loading.complete {
		logger.Warn("mcp connect: switch already in progress, dropping", "id", msg.id)
		return m, nil
	}
	if m.dbSettings.showAddForm || m.dbSettings.editingID != "" {
		logger.Warn("mcp connect: database settings edit in progress, dropping", "id", msg.id)
		return m, nil
	}
	if m.screen == screenOnboarding {
		logger.Warn("mcp connect: onboarding in progress, dropping", "id", msg.id)
		return m, nil
	}
	if m.dbSettingsRepo == nil {
		logger.Warn("mcp connect: DBSettingsRepo missing")
		return m, nil
	}
	settings, err := m.dbSettingsRepo.GetByID(m.ctx, msg.id)
	if err != nil || settings == nil {
		logger.Warn("mcp connect: lookup failed", "id", msg.id, "error", err)
		return m, nil
	}
	// Same logical DB as appConfig — skip reload, keep list nav on resolved ID().
	if m.appConfig != nil && m.appConfig.ID() == settings.ID() {
		m.dbSettings.activeID = settings.ID()
		return m, nil
	}

	newAdapter, err := m.dbFactory(settings)
	if err != nil {
		return m.showDatabaseSwitchError(fmt.Errorf("failed to initialize database %q: %w", settings.DatabaseID(), err))
	}
	if newAdapter == nil {
		return m.showDatabaseSwitchError(fmt.Errorf("failed to initialize database %q: adapter is nil", settings.DatabaseID()))
	}

	oldTracer := m.tracerService
	oldAdapter := m.dbAdapter
	dbID := ""
	if m.appConfig != nil {
		dbID = m.appConfig.DatabaseID()
	}

	m.resetConnectionEventStream()
	m.appConfig = settings
	m.dbAdapter = newAdapter
	m.dbSettings.activeID = settings.ID()
	m.syncDatabaseSettingsDefaults(*m.appConfig)

	m.permissionService = nil
	m.tracerService = nil
	m.subscriberService = nil
	m.resetMainLogState()
	m.main.ready = false
	if m.dbSettings.visible {
		m.closeDatabaseSettings()
	}
	m.stopLoadingRetryTimer()
	m.screen = screenLoading
	m.loading.steps = nil
	m.loading.err = nil
	m.loading.started = true
	m.loading.complete = false
	m.loading.retryCount = 0
	m.loading.current = "Connecting..."
	return m, tea.Batch(
		teardownPriorConnectionCmd(m.ctx, oldTracer, oldAdapter, dbID),
		connectDBCmd(m, true),
	)
}
