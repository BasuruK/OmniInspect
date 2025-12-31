package config

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Adapter: Loads configurations from database
type ConfigLoader struct {
	configRepo ports.ConfigRepository
	reader     *bufio.Reader
}

// NewConfigLoader creates a new instance of ConfigLoader
func NewConfigLoader(configRepo ports.ConfigRepository) *ConfigLoader {
	return &ConfigLoader{
		configRepo: configRepo,
		reader:     bufio.NewReader(os.Stdin),
	}
}

// LoadClientConfigurations loads client configurations from boltDB or prompts user for input
func (cl *ConfigLoader) LoadClientConfigurations() (*domain.DatabaseSettings, error) {
	// 1. Try to load Database Settings from BoltDB
	config, err := cl.configRepo.GetDefaultDatabaseConfig()
	if err == nil && config != nil {
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
	if err := cl.configRepo.SaveDatabaseConfig(*config); err != nil {
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
	config.Host, err = cl.promptUserRequired("Database Host (e.g., localhost)")
	if err != nil {
		return nil, err
	}

	// Port
	portStr, err := cl.promptUserRequired("Database Port (e.g., 1521)")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %w", err)
	}
	config.Port = port

	// Database Name (Service/SID)
	config.Database, err = cl.promptUserRequired("Database Name (Service/SID)")
	if err != nil {
		return nil, err
	}

	// Username
	config.Username, err = cl.promptUserRequired("Username")
	if err != nil {
		return nil, err
	}

	// Password
	config.Password, err = cl.promptUserRequired("Password")
	if err != nil {
		return nil, err
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
	return strings.TrimSpace(input), nil // Remove newline characters
}

// Prompt user for required input (non-empty)
func (cl *ConfigLoader) promptUserRequired(prompt string) (string, error) {
	for {
		input, err := cl.promptUser(prompt)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(input) == "" {
			fmt.Printf("%s cannot be empty!\n", prompt)
		} else {
			return input, nil
		}
	}
}
