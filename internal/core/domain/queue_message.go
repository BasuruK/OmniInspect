package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel string

// ==========================================
// Constants
// ==========================================

const (
	LogLevelDebug    LogLevel = "DEBUG"
	LogLevelInfo     LogLevel = "INFO"
	LogLevelWarning  LogLevel = "WARNING"
	LogLevelError    LogLevel = "ERROR"
	LogLevelCritical LogLevel = "CRITICAL"
)

const (
	QueueName        = "OMNI_TRACER_QUEUE"
	QueueTableName   = "AQ$OMNI_TRACER_QUEUE"
	QueuePayloadType = "OMNI_TRACER_PAYLOAD_TYPE"
)

// ==========================================
// Errors
// ==========================================

var (
	ErrInvalidMessageID = errors.New("invalid message ID")
	ErrInvalidLogLevel  = errors.New("invalid log level")
	ErrInvalidPayload   = errors.New("payload cannot be empty")
	ErrInvalidTimestamp = errors.New("invalid timestamp")
)

// NewLogLevel creates a LogLevel with validation
func NewLogLevel(level string) (LogLevel, error) {
	normalized := strings.ToUpper(strings.TrimSpace(level))
	switch LogLevel(normalized) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError, LogLevelCritical:
		return LogLevel(normalized), nil
	default:
		return "", fmt.Errorf("invalid log level: %s", level)
	}
}

func (l LogLevel) String() string { return string(l) }
func (l LogLevel) IsError() bool  { return l == LogLevelError || l == LogLevelCritical }

// Entity : Represents a message in the tracer queue
type QueueMessage struct {
	messageID   string
	processName string
	logLevel    LogLevel
	payload     string
	timestamp   time.Time
}

// NewQueueMessage creates a new QueueMessage with validation
func NewQueueMessage(messageID string, processName string, logLevel LogLevel, payload string, timestamp time.Time,
) (*QueueMessage, error) {
	// Validate message ID
	if strings.TrimSpace(messageID) == "" {
		return nil, ErrInvalidMessageID
	}

	// Validate payload
	if strings.TrimSpace(payload) == "" {
		return nil, ErrInvalidPayload
	}

	return &QueueMessage{
		messageID:   messageID,
		processName: processName,
		logLevel:    logLevel,
		payload:     payload,
		timestamp:   timestamp,
	}, nil
}

// ==========================================
// Getters (Read-Only Accessors)
// ==========================================

func (m *QueueMessage) MessageID() string    { return m.messageID }
func (m *QueueMessage) ProcessName() string  { return m.processName }
func (m *QueueMessage) LogLevel() LogLevel   { return m.logLevel }
func (m *QueueMessage) Payload() string      { return m.payload }
func (m *QueueMessage) Timestamp() time.Time { return m.timestamp }

// ==========================================
// Business Methods
// ==========================================

// IsCritical returns true if this is an error or critical message
func (m *QueueMessage) IsCritical() bool {
	return m.logLevel.IsError()
}

// Format returns a formatted string for display
// This replaces the formatting logic currently in TracerService
func (m *QueueMessage) Format() string {
	return fmt.Sprintf("[%s] [%s] %s: %s",
		m.timestamp.Format("2006-01-02 15:04:05"),
		m.logLevel,
		m.processName,
		m.payload,
	)
}

// String returns string representation
func (m *QueueMessage) String() string {
	return m.Format()
}
