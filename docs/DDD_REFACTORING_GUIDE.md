# Domain-Driven Design: Practical Refactoring Guide for OmniInspect

## Document Information

| Field | Value |
|-------|-------|
| **Document Version** | 4.0 |
| **Updated** | February 23, 2026 |
| **Project** | OmniInspect |
| **Primary Goal** | Refactor the codebase to follow DDD |
| **Secondary Goal** | Learn DDD concepts along the way |

---

## Table of Contents

1. [Why Refactor?](#1-why-refactor)
2. [The Current State](#2-the-current-state)
3. [Subscriber Entity - Complete Refactoring](#3-subscriber-entity---complete-refactoring)
4. [QueueMessage Entity - Complete Refactoring](#4-queuemessage-entity---complete-refactoring)
5. [DatabaseSettings Entity - Complete Refactoring](#5-databasesettings-entity---complete-refactoring)
6. [DatabasePermissions Entity - Complete Refactoring](#6-databasepermissions-entity---complete-refactoring)
7. [Remaining Entities](#7-remaining-entities)
8. [Repository Pattern - Implementation](#8-repository-pattern---implementation)
9. [Service Refactoring - Complete](#9-service-refactoring---complete)
10. [Error Handling](#10-error-handling)
11. [Summary of Changes](#11-summary-of-changes)

---

## 1. Why Refactor?

### The Problem with Current Code

The current codebase has **anemic domain models** - entities that are just data structures with no behavior:

```go
// CURRENT - What's in your project now
type Subscriber struct {
    Name      string
    BatchSize int
    WaitTime  int
}
```

**Problems:**
1. **No validation** - Anyone can create invalid subscribers
2. **Business logic scattered** - Validation lives in services
3. **Duplication** - Same validation in multiple places
4. **Hard to test** - Can't test business rules in isolation

### The Goal

A **rich domain model** where:
- Entities validate themselves
- Business logic lives in the domain
- Services become thin orchestrators
- Code is easier to test and understand

---

## 2. The Current State

### Files to Modify

| File | Current State | Action |
|------|---------------|--------|
| `internal/core/domain/subscriber.go` | Anemic | Refactor to rich entity |
| `internal/core/domain/queue_message.go` | Anemic | Refactor to rich entity |
| `internal/core/domain/database_settings.go` | Anemic | Refactor to rich entity |
| `internal/core/domain/permissions.go` | Anemic | Refactor to rich entity |
| `internal/core/domain/config.go` | Simple | Add behavior if needed |
| `internal/core/ports/repository.go` | Interfaces | Add repository interfaces |
| `internal/service/subscribers/subscriber_service.go` | Thick | Simplify |
| `internal/service/tracer/tracer_service.go` | Thick | Simplify |
| `internal/service/permissions/permissions_service.go` | Thick | Simplify |

### What We're NOT Changing

- The Oracle adapter (works fine)
- The BoltDB adapter (works fine)
- The main entry point
- The updater

---

## 3. Subscriber Entity - Complete Refactoring

### What This Section Covers

Everything needed to refactor `subscriber.go` - self-contained:

1. Value objects (BatchSize, WaitTime) - defined right here
2. Errors specific to Subscriber
3. The complete Subscriber entity
4. Repository interface for Subscriber

### Step 3.1: Add Value Objects

Add these to the TOP of `internal/core/domain/subscriber.go`:

```go
package domain

import (
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
)

// ==================== VALUE OBJECTS ====================

// BatchSize represents how many messages to process at once
type BatchSize int

const (
    MinBatchSize     BatchSize = 1
    MaxBatchSize     BatchSize = 10000
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

// WaitTime represents how long to wait between dequeues (in seconds)
type WaitTime int

const (
    MinWaitTime     WaitTime = 1
    MaxWaitTime     WaitTime = 300
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
```

### Step 3.2: Add Errors

Add these after the value objects in the same file:

```go
// ==================== ERRORS ====================

var (
    ErrSubscriberNotFound    = errors.New("subscriber not found")
    ErrInvalidSubscriberName = errors.New("invalid subscriber name")
)
```

### Step 3.3: Add the Subscriber Entity

Replace the entire Subscriber struct with this (append to same file):

```go
// ==================== ENTITY ====================

// Subscriber represents a subscriber to the tracer queue
// This is a RICH ENTITY - it validates itself and contains business logic
type Subscriber struct {
    name      string      // private - must use getters
    batchSize BatchSize  // validated at creation
    waitTime  WaitTime   // validated at creation
    createdAt time.Time
    active    bool
}

// NewSubscriber creates a new Subscriber with validation
// This is the FACTORY - the ONLY way to create a valid Subscriber
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

// NewSubscriberWithDefaults creates a Subscriber with default values
func NewSubscriberWithDefaults(name string) (*Subscriber, error) {
    return NewSubscriber(name, DefaultBatchSize, DefaultWaitTime)
}

// NewRandomSubscriber creates a Subscriber with a generated UUID name
func NewRandomSubscriber() (*Subscriber, error) {
    uuidStr := strings.ReplaceAll(uuid.New().String(), "-", "_")
    subscriberName := "SUB_" + strings.ToUpper(uuidStr)
    return NewSubscriberWithDefaults(subscriberName)
}

// ==================== GETTERS ====================

func (s *Subscriber) Name() string       { return s.name }
func (s *Subscriber) BatchSize() BatchSize { return s.batchSize }
func (s *Subscriber) WaitTime() WaitTime  { return s.waitTime }
func (s *Subscriber) CreatedAt() time.Time { return s.createdAt }
func (s *Subscriber) IsActive() bool      { return s.active }

// ==================== BUSINESS METHODS ====================

// CanProcess returns true if this subscriber can process messages
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

// String returns a readable representation
func (s *Subscriber) String() string {
    return fmt.Sprintf("Subscriber{Name: %s, BatchSize: %d, WaitTime: %ds, Active: %v}",
        s.name, s.batchSize, s.waitTime, s.active)
}
```

### Step 3.4: Add Repository Interface

Add this to `internal/core/ports/repository.go`:

```go
// ==================== SUBSCRIBER REPOSITORY ====================

// SubscriberRepository defines how to persist and retrieve Subscribers
type SubscriberRepository interface {
    // Save stores a subscriber
    Save(ctx context.Context, subscriber domain.Subscriber) error

    // GetByName retrieves a subscriber by name
    GetByName(ctx context.Context, name string) (*domain.Subscriber, error)

    // Exists checks if a subscriber exists
    Exists(ctx context.Context, name string) (bool, error)

    // Delete removes a subscriber
    Delete(ctx context.Context, name string) error
}
```

### Summary of Changes for Subscriber

| Change | Purpose |
|--------|---------|
| Added BatchSize value object | Type-safe batch size with validation |
| Added WaitTime value object | Type-safe wait time with validation |
| Made fields private | Enforce encapsulation |
| Added NewSubscriber factory | Only way to create valid subscribers |
| Added CanProcess method | Business logic lives in entity |
| Added repository interface | Define persistence contract |

---

## 4. QueueMessage Entity - Complete Refactoring

### What This Section Covers

Everything for `queue_message.go` - self-contained:

1. LogLevel value object
2. QueueMessage entity
3. Repository interface

### Step 4.1: Add to queue_message.go

Replace the entire file with:

```go
package domain

import (
    "errors"
    "fmt"
    "strings"
    "time"
)

// ==================== ERRORS ====================

var (
    ErrInvalidMessageID = errors.New("invalid message ID")
    ErrInvalidPayload  = errors.New("payload cannot be empty")
)

// ==================== VALUE OBJECT ====================

// LogLevel represents the severity of a log message
type LogLevel string

const (
    LogLevelDebug    LogLevel = "DEBUG"
    LogLevelInfo     LogLevel = "INFO"
    LogLevelWarning  LogLevel = "WARNING"
    LogLevelError    LogLevel = "ERROR"
    LogLevelCritical LogLevel = "CRITICAL"
)

// NewLogLevel creates a LogLevel with validation
func NewLogLevel(level string) (LogLevel, error) {
    normalized := strings.ToUpper(strings.TrimSpace(level))
    switch LogLevel(normalized) {
    case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError, LogLevelCritical:
        return LogLevel(normalized), nil
    default:
        return "", fmt.Errorf("invalid log level: %s", level)
    }
}

func (l LogLevel) String() string { return string(l) }

// IsError returns true if this is ERROR or CRITICAL
func (l LogLevel) IsError() bool {
    return l == LogLevelError || l == LogLevelCritical
}

// ==================== ENTITY ====================

// QueueMessage represents a message from the tracer queue
type QueueMessage struct {
    messageID   string
    processName string
    logLevel    LogLevel
    payload     string
    timestamp   time.Time
}

// NewQueueMessage creates a new QueueMessage
func NewQueueMessage(
    messageID string,
    processName string,
    logLevel LogLevel,
    payload string,
    timestamp time.Time,
) (*QueueMessage, error) {
    if strings.TrimSpace(messageID) == "" {
        return nil, ErrInvalidMessageID
    }
    if strings.TrimSpace(payload) == "" {
        return nil, ErrInvalidPayload
    }

    return &QueueMessage{
        messageID:   messageID,
        processName: processName,
        logLevel:    logLevel,
        payload:     payload,
        timestamp:   timestamp,
    }, nil
}

// ==================== GETTERS ====================

func (m *QueueMessage) MessageID() string    { return m.messageID }
func (m *QueueMessage) ProcessName() string  { return m.processName }
func (m *QueueMessage) LogLevel() LogLevel  { return m.logLevel }
func (m *QueueMessage) Payload() string      { return m.payload }
func (m *QueueMessage) Timestamp() time.Time { return m.timestamp }

// ==================== BUSINESS METHODS ====================

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
```

### Summary of Changes for QueueMessage

| Change | Purpose |
|--------|---------|
| Added LogLevel value object | Type-safe log levels with validation |
| Made fields private | Enforce encapsulation |
| Added IsCritical method | Business logic in entity |
| Added Format method | Replaces formatting code in TracerService |

---

## 5. DatabaseSettings Entity - Complete Refactoring

### What This Section Covers

Everything for `database_settings.go` - self-contained:

1. Port value object
2. DatabaseSettings entity
3. Repository interface

### Step 5.1: Create database_settings.go

Create or replace `internal/core/domain/database_settings.go`:

```go
package domain

import (
    "errors"
    "fmt"
    "strings"
)

// ==================== ERRORS ====================

var (
    ErrEmptyDatabase = errors.New("database name cannot be empty")
    ErrEmptyHost     = errors.New("host cannot be empty")
    ErrInvalidPort   = errors.New("invalid port number")
    ErrEmptyUsername = errors.New("username cannot be empty")
)

// ==================== VALUE OBJECT ====================

// Port represents a database port number
type Port int

const (
    MinPort          Port = 1
    MaxPort          Port = 65535
    DefaultOraclePort Port = 1521
)

func NewPort(p int) (Port, error) {
    if p < int(MinPort) || p > int(MaxPort) {
        return 0, fmt.Errorf("port must be between %d and %d", MinPort, MaxPort)
    }
    return Port(p), nil
}

func (p Port) Int() int { return int(p) }

// ==================== ENTITY ====================

// DatabaseSettings represents Oracle database connection settings
type DatabaseSettings struct {
    database  string
    host      string
    port      Port
    username  string
    password  string
    isDefault bool
}

// NewDatabaseSettings creates new database settings with validation
func NewDatabaseSettings(
    database string,
    host string,
    port Port,
    username string,
    password string,
) (*DatabaseSettings, error) {
    if strings.TrimSpace(database) == "" {
        return nil, ErrEmptyDatabase
    }
    if strings.TrimSpace(host) == "" {
        return nil, ErrEmptyHost
    }
    if port == 0 {
        return nil, ErrInvalidPort
    }
    if strings.TrimSpace(username) == "" {
        return nil, ErrEmptyUsername
    }

    return &DatabaseSettings{
        database:  database,
        host:      host,
        port:      port,
        username:  username,
        password:  password,
        isDefault: false,
    }, nil
}

// ==================== GETTERS ====================

func (s *DatabaseSettings) Database() string { return s.database }
func (s *DatabaseSettings) Host() string     { return s.host }
func (s *DatabaseSettings) Port() Port       { return s.port }
func (s *DatabaseSettings) Username() string { return s.username }
func (s *DatabaseSettings) Password() string { return s.password }
func (s *DatabaseSettings) IsDefault() bool  { return s.isDefault }

// ==================== BUSINESS METHODS ====================

// GetConnectionString returns the Oracle Easy Connect string
func (s *DatabaseSettings) GetConnectionString() string {
    return fmt.Sprintf("%s:%d/%s", s.host, s.port, s.database)
}

// GetConnectionDetails returns a safe string for logging (without password)
func (s *DatabaseSettings) GetConnectionDetails() string {
    return fmt.Sprintf("%s@%s:%d/%s", s.username, s.host, s.port, s.database)
}

// SetAsDefault marks this as the default database
func (s *DatabaseSettings) SetAsDefault() {
    s.isDefault = true
}
```

### Step 5.2: Add Repository Interface

Add to `internal/core/ports/repository.go`:

```go
// ==================== DATABASE SETTINGS REPOSITORY ====================

// DatabaseSettingsRepository defines how to persist and retrieve DatabaseSettings
type DatabaseSettingsRepository interface {
    Save(ctx context.Context, settings domain.DatabaseSettings) error
    GetByID(ctx context.Context, id string) (*domain.DatabaseSettings, error)
    GetDefault(ctx context.Context) (*domain.DatabaseSettings, error)
    Delete(ctx context.Context, id string) error
}
```

### Summary of Changes for DatabaseSettings

| Change | Purpose |
|--------|---------|
| Added Port value object | Type-safe port with validation |
| Added validation in constructor | Fail fast on invalid data |
| Added GetConnectionString | Encapsulates connection string building |
| Added GetConnectionDetails | Safe logging without password |

---

## 6. DatabasePermissions Entity - Complete Refactoring

### What This Section Covers

Everything for `permissions.go` - self-contained:

1. PermissionStatus value object
2. DatabasePermissions entity

### Step 6.1: Create permissions.go

Create or replace `internal/core/domain/permissions.go`:

```go
package domain

import (
    "errors"
    "fmt"
)

// ==================== ERRORS ====================

var (
    ErrPermissionDenied   = errors.New("permission denied")
    ErrMissingPermissions = errors.New("missing required permissions")
)

// ==================== VALUE OBJECT ====================

// PermissionStatus holds the status of all required permissions
type PermissionStatus struct {
    CreateSequence       bool
    CreateProcedure     bool
    CreateType          bool
    AQAdministratorRole bool
    AQUserRole          bool
    DBMSAQADMExecute    bool
    DBMSAQExecute       bool
}

// HasAllPermissions returns true if all permissions are granted
func (ps *PermissionStatus) HasAllPermissions() bool {
    return ps.CreateSequence &&
        ps.CreateProcedure &&
        ps.CreateType &&
        ps.AQAdministratorRole &&
        ps.AQUserRole &&
        ps.DBMSAQADMExecute &&
        ps.DBMSAQExecute
}

// ==================== ENTITY ====================

// DatabasePermissions represents the result of a permission check
type DatabasePermissions struct {
    schema      string
    permissions PermissionStatus
}

// NewDatabasePermissions creates new database permissions
func NewDatabasePermissions(schema string, status PermissionStatus) *DatabasePermissions {
    return &DatabasePermissions{
        schema:      schema,
        permissions: status,
    }
}

// ==================== GETTERS ====================

func (p *DatabasePermissions) Schema() string            { return p.schema }
func (p *DatabasePermissions) Permissions() PermissionStatus { return p.permissions }

// ==================== BUSINESS METHODS ====================

// IsValid returns true if all permissions are granted
func (p *DatabasePermissions) IsValid() bool {
    return p.permissions.HasAllPermissions()
}

// Validate returns an error if permissions are insufficient
func (p *DatabasePermissions) Validate() error {
    if !p.IsValid() {
        return ErrMissingPermissions
    }
    return nil
}
```

### Summary of Changes for DatabasePermissions

| Change | Purpose |
|--------|---------|
| Added PermissionStatus value object | Groups all permission flags |
| Added IsValid method | Single method to check all permissions |
| Added Validate method | Returns error if not valid |

---

## 7. Remaining Entities

### These entities are simple and don't need much refactoring:

### 7.1 ClientSettings (config.go)

Current code is fine, but add defaults:

```go
// ClientSettings represents client-specific settings
type ClientSettings struct {
    enableUtf8 bool
}

func NewClientSettings(enableUtf8 bool) *ClientSettings {
    return &ClientSettings{enableUtf8: enableUtf8}
}

func DefaultClientSettings() *ClientSettings {
    return &ClientSettings{enableUtf8: true}
}

func (c *ClientSettings) EnableUtf8() bool { return c.enableUtf8 }
```

### 7.2 RunCycleStatus (config.go)

```go
// RunCycleStatus represents the status of the application's run cycle
type RunCycleStatus struct {
    isFirstRun bool
}

func NewRunCycleStatus(isFirstRun bool) *RunCycleStatus {
    return &RunCycleStatus{isFirstRun: isFirstRun}
}

func (r *RunCycleStatus) IsFirstRun() bool { return r.isFirstRun }
```

### 7.3 QueueConfig (queue.go)

This is already a simple value object, no changes needed:

```go
const (
    QueueName      = "OMNI_TRACER_QUEUE"
    QueueTableName = "AQ$OMNI_TRACER_QUEUE"
)
```

---

## 8. Repository Pattern - Implementation

### Step 8.1: Update ports/repository.go

Replace the entire file with:

```go
package ports

import (
    "context"

    "OmniView/internal/core/domain"
)

// ==================== SUBSCRIBER REPOSITORY ====================

type SubscriberRepository interface {
    Save(ctx context.Context, subscriber domain.Subscriber) error
    GetByName(ctx context.Context, name string) (*domain.Subscriber, error)
    Exists(ctx context.Context, name string) (bool, error)
    Delete(ctx context.Context, name string) error
}

// ==================== DATABASE SETTINGS REPOSITORY ====================

type DatabaseSettingsRepository interface {
    Save(ctx context.Context, settings domain.DatabaseSettings) error
    GetByID(ctx context.Context, id string) (*domain.DatabaseSettings, error)
    GetDefault(ctx context.Context) (*domain.DatabaseSettings, error)
    Delete(ctx context.Context, id string) error
}

// ==================== PERMISSIONS REPOSITORY ====================

type PermissionsRepository interface {
    Save(ctx context.Context, perms *domain.DatabasePermissions) error
    Get(ctx context.Context, schema string) (*domain.DatabasePermissions, error)
    Exists(ctx context.Context, schema string) (bool, error)
}
```

### Step 8.2: Update BoltDB Adapter

The existing BoltDB adapter already implements these. You just need to wrap it:

```go
// internal/adapter/storage/boltdb/subscriber_repository.go

package boltdb

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
    "context"
    "encoding/json"
)

type SubscriberRepository struct {
    adapter *BoltAdapter
    bucket  string
}

func NewSubscriberRepository(adapter *BoltAdapter) *SubscriberRepository {
    return &SubscriberRepository{
        adapter: adapter,
        bucket:  "subscribers",
    }
}

func (r *SubscriberRepository) Save(ctx context.Context, subscriber domain.Subscriber) error {
    data, err := json.Marshal(subscriber)
    if err != nil {
        return err
    }
    return r.adapter.db.Update(func(tx *bolt.Tx) error {
        return tx.Bucket([]byte(r.bucket)).Put([]byte(subscriber.Name()), data)
    })
}

func (r *SubscriberRepository) GetByName(ctx context.Context, name string) (*domain.Subscriber, error) {
    var subscriber domain.Subscriber
    err := r.adapter.db.View(func(tx *bolt.Tx) error {
        data := tx.Bucket([]byte(r.bucket)).Get([]byte(name))
        if data == nil {
            return domain.ErrSubscriberNotFound
        }
        return json.Unmarshal(data, &subscriber)
    })
    if err != nil {
        return nil, err
    }
    return &subscriber, nil
}

func (r *SubscriberRepository) Exists(ctx context.Context, name string) (bool, error) {
    var exists bool
    err := r.adapter.db.View(func(tx *bolt.Tx) error {
        exists = tx.Bucket([]byte(r.bucket)).Get([]byte(name)) != nil
        return nil
    })
    return exists, err
}

func (r *SubscriberRepository) Delete(ctx context.Context, name string) error {
    return r.adapter.db.Update(func(tx *bolt.Tx) error {
        return tx.Bucket([]byte(r.bucket)).Delete([]byte(name))
    })
}
```

---

## 9. Service Refactoring - Complete

### 9.1 SubscriberService

**Before (current):**

```go
// internal/service/subscribers/subscriber_service.go

package subscribers

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
    "errors"
    "strings"

    "github.com/google/uuid"
)

type SubscriberService struct {
    db   ports.DatabaseRepository
    bolt ports.ConfigRepository
}

func NewSubscriberService(db ports.DatabaseRepository, bolt ports.ConfigRepository) *SubscriberService {
    return &SubscriberService{db: db, bolt: bolt}
}

// PROBLEM: Business logic mixed with persistence
func (ss *SubscriberService) RegisterSubscriber() (domain.Subscriber, error) {
    subscriber, err := ss.bolt.GetSubscriber()
    if err != nil {
        if !errors.Is(err, domain.ErrSubscriberNotFound) {
            return domain.Subscriber{}, err
        }
        // Create new - but validation is hidden here
        uuidWithHyphen := uuid.New()
        subscriber = domain.Subscriber{
            Name:      "SUB_" + strings.ToUpper(strings.ReplaceAll(uuidWithHyphen.String(), "-", "_")),
            BatchSize: 1000,  // Magic number!
            WaitTime:  5,      // Magic number!
        }
    }

    // Register in Oracle
    if err := ss.db.RegisterNewSubscriber(subscriber); err != nil {
        return domain.Subscriber{}, err
    }

    return subscriber, nil
}
```

**After (refactored):**

```go
// internal/service/subscribers/subscriber_service.go

package subscribers

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
    "context"
    "errors"
)

type SubscriberService struct {
    subscriberRepo ports.SubscriberRepository
    oracleDB      ports.DatabaseRepository
}

func NewSubscriberService(
    subRepo ports.SubscriberRepository,
    oracleDB ports.DatabaseRepository,
) *SubscriberService {
    return &SubscriberService{
        subscriberRepo: subRepo,
        oracleDB:      oracleDB,
    }
}

// Now thin - just orchestrates, doesn't contain business logic
func (ss *SubscriberService) RegisterSubscriber(ctx context.Context) (*domain.Subscriber, error) {
    // Get existing or create new
    existing, err := ss.subscriberRepo.GetByName(ctx, "")
    if err != nil && !errors.Is(err, domain.ErrSubscriberNotFound) {
        return nil, err
    }
    if existing != nil {
        return existing, nil
    }

    // Use domain factory - validation happens there
    subscriber, err := domain.NewRandomSubscriber()
    if err != nil {
        return nil, err
    }

    // Save locally
    if err := ss.subscriberRepo.Save(ctx, *subscriber); err != nil {
        return nil, err
    }

    // Register in Oracle
    if err := ss.oracleDB.RegisterNewSubscriber(*subscriber); err != nil {
        // Rollback local on Oracle failure
        ss.subscriberRepo.Delete(ctx, subscriber.Name())
        return nil, err
    }

    return subscriber, nil
}

// GetSubscriber is now a simple delegation
func (ss *SubscriberService) GetSubscriber(ctx context.Context) (*domain.Subscriber, error) {
    return ss.subscriberRepo.GetByName(ctx, "")
}
```

### 9.2 TracerService

**Before (current):**

```go
// internal/service/tracer/tracer_service.go

// Has mixed responsibilities:
// - Package deployment
// - Event listening
// - Message formatting
// - All in one file
```

**After (refactored):**

```go
// internal/service/tracer/tracer_service.go

package tracer

import (
    "OmniView/internal/core/domain"
    "context"
    "fmt"
    "sync"
)

type TracerService struct {
    subscriberRepo ports.SubscriberRepository
    oracleDB       ports.DatabaseRepository
    processMu      sync.Mutex
}

func NewTracerService(
    subRepo ports.SubscriberRepository,
    oracleDB ports.DatabaseRepository,
) *TracerService {
    return &TracerService{
        subscriberRepo: subRepo,
        oracleDB:      oracleDB,
    }
}

// StartEventListener starts listening for messages
func (ts *TracerService) StartEventListener(ctx context.Context, subscriberName string) error {
    subscriber, err := ts.subscriberRepo.GetByName(ctx, subscriberName)
    if err != nil {
        return err
    }

    fmt.Println("[Tracer] Starting event listener for subscriber:", subscriber.Name())

    // Initial processing
    go func() {
        ts.processBatch(ctx, *subscriber)
    }()

    // Blocking wait loop
    go ts.blockingConsumerLoop(ctx, *subscriber)

    return nil
}

// Process a batch of messages
func (ts *TracerService) processBatch(ctx context.Context, subscriber domain.Subscriber) error {
    ts.processMu.Lock()
    defer ts.processMu.Unlock()

    messages, msgIDs, count, err := ts.oracleDB.BulkDequeueTracerMessages(subscriber)
    if err != nil {
        return err
    }

    if count == 0 {
        return nil
    }

    for i := 0; i < count; i++ {
        var msg domain.QueueMessage
        // Parse message (you may need to adjust this based on actual format)
        // The key change: using domain.QueueMessage's methods

        ts.handleTracerMessage(msg)
    }

    return nil
}

// Handle a single message - now uses domain formatting
func (ts *TracerService) handleTracerMessage(msg domain.QueueMessage) {
    // Using the domain's Format method instead of inline formatting
    fmt.Println(msg.Format())
}

func (ts *TracerService) blockingConsumerLoop(ctx context.Context, subscriber domain.Subscriber) {
    for {
        select {
        case <-ctx.Done():
            fmt.Println("Event Listener stopping for subscriber:", subscriber.Name())
            return
        default:
            err := ts.processBatch(ctx, subscriber)
            if err != nil {
                fmt.Printf("Error processing batch: %v\n", err)
            }
        }
    }
}
```

### 9.3 PermissionService

**Before (current):**
- Deploys package
- Checks permissions
- Saves status
- Drops package
- All in one service

**After (refactored):**

```go
// internal/service/permissions/permissions_service.go

package permissions

import (
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
    "context"
)

type PermissionService struct {
    permissionsRepo ports.PermissionsRepository
    db              ports.DatabaseRepository
}

func NewPermissionService(
    permsRepo ports.PermissionsRepository,
    db ports.DatabaseRepository,
) *PermissionService {
    return &PermissionService{
        permissionsRepo: permsRepo,
        db:              db,
    }
}

// CheckPermissions verifies database permissions
// Simplified - still has package deployment logic but cleaner
func (ps *PermissionService) CheckPermissions(ctx context.Context, schema string) (*domain.DatabasePermissions, error) {
    // Check if already verified
    exists, err := ps.permissionsRepo.Exists(ctx, schema)
    if err != nil {
        return nil, err
    }
    if exists {
        return ps.permissionsRepo.Get(ctx, schema)
    }

    // Note: Package deployment logic would stay here
    // (or be moved to a separate deployment service)

    // Run permission check
    // ... (existing logic)

    // Save result using domain entity
    // ... (existing logic)

    return nil, nil // Placeholder
}
```

---

## 10. Error Handling

### Add to internal/core/domain/errors.go

```go
package domain

import "fmt"

// Sentinel errors - predefined, checkable with errors.Is()
var (
    ErrSubscriberNotFound     = fmt.Errorf("subscriber not found")
    ErrInvalidSubscriberName = fmt.Errorf("invalid subscriber name")
    ErrQueueEmpty            = fmt.Errorf("queue is empty")
    ErrPermissionDenied      = fmt.Errorf("permission denied")
    ErrConnectionFailed      = fmt.Errorf("connection failed")
)

// WrapError adds context to an error
func WrapError(err error, context string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", context, err)
}
```

---

## 11. Summary of Changes

### Files Modified

| File | Changes |
|------|---------|
| `internal/core/domain/subscriber.go` | Added BatchSize, WaitTime value objects, made entity rich |
| `internal/core/domain/queue_message.go` | Added LogLevel value object, rich entity |
| `internal/core/domain/database_settings.go` | NEW - Added Port value object, rich entity |
| `internal/core/domain/permissions.go` | Added PermissionStatus, rich entity |
| `internal/core/domain/config.go` | Added ClientSettings defaults |
| `internal/core/ports/repository.go` | Added repository interfaces |
| `internal/adapter/storage/boltdb/subscriber_repository.go` | NEW - Implements SubscriberRepository |
| `internal/service/subscribers/subscriber_service.go` | Simplified |
| `internal/service/tracer/tracer_service.go` | Simplified |
| `internal/service/permissions/permissions_service.go` | Simplified |

### Key Benefits

1. **Self-validating entities** - Invalid data can't be created
2. **Business logic in domain** - Services are thin
3. **Type-safe value objects** - BatchSize, WaitTime, etc.
4. **Testable** - Can test business rules without database
5. **Clear interfaces** - Repository pattern defines contracts

---

*Document Version: 4.0*
*Last Updated: February 23, 2026*
