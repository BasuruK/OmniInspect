package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ==========================================
// Constants
// ==========================================

const (
	MinPort           Port = 1
	MaxPort           Port = 65535
	DefaultOraclePort Port = 1521
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
	database  string
	host      string
	port      Port
	username  string
	password  string
	isDefault bool
}

// NewDatabaseSettings creates new database settings with validation
func NewDatabaseSettings(database string, host string, port Port, username string, password string) (*DatabaseSettings, error) {
	database = strings.TrimSpace(database)
	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	// Validate database
	if database == "" {
		return nil, ErrEmptyDatabase
	}

	// Validate host
	if host == "" {
		return nil, ErrEmptyHost
	}

	// Validate port
	if port < MinPort || port > MaxPort {
		return nil, fmt.Errorf("%w: must be between %d and %d", ErrInvalidPort, MinPort, MaxPort)
	}

	// Validate username
	if username == "" {
		return nil, ErrEmptyUsername
	}

	// Validate password
	if password == "" {
		return nil, ErrEmptyPassword
	}

	return &DatabaseSettings{
		database:  database,
		host:      host,
		port:      port,
		username:  username,
		password:  password,
		isDefault: false,
	}, nil
}

// ==========================================
// Getters (Read-Only Accessors)
// ==========================================

func (dbs *DatabaseSettings) Database() string { return dbs.database }
func (dbs *DatabaseSettings) Host() string     { return dbs.host }
func (dbs *DatabaseSettings) Port() Port       { return dbs.port }
func (dbs *DatabaseSettings) Username() string { return dbs.username }
func (dbs *DatabaseSettings) Password() string { return dbs.password }
func (dbs *DatabaseSettings) IsDefault() bool  { return dbs.isDefault }

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

// SetAsDefault marks this as the default database
func (dbs *DatabaseSettings) SetAsDefault() {
	dbs.isDefault = true
}

// ==========================================
// JSON Marshaling
// ==========================================

// databaseSettingsJSON provides a JSON-friendly intermediate representation
type databaseSettingsJSON struct {
	Database  string `json:"database"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	IsDefault bool   `json:"isDefault"`
}

// MarshalJSON implements custom JSON marshaling for DatabaseSettings
func (dbs *DatabaseSettings) MarshalJSON() ([]byte, error) {
	j := databaseSettingsJSON{
		Database:  dbs.database,
		Host:      dbs.host,
		Port:      int(dbs.port),
		Username:  dbs.username,
		Password:  dbs.password,
		IsDefault: dbs.isDefault,
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

	cfg, err := NewDatabaseSettings(dbSettingJson.Database, dbSettingJson.Host, p, dbSettingJson.Username, dbSettingJson.Password)
	if err != nil {
		return err
	}
	cfg.isDefault = dbSettingJson.IsDefault
	*dbs = *cfg

	return nil
}
