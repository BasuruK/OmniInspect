package domain

// Value Object : Results of permission checks
type PermissionStatus struct {
	Schema              string
	CreateSequence      bool
	CreateProcedure     bool
	AQAdministratorRole bool
	AQUserRole          bool
	DBMSAQADMExecute    bool
	DBMSAQExecute       bool
	AllValid            bool
}

// Entity : Represents database permissions for operations
type DatabasePermissions struct {
	Permissions PermissionStatus
}
