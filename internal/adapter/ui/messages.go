package ui

import (
	"OmniView/internal/core/domain"
	"time"
)

// ==========================================
// Welcome screen messages
// ==========================================

// tickMsg drives the welcome animation at 80ms intervals.
type tickMsg struct {
	time time.Time
}

// welcomeCompleteMsg signals the welcome animation is done.
type welcomeCompleteMsg struct{}

// ==========================================
// Loading Screen messages
// ==========================================

// startLoadingMsg tells Update() to begin the loading sequence.
type startLoadingMsg struct{}

// dbConnectedMsg is returned after Oracle DB connection attempt.
type dbConnectedMsg struct {
	err error
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
