package ports

// Port: QueueRepository Defines the interface for queue operations
type QueueRepository interface {
	Start() error
}
