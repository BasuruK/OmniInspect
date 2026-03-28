# Agent Guidelines for OmniView/OmniInspect

## Project Overview

OmniView is a Message Passing TUI application that connects to Oracle Database and displays real-time trace messages via Oracle Advanced Queuing (AQ). Built with Go, Bubble Tea v2, and ODPI-C for Oracle connectivity.

## Build Commands

```bash
# Build the application (REQUIRED - not 'go run')
make build                    # Build with default version
make build VERSION=v1.0.0     # Build with specific version

# Build ODPI-C library only
make odpi

# Run the application
make run                      # Build and run

# Run tests
make test                     # All tests
go test -v ./...              # Equivalent

# Run a single test
go test -v ./internal/core/domain    # Test a specific package
go test -v -run TestSubscriber ./... # Test matching pattern

# Lint and format
make lint                     # go vet
make fmt                      # go fmt

# Clean
make clean                    # Remove build artifacts

# Install dependencies
make install                  # go mod download && go mod tidy
```

### Important Build Notes

- **Always use `make build` or `make run`** - never `go run cmd/omniview/main.go`
- The Makefile sets required CGO environment variables and Oracle client linker paths
- On macOS ARM64, Oracle Instant Client is at `/opt/oracle/instantclient_23_7`
- On Windows, Oracle Instant Client is at `C:\oracle_inst\instantclient_23_7`

## Architecture

Hexagonal (Ports and Adapters) architecture:

```
cmd/omniview/main.go (Composition Root)
        │
        ▼
internal/service/ (Business Logic)
        │
        ▼
internal/core/ports/ (Interfaces)
        │
        ▼
internal/adapter/ (Implementations: storage/oracle, storage/boltdb, ui, config)
```

- **Composition root**: `cmd/omniview/main.go` - wire dependencies here, not in adapters
- **Domain**: `internal/core/domain` - entities, value objects, sentinel errors
- **Ports**: `internal/core/ports` - repository interfaces
- **Services**: `internal/service` - business logic coordination
- **Adapters**: `internal/adapter/storage/oracle`, `internal/adapter/storage/boltdb`, `internal/adapter/ui`

## Code Style Guidelines

### Naming Conventions

- **Constructor functions**: `New...` (e.g., `NewSubscriber`, `NewTracerService`)
- **Interfaces**: `...er` suffix (e.g., `SubscriberRepository`, `DatabaseRepository`)
- **Package names**: lowercase, single word or short phrase (e.g., `tracer`, `permissions`)
- **Go files**: lowercase, underscore allowed (e.g., `tracer_service.go`)
- **Types**: PascalCase (e.g., `BatchSize`, `WaitTime`)
- **Constants**: PascalCase for typed, UPPER_SNAKE for untyped

### Imports

```go
import (
    "OmniView/internal/core/domain"      // Project imports
    "OmniView/internal/core/ports"
    "context"
    "fmt"

    "charm.land/bubbletea/v2"         // Bubble Tea v2 (charm.land, NOT github.com/charmbracelet)
    "charm.land/lipgloss/v2"          // Lipgloss styling
    "charm.land/bubbles/v2/spinner"   // Bubbles components
)
```

### Section Divider Comments

Use this style for major code sections:

```go
// ==========================================
// Subscriber Entity
// ==========================================

// Or for subsections:

// ─────────────────────────
// Getters (Read-Only Accessors)
// ─────────────────────────
```

### Error Handling

- **Sentinel errors**: Define in `internal/core/domain/errors.go`
- **Wrap errors with context**: `fmt.Errorf("operation: %w", err)`
- **Domain validation errors**: Use domain sentinel errors
- **Do not introduce ad hoc error strings**

```go
// Good
var ErrSubscriberNotFound = errors.New("subscriber not found")

func (s *SubscriberRepository) GetByName(ctx context.Context, name string) (*Subscriber, error) {
    if name == "" {
        return nil, ErrInvalidSubscriberName
    }
    // ...
    return nil, fmt.Errorf("GetByName: %w", ErrSubscriberNotFound)
}
```

### Constructor Patterns

Constructors return pointers and handle validation:

```go
func NewSubscriber(name string, batchSize BatchSize, waitTime WaitTime) (*Subscriber, error) {
    if strings.TrimSpace(name) == "" {
        return nil, ErrInvalidSubscriberName
    }
    // ...
    return &Subscriber{...}, nil
}

func NewTracerService(
    db ports.DatabaseRepository,
    bolt ports.ConfigRepository,
    eventChannel chan *domain.QueueMessage,
) *TracerService {
    return &TracerService{...}
}
```

### Dependency Injection

- Inject dependencies through constructor arguments
- Use option structs for optional dependencies

```go
type ModelOpts struct {
    App               *app.App
    BoltAdapter       *boltdb.BoltAdapter
    DBAdapter         *oracle.OracleAdapter    // Optional
    PermissionService *permissions.PermissionService  // Optional
    TracerService     *tracer.TracerService    // Optional
    AppConfig         *domain.DatabaseSettings // Optional
    EventChannel      chan *domain.QueueMessage
}

func NewModel(opts ModelOpts) (*Model, error) {
    if opts.App == nil {
        return nil, fmt.Errorf("missing required dependency: App")
    }
    // ...
}
```

### Pointer Receivers

Use pointer receivers for services and adapters:

```go
func (ts *TracerService) StartEventListener(...) error { ... }
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
```

### Value Objects

Immutable types with validation constructors:

```go
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
```

### Entity Pattern

Entities encapsulate state and expose it through methods:

```go
type Subscriber struct {
    name      string
    batchSize BatchSize
    waitTime  WaitTime
    createdAt time.Time
    active    bool
}

// Read-only accessors
func (s *Subscriber) Name() string         { return s.name }
func (s *Subscriber) BatchSize() BatchSize { return s.batchSize }
func (s *Subscriber) IsActive() bool       { return s.active }
```

### Bubble Tea v2 Patterns

- **Model**: `internal/adapter/ui/model.go`
- **Update**: `internal/adapter/ui/messages.go` - message handlers
- **View**: `internal/adapter/ui/main_screen.go`, `welcome.go`, etc.
- **Styles**: `internal/adapter/ui/styles/styles.go`

```go
import (
    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/viewport"
    "charm.land/lipgloss/v2"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
func (m *Model) View() tea.View { ... }
func (m *Model) Init() tea.Cmd { ... }
```

### CGO / Oracle Code

When changing Oracle dequeueing or CGO code, keep Go and C sides aligned across:
- `internal/adapter/storage/oracle/oracle_adapter.go`
- `internal/adapter/storage/oracle/dequeue_ops.c`
- `internal/adapter/storage/oracle/dequeue_ops.h`

## File Organization

```
internal/
├── adapter/
│   ├── config/          # Settings loader
│   ├── storage/
│   │   ├── boltdb/      # BoltDB implementations
│   │   └── oracle/     # Oracle/ODPI-C implementations
│   └── ui/              # Bubble Tea TUI
├── app/                 # Application entry point
├── core/
│   ├── domain/          # Entities, value objects, errors
│   └── ports/           # Repository interfaces
├── service/            # Business logic services
└── updater/            # Self-update functionality
```

## Key Files for Reference

- `cmd/omniview/main.go` - Composition root
- `internal/core/domain/subscriber.go` - Entity pattern
- `internal/core/domain/errors.go` - Sentinel errors
- `internal/core/ports/repository.go` - Interface definitions
- `internal/adapter/ui/model.go` - Bubble Tea model

## Local Storage

- BoltDB database: `omniview.bolt` (created on first run)
- Stores: database connection settings, subscriber config, permissions
- Delete `omniview.bolt` and restart to switch databases

## Testing

- Run all tests: `make test`
- Run package tests: `go test -v ./internal/core/domain`
- Run with coverage: `go test -v -cover ./...`
