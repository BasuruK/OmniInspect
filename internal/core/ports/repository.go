package ports

import (
	"OmniView/internal/core/domain"
	"context"
)

// ==========================================
// Subscriber Repository Interface
// ==========================================

type SubscriberRepository interface {
	// Save stores a subscriber
	Save(ctx context.Context, subscriber domain.Subscriber) error

	// GetByName retrieves a subscriber by name
	GetByName(ctx context.Context, name string) (*domain.Subscriber, error)

	// List returns all subscribers
	List(ctx context.Context) ([]domain.Subscriber, error)

	// Exists checks if a subscriber exists
	Exists(ctx context.Context, name string) (bool, error)

	// Delete removes a subscriber
	Delete(ctx context.Context, name string) error
}

// ==========================================
// Database Settings Repository Interface
// ==========================================

type DatabaseSettingsRepository interface {
	// Save stores database settings
	Save(ctx context.Context, settings domain.DatabaseSettings) error

	// SaveAndSelectIfNone persists settings. If no usable default exists, it also marks them as the default in the same transaction. becameDefault is true iff this call installed the default pointer.
	SaveAndSelectIfNone(ctx context.Context, settings domain.DatabaseSettings) (becameDefault bool, err error)

	// CreateAndSelectIfNone persists settings only when the storage key is absent, in one transaction. If no usable default exists, it also marks them as the default. Duplicate keys return a wrapped domain.ErrKeyCollision. becameDefault is true iff this call installed the default pointer.
	CreateAndSelectIfNone(ctx context.Context, settings domain.DatabaseSettings) (becameDefault bool, err error)

	// GetByID retrieves database settings by ID. When no record exists, the
	// returned error satisfies errors.Is(err, domain.ErrDatabaseSettingsNotFound).
	GetByID(ctx context.Context, id string) (*domain.DatabaseSettings, error)

	// GetDefault retrieves the default database settings
	GetDefault(ctx context.Context) (*domain.DatabaseSettings, error)

	// SetDefault marks settings as the default database, persisting it (creating the record if it doesn't already exist) and clearing the previous default (if different) in one call. This is the single entry point for changing "which database is current". On a nil error the returned pointer is always non-nil.
	SetDefault(ctx context.Context, settings domain.DatabaseSettings) (*domain.DatabaseSettings, error)

	// GetAll retrieves all stored database settings
	GetAll(ctx context.Context) ([]domain.DatabaseSettings, error)

	// Delete removes database settings by ID
	Delete(ctx context.Context, id string) error

	// Replace atomically removes the record stored under id and writes newRecord in a single transaction. When id equals newRecord.StorageKey() the call is equivalent to Save. Use this when renaming a database ID to avoid a window where neither key exists.
	Replace(ctx context.Context, id string, newRecord domain.DatabaseSettings) error
}

// ==========================================
// Permissions Repository Interface
// ==========================================

type PermissionsRepository interface {
	// Save stores database permissions for a schema
	Save(ctx context.Context, perms *domain.DatabasePermissions) error

	// Get retrieves database permissions for a schema
	Get(ctx context.Context, schema string) (*domain.DatabasePermissions, error)

	// Exists checks if permissions exist for a schema
	Exists(ctx context.Context, schema string) (bool, error)
}

// ==========================================
// Database Repository Interface (Oracle)
// ==========================================

type DatabaseRepository interface {
	// RegisterNewSubscriber registers a new subscriber in the database
	RegisterNewSubscriber(ctx context.Context, subscriber domain.Subscriber) error

	// UnregisterSubscriber removes a subscriber from Oracle AQ. Returns nil if the subscriber does not exist (idempotent).
	UnregisterSubscriber(ctx context.Context, subscriber domain.Subscriber) error

	// BulkDequeueTracerMessages dequeues multiple messages for a subscriber
	BulkDequeueTracerMessages(ctx context.Context, subscriber domain.Subscriber) ([]string, [][]byte, int, error)

	// CheckQueueDepth returns the number of messages in the queue
	CheckQueueDepth(ctx context.Context, subscriberID string, queueTableName string) (int, error)

	// Fetch executes a SELECT query and returns all results
	Fetch(ctx context.Context, query string) ([]string, error)

	// ExecuteStatement executes a SQL statement
	ExecuteStatement(ctx context.Context, query string) error

	// ExecuteWithParams executes a SQL statement with parameters
	ExecuteWithParams(ctx context.Context, query string, params map[string]interface{}) error

	// FetchWithParams executes a SELECT query with parameters
	FetchWithParams(ctx context.Context, query string, params map[string]interface{}) ([]string, error)

	// PackageExists checks if a package exists
	PackageExists(ctx context.Context, packageName string) (bool, error)

	// ProcedureExists checks if a procedure exists inside the given package.
	ProcedureExists(ctx context.Context, packageName string, procedureName string) (bool, error)

	// DeployPackages deploys PL/SQL packages
	DeployPackages(ctx context.Context, sequences []string, types []string, packageSpec []string, packageBody []string) error

	// DeployFile deploys a single SQL file
	DeployFile(ctx context.Context, sqlContent string) error

	// Connect establishes a database connection
	Connect(ctx context.Context) error

	// Close closes the database connection
	Close(ctx context.Context) error
}

// ==========================================
// Procedure Generator Interface
// ==========================================

type ProcedureGeneratorRepository interface {
	// ReserveFunnyName reserves a funny name for the subscriber. slotConsumed is true when the generator's list had to be modified
	// (either a new slot was claimed from AvailableNames, or an existing name was marked as Used). It is false when no reservation occurred because the call failed before mutating
	// generator state. Use slotConsumed to decide whether to ReleaseFunnyName on failure.
	ReserveFunnyName(ctx context.Context, subscriber *domain.Subscriber) (name string, slotConsumed bool, err error)

	// ReleaseFunnyName releases a previously reserved funny name
	ReleaseFunnyName(ctx context.Context, funnyName string) error

	// EnsureOwnedFunnyName ensures the subscriber has an unclaimed funny name.
	EnsureOwnedFunnyName(ctx context.Context, subscriber *domain.Subscriber) (changed bool, err error)

	// EnsureSubscriberProcedure ensures a PL/SQL procedure is owned by and routed to the subscriber.
	EnsureSubscriberProcedure(ctx context.Context, subscriber *domain.Subscriber) error

	// DropSubscriberProcedure drops the PL/SQL procedure for the subscriber
	DropSubscriberProcedure(ctx context.Context, funnyName string) error
}

// ==========================================
// Config Repository Interface (BoltDB)
// ==========================================

type ConfigRepository interface {
	// IsApplicationFirstRun checks if this is the first run
	IsApplicationFirstRun() (bool, error)

	// SetFirstRunCycleStatus saves the run cycle status
	SetFirstRunCycleStatus(status RunCycleStatus) error

	// SaveWebhookConfig saves a webhook configuration
	SaveWebhookConfig(config *domain.WebhookConfig) error

	// GetWebhookConfig retrieves the webhook configuration (uses default ID)
	GetWebhookConfig() (*domain.WebhookConfig, error)

	// DeleteWebhookConfig deletes a webhook configuration
	DeleteWebhookConfig(id string) error

	// GetTracerPackageVersion retrieves the stored package version hash.
	// Returns empty string if no hash is stored.
	GetTracerPackageVersion() (string, error)

	// SetTracerPackageVersion stores the package version hash.
	SetTracerPackageVersion(version string) error

	// GetBroadcastMode retrieves the stored broadcast mode.
	// Returns BroadcastModeGlobal when no value has been stored yet.
	GetBroadcastMode() (domain.BroadcastMode, error)

	// SetBroadcastMode stores the broadcast mode.
	SetBroadcastMode(mode domain.BroadcastMode) error

	// GetMCPAuthToken retrieves the stored MCP server bearer token. Returns empty string when no token has been generated yet.
	GetMCPAuthToken() (string, error)

	// SetMCPAuthToken stores the MCP server bearer token. Empty string clears the entry.
	SetMCPAuthToken(token string) error
}

// ==========================================
// Trace Appender Interface (in-memory trace store)
// ==========================================

// TraceAppender is the shared, bounded store every dequeued trace message is published to, so non-UI consumers (MCP server, tests) observe the same stream as the TUI. Implementations must be safe for concurrent use.
type TraceAppender interface {
	// Append adds a message, evicting oldest entries when full. Nil messages are ignored. Implementations may reject a message that cannot fit under their own size limits; such drops count toward Evicted.
	Append(ctx context.Context, msg *domain.QueueMessage) error
	// Len returns the current number of stored messages.
	Len(ctx context.Context) int
	// Evicted returns the lifetime count of messages lost: entries overwritten at capacity, entries dropped to stay under maxBytes, and payloads rejected for exceeding maxBytes on their own.
	Evicted(ctx context.Context) int
	// List returns up to `limit` messages in newest-first order. When sinceID is non-empty, only messages appended strictly after the message with that ID are returned. A non-positive limit returns no entries.
	List(ctx context.Context, limit int, sinceID string) ([]*domain.QueueMessage, error)
	// Clear removes all messages and releases the references held by the backing array.
	Clear(ctx context.Context) error
}
