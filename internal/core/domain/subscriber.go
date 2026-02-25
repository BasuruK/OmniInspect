package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ==========================================
// Constants
// ==========================================

const (
	SubscriberNamePrefix          = "SUB_"
	SubscriberIDLength            = 36 // UUID
	MinWaitTime          WaitTime = 1
	MaxWaitTime          WaitTime = 300
	DefaultWaitTime      WaitTime = 5
)

// ==========================================
// Value Objects
// ==========================================

// BatchSize represents the batch size for processing
type BatchSize int

const (
	MinBatchSize     BatchSize = 1
	MaxBatchSize     BatchSize = 10000
	DefaultBatchSize BatchSize = 1000
)

func NewBatchSize(size int) (BatchSize, error) {
	if size < int(MinBatchSize) || size > int(MaxBatchSize) {
		return 0, fmt.Errorf("batch size must be between %d and %d", MinBatchSize, MaxBatchSize)
	}
	return BatchSize(size), nil
}

func (b BatchSize) Int() int { return int(b) }

// WaitTime represents wait time in seconds
type WaitTime int

func NewWaitTime(seconds int) (WaitTime, error) {
	if seconds < int(MinWaitTime) || seconds > int(MaxWaitTime) {
		return 0, fmt.Errorf("wait time must be between %d and %d seconds", MinWaitTime, MaxWaitTime)
	}
	return WaitTime(seconds), nil
}

func (w WaitTime) Int() int { return int(w) }

// ==========================================
// Errors
// ==========================================

// Errors: Subscriber Entity
var (
	ErrSubscriberNotFound    = errors.New("subscriber not found")
	ErrInvalidSubscriberName = errors.New("invalid subscriber name")
	ErrSubscriberNotActive   = errors.New("subscriber is not active")
	ErrInvalidBatchSize      = errors.New("invalid batch size")
	ErrInvalidWaitTime       = errors.New("invalid wait time")
)

// ==========================================
// Subscriber Entity
// ==========================================

// Entity : Subscriber information
type Subscriber struct {
	id        string
	name      string
	batchSize BatchSize
	waitTime  WaitTime
	createdAt time.Time
	active    bool
}

// NewSubscriber creates a new Subscriber instance
func NewSubscriber(name string, batchSize BatchSize, waitTime WaitTime) (*Subscriber, error) {
	// Validate subscriber name
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidSubscriberName
	}

	return &Subscriber{
		id:        uuid.New().String(),
		name:      name,
		batchSize: batchSize,
		waitTime:  waitTime,
		createdAt: time.Now(),
		active:    true,
	}, nil
}

// NewSubscriberWithDefaults creates a subscriber with default values
func NewSubscriberWithDefaults(name string) (*Subscriber, error) {
	return NewSubscriber(name, DefaultBatchSize, DefaultWaitTime)
}

// NewRandomSubscriber creates a subscriber with a generated UUID-based name
func NewRandomSubscriber() (*Subscriber, error) {
	uuidStr := strings.ReplaceAll(uuid.New().String(), "-", "_")
	subscriberName := SubscriberNamePrefix + strings.ToUpper(uuidStr)
	return NewSubscriberWithDefaults(subscriberName)
}

// ==========================================
// Getters (Read-Only Accessors)
// ==========================================

func (s *Subscriber) Name() string         { return s.name }
func (s *Subscriber) BatchSize() BatchSize { return s.batchSize }
func (s *Subscriber) WaitTime() WaitTime   { return s.waitTime }
func (s *Subscriber) CreatedAt() time.Time { return s.createdAt }
func (s *Subscriber) IsActive() bool       { return s.active }

// ==========================================
// Business Methods
// ==========================================

// CanProcess returns true if the subscriber can process messages
func (s *Subscriber) CanProcess() bool {
	return s.active && s.batchSize > 0
}

// Deactivate marks the subscriber as inactive
func (s *Subscriber) Deactivate() {
	s.active = false
}

// Reactivate marks the subscriber as active
func (s *Subscriber) Reactivate() {
	s.active = true
}

// String returns a string representation
func (s *Subscriber) String() string {
	return fmt.Sprintf("Subscriber{Name: %s, BatchSize: %d, WaitTime: %ds, Active: %v}",
		s.name, s.batchSize, s.waitTime, s.active)
}
