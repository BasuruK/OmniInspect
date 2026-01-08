package domain

import "errors"

// Value Object : Queue Name
const (
	QueueName      = "OMNI_TRACER_QUEUE"
	QueueTableName = "AQ$OMNI_TRACER_QUEUE"
)

// Entity : Subscriber information
type Subscriber struct {
	SubscriberID string
	Name         string
	BatchSize    int
	WaitTime     int
}

// Entity : Represents a message in the tracer queue
type QueueMessage struct {
	MessageID   string `json:"MESSAGE_ID"`
	ProcessName string `json:"PROCESS_NAME"`
	LogLevel    string `json:"LOG_LEVEL"`
	Payload     string `json:"PAYLOAD"`
	Timestamp   string `json:"TIMESTAMP"`
}

// Errors: Subscriber Entity
var (
	ErrSubscriberNotFound = errors.New("subscriber name not found")
)
