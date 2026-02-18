# Domain-Driven Design: From Anemic to Rich Domain

## Document Information

| Field | Value |
|-------|-------|
| **Document Version** | 1.0 |
| **Created** | February 19, 2026 |
| **Project** | OmniInspect (OmniView) |
| **Purpose** | Learn DDD concepts and refactor guide to transform from anemic to rich domain model |

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Anemic Domain Model vs Rich Domain Model](#2-anemic-domain-model-vs-rich-domain-model)
3. [Domain Entities with Behavior](#3-domain-entities-with-behavior)
4. [Domain Events](#4-domain-events)
5. [Aggregates and Aggregate Roots](#5-aggregates-and-aggregate-roots)
6. [Domain Services](#6-domain-services)
7. [Application Services vs Domain Services](#7-application-services-vs-domain-services)
8. [Bounded Contexts](#8-bounded-contexts)
9. [Refactoring Guide for OmniInspect](#9-refactoring-guide-for-omniinspect)
10. [Summary and Next Steps](#10-summary-and-next-steps)

---

## 1. Introduction

### What is Domain-Driven Design?

Domain-Driven Design (DDD) is a software development approach that emphasizes:
- **Modeling software based on the business domain**
- **Collaboration with domain experts**
- **Iterative refinement of the model**

### The Core Philosophy

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         DDD CORE PHILOSOPHY                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌──────────────┐         ┌──────────────┐         ┌──────────────┐      │
│   │   Domain     │         │    Model     │         │    Code      │      │
│   │   Expert    │◄───────►│   (Shared    │◄───────►│  (Reflects   │      │
│   │  Knowledge  │         │   Language)  │         │   Reality)   │      │
│   └──────────────┘         └──────────────┘         └──────────────┘      │
│                                                                             │
│   The code should speak the language of the business, not technical jargon  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Why DDD Matters

| Traditional Approach | DDD Approach |
|----------------------|---------------|
| Data-centric | Behavior-centric |
| Tables and records | Rich models |
| Anemic objects | Healthy, behavior-rich objects |
| Database-driven | Domain-driven |

---

## 2. Anemic Domain Model vs Rich Domain Model

### The Anemic Domain Model (Anti-Pattern)

An **anemic domain model** is a model where domain objects contain little or no business logic. They are essentially data containers (DTOs with getters/setters) while all business logic resides in services.

#### Characteristics

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ANEMIC DOMAIN MODEL STRUCTURE                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                         DOMAIN LAYER                           │        │
│   │  ┌─────────────────────┐  ┌─────────────────────┐             │        │
│   │  │   Subscriber        │  │   QueueMessage      │             │        │
│   │  │   ─────────────     │  │   ─────────────     │             │        │
│   │  │   Name: string      │  │   MessageID: string │             │        │
│   │  │   BatchSize: int    │  │   ProcessName: str  │             │        │
│   │  │   WaitTime: int     │  │   LogLevel: string  │             │        │
│   │  │                     │  │   Payload: string   │             │        │
│   │  │   + Getters()       │  │   Timestamp: str   │             │        │
│   │  │   + Setters()       │  │   + Getters()      │             │        │
│   │  └─────────────────────┘  └─────────────────────┘             │        │
│   │         DATA ONLY              DATA ONLY                        │        │
│   │         (Anemic)              (Anemic)                         │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                    │                                        │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                        SERVICE LAYER                             │        │
│   │  ┌─────────────────────────────────────────────────────────┐    │        │
│   │  │                  SubscriberService                      │    │        │
│   │  │  ─────────────────────────────────────────────────────  │    │        │
│   │  │  + NewSubscriber()       // Business logic here          │    │        │
│   │  │  + RegisterSubscriber() // Business logic here          │    │        │
│   │  │  + ValidateSubscriber()  // Business logic here          │    │        │
│   │  │  + GenerateUUID()       // Business logic here          │    │        │
│   │  └─────────────────────────────────────────────────────────┘    │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                                                             │
│   PROBLEM: Domain objects are passive; logic is scattered in services       │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Problems with Anemic Model

1. **Encapsulation is broken** - Data and behavior are separated
2. **Validation is inconsistent** - Each service might validate differently
3. **Harder to understand** - Business logic is hidden in services
4. **Duplication** - Similar logic repeated across services
5. **Testing is harder** - Hard to test business rules in isolation

### The Rich Domain Model (Goal)

A **rich domain model** places business logic, validation, and behavior directly within the domain objects themselves.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    RICH DOMAIN MODEL STRUCTURE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                         DOMAIN LAYER                           │        │
│   │  ┌─────────────────────────────────────────────────────────┐    │        │
│   │  │                    Subscriber                          │    │        │
│   │  │  ─────────────────────────────────────────────────────  │    │        │
│   │  │  - name: string                                        │    │        │
│   │  │  - batchSize: int                                      │    │        │
│   │  │  - waitTime: int                                      │    │        │
│   │  │  ─────────────────────────────────────────────────────  │    │        │
│   │  │  + NewSubscriber()           // Factory method         │    │        │
│   │  │  + Validate()                // Business rule          │    │        │
│   │  │  + CanProcess()              // Business rule          │    │        │
│   │  │  + UpdateBatchSize(size)     // Behavior               │    │        │
│   │  │  + IsValid() : bool          // Validation             │    │        │
│   │  └─────────────────────────────────────────────────────────┘    │        │
│   │                        (Rich, Behaviorful)                        │        │
│   │                                                                    │        │
│   │  ┌─────────────────────────────────────────────────────────┐    │        │
│   │  │                  QueueMessage                            │    │        │
│   │  │  ─────────────────────────────────────────────────────  │    │        │
│   │  │  - messageID: string                                    │    │        │
│   │  │  - processName: string                                  │    │        │
│   │  │  - logLevel: string                                     │    │        │
│   │  │  - payload: string                                      │    │        │
│   │  │  - timestamp: string                                    │    │        │
│   │  │  ─────────────────────────────────────────────────────  │    │        │
│   │  │  + IsCritical()           // Business rule             │    │        │
│   │  │  + GetFormattedMessage()  // Behavior                  │    │        │
│   │  │  + MatchesFilter(filter) // Business rule            │    │        │
│   │  └─────────────────────────────────────────────────────────┘    │        │
│   │                        (Rich, Behaviorful)                        │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                    │                                        │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                      APPLICATION LAYER                          │        │
│   │              (Orchestration, use case coordination)              │        │
│   │  ┌─────────────────────────────────────────────────────────┐    │        │
│   │  │              RegisterSubscriberUseCase                  │    │        │
│   │  │  ─────────────────────────────────────────────────────  │    │        │
│   │  │  + Execute()  // Coordinates, delegates to domain        │    │        │
│   │  └─────────────────────────────────────────────────────────┘    │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                                                             │
│   BENEFIT: Business logic lives with the data it operates on               │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Domain Entities with Behavior

### What Makes Something an Entity?

An **Entity** is a domain object with:
- **Unique identity** that persists over time
- **Mutable state**
- **Behavior** related to its identity

#### Entity vs Value Object

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         ENTITY VS VALUE OBJECT                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌──────────────────────────┐      ┌──────────────────────────┐           │
│   │        ENTITY            │      |      VALUE OBJECT        │           │
│   ├──────────────────────────┤      ├──────────────────────────┤           │
│   │ Has unique identity      │      │ No identity              │           │
│   │ (Subscriber by ID)       │      │ (Money, Address)         │           │
│   ├──────────────────────────┤      ├──────────────────────────┤           │
│   │ Mutable                  │      │ Immutable                │           │
│   │ (can change state)       │      │ (cannot change state)     │           │
│   ├──────────────────────────┤      ├──────────────────────────┤           │
│   │ Equality by ID           │      │ Equality by attributes   │           │
│   │ (two subs with same      │      | (two $10 are equal)      │           │
│   │  name are the same)      │      │                           │           │
│   ├──────────────────────────┤      ├──────────────────────────┤           │
│   │ Use factory methods      │      | Use constructors         │           │
│   │ for creation             │      | for creation             │           │
│   └──────────────────────────┘      └──────────────────────────┘           │
│                                                                             │
│   EXAMPLES:                         EXAMPLES:                               │
│   - Subscriber (has name/ID)        - QueueConfig (no identity)            │
│   - QueueMessage (has messageID)    - LogLevel (ERROR, INFO, DEBUG)         │
│   - User, Order, Account             - BatchSize, WaitTime (simple values)   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Converting Your Subscriber Entity

#### Before (Anemic)

```go
// internal/core/domain/subscriber.go

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
```

#### After (Rich)

```go
// internal/core/domain/subscriber.go

package domain

import (
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
)

// ==================== VALUE OBJECTS ====================

// BatchSize represents the batch size for processing
type BatchSize int

const (
    MinBatchSize BatchSize = 1
    MaxBatchSize BatchSize = 10000
    DefaultBatchSize BatchSize = 1000
)

// NewBatchSize creates a BatchSize with validation
func NewBatchSize(size int) (BatchSize, error) {
    if size < int(MinBatchSize) || size > int(MaxBatchSize) {
        return 0, fmt.Errorf("batch size must be between %d and %d", MinBatchSize, MaxBatchSize)
    }
    return BatchSize(size), nil
}

func (b BatchSize) Int() int { return int(b) }

// WaitTime represents wait time in seconds
type WaitTime int

const (
    MinWaitTime WaitTime = 1
    MaxWaitTime WaitTime = 300
    DefaultWaitTime WaitTime = 5
)

// NewWaitTime creates a WaitTime with validation
func NewWaitTime(seconds int) (WaitTime, error) {
    if seconds < int(MinWaitTime) || seconds > int(MaxWaitTime) {
        return 0, fmt.Errorf("wait time must be between %d and %d seconds", MinWaitTime, MaxWaitTime)
    }
    return WaitTime(seconds), nil
}

func (w WaitTime) Int() int { return int(w) }

// ==================== ENTITY ERRORS ====================

var (
    ErrSubscriberNotFound     = errors.New("subscriber not found")
    ErrInvalidSubscriberName  = errors.New("invalid subscriber name")
    ErrEmptySubscriber        = errors.New("subscriber cannot be empty")
)

// ==================== ENTITY ====================

// Subscriber represents a subscriber in the tracer queue system
// This is a RICH ENTITY with behavior encapsulated
type Subscriber struct {
    name      string      // Private - enforced encapsulation
    batchSize BatchSize
    waitTime  WaitTime
    createdAt time.Time
    active    bool
}

// Factory method - the ONLY way to create a valid Subscriber
func NewSubscriber(name string, batchSize BatchSize, waitTime WaitTime) (*Subscriber, error) {
    if strings.TrimSpace(name) == "" {
        return nil, ErrInvalidSubscriberName
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

// Factory for generating a new unique subscriber
func NewRandomSubscriber() (*Subscriber, error) {
    uuidWithHyphen := uuid.New()
    subscriberName := "SUB_" + strings.ToUpper(
        strings.ReplaceAll(uuidWithHyphen.String(), "-", "_"),
    )
    return NewSubscriber(subscriberName, DefaultBatchSize, DefaultWaitTime)
}

// ==================== BEHAVIOR (Getters with Encapsulation) ====================

// Name returns the subscriber name (read-only)
func (s *Subscriber) Name() string {
    return s.name
}

// BatchSize returns the batch size
func (s *Subscriber) BatchSize() BatchSize {
    return s.batchSize
}

// WaitTime returns the wait time
func (s *Subscriber) WaitTime() WaitTime {
    return s.waitTime
}

// CreatedAt returns when the subscriber was created
func (s *Subscriber) CreatedAt() time.Time {
    return s.createdAt
}

// IsActive returns whether the subscriber is active
func (s *Subscriber) IsActive() bool {
    return s.active
}

// ==================== BUSINESS METHODS ====================

// Validate checks if the subscriber is valid
func (s *Subscriber) Validate() error {
    if strings.TrimSpace(s.name) == "" {
        return ErrInvalidSubscriberName
    }
    if !s.active {
        return errors.New("subscriber is not active")
    }
    return nil
}

// CanProcess returns true if the subscriber can process messages
func (s *Subscriber) CanProcess() bool {
    return s.active && s.batchSize > 0
}

// UpdateBatchSize updates the batch size with validation
func (s *Subscriber) UpdateBatchSize(newSize BatchSize) error {
    if newSize < MinBatchSize || newSize > MaxBatchSize {
        return fmt.Errorf("batch size must be between %d and %d", MinBatchSize, MaxBatchSize)
    }
    s.batchSize = newSize
    return nil
}

// UpdateWaitTime updates the wait time with validation
func (s *Subscriber) UpdateWaitTime(newTime WaitTime) error {
    if newTime < MinWaitTime || newTime > MaxWaitTime {
        return fmt.Errorf("wait time must be between %d and %d seconds", MinWaitTime, MaxWaitTime)
    }
    s.waitTime = newTime
    return nil
}

// Deactivate marks the subscriber as inactive
func (s *Subscriber) Deactivate() {
    s.active = false
}

// Reactivate marks the subscriber as active
func (s *Subscriber) Reactivate() {
    s.active = true
}

// String returns string representation
func (s *Subscriber) String() string {
    return fmt.Sprintf("Subscriber{Name: %s, BatchSize: %d, WaitTime: %ds, Active: %v}",
        s.name, s.batchSize, s.waitTime, s.active)
}
```

### Benefits of This Approach

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    BENEFITS OF RICH ENTITIES                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   1. ENCAPSULATION                                                          │
│      ┌────────────────────────────────────────────────────────────────┐      │
│      │  - Fields are private (name, batchSize, waitTime)           │      │
│      │  - Access only through methods                               │      │
│      │  - Invariants are always maintained                          │      │
│      └────────────────────────────────────────────────────────────────┘      │
│                                                                             │
│   2. VALIDATION CENTRALIZED                                                 │
│      ┌────────────────────────────────────────────────────────────────┐      │
│      │  - BatchSize can never be invalid (enforced at creation)    │      │
│      │  - WaitTime can never be invalid (enforced at creation)     │      │
│      │  - No need to validate in multiple places                    │      │
│      └────────────────────────────────────────────────────────────────┘      │
│                                                                             │
│   3. TESTABILITY                                                            │
│      ┌────────────────────────────────────────────────────────────────┐      │
│      │  - Easy to test business rules in isolation                  │      │
│      │  - No need to mock services for entity tests                 │      │
│      │  - Test invariants directly                                  │      │
│      └────────────────────────────────────────────────────────────────┘      │
│                                                                             │
│   4. SELF-DOCUMENTING                                                       │
│      ┌────────────────────────────────────────────────────────────────┐      │
│      │  - Business rules are visible in the entity                  │      │
│      │  - Code reads like domain language                           │      │
│      │  - No need to search for logic in services                  │      │
│      └────────────────────────────────────────────────────────────────┘      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Domain Events

### What are Domain Events?

**Domain Events** represent something significant that happened in the domain. They are used to:
- Capture domain changes
- Enable loose coupling between components
- Support event-driven architectures
- Maintain audit trails

### Event Structure

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        DOMAIN EVENT STRUCTURE                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                     DomainEvent (Interface)                     │        │
│   ├─────────────────────────────────────────────────────────────────┤        │
│   │  + EventID() string        : Unique identifier for event      │        │
│   │  + EventType() string     : Type of event                     │        │
│   │  + OccurredAt() time.Time : When the event occurred           │        │
│   │  + AggregateID() string   : Which aggregate caused it         │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                    │                                        │
│                                    ▼                                        │
│   ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐         │
│   │ Subscriber       │  │ QueueMessage     │  │ TracerPackage   │         │
│   │ Registered       │  │ Received         │  │ Deployed         │         │
│   │                  │  │                  │  │                  │         │
│   │ - name           │  │ - messageID      │  │ - packageName    │         │
│   │ - batchSize      │  │ - processName    │  │ - version        │         │
│   │ - waitTime       │  │ - payload        │  │ - schema         │         │
│   └──────────────────┘  └──────────────────┘  └──────────────────┘         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Implementing Domain Events in Your Project

#### Step 1: Define the Event Interface

```go
// internal/core/domain/events.go

package domain

import (
    "time"

    "github.com/google/uuid"
)

// EventID generates a unique event ID
type EventID string

func NewEventID() EventID {
    return EventID(uuid.New().String())
}

// DomainEvent is the interface all domain events must implement
type DomainEvent interface {
    EventID() string
    EventType() string
    OccurredAt() time.Time
    AggregateID() string
}

// baseEvent provides common event functionality
type baseEvent struct {
    id          EventID
    eventType   string
    occurredAt  time.Time
    aggregateID string
}

func (e *baseEvent) EventID() string       { return string(e.id) }
func (e *baseEvent) EventType() string     { return e.eventType }
func (e *baseEvent) OccurredAt() time.Time { return e.occurredAt }
func (e *baseEvent) AggregateID() string   { return e.aggregateID }
```

#### Step 2: Define Concrete Events

```go
// internal/core/domain/subscriber_events.go

package domain

import "time"

// SubscriberRegistered occurs when a new subscriber is created
type SubscriberRegistered struct {
    baseEvent
    SubscriberName string
    BatchSize      int
    WaitTime       int
}

// NewSubscriberRegistered creates a new SubscriberRegistered event
func NewSubscriberRegistered(subscriber *Subscriber) *SubscriberRegistered {
    return &SubscriberRegistered{
        baseEvent: baseEvent{
            id:          NewEventID(),
            eventType:   "SubscriberRegistered",
            occurredAt:  time.Now(),
            aggregateID: subscriber.Name(),
        },
        SubscriberName: subscriber.Name(),
        BatchSize:      subscriber.BatchSize().Int(),
        WaitTime:       subscriber.WaitTime().Int(),
    }
}

// SubscriberBatchSizeChanged occurs when batch size is updated
type SubscriberBatchSizeChanged struct {
    baseEvent
    SubscriberName string
    OldBatchSize   int
    NewBatchSize   int
}

// NewSubscriberBatchSizeChanged creates a new event
func NewSubscriberBatchSizeChanged(subscriber *Subscriber, oldSize, newSize int) *SubscriberBatchSizeChanged {
    return &SubscriberBatchSizeChanged{
        baseEvent: baseEvent{
            id:          NewEventID(),
            eventType:   "SubscriberBatchSizeChanged",
            occurredAt:  time.Now(),
            aggregateID: subscriber.Name(),
        },
        SubscriberName: subscriber.Name(),
        OldBatchSize:   oldSize,
        NewBatchSize:   newSize,
    }
}
```

```go
// internal/core/domain/message_events.go

package domain

import "time"

// MessageReceived occurs when a tracer message is dequeued
type MessageReceived struct {
    baseEvent
    MessageID   string
    ProcessName string
    LogLevel    string
    Payload     string
}

// NewMessageReceived creates a new MessageReceived event
func NewMessageReceived(msg *QueueMessage) *MessageReceived {
    return &MessageReceived{
        baseEvent: baseEvent{
            id:          NewEventID(),
            eventType:   "MessageReceived",
            occurredAt:  time.Now(),
            aggregateID: msg.MessageID,
        },
        MessageID:   msg.MessageID,
        ProcessName: msg.ProcessName,
        LogLevel:    msg.LogLevel,
        Payload:     msg.Payload,
    }
}

// CriticalMessageReceived occurs when a critical log is received
type CriticalMessageReceived struct {
    baseEvent
    MessageID string
    ProcessName string
    Payload string
}

// NewCriticalMessageReceived creates a new event for critical messages
func NewCriticalMessageReceived(msg *QueueMessage) *CriticalMessageReceived {
    return &CriticalMessageReceived{
        baseEvent: baseEvent{
            id:          NewEventID(),
            eventType:   "CriticalMessageReceived",
            occurredAt:  time.Now(),
            aggregateID: msg.MessageID,
        },
        MessageID:   msg.MessageID,
        ProcessName: msg.ProcessName,
        Payload:     msg.Payload,
    }
}
```

#### Step 3: Event Dispatcher

```go
// internal/core/domain/event_dispatcher.go

package domain

// EventDispatcher manages event handlers
type EventDispatcher interface {
    Register(eventType string, handler EventHandler)
    Dispatch(event DomainEvent) error
}

// EventHandler handles domain events
type EventHandler func(event DomainEvent) error

// SimpleEventDispatcher is a basic implementation
type SimpleEventDispatcher struct {
    handlers map[string][]EventHandler
}

// NewSimpleEventDispatcher creates a new dispatcher
func NewSimpleEventDispatcher() *SimpleEventDispatcher {
    return &SimpleEventDispatcher{
        handlers: make(map[string][]EventHandler),
    }
}

// Register adds a handler for an event type
func (d *SimpleEventDispatcher) Register(eventType string, handler EventHandler) {
    d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Dispatch sends an event to all registered handlers
func (d *SimpleEventDispatcher) Dispatch(event DomainEvent) error {
    handlers, ok := d.handlers[event.EventType()]
    if !ok {
        return nil // No handlers registered
    }

    for _, handler := range handlers {
        if err := handler(event); err != nil {
            return err
        }
    }
    return nil
}
```

### Using Events in Your Services

```go
// internal/service/subscribers/subscriber_service.go

package subscribers

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
)

type SubscriberService struct {
    db          ports.DatabaseRepository
    bolt        ports.ConfigRepository
    dispatcher  domain.EventDispatcher // Add event dispatcher
}

// RegisterSubscriber with event publishing
func (ss *SubscriberService) RegisterSubscriber() (domain.Subscriber, error) {
    subscriber, err := ss.GetSubscriber()
    if err != nil {
        if !errors.Is(err, domain.ErrSubscriberNotFound) {
            return domain.Subscriber{}, err
        }
        // Create new subscriber
        newSub, err := domain.NewRandomSubscriber()
        if err != nil {
            return domain.Subscriber{}, err
        }
        subscriber = newSub
    }

    // Register in Oracle DB
    if err := ss.db.RegisterNewSubscriber(*subscriber); err != nil {
        return domain.Subscriber{}, err
    }

    // PUBLISH DOMAIN EVENT
    event := domain.NewSubscriberRegistered(subscriber)
    if err := ss.dispatcher.Dispatch(event); err != nil {
        // Log error but don't fail the operation
        // Event publishing failure shouldn't block the main flow
    }

    return *subscriber, nil
}
```

---

## 5. Aggregates and Aggregate Roots

### What is an Aggregate?

An **Aggregate** is a cluster of related domain objects that are treated as a single unit for data changes. The **Aggregate Root** is the main entity that controls access to the entire cluster.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          AGGREGATE PATTERN                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                    SUBSCRIBER AGGREGATE                         │        │
│   │  ┌─────────────────────────────────────────────────────────┐   │        │
│   │  │              Subscriber (AGGREGATE ROOT)                  │   │        │
│   │  │  - name: string                                          │   │        │
│   │  │  - batchSize: BatchSize                                  │   │        │
│   │  │  - waitTime: WaitTime                                    │   │        │
│   │  │  - status: SubscriberStatus                               │   │        │
│   │  │  ─────────────────────────────────────────────────────    │   │        │
│   │  │  + Register()                                            │   │        │
│   │  │  + ProcessMessages()                                      │   │        │
│   │  │  + UpdateConfiguration()                                  │   │        │
│   │  └─────────────────────────────────────────────────────────┘   │        │
│   │                            │                                      │        │
│   │                            │ Contains                              │        │
│   │                            ▼                                      │        │
│   │  ┌─────────────────────────────────────────────────────────┐   │        │
│   │  │              SubscriberState (Entity)                   │   │        │
│   │  │  - lastProcessedAt: time.Time                          │   │        │
│   │  │  - totalProcessed: int                                  │   │        │
│   │  │  - status: ProcessingStatus                             │   │        │
│   │  └─────────────────────────────────────────────────────────┘   │        │
│   │                            │                                      │        │
│   │                            │ Contains                              │        │
│   │                            ▼                                      │        │
│   │  ┌─────────────────────────────────────────────────────────┐   │        │
│   │  │           ProcessingSession (Value Object)              │   │        │
│   │  │  - sessionID: string                                    │   │        │
│   │  │  - startedAt: time.Time                                │   │        │
│   │  │  - messagesProcessed: int                               │   │        │
│   │  └─────────────────────────────────────────────────────────┘   │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                                                             │
│   KEY RULES:                                                                 │
│   1. Only the Aggregate Root can be referenced from outside               │
│   2. All changes to objects within the aggregate go through the root      │
│   3. The aggregate enforces its own invariants                             │
│   4. Database operations load/save the entire aggregate                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Implementing Aggregates

```go
// internal/core/domain/subscriber_aggregate.go

package domain

import (
    "errors"
    "time"
)

/ SubscriberStatus represents the status of a subscriber
type SubscriberStatus string

const (
    SubscriberStatusActive   SubscriberStatus = "ACTIVE"
    SubscriberStatusInactive SubscriberStatus = "INACTIVE"
    SubscriberStatusError   SubscriberStatus = "ERROR"
)

// ProcessingStatus represents the processing state
type ProcessingStatus string

const (
    ProcessingStatusIdle       ProcessingStatus = "IDLE"
    ProcessingStatusRunning   ProcessingStatus = "RUNNING"
    ProcessingStatusPaused    ProcessingStatus = "PAUSED"
    ProcessingStatusError     ProcessingStatus = "ERROR"
)

// ==================== VALUE OBJECTS ====================

// ProcessingSession represents a single processing session
type ProcessingSession struct {
    sessionID        string
    startedAt        time.Time
    messagesProcessed int
    lastMessageAt    time.Time
}

func NewProcessingSession() *ProcessingSession {
    return &ProcessingSession{
        sessionID: NewEventID().String(),
        startedAt: time.Now(),
    }
}

func (ps *ProcessingSession) SessionID() string { return ps.sessionID }
func (ps *ProcessingSession) StartedAt() time.Time { return ps.startedAt }
func (ps *ProcessingSession) MessagesProcessed() int { return ps.messagesProcessed }

func (ps *ProcessingSession) RecordMessage() {
    ps.messagesProcessed++
    ps.lastMessageAt = time.Now()
}

// ==================== SUBSCRIBER STATE ====================

// SubscriberState tracks runtime state of a subscriber
type SubscriberState struct {
    lastProcessedAt   time.Time
    totalProcessed   int
    processingStatus ProcessingStatus
    lastError        string
}

func NewSubscriberState() *SubscriberState {
    return &SubscriberState{
        processingStatus: ProcessingStatusIdle,
    }
}

func (ss *SubscriberState) LastProcessedAt() time.Time { return ss.lastProcessedAt }
func (ss *SubscriberState) TotalProcessed() int { return ss.totalProcessed }
func (ss *SubscriberState) Status() ProcessingStatus { return ss.processingStatus }
func (ss *SubscriberState) LastError() string { return ss.lastError }

func (ss *SubscriberState) MarkProcessing() {
    ss.processingStatus = ProcessingStatusRunning
}

func (ss *SubscriberState) MarkIdle() {
    ss.processingStatus = ProcessingStatusIdle
}

func (ss *SubscriberState) MarkError(err error) {
    ss.processingStatus = ProcessingStatusError
    ss.lastError = err.Error()
}

func (ss *SubscriberState) RecordProcessed(count int) {
    ss.totalProcessed += count
    ss.lastProcessedAt = time.Now()
}

// ==================== AGGREGATE ROOT ====================

// SubscriberAggregate is the aggregate root for subscriber operations
// It encapsulates all behavior and state related to a subscriber
type SubscriberAggregate struct {
    subscriber *Subscriber    // The main entity
    state      *SubscriberState
    session    *ProcessingSession
}

// NewSubscriberAggregate creates a new aggregate from scratch
func NewSubscriberAggregate(name string) (*SubscriberAggregate, error) {
    sub, err := NewSubscriberWithDefaults(name)
    if err != nil {
        return nil, err
    }

    return &SubscriberAggregate{
        subscriber: sub,
        state:      NewSubscriberState(),
        session:    nil,
    }, nil
}

// NewSubscriberAggregateFromEntity creates an aggregate from existing data
func NewSubscriberAggregateFromEntity(sub *Subscriber) *SubscriberAggregate {
    return &SubscriberAggregate{
        subscriber: sub,
        state:      NewSubscriberState(),
        session:    nil,
    }
}

// ==================== AGGREGATE ROOT METHODS ====================

// Entity access - all access goes through aggregate root

func (sa *SubscriberAggregate) Subscriber() *Subscriber {
    return sa.subscriber
}

func (sa *SubscriberAggregate) State() *SubscriberState {
    return sa.state
}

// Register performs subscriber registration
func (sa *SubscriberAggregate) Register() error {
    if err := sa.subscriber.Validate(); err != nil {
        return err
    }
    sa.state.MarkIdle()
    return nil
}

// StartProcessing begins a processing session
func (sa *SubscriberAggregate) StartProcessing() error {
    if !sa.subscriber.CanProcess() {
        return errors.New("subscriber cannot process")
    }

    if sa.session != nil {
        return errors.New("processing session already active")
    }

    sa.session = NewProcessingSession()
    sa.state.MarkProcessing()
    return nil
}

// RecordProcessed records processed messages
func (sa *SubscriberAggregate) RecordProcessed(count int) {
    if sa.session != nil {
        sa.session.RecordMessage()
    }
    sa.state.RecordProcessed(count)
}

// StopProcessing ends the current processing session
func (sa *SubscriberAggregate) StopProcessing() {
    if sa.session != nil {
        sa.session = nil
    }
    sa.state.MarkIdle()
}

// UpdateConfiguration safely updates configuration
func (sa *SubscriberAggregate) UpdateConfiguration(batchSize BatchSize, waitTime WaitTime) error {
    oldSize := sa.subscriber.BatchSize()

    if err := sa.subscriber.UpdateBatchSize(batchSize); err != nil {
        return err
    }

    if err := sa.subscriber.UpdateWaitTime(waitTime); err != nil {
        // Rollback
        sa.subscriber.UpdateBatchSize(oldSize)
        return err
    }

    return nil
}
```

---

## 6. Domain Services

### What are Domain Services?

**Domain Services** contain business logic that doesn't naturally fit within an Entity or Value Object. They're used when an operation conceptually belongs to the domain but doesn't have a natural home in a single entity.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      WHEN TO USE DOMAIN SERVICES                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                     DOMAIN SERVICE USAGE                        │        │
│   ├─────────────────────────────────────────────────────────────────┤        │
│   │                                                                  │        │
│   │  USE DOMAIN SERVICE WHEN:                                       │        │
│   │  ─────────────────────────────                                  │        │
│   │                                                                  │        │
│   │  1. Operation involves multiple aggregates                    │        │
│   │     Example: Transfer money between two accounts               │        │
│   │                                                                  │        │
│   │  2. Operation is stateless                                      │        │
│   │     Example: Calculate shipping cost based on order             │        │
│   │                                                                  │        │
│   │  3. Operation doesn't belong to any single entity              │        │
│   │     Example: Validate password strength                         │        │
│   │                                                                  │        │
│   │  4. Operation is a transformation                               │        │
│   │     Example: Convert currency                                   │        │
│   │                                                                  │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                                                             │
│   ⚠️  WARNING: Don't use Domain Services as a "catch-all"                  │
│       If behavior naturally belongs to an entity, put it there!            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Example: Subscriber Validation Service

```go
// internal/core/domain/subscriber_validation_service.go

package domain

import "fmt"

// SubscriberValidationService contains subscriber validation logic
type SubscriberValidationService struct{}

// NewSubscriberValidationService creates a new validation service
func NewSubscriberValidationService() *SubscriberValidationService {
    return &SubscriberValidationService{}
}

// ValidationResult contains the result of validation
type ValidationResult struct {
    Valid   bool
    Errors  []string
}

func (vr *ValidationResult) AddError(err string) {
    vr.Errors = append(vr.Errors, err)
}

func (vr *ValidationResult) IsValid() bool {
    return vr.Valid && len(vr.Errors) == 0
}

// ValidateName validates a subscriber name
func (vs *SubscriberValidationService) ValidateName(name string) error {
    if len(name) < 3 {
        return fmt.Errorf("name must be at least 3 characters")
    }
    if len(name) > 128 {
        return fmt.Errorf("name must be at most 128 characters")
    }
    // Oracle naming conventions
    if !isValidOracleIdentifier(name) {
        return fmt.Errorf("name must be a valid Oracle identifier")
    }
    return nil
}

// ValidateConfiguration validates batch size and wait time
func (vs *SubscriberValidationService) ValidateConfiguration(batchSize BatchSize, waitTime WaitTime) error {
    var errors []string

    if batchSize < MinBatchSize || batchSize > MaxBatchSize {
        errors = append(errors, fmt.Sprintf("batch size must be between %d and %d", MinBatchSize, MaxBatchSize))
    }

    if waitTime < MinWaitTime || waitTime > MaxWaitTime {
        errors = append(errors, fmt.Sprintf("wait time must be between %d and %d", MinWaitTime, MaxWaitTime))
    }

    if len(errors) > 0 {
        return fmt.Errorf("validation failed: %v", errors)
    }

    return nil
}

// isValidOracleIdentifier checks if a name is valid for Oracle
func isValidOracleIdentifier(name string) bool {
    if len(name) == 0 {
        return false
    }
    // Must start with letter
    first := name[0]
    if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
        return false
    }
    // Rest can be letters, numbers, underscore
    for _, c := range name[1:] {
        if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
            return false
        }
    }
    return true
}
```

---

## 7. Application Services vs Domain Services

### Understanding the Layers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    APPLICATION vs DOMAIN SERVICES                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                    APPLICATION LAYER                           │        │
│   │                                                                  │        │
│   │  Application Services (Use Cases)                              │        │
│   │  ───────────────────────────────────────                        │        │
│   │  • Coordinate workflow                                         │        │
│   │  • Handle transactions                                          │        │
│   │  • Delegate to domain objects                                   │        │
│   │  • Handle cross-cutting concerns                                │        │
│   │  • Transform DTOs                                              │        │
│   │                                                                  │        │
│   │  Examples:                                                      │        │
│   │  • RegisterSubscriberUseCase                                   │        │
│   │  • ProcessMessagesUseCase                                      │        │
│   │  • DeployTracerPackageUseCase                                  │        │
│   │                                                                  │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                    │                                        │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                      DOMAIN LAYER                               │        │
│   │                                                                  │        │
│   │  Domain Services                                                │        │
│   │  ─────────────────                                              │        │
│   │  • Pure business logic                                          │        │
│   │  • No infrastructure dependencies                               │        │
│   │  • Stateless                                                    │        │
│   │  • Operations that span multiple entities                      │        │
│   │                                                                  │        │
│   │  Domain Entities & Aggregates                                   │        │
│   │  ───────────────────────────────                                │        │
│   │  • Core business rules                                          │        │
│   │  • Encapsulated state                                           │        │
│   │  • Self-validating                                               │        │
│   │                                                                  │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                    │                                        │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                   INFRASTRUCTURE LAYER                         │        │
│   │                                                                  │        │
│   │  Adapters (Repository, External Services)                      │        │
│   │  ───────────────────────────────────────                        │        │
│   │  • Implement ports (interfaces)                                 │        │
│   │  • Database operations                                         │        │
│   │  • External API calls                                           │        │
│   │                                                                  │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Example: Refactoring Subscriber Service

#### Before (Current Structure - Mixed Concerns)

```go
// internal/service/subscribers/subscriber_service.go

package subscribers

type SubscriberService struct {
    db   ports.DatabaseRepository
    bolt ports.ConfigRepository
}

// Contains: business logic + persistence + registration
func (ss *SubscriberService) RegisterSubscriber() (domain.Subscriber, error) {
    // Business logic here (creating subscriber)
    // Database operations here (registering in Oracle)
    // Bolt operations here (storing in local DB)
    // ... mixed together!
}
```

#### After (Separated Concerns)

```go
// internal/core/domain/subscriber_factory.go

package domain

// This is DOMAIN - pure business logic
type SubscriberFactory struct {
    validationService *SubscriberValidationService
}

func NewSubscriberFactory() *SubscriberFactory {
    return &SubscriberFactory{
        validationService: NewSubscriberValidationService(),
    }
}

// CreateSubscriber creates a new subscriber (business logic only)
func (f *SubscriberFactory) CreateSubscriber(name string) (*Subscriber, error) {
    // Validate
    if err := f.validationService.ValidateName(name); err != nil {
        return nil, err
    }

    return NewSubscriberWithDefaults(name)
}

// CreateSubscriberWithConfig creates a subscriber with custom config
func (f *SubscriberFactory) CreateSubscriberWithConfig(
    name string,
    batchSize BatchSize,
    waitTime WaitTime,
) (*Subscriber, error) {
    // Validate
    if err := f.validationService.ValidateName(name); err != nil {
        return nil, err
    }
    if err := f.validationService.ValidateConfiguration(batchSize, waitTime); err != nil {
        return nil, err
    }

    return NewSubscriber(name, batchSize, waitTime)
}
```

```go
// internal/application/subscriber_commands.go

package application

// This is APPLICATION LAYER - orchestration
// Use case: Register Subscriber

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
)

type RegisterSubscriberCommand struct {
    Name      string
    BatchSize int
    WaitTime  int
}

type RegisterSubscriberHandler struct {
    db          ports.DatabaseRepository
    bolt        ports.ConfigRepository
    factory     *domain.SubscriberFactory
    dispatcher  domain.EventDispatcher
}

func NewRegisterSubscriberHandler(
    db ports.DatabaseRepository,
    bolt ports.ConfigRepository,
    factory *domain.SubscriberFactory,
    dispatcher domain.EventDispatcher,
) *RegisterSubscriberHandler {
    return &RegisterSubscriberHandler{
        db:         db,
        bolt:       bolt,
        factory:    factory,
        dispatcher: dispatcher,
    the use case
func (h * }
}

// Handle executesRegisterSubscriberHandler) Handle(cmd RegisterSubscriberCommand) (*domain.Subscriber, error) {
    // 1. Create domain object (delegate to factory)
    var subscriber *domain.Subscriber
    var err error

    if cmd.BatchSize > 0 && cmd.WaitTime > 0 {
        subscriber, err = h.factory.CreateSubscriberWithConfig(
            cmd.Name,
            domain.BatchSize(cmd.BatchSize),
            domain.WaitTime(cmd.WaitTime),
        )
    } else {
        subscriber, err = h.factory.CreateSubscriber(cmd.Name)
    }
    if err != nil {
        return nil, err
    }

    // 2. Persist to local storage
    if err := h.bolt.SetSubscriber(*subscriber); err != nil {
        return nil, err
    }

    // 3. Register in Oracle
    if err := h.db.RegisterNewSubscriber(*subscriber); err != nil {
        return nil, err
    }

    // 4. Publish domain event
    event := domain.NewSubscriberRegistered(subscriber)
    if err := h.dispatcher.Dispatch(event); err != nil {
        // Log but don't fail - event failure shouldn't block
    }

    return subscriber, nil
}
```

---

## 8. Bounded Contexts

### What is a Bounded Context?

A **Bounded Context** is a logical boundary within which a particular domain model exists. Each context has its own:
- Ubiquitous Language (terminology)
- Domain Model
- Team Ownership

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    BOUNDED CONTEXTS IN OMNIINSPECT                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐        │
│   │                   OMNIINSPECT SYSTEM                             │        │
│   │                                                                  │        │
│   │  ┌──────────────────────────┐  ┌──────────────────────────┐     │        │
│   │  │   SUBSCRIBER CONTEXT    │  │   TRACER CONTEXT        │     │        │
│   │  │                          │  │                          │     │        │
│   │  │  Entities:              │  │  Entities:               │     │        │
│   │  │  - Subscriber          │  │  - QueueMessage          │     │        │
│   │  │  - SubscriberState      │  │  - TracerPackage        │     │        │
│   │  │                          │  │                          │     │        │
│   │  │  Services:              │  │  Services:               │     │        │
│   │  │  - SubscriberService    │  │  - TracerService         │     │        │
│   │  │  - ValidationService    │  │  - DeploymentService     │     │        │
│   │  │                          │  │                          │     │        │
│   │  │  Language:              │  │  Language:               │     │        │
│   │  │  - subscriber          │  │  - queue message         │     │        │
│   │  │  - batch_size           │  │  - dequeue               │     │        │
│   │  │  - wait_time            │  │  - log_level             │     │        │
│   │  │                          │  │                          │     │        │
│   │  └──────────────────────────┘  └──────────────────────────┘     │        │
│   │                  │                        │                     │        │
│   │                  │        CONTEXT        │                     │        │
│   │                  │        MAPPING        │                     │        │
│   │                  ▼                        ▼                     │        │
│   │  ┌─────────────────────────────────────────────────────────┐  │        │
│   │  │              CONFIGURATION CONTEXT                       │  │        │
│   │  │                                                          │  │        │
│   │  │  Entities:              Services:                         │  │        │
│   │  │  - DatabaseConfig       - SettingsService               │  │        │
│   │  │  - AppSettings          - ConfigLoader                  │  │        │
│   │  │                                                          │  │        │
│   │  └─────────────────────────────────────────────────────────┘  │        │
│   │                                                                  │        │
│   └─────────────────────────────────────────────────────────────────┘        │
│                                                                             │
│   Context Map:                                                               │
│   ┌─────────────────┐     uses      ┌─────────────────┐                     │
│   │ Subscriber     │◄──────────────►│    Tracer       │                     │
│   │ Context        │                │    Context      │                     │
│   └─────────────────┘                └─────────────────┘                     │
│          │                                    │                             │
│          │          conforms to              │                             │
│          ▼                                    ▼                             │
│   ┌─────────────────────────────────────────────────────┐                  │
│   │            Configuration Context                     │                  │
│   └─────────────────────────────────────────────────────┘                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Implementing Bounded Contexts in Go

```
internal/
├── core/
│   ├── domain/
│   │   ├── subscriber/        # Subscriber Bounded Context
│   │   │   ├── subscriber.go
│   │   │   ├── subscriber_events.go
│   │   │   ├── subscriber_aggregate.go
│   │   │   └── validation.go
│   │   │
│   │   ├── tracer/           # Tracer Bounded Context
│   │   │   ├── queue_message.go
│   │   │   ├── tracer_package.go
│   │   │   └── events.go
│   │   │
│   │   └── config/           # Configuration Bounded Context
│   │       ├── database_settings.go
│   │       └── app_settings.go
│   │
│   └── ports/
│       ├── subscriber/        # Subscriber Ports
│       │   └── repository.go
│       ├── tracer/           # Tracer Ports
│       │   └── repository.go
│       └── config/           # Configuration Ports
│           └── repository.go
│
├── adapter/
│   ├── oracle/               # Implements tracer ports
│   ├── boltdb/               # Implements config ports
│   └── ...
│
├── application/
│   ├── subscriber/          # Subscriber Use Cases
│   │   ├── commands.go
│   │   └── queries.go
│   └── tracer/              # Tracer Use Cases
│       ├── commands.go
│       └── queries.go
│
└── service/                 # (Legacy - to be refactored)
```

---

## 9. Refactoring Guide for OmniInspect

### Step-by-Step Refactoring Plan

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    REFACTORING ROADMAP                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   PHASE 1: Enrich Domain Entities (Week 1)                                  │
│   ─────────────────────────────────────                                     │
│   □ Add validation to Subscriber entity                                    │
│   □ Add value objects for BatchSize, WaitTime                               │
│   □ Add business methods (CanProcess, UpdateConfiguration)                 │
│   □ Add validation to QueueMessage                                         │
│   □ Add business methods (IsCritical, GetFormattedMessage)                 │
│                                                                             │
│   PHASE 2: Add Domain Events (Week 2)                                      │
│   ──────────────────────────────                                           │
│   □ Define event interfaces                                                │
│   □ Create SubscriberRegistered event                                      │
│   □ Create MessageReceived event                                           │
│   □ Create EventDispatcher                                                 │
│   □ Integrate events into services                                         │
│                                                                             │
│   PHASE 3: Implement Aggregates (Week 3)                                   │
│   ─────────────────────────────────                                         │
│   □ Create SubscriberAggregate                                             │
│   □ Move business logic from service to aggregate                          │
│   □ Define aggregate boundaries                                            │
│                                                                             │
│   PHASE 4: Separate Application Layer (Week 4)                            │
│   ───────────────────────────────────────                                   │
│   □ Create use case handlers                                                │
│   □ Move orchestration logic from services                                 │
│   □ Keep services as thin domain service wrappers                          │
│                                                                             │
│   PHASE 5: Bounded Contexts (Optional, Week 5)                             │
│   ───────────────────────────────────────                                   │
│   □ Organize by context                                                     │
│   □ Define context boundaries                                              │
│   □ Create context maps                                                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Current vs Target Structure

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    CURRENT STRUCTURE                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   internal/                                                                 │
│   ├── core/                                                                  │
│   │   ├── domain/                        ◄── Anemic entities               │
│   │   │   ├── config.go                                                   │
│   │   │   ├── permissions.go                                              │
│   │   │   ├── queue.go                                                   │
│   │   │   └── subscriber.go                                              │
│   │   │                                                                   │
│   │   └── ports/                        ◄── Defined correctly              │
│   │       ├── config.go                                                   │
│   │       └── repository.go                                               │
│   │                                                                           │
│   ├── service/                           ◄── Too much logic here            │
│   │   ├── permissions/                                                     │
│   │   │   └── permissions_service.go                                       │
│   │   ├── subscribers/                                                     │
│   │   │   └── subscriber_service.go                                       │
│   │   └── tracer/                                                          │
│   │       └── tracer_service.go                                            │
│   │                                                                           │
│   └── app/                                ◄── Very thin                    │
│       └── app.go                                                           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

                              ▼ TRANSFORM ▼

┌─────────────────────────────────────────────────────────────────────────────┐
│                    TARGET STRUCTURE                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   internal/                                                                 │
│   ├── core/                                                                  │
│   │   ├── domain/                        ◄── Rich domain                   │
│   │   │   ├── events.go                  ◄── Domain events base           │
│   │   │   ├── dispatcher.go                                               │
│   │   │   │                                                                   │
│   │   │   ├── subscriber/                ◄── Subscriber Context           │
│   │   │   │   ├── subscriber.go           ◄── Rich entity                   │
│   │   │   │   ├── subscriber_events.go                                    │
│   │   │   │   ├── subscriber_aggregate.go                                 │
│   │   │   │   └── validation.go                                          │
│   │   │   │                                                                   │
│   │   │   ├── tracer/                     ◄── Tracer Context               │
│   │   │   │   ├── queue_message.go        ◄── Rich entity                 │
│   │   │   │   ├── queue_config.go         ◄── Value object                │
│   │   │   │   ├── tracer_events.go                                         │
│   │   │   │   └── tracer_aggregate.go                                    │
│   │   │   │                                                                   │
│   │   │   └── config/                   ◄── Config Context                │
│   │   │       ├── database_settings.go                                    │
│   │   │       └── app_settings.go                                         │
│   │   │                                                                   │
│   │   └── ports/                        ◄── Unchanged                      │
│   │       ├── config.go                                                   │
│   │       └── repository.go                                               │
│   │                                                                           │
│   ├── application/                     ◄── NEW: Use cases                    │
│   │   ├── subscriber/                                                      │
│   │   │   ├── commands.go                                                 │
│   │   │   └── queries.go                                                  │
│   │   └── tracer/                                                          │
│   │       ├── commands.go                                                  │
│   │       └── queries.go                                                  │
│   │                                                                           │
│   ├── adapter/                         ◄── Unchanged                         │
│   │   ├── config/                                                         │
│   │   └── storage/                                                       │
│   │                                                                           │
│   └── service/                         ◄── Simplified                       │
│       ├── permissions/                  ◄── Domain service only             │
│       ├── subscribers/                                                     │
│       └── tracer/                                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Summary and Next Steps

### Key Takeaways

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         KEY TAKEAWAYS                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   1. RICH ENTITIES vs ANEMIC ENTITIES                                      │
│      • Move behavior to domain objects                                      │
│      • Use private fields with getter/setter methods                        │
│      • Validate at creation time                                            │
│                                                                             │
│   2. DOMAIN EVENTS                                                          │
│      • Capture significant occurrences                                      │
│      • Enable loose coupling                                                │
│      • Support audit trails and reaction                                    │
│                                                                             │
│   3. AGGREGATES                                                             │
│      • Group related entities under a root                                  │
│      • Root controls all access                                             │
│      • Enforce invariants                                                   │
│                                                                             │
│   4. DOMAIN vs APPLICATION SERVICES                                         │
│      • Domain: pure business logic, stateless                               │
│      • Application: orchestration, transactions, use cases                 │
│                                                                             │
│   5. BOUNDED CONTEXTS                                                       │
│      • Define clear boundaries                                              │
│      • Each context has its own language                                    │
│      • Map relationships between contexts                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Recommended Learning Path

1. **Start with Phase 1** - Enrich your entities first
2. **Read "Domain-Driven Design" by Eric Evans** - The definitive book
3. **Study "Implementing Domain-Driven Design" by Vaughn Vernon** - Practical implementation
4. **Look at real-world DDD projects** - Study the codebases

### Additional Resources

| Resource | Description |
|----------|-------------|
| [DDD Community](https://domainlanguage.com/ddd/) | Official DDD resources |
| [Martin Fowler's DDD Articles](https://martinfowler.com/bliki/DomainDrivenDesign.html) | Excellent explanations |
| [DDD by Example](https://github.com/ddd-by-examples/library) | Full example project |

---

## Appendix: Quick Reference Cards

### Entity Checklist

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ENTITY CHECKLIST                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   □ Private fields (no public fields)                                      │
│   □ Factory method for creation                                             │
│   □ Validation in constructor                                               │
│   □ Business methods (not just getters/setters)                             │
│   □ Domain errors defined                                                   │
│   □ Documented with domain language                                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Service Checklist

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SERVICE CHECKLIST                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   DOMAIN SERVICE:                                                           │
│   □ No infrastructure dependencies                                         │
│   □ Stateless                                                              │
│   □ Pure business logic                                                     │
│   □ Operation spans multiple entities                                      │
│                                                                             │
│   APPLICATION SERVICE:                                                      │
│   □ Orchestrates use cases                                                 │
│   □ Manages transactions                                                   │
│   □ Handles DTO conversion                                                  │
│   □ Delegates to domain objects                                             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

*Document Version: 1.0*
*Last Updated: February 19, 2026*
*Author: Claude (DDD Evaluation)*
