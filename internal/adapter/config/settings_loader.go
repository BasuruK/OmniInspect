package config

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"bufio"
	"fmt"
	"os"
	"strconv"
)

// Adapter: Loads configurations from database
type ConfigLoader struct {
	ConfigRepo ports.ConfigRepository
	reader     *bufio.Reader
}

// NewConfigLoader creates a new instance of ConfigLoader
func NewConfigLoader(configRepo ports.ConfigRepository) *ConfigLoader {
	return &ConfigLoader{
		ConfigRepo: configRepo,
		reader:     bufio.NewReader(os.Stdin),
	}
}

// LoadClientConfigurations loads client configurations from a file
func (cl *ConfigLoader) LoadClientConfigurations() (*domain.DatabaseSettings, error) {
	// 1. Try to load Database Settings from BoltDB
	config, err := cl.ConfigRepo.GetDefaultDatabaseConfig()
	if err == nil {
		fmt.Println("✓ loaded database from boltDB")
		return config, nil
	}

	// 2. Handle missing config in BoltDB
	fmt.Println("✗ no database config found in boltDB, requesting user input")
	config, err = cl.GetDatabaseDetailsFromUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get database details from user: %w", err)
	}

	// Save the new configuration to BoltDB
	if err := cl.ConfigRepo.SaveDatabaseConfig(*config); err != nil {
		return nil, fmt.Errorf("failed to save database config to boltDB: %w", err)
	}
	fmt.Println("✓ saved database config to boltDB")

	return config, nil
}

func (cl *ConfigLoader) GetDatabaseDetailsFromUser() (*domain.DatabaseSettings, error) {
	config := &domain.DatabaseSettings{
		ID:      "default",
		Default: true,
	}

	var err error

	// Host
	config.Host, err = cl.promptUser("Database Host (e.g., localhost)")
	if err != nil {
		return &domain.DatabaseSettings{}, err
	}

	// Port
	portStr, err := cl.promptUser("Database Port (e.g., 1521)")
	if err != nil {
		return &domain.DatabaseSettings{}, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return &domain.DatabaseSettings{}, fmt.Errorf("invalid port number: %w", err)
	}
	config.Port = port

	// Database Name (Service/SID)
	config.Database, err = cl.promptUser("Database Name (Service/SID)")
	if err != nil {
		return &domain.DatabaseSettings{}, err
	}

	// Username
	config.Username, err = cl.promptUser("Username")
	if err != nil {
		return &domain.DatabaseSettings{}, err
	}

	// Password
	config.Password, err = cl.promptUser("Password")
	if err != nil {
		return &domain.DatabaseSettings{}, err
	}

	return config, nil
}

// Prompt user for input
func (cl *ConfigLoader) promptUser(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	input, err := cl.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return input[:len(input)-1], nil // Remove newline character
}
