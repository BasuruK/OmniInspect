package ports

import "OmniView/internal/core/domain"

// Port: DatabaseRepository Defines the interface for database repository operations
type DatabaseRepository interface {
	ExecuteStatement(query string) error
	Fetch(query string) ([]string, error)
	FetchWithParams(query string, params map[string]interface{}) ([]string, error)
	PackageExists(packageName string) (bool, error)
	DeployPackages(sequences []string, packageSpecs []string, packageBodies []string) error
	DeployFile(sqlContent string) error
}

// Port: ConfigRepository Defines the interface for configuration storage in boltDB
type ConfigRepository interface {
	Initialize() error
	Close() error
	SaveDatabaseConfig(config domain.DatabaseSettings) error
	GetDefaultDatabaseConfig() (domain.DatabaseSettings, error)
	DatabaseConfigExists(key string) (bool, error)
}
