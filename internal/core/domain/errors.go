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

	// Queue message errors
	ErrInvalidMessageID = errors.New("invalid message ID")
	ErrInvalidLogLevel  = errors.New("invalid log level")
	ErrInvalidPayload   = errors.New("payload cannot be empty")
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// Database settings errors
	ErrEmptyDatabaseID         = errors.New("database ID cannot be empty")
	ErrEmptyDatabase           = errors.New("database name cannot be empty")
	ErrEmptyHost               = errors.New("host cannot be empty")
	ErrInvalidPort             = errors.New("invalid port number")
	ErrEmptyUsername           = errors.New("username cannot be empty")
	ErrEmptyPassword           = errors.New("password cannot be empty")
	ErrInvalidConnection       = errors.New("invalid connection string")
	ErrHostUnreachable         = errors.New("host is unreachable")
	ErrDefaultSettingsNotFound = errors.New("default database settings not found")

	// Permission errors
	ErrPermissionDenied      = errors.New("permission denied")
	ErrMissingPermissions    = errors.New("missing required permissions")
	ErrPermissionCheckFailed = errors.New("permission check failed")
)
