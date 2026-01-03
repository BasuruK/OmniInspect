package ports

import (
	"OmniView/internal/core/domain"
)

// Oracle
// Port: DatabaseRepository Defines the interface for database repository operations
type DatabaseRepository interface {
	ExecuteStatement(query string) error
	Fetch(query string) ([]string, error)
	FetchWithParams(query string, params map[string]interface{}) ([]string, error)
	ExecuteWithParams(query string, params map[string]interface{}) error
	PackageExists(packageName string) (bool, error)
	DeployPackages(sequences []string, packageSpecs []string, packageBodies []string) error
	DeployFile(sqlContent string) error
	RegisterNewSubscriber(subscriber domain.Subscriber) error
}

// BoltDB
// Port: ConfigRepository Defines the interface for configuration storage in boltDB
type ConfigRepository interface {
	// Application Initialization
	Initialize() error
	Close() error
	// Database Configurations
	SaveDatabaseConfig(config domain.DatabaseSettings) error
	GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error)
	SaveClientConfig(config domain.DatabasePermissions) error
	DatabaseConfigExists(key string) (bool, error)
	// Application Startup Settings
	// Application Run Cycle
	SetFirstRunCycleStatus(status domain.RunCycleStatus) error
	IsApplicationFirstRun() (bool, error)
	// Subscriber Information
	SetSubscriberName(subscriber domain.Subscriber) error
	GetSubscriberName() (*domain.Subscriber, error)
}
