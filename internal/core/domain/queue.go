package domain

// Value Object : Queue Name
const (
	QueueName = "OMNI_TRACER_QUEUE"
)

// Entity : Subscriber information
type Subscriber struct {
	Name string
}

// Errors: Subscriber Entity
var (
	ErrSubscriberNotFound = "subscriber name not found"
)
