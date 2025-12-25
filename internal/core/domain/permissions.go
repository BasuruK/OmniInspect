package domain

// Value Object : Results of permission checks
type PermissionStatus struct {
	CanCreateSequence      bool
	CanCreateTable         bool
	CanCreateProcedure     bool
	HasAQAdministratorRole bool
	HasAQUserRole          bool
	HasDBMSAQADMExec       bool
	HasDBMSAQExec          bool
	AllPermissionsValid    bool
}

// Entity : Represents database permissions for operations
type DatabasePermissions struct {
	Permissions PermissionStatus
}
