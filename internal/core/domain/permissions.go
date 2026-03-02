package domain

import "errors"

// ==========================================
// Constants
// ==========================================

// ==========================================
// Errors
// ==========================================

var (
	ErrPermissionDenied      = errors.New("permission denied")
	ErrMissingPermissions    = errors.New("missing required permissions")
	ErrPermissionCheckFailed = errors.New("permission check failed")
)

// ==========================================
// Value Objects
// ==========================================

// Value Object : Results of permission checks
type PermissionStatus struct {
	CreateSequence      bool
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

// ==========================================
// Entities
// ==========================================

// Entity : Represents database permissions for operations
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

// ==========================================
// Getters (Read-Only Accessors)
// ==========================================

func (p *DatabasePermissions) Schema() string                { return p.schema }
func (p *DatabasePermissions) Permissions() PermissionStatus { return p.permissions }

// ==========================================
// Business Methods
// ==========================================

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
