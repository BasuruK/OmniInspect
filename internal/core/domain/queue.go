package domain

// Value Object : Queue Name
const (
	QueueName        = "OMNI_TRACER_QUEUE"
	QueueTableName   = "AQ$OMNI_TRACER_QUEUE"
	QueuePayloadType = "OMNI_TRACER_PAYLOAD_TYPE"
)

// Value Object : Queue Configuration
type QueueConfig struct{}

func (QueueConfig) Name() string        { return QueueName }
func (QueueConfig) TableName() string   { return QueueTableName }
func (QueueConfig) PayloadType() string { return QueuePayloadType }

// NewQueueConfig creates a new QueueConfig instance
func NewQueueConfig() QueueConfig {
	return QueueConfig{}
}

// Entity : Represents a message in the tracer queue
type QueueMessage struct {
	MessageID   string `json:"MESSAGE_ID"`
	ProcessName string `json:"PROCESS_NAME"`
	LogLevel    string `json:"LOG_LEVEL"`
	Payload     string `json:"PAYLOAD"`
	Timestamp   string `json:"TIMESTAMP"`
}
