package domain

// Entity: Represents database connection settings
type DatabaseSettings struct {
	ID       string
	Database string
	Host     string
	Port     int
	Username string
	Password string
	Default  bool
}

// Entity: Represents client-specific settings
type ClientSettings struct {
	EnableUtf8 bool
}

// Entity : Status of the application's run cycle
type RunCycleStatus struct {
	IsFirstRun bool
}

// Entity : Status of deployed database packages
type DatabasePackageStatus struct {
	TracerAPIExists bool
}

// Entity : Aggregates of Application Startup settings
type AppStartupSettings struct {
	RunCycleStatus        RunCycleStatus
	DatabasePackageStatus DatabasePackageStatus
}

// Entity : Aggregates full application configurations
type AppConfigurations struct {
	DatabaseSettings DatabaseSettings
	ClientSettings   ClientSettings
}
