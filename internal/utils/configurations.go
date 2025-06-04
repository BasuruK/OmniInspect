package utils

import (
	"encoding/json" // Removed duplicate import
	"fmt"
	"os"
)

type Configurations struct {
	DatabaseSettings struct {
		Database string `json:"database"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"database_settings"`
	ClientSettings struct {
		EnableUtf8 bool `json:"enable_utf8"`
	} `json:"client_settings"`
}

/*
This is the constructor for Configurations structure.
It attempts to load configurations from "settings.json" and panics on error.

	Return: DatabaseSetting structure, ClientSetting Structure
*/
func NewConfigFile() *Configurations {
	configurations, err := LoadConfigurations("settings.json")
	if err != nil {
		panic(fmt.Errorf("failed to load settings.json for NewConfigFile: %w", err))
	}
	return configurations // LoadConfigurations now returns a pointer
}

// GetDatabaseSettings returns the database settings as a struct
func (c *Configurations) GetDatabaseSettingsStruct() struct {
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
} {
	return c.DatabaseSettings
}

// GetClientSettings returns the client settings as a struct
func (c *Configurations) GetClientSettingsStruct() struct {
	EnableUtf8 bool `json:"enable_utf8"`
} {
	return c.ClientSettings
}

// LoadConfigurations loads configurations from the given filePath.
// If filePath is empty, it returns default configurations.
// It returns a pointer to Configurations and an error if any occurs.
func LoadConfigurations(filePath string) (*Configurations, error) {
	if filePath == "" {
		// Return default configurations
		return &Configurations{
			DatabaseSettings: struct {
				Database string `json:"database"`
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Username string `json:"username"`
				Password string `json:"password"`
			}{
				Database: "default_db",
				Host:     "localhost",
				Port:     5432,
				Username: "user",
				Password: "password",
			},
			ClientSettings: struct {
				EnableUtf8 bool `json:"enable_utf8"`
			}{
				EnableUtf8: true,
			},
		}, nil
	}

	var configStruct Configurations

	// Open the JSON file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening configuration file %s: %w", filePath, err)
	}
	defer file.Close()

	// Decode the JSON data into the struct
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configStruct)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON from file %s: %w", filePath, err)
	}
	return &configStruct, nil
}
