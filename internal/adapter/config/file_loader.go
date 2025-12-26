package config

import (
	"OmniView/internal/core/domain"
	"encoding/json"
	"fmt"
	"os"
)

// Adapter: Loads configurations from a file
type FileConfigLoader struct {
	FilePath string
}

// NewFileConfigLoader creates a new instance of FileConfigLoader
func NewFileConfigLoader(filePath string) *FileConfigLoader {
	return &FileConfigLoader{
		FilePath: filePath,
	}
}

// LoadClientConfigurations loads client configurations from a file
func (fcl *FileConfigLoader) LoadClientConfigurations() (*domain.AppConfigurations, error) {
	// Implementation to read from file and parse into AppConfigurations
	// Open the JSON file
	file, err := os.Open(fcl.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", fcl.FilePath, err)
	}
	defer file.Close()

	// Decode the JSON data into the struct
	var config domain.AppConfigurations
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file %s: %w", fcl.FilePath, err)
	}

	return &config, nil
}
