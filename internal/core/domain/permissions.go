package domain

import "encoding/json"

// ==========================================
// Value Objects
// ==========================================

// Value Object : Results of permission checks
type PermissionStatus struct {
	CreateSequence      bool `json:"CreateSequence"`
	CreateProcedure     bool `json:"CreateProcedure"`
	CreateType          bool `json:"CreateType"`
	AQAdministratorRole bool `json:"AQAdministratorRole"`
	AQUserRole          bool `json:"AQUserRole"`
	DBMSAQADMExecute    bool `json:"DBMSAQADMExecute"`
	DBMSAQExecute       bool `json:"DBMSAQExecute"`
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

// ==========================================
// JSON Marshaling
// ==========================================

// databasePermissionsJSON provides a JSON-friendly intermediate representation
type databasePermissionsJSON struct {
	Schema      string           `json:"schema"`
	Permissions PermissionStatus `json:"permissions"`
}

// MarshalJSON implements custom JSON marshaling for DatabasePermissions
func (p DatabasePermissions) MarshalJSON() ([]byte, error) {
	j := databasePermissionsJSON{
		Schema:      p.schema,
		Permissions: p.permissions,
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements custom JSON unmarshaling for DatabasePermissions
func (p *DatabasePermissions) UnmarshalJSON(data []byte) error {
	var j databasePermissionsJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	p.schema = j.Schema
	p.permissions = j.Permissions
	return nil
}
