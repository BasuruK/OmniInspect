package domain

import (
	"encoding/json"
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
		return 0, fmt.Errorf("%w: must be between %d and %d", ErrInvalidBatchSize, MinBatchSize, MaxBatchSize)
	}
	return BatchSize(size), nil
}

func (b BatchSize) Int() int { return int(b) }

// WaitTime represents wait time in seconds
type WaitTime int

func NewWaitTime(seconds int) (WaitTime, error) {
	if seconds < int(MinWaitTime) || seconds > int(MaxWaitTime) {
		return 0, fmt.Errorf("%w: must be between %d and %d seconds", ErrInvalidWaitTime, MinWaitTime, MaxWaitTime)
	}
	return WaitTime(seconds), nil
}

func (w WaitTime) Int() int { return int(w) }

// ==========================================
// Subscriber Entity
// ==========================================

// Entity : Subscriber information
type Subscriber struct {
	name      string
	funnyName string
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
	// Validate batch size
	if batchSize < MinBatchSize || batchSize > MaxBatchSize {
		return nil, ErrInvalidBatchSize
	}
	// Validate wait time
	if waitTime < MinWaitTime || waitTime > MaxWaitTime {
		return nil, ErrInvalidWaitTime
	}

	return &Subscriber{
		name:      name,
		funnyName: "",
		batchSize: batchSize,
		waitTime:  waitTime,
		createdAt: time.Now(),
		active:    true,
	}, nil
}

// NewSubscriberWithFunnyName creates a subscriber with a funny name assigned
func NewSubscriberWithFunnyName(name string, funnyName string, batchSize BatchSize, waitTime WaitTime) (*Subscriber, error) {
	subscriber, err := NewSubscriber(name, batchSize, waitTime)
	if err != nil {
		return nil, err
	}
	if err := subscriber.AssignFunnyName(funnyName); err != nil {
		return nil, err
	}
	return subscriber, nil
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

func (s *Subscriber) Name() string      { return s.name }
func (s *Subscriber) FunnyName() string { return s.funnyName }

// ConsumerName returns the Oracle AQ consumer name used for registration and dequeue.
func (s *Subscriber) ConsumerName() string {
	if s.funnyName != "" {
		return s.funnyName
	}
	return s.name
}

func (s *Subscriber) BatchSize() BatchSize { return s.batchSize }
func (s *Subscriber) WaitTime() WaitTime   { return s.waitTime }
func (s *Subscriber) CreatedAt() time.Time { return s.createdAt }
func (s *Subscriber) IsActive() bool       { return s.active }

// ==========================================
// Business Methods
// ==========================================

// AssignFunnyName validates and stores the subscriber's procedure alias.
func (s *Subscriber) AssignFunnyName(funnyName string) error {
	if s == nil {
		return ErrNilSubscriber
	}
	validated, err := NewFunnyName(strings.ToUpper(funnyName))
	if err != nil {
		return err
	}
	s.funnyName = validated.Name()
	return nil
}

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

// ==========================================
// JSON Marshaling
// ==========================================

// subscriberJSON provides a JSON-friendly intermediate representation
type subscriberJSON struct {
	Name      string `json:"name"`
	FunnyName string `json:"funny_name,omitempty"`
	BatchSize int    `json:"batch_size"`
	WaitTime  int    `json:"wait_time"`
	CreatedAt int64  `json:"created_at"`
	Active    *bool  `json:"active"`
}

// MarshalJSON implements custom JSON marshaling for Subscriber
func (s *Subscriber) MarshalJSON() ([]byte, error) {
	subscriberObj := subscriberJSON{
		Name:      s.name,
		FunnyName: s.funnyName,
		BatchSize: int(s.batchSize),
		WaitTime:  int(s.waitTime),
		CreatedAt: s.createdAt.Unix(),
		Active:    &s.active,
	}
	return json.Marshal(subscriberObj)
}

// UnmarshalJSON implements custom JSON unmarshaling for Subscriber
func (s *Subscriber) UnmarshalJSON(data []byte) error {
	var subscriberObj subscriberJSON
	if err := json.Unmarshal(data, &subscriberObj); err != nil {
		return fmt.Errorf("failed to unmarshal Subscriber: %w", err)
	}
	// Validate subscriber name
	name := strings.TrimSpace(subscriberObj.Name)
	if name == "" {
		return ErrInvalidSubscriberName
	}

	// Use defaults for zero values
	batchSize := subscriberObj.BatchSize
	if batchSize == 0 {
		batchSize = int(DefaultBatchSize)
	}
	bs, err := NewBatchSize(batchSize)
	if err != nil {
		return fmt.Errorf("invalid batch size: %w", err)
	}

	// Use defaults for zero values
	waitTime := subscriberObj.WaitTime
	if waitTime == 0 {
		waitTime = int(DefaultWaitTime)
	}
	wt, err := NewWaitTime(waitTime)
	if err != nil {
		return fmt.Errorf("invalid wait time: %w", err)
	}

	sub, err := NewSubscriber(name, bs, wt)
	// If createdAt is provided, override the default value
	if err != nil {
		return err
	}
	if subscriberObj.FunnyName != "" {
		if err := sub.AssignFunnyName(subscriberObj.FunnyName); err != nil {
			return fmt.Errorf("invalid funny name: %w", err)
		}
	}
	if subscriberObj.CreatedAt != 0 {
		sub.createdAt = time.Unix(subscriberObj.CreatedAt, 0)
	}
	if subscriberObj.Active != nil {
		sub.active = *subscriberObj.Active
	}
	*s = *sub

	return nil
}
