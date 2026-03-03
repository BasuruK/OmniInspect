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
// Subscriber Entity
// ==========================================

// Entity : Subscriber information
type Subscriber struct {
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

// ==========================================
// JSON Marshaling
// ==========================================

// subscriberJSON provides a JSON-friendly intermediate representation
type subscriberJSON struct {
	Name      string `json:"name"`
	BatchSize int    `json:"batch_size"`
	WaitTime  int    `json:"wait_time"`
	CreatedAt int64  `json:"created_at"`
	Active    bool   `json:"active"`
}

// MarshalJSON implements custom JSON marshaling for Subscriber
func (s *Subscriber) MarshalJSON() ([]byte, error) {
	j := subscriberJSON{
		Name:      s.name,
		BatchSize: int(s.batchSize),
		WaitTime:  int(s.waitTime),
		CreatedAt: s.createdAt.Unix(),
		Active:    s.active,
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements custom JSON unmarshaling for Subscriber
func (s *Subscriber) UnmarshalJSON(data []byte) error {
	var j subscriberJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return fmt.Errorf("failed to unmarshal Subscriber: %w", err)
	}

	// Use defaults for zero values
	batchSize := j.BatchSize
	if batchSize == 0 {
		batchSize = int(DefaultBatchSize)
	}
	bs, err := NewBatchSize(batchSize)
	if err != nil {
		return fmt.Errorf("invalid batch size: %w", err)
	}

	// Use defaults for zero values
	waitTime := j.WaitTime
	if waitTime == 0 {
		waitTime = int(DefaultWaitTime)
	}
	wt, err := NewWaitTime(waitTime)
	if err != nil {
		return fmt.Errorf("invalid wait time: %w", err)
	}

	// Use current time if created_at is 0
	createdAt := j.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}

	s.name = j.Name
	s.batchSize = bs
	s.waitTime = wt
	s.createdAt = time.Unix(createdAt, 0)
	s.active = j.Active

	return nil
}
