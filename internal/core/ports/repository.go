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

	// GetByID retrieves database settings by ID
	GetByID(ctx context.Context, id string) (*domain.DatabaseSettings, error)

	// GetDefault retrieves the default database settings
	GetDefault(ctx context.Context) (*domain.DatabaseSettings, error)

	// GetAll retrieves all stored database settings
	GetAll(ctx context.Context) ([]domain.DatabaseSettings, error)

	// Delete removes database settings by ID
	Delete(ctx context.Context, id string) error

	// Replace atomically removes the record stored under oldKey and writes newRecord
	// in a single transaction. When oldKey and newRecord.StorageKey() are identical
	// the call is equivalent to Save. Use this when renaming a database ID to avoid
	// a window where neither key exists.
	Replace(ctx context.Context, oldKey string, newRecord domain.DatabaseSettings) error
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
// Config Repository Interface (BoltDB)
// ==========================================

type ConfigRepository interface {
	// SaveDatabaseConfig saves database configuration
	SaveDatabaseConfig(config *domain.DatabaseSettings) error

	// GetDefaultDatabaseConfig retrieves the default database configuration
	GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error)

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
}
