package domain

import "errors"

// ==========================================
// Sentinel Errors
// ==========================================

var (
	// Subscriber errors
	ErrSubscriberNotFound    = errors.New("subscriber not found")
	ErrInvalidSubscriberName = errors.New("invalid subscriber name")
	ErrSubscriberNotActive   = errors.New("subscriber is not active")
	ErrInvalidBatchSize      = errors.New("invalid batch size")
	ErrInvalidWaitTime       = errors.New("invalid wait time")
	ErrInvalidFunnyName      = errors.New("invalid funny name")
	ErrNoAvailableNames      = errors.New("no available funny names")
	ErrFunnyNameTooLong      = errors.New("funny name exceeds maximum length")
	ErrFunnyNameTooShort     = errors.New("funny name too short")
	ErrNilSubscriber         = errors.New("nil subscriber")
	ErrTracerNotInitialized  = errors.New("tracer service not initialized")

	// Queue message errors
	ErrInvalidMessageID = errors.New("invalid message ID")
	ErrInvalidLogLevel  = errors.New("invalid log level")
	ErrInvalidPayload   = errors.New("payload cannot be empty")
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// Database settings errors
	ErrEmptyDatabaseID         = errors.New("database ID cannot be empty")
	ErrKeyCollision            = errors.New("key collision")
	ErrEmptyDatabase           = errors.New("database name cannot be empty")
	ErrEmptyHost               = errors.New("host cannot be empty")
	ErrInvalidPort             = errors.New("invalid port number")
	ErrEmptyUsername           = errors.New("username cannot be empty")
	ErrEmptyPassword           = errors.New("password cannot be empty")
	ErrInvalidConnection       = errors.New("invalid connection string")
	ErrHostUnreachable         = errors.New("host is unreachable")
	ErrDefaultSettingsNotFound = errors.New("default database settings not found")
	ErrNilRepository           = errors.New("repository cannot be nil")
	ErrNilConfig               = errors.New("config repository cannot be nil")

	// Permission errors
	ErrPermissionDenied      = errors.New("permission denied")
	ErrMissingPermissions    = errors.New("missing required permissions")
	ErrPermissionCheckFailed = errors.New("permission check failed")

	// Updater errors
	ErrNoMatchingReleaseAsset = errors.New("no matching release asset")
	ErrNoUpdateInfo           = errors.New("no update info provided")

	// Webhook config errors
	ErrWebhookConfigNotFound = errors.New("webhook config not found")
)
