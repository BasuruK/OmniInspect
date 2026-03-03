package domain

import (
	"encoding/json"
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

// NewLogLevel creates a LogLevel with validation
func NewLogLevel(level string) (LogLevel, error) {
	normalized := strings.ToUpper(strings.TrimSpace(level))
	switch LogLevel(normalized) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError, LogLevelCritical:
		return LogLevel(normalized), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidLogLevel, level)
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
func NewQueueMessage(messageID string, processName string, logLevel LogLevel, payload string, timestamp time.Time) (*QueueMessage, error) {
	messageID = strings.TrimSpace(messageID)
	payload = strings.TrimSpace(payload)

	// Validate message ID
	if messageID == "" {
		return nil, ErrInvalidMessageID
	}

	// Validate payload
	if payload == "" {
		return nil, ErrInvalidPayload
	}

	// Validate log level
	normalizedLevel, err := NewLogLevel(logLevel.String())
	if err != nil {
		return nil, err
	}

	// Validate timestamp
	if timestamp.IsZero() {
		return nil, ErrInvalidTimestamp
	}

	return &QueueMessage{
		messageID:   messageID,
		processName: processName,
		logLevel:    normalizedLevel,
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

// ==========================================
// JSON Marshaling
// ==========================================

// queueMessageJSON provides a JSON-friendly intermediate representation
// Uses json.RawMessage for timestamp to handle both int64 and string formats
type queueMessageJSON struct {
	MessageID   string          `json:"message_id"`
	ProcessName string          `json:"process_name"`
	LogLevel    string          `json:"log_level"`
	Payload     string          `json:"payload"`
	Timestamp   json.RawMessage `json:"timestamp"`
}

// MarshalJSON implements custom JSON marshaling for QueueMessage
func (m *QueueMessage) MarshalJSON() ([]byte, error) {
	j := queueMessageJSON{
		MessageID:   m.messageID,
		ProcessName: m.processName,
		LogLevel:    string(m.logLevel),
		Payload:     m.payload,
		Timestamp:   []byte(fmt.Sprintf(`%d`, m.timestamp.Unix())),
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements custom JSON unmarshaling for QueueMessage
// Handles timestamp as both int64 and string formats from Oracle
func (m *QueueMessage) UnmarshalJSON(data []byte) error {
	var j queueMessageJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return fmt.Errorf("failed to unmarshal QueueMessage: %w", err)
	}

	normalizedLevel, err := NewLogLevel(j.LogLevel)
	if err != nil {
		return err
	}

	// Parse timestamp - handle both int64 and string formats
	var ts time.Time
	if len(j.Timestamp) == 0 || string(j.Timestamp) == "null" {
		return ErrInvalidTimestamp
	} else {
		// Try parsing as int64 first (Unix timestamp)
		var unixTs int64
		if err := json.Unmarshal(j.Timestamp, &unixTs); err == nil {
			ts = time.Unix(unixTs, 0)
		} else {
			// Try parsing as string (Oracle date format like "03-MAR-26" or "2026-03-03 13:43:07")
			var tsStr string
			if err := json.Unmarshal(j.Timestamp, &tsStr); err == nil {
				// Try multiple date formats
				formats := []string{
					"2006-01-02 15:04:05",
					"02-JAN-06",
					"02-JAN-2006",
					time.RFC3339,
				}
				for _, format := range formats {
					if parsed, err := time.Parse(format, tsStr); err == nil {
						ts = parsed
						break
					}
				}
				if ts.IsZero() {
					return fmt.Errorf("%w: %q", ErrInvalidTimestamp, tsStr)
				}
			} else {
				return ErrInvalidTimestamp
			}
		}
	}

	qm, err := NewQueueMessage(j.MessageID, j.ProcessName, normalizedLevel, j.Payload, ts)
	if err != nil {
		return err
	}
	*m = *qm
	return nil
}
