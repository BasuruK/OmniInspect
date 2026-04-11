package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ==========================================
// Constants
// ==========================================

const (
	MinPort              Port = 1
	MaxPort              Port = 65535
	DefaultOraclePort    Port = 1521
	settingsIDPrefix          = "DBconfig:"
	legacySettingsPrefix      = "cfg:"
)

// ==========================================
// Value Objects
// ==========================================

// Port represents a database port number
type Port int

func NewPort(p int) (Port, error) {
	if p < int(MinPort) || p > int(MaxPort) {
		return 0, fmt.Errorf("%w: must be between %d and %d", ErrInvalidPort, MinPort, MaxPort)
	}
	return Port(p), nil
}

func (p Port) Int() int { return int(p) }

// ==========================================
// Database Settings Entity
// ==========================================

// Entity: Represents database connection settings
type DatabaseSettings struct {
	id           string
	persistedKey string
	databaseID   string
	database     string
	host         string
	port         Port
	username     string
	password     string
	isDefault    bool
	validated    bool
}

// makeSettingsID constructs a stable unique ID from the user-facing database ID.
func makeSettingsID(databaseID string) string {
	return settingsIDPrefix + url.PathEscape(databaseID)
}

// validateSettingsFields checks that all required database settings fields are
// non-empty and the port falls within the allowed range.
func validateSettingsFields(databaseID, database, host, username, password string, port Port) error {
	if databaseID == "" {
		return ErrEmptyDatabaseID
	}
	if database == "" {
		return ErrEmptyDatabase
	}
	if host == "" {
		return ErrEmptyHost
	}
	if port < MinPort || port > MaxPort {
		return fmt.Errorf("%w: must be between %d and %d", ErrInvalidPort, MinPort, MaxPort)
	}
	if username == "" {
		return ErrEmptyUsername
	}
	if password == "" {
		return ErrEmptyPassword
	}
	return nil
}

// NewDatabaseSettings creates new database settings with validation
func NewDatabaseSettings(databaseID string, database string, host string, port Port, username string, password string) (*DatabaseSettings, error) {
	databaseID = strings.TrimSpace(databaseID)
	database = strings.TrimSpace(database)
	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if err := validateSettingsFields(databaseID, database, host, username, password, port); err != nil {
		return nil, err
	}

	return &DatabaseSettings{
		id:         makeSettingsID(databaseID),
		databaseID: databaseID,
		database:   database,
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		isDefault:  false,
		validated:  false,
	}, nil
}

func (dbs *DatabaseSettings) Update(databaseID string, database string, host string, port Port, username string, password string) error {
	databaseID = strings.TrimSpace(databaseID)
	database = strings.TrimSpace(database)
	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if err := validateSettingsFields(databaseID, database, host, username, password, port); err != nil {
		return err
	}

	dbs.databaseID = databaseID
	dbs.id = makeSettingsID(databaseID)
	dbs.database = database
	dbs.host = host
	dbs.port = port
	dbs.username = username
	dbs.password = password
	dbs.validated = false
	return nil
}

// ==========================================
// Getters (Read-Only Accessors)
// ==========================================

func (dbs *DatabaseSettings) ID() string {
	trimmed := strings.TrimPrefix(dbs.id, settingsIDPrefix)
	trimmed = strings.TrimPrefix(trimmed, legacySettingsPrefix)
	if unescaped, err := url.PathUnescape(trimmed); err == nil {
		return unescaped
	}
	return trimmed
}

func (dbs *DatabaseSettings) StorageKey() string   { return dbs.id }
func (dbs *DatabaseSettings) PersistedKey() string { return dbs.persistedKey }
func (dbs *DatabaseSettings) DatabaseID() string   { return dbs.databaseID }
func (dbs *DatabaseSettings) Database() string     { return dbs.database }
func (dbs *DatabaseSettings) Host() string         { return dbs.host }
func (dbs *DatabaseSettings) Port() Port           { return dbs.port }
func (dbs *DatabaseSettings) Username() string     { return dbs.username }
func (dbs *DatabaseSettings) Password() string     { return dbs.password }
func (dbs *DatabaseSettings) IsDefault() bool      { return dbs.isDefault }
func (dbs *DatabaseSettings) PermissionsValidated() bool {
	return dbs.validated
}

// ==========================================
// Business Methods
// ==========================================

// GetConnectionString returns the Oracle Easy Connect string
func (dbs *DatabaseSettings) GetConnectionString() string {
	return fmt.Sprintf("%s:%d/%s", dbs.host, dbs.port, dbs.database)
}

// GetConnectionDetails returns a safe string for logging (without password)
func (dbs *DatabaseSettings) GetConnectionDetails() string {
	return fmt.Sprintf("%s@%s:%d/%s", dbs.username, dbs.host, dbs.port, dbs.database)
}

// DisplayTarget returns the compact secondary label shown in the UI.
func (dbs *DatabaseSettings) DisplayTarget() string {
	return fmt.Sprintf("%s @ %s", dbs.database, dbs.host)
}

// SetAsDefault marks this as the default database
func (dbs *DatabaseSettings) SetAsDefault() {
	dbs.isDefault = true
}

// MarkPermissionsValidated records that permissions were successfully verified for this connection.
func (dbs *DatabaseSettings) MarkPermissionsValidated() {
	dbs.validated = true
}

// ClearPermissionsValidated removes the cached permission validation marker.
func (dbs *DatabaseSettings) ClearPermissionsValidated() {
	dbs.validated = false
}

// SetPersistedKey sets the actual BoltDB storage key for this record.
// An empty key indicates the record has not yet been persisted.
func (dbs *DatabaseSettings) SetPersistedKey(key string) {
	dbs.persistedKey = key
}

// ==========================================
// JSON Marshaling
// ==========================================

// databaseSettingsJSON provides a JSON-friendly intermediate representation
type databaseSettingsJSON struct {
	DatabaseID string `json:"databaseId"`
	Database   string `json:"database"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	IsDefault  bool   `json:"isDefault"`
	Validated  bool   `json:"validated,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for DatabaseSettings
func (dbs *DatabaseSettings) MarshalJSON() ([]byte, error) {
	j := databaseSettingsJSON{
		DatabaseID: dbs.databaseID,
		Database:   dbs.database,
		Host:       dbs.host,
		Port:       int(dbs.port),
		Username:   dbs.username,
		Password:   dbs.password,
		IsDefault:  dbs.isDefault,
		Validated:  dbs.validated,
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements custom JSON unmarshaling for DatabaseSettings
func (dbs *DatabaseSettings) UnmarshalJSON(data []byte) error {
	var dbSettingJson databaseSettingsJSON
	if err := json.Unmarshal(data, &dbSettingJson); err != nil {
		return fmt.Errorf("failed to unmarshal DatabaseSettings: %w", err)
	}

	// Use default port if not specified
	port := dbSettingJson.Port
	if port == 0 {
		port = int(DefaultOraclePort)
	}
	p, err := NewPort(port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	databaseID := strings.TrimSpace(dbSettingJson.DatabaseID)
	if databaseID == "" {
		// Backward compatibility for older saved configs.
		databaseID = dbSettingJson.Database
	}

	cfg, err := NewDatabaseSettings(databaseID, dbSettingJson.Database, dbSettingJson.Host, p, dbSettingJson.Username, dbSettingJson.Password)
	if err != nil {
		return err
	}
	cfg.isDefault = dbSettingJson.IsDefault
	cfg.validated = dbSettingJson.Validated
	*dbs = *cfg

	return nil
}
