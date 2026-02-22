package domain

import "errors"

// Entity : Subscriber information
type Subscriber struct {
	Name      string
	BatchSize int
	WaitTime  int
}

// Errors: Subscriber Entity
var (
	ErrSubscriberNotFound = errors.New("subscriber name not found")
)
