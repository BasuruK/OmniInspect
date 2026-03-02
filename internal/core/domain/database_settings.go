package domain

import (
	"errors"
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
// Errors
// ==========================================

var (
	ErrEmptyDatabase     = errors.New("database name cannot be empty")
	ErrEmptyHost         = errors.New("host cannot be empty")
	ErrInvalidPort       = errors.New("invalid port number")
	ErrEmptyUsername     = errors.New("username cannot be empty")
	ErrEmptyPassword     = errors.New("password cannot be empty")
	ErrInvalidConnection = errors.New("invalid connection string")
	ErrHostUnreachable   = errors.New("host is unreachable")
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
	if port == 0 {
		return nil, ErrInvalidPort
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
