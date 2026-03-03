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
		return 0, fmt.Errorf("port must be between %d and %d", MinPort, MaxPort)
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
	// Validate database
	if strings.TrimSpace(database) == "" {
		return nil, ErrEmptyDatabase
	}

	// Validate host
	if strings.TrimSpace(host) == "" {
		return nil, ErrEmptyHost
	}

	// Validate port
	if port < MinPort || port > MaxPort {
		return nil, fmt.Errorf("%w: must be between %d and %d", ErrInvalidPort, MinPort, MaxPort)
	}

	// Validate username
	if strings.TrimSpace(username) == "" {
		return nil, ErrEmptyUsername
	}

	// Validate password
	if strings.TrimSpace(password) == "" {
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

func (s *DatabaseSettings) Database() string { return s.database }
func (s *DatabaseSettings) Host() string     { return s.host }
func (s *DatabaseSettings) Port() Port       { return s.port }
func (s *DatabaseSettings) Username() string { return s.username }
func (s *DatabaseSettings) Password() string { return s.password }
func (s *DatabaseSettings) IsDefault() bool  { return s.isDefault }

// ==========================================
// Business Methods
// ==========================================

// GetConnectionString returns the Oracle Easy Connect string
func (s *DatabaseSettings) GetConnectionString() string {
	return fmt.Sprintf("%s:%d/%s", s.host, s.port, s.database)
}

// GetConnectionDetails returns a safe string for logging (without password)
func (s *DatabaseSettings) GetConnectionDetails() string {
	return fmt.Sprintf("%s@%s:%d/%s", s.username, s.host, s.port, s.database)
}

// SetAsDefault marks this as the default database
func (s *DatabaseSettings) SetAsDefault() {
	s.isDefault = true
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
func (s *DatabaseSettings) MarshalJSON() ([]byte, error) {
	j := databaseSettingsJSON{
		Database:  s.database,
		Host:      s.host,
		Port:      int(s.port),
		Username:  s.username,
		Password:  s.password,
		IsDefault: s.isDefault,
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements custom JSON unmarshaling for DatabaseSettings
func (s *DatabaseSettings) UnmarshalJSON(data []byte) error {
	var j databaseSettingsJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return fmt.Errorf("failed to unmarshal DatabaseSettings: %w", err)
	}

	// Use default port if not specified
	port := j.Port
	if port == 0 {
		port = int(DefaultOraclePort)
	}
	p, err := NewPort(port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	s.database = j.Database
	s.host = j.Host
	s.port = p
	s.username = j.Username
	s.password = j.Password
	s.isDefault = j.IsDefault

	return nil
}
