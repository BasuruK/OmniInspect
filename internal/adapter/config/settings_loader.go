package config

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Adapter: Loads configurations from database
type ConfigLoader struct {
	dbSettingsRepo ports.DatabaseSettingsRepository
	configRepo     ports.ConfigRepository
	reader         *bufio.Reader
}

// NewConfigLoader creates a new instance of ConfigLoader
func NewConfigLoader(dbSettingsRepo ports.DatabaseSettingsRepository, configRepo ports.ConfigRepository) *ConfigLoader {
	return &ConfigLoader{
		dbSettingsRepo: dbSettingsRepo,
		configRepo:     configRepo,
		reader:         bufio.NewReader(os.Stdin),
	}
}

// LoadClientConfigurations loads client configurations from boltDB or prompts user for input
func (cl *ConfigLoader) LoadClientConfigurations() (*domain.DatabaseSettings, error) {
	ctx := context.Background()

	// 1. Try to load Database Settings from BoltDB
	config, err := cl.dbSettingsRepo.GetDefault(ctx)
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
	if err := cl.dbSettingsRepo.Save(ctx, *config); err != nil {
		return nil, fmt.Errorf("failed to save database config to boltDB: %w", err)
	}
	fmt.Println("✓ saved database config to boltDB")

	// Prompt for webhook configuration
	cl.PromptForWebhookConfig()

	return config, nil
}

func (cl *ConfigLoader) GetDatabaseDetailsFromUser() (*domain.DatabaseSettings, error) {
	var err error

	// Host
	host, err := cl.promptUserRequired("Database Host (e.g., localhost)")
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
	dbPort, err := domain.NewPort(port)
	if err != nil {
		return nil, err
	}

	// Database Name (Service/SID)
	database, err := cl.promptUserRequired("Database Name (Service/SID)")
	if err != nil {
		return nil, err
	}

	// Username
	username, err := cl.promptUserRequired("Username")
	if err != nil {
		return nil, err
	}

	// Password
	password, err := cl.promptUserRequired("Password")
	if err != nil {
		return nil, err
	}

	// Use domain factory to create DatabaseSettings
	config, err := domain.NewDatabaseSettings(database, host, dbPort, username, password)
	if err != nil {
		return nil, err
	}

	// Set as default
	config.SetAsDefault()

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

// PromptForWebhookConfig prompts user for webhook URL if not configured
func (cl *ConfigLoader) PromptForWebhookConfig() {
	// Check if webhook already configured
	config, err := cl.configRepo.GetWebhookConfig()
	if err == nil && config != nil && config.URL != "" {
		fmt.Println("✓ webhook already configured")
		return
	}

	// Prompt for webhook URL (optional - user can skip by pressing enter)
	fmt.Print("(Optional) Enter webhook URL (or press Enter to skip): ")
	input, err := cl.reader.ReadString('\n')
	if err != nil {
		fmt.Println("Failed to read input, skipping webhook configuration")
		return
	}
	url := strings.TrimSpace(input)

	if url != "" {
		webhookConfig := domain.NewWebhookConfig(domain.DefaultWebhookID, url, true)
		if err := cl.configRepo.SaveWebhookConfig(webhookConfig); err != nil {
			fmt.Printf("Failed to save webhook config: %v\n", err)
			return
		}
		fmt.Println("✓ webhook URL saved!")
	} else {
		fmt.Println("Webhook configuration skipped")
	}
}
