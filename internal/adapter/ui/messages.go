package ui

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/updater"
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
