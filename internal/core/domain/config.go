package domain

// Value Object : Represents database connection settings
type DatabaseSettings struct {
	ID       string
	Database string
	Host     string
	Port     int
	Username string
	Password string
	Default  bool
}

// Value Object : Represents client-specific settings
type ClientSettings struct {
	EnableUtf8 bool
}

// Value Object : Status of the application's run cycle
type RunCycleStatus struct {
	IsFirstRun bool
}

// Value Object : Status of deployed database packages
type DatabasePackageStatus struct {
	TracerAPIExists bool
}

// Entity : Aggregates full application configurations
type AppConfigurations struct {
	DatabaseSettings DatabaseSettings
	ClientSettings   ClientSettings
}
