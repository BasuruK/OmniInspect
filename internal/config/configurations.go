package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ClientConfigurations struct {
	DatabaseSettings struct {
		Database string
		Host     string
		Port     int
		Username string
		Password string
	}
	ClientSettings struct {
		EnableUtf8 bool
	}
}

type SystemConfigurations struct {
	DatabasePackagesStruct struct {
		TracerAPIExists bool
	}
	DatabasePermissionsStruct struct {
		CanCreateSequence      bool
		CanCreateTable         bool
		CanCreateProcedure     bool
		HasAQAdministratorRole bool
		HasAQUserRole          bool
		HasDBMSAQADMExec       bool
		HasDBMSAQExec          bool
		AllPermissionsValid    bool
	}
	RunCycleStruct struct {
		IsFirstRun bool
	}
}

// GetDatabaseSettings returns the database settings as a struct
func (c *ClientConfigurations) GetDatabaseSettingsStruct() struct {
	Database string
	Host     string
	Port     int
	Username string
	Password string
} {
	return c.DatabaseSettings
}

// GetClientSettings returns the client settings as a struct
func (c *ClientConfigurations) GetClientSettingsStruct() struct {
	EnableUtf8 bool
} {
	return c.ClientSettings
}

func LoadClientConfigurations() ClientConfigurations {
	var configStruct ClientConfigurations

	// Open the JSON file
	file, err := os.Open("settings.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Decode the JSON data into the struct
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configStruct)
	if err != nil {
		panic(err)
	}
	return ClientConfigurations{
		DatabaseSettings: configStruct.DatabaseSettings,
		ClientSettings:   configStruct.ClientSettings}
}

func saveSystemConfigurations(config SystemConfigurations) error {
	// Open the .cfg file for writing
	file, err := os.Create("config.cfg")
	if err != nil {
		return err
	}

	// Encode the struct as JSON with indentation for readability
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print with 2-space indentation

	if err := encoder.Encode(config); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	defer file.Close()

	return nil
}

func checkSystemConfigurations() bool {
	// Check if the cfg file exists in the current directory
	_, err := os.Stat("config.cfg")
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func createDefaultSystemConfigurations() error {
	// Create a default configuration file
	defaultSystemConfig := SystemConfigurations{
		DatabasePackagesStruct: struct {
			TracerAPIExists bool
		}{
			TracerAPIExists: false,
		},
		DatabasePermissionsStruct: struct {
			CanCreateSequence      bool
			CanCreateTable         bool
			CanCreateProcedure     bool
			HasAQAdministratorRole bool
			HasAQUserRole          bool
			HasDBMSAQADMExec       bool
			HasDBMSAQExec          bool
			AllPermissionsValid    bool
		}{
			CanCreateSequence:      false,
			CanCreateTable:         false,
			CanCreateProcedure:     false,
			HasAQAdministratorRole: false,
			HasAQUserRole:          false,
			HasDBMSAQADMExec:       false,
			HasDBMSAQExec:          false,
			AllPermissionsValid:    false,
		},
		RunCycleStruct: struct {
			IsFirstRun bool
		}{
			IsFirstRun: true,
		},
	}
	return saveSystemConfigurations(defaultSystemConfig)
}

func loadSystemConfigurations() SystemConfigurations {
	var configStruct SystemConfigurations

	// Open the JSON file
	file, err := os.Open("config.cfg")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Decode the JSON data into the struct
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configStruct)
	if err != nil {
		panic(err)
	}
	return SystemConfigurations{
		DatabasePackagesStruct:    configStruct.DatabasePackagesStruct,
		DatabasePermissionsStruct: configStruct.DatabasePermissionsStruct,
		RunCycleStruct:            configStruct.RunCycleStruct,
	}
}

func SystemsSettingsCheck() bool {
	// Check if System Configurations file exists, if not create one with default values
	if !checkSystemConfigurations() {
		err := createDefaultSystemConfigurations()
		if err != nil {
			fmt.Printf("Failiure creating config.sys file %s", err)
			return false
		}
	} else {
		// Load the configurations from the file
		systemSettings := loadSystemConfigurations()
		fmt.Println(systemSettings)
		// Check if it's the first run
		if systemSettings.RunCycleStruct.IsFirstRun {
			fmt.Println("Systems Initializing...")
			// Perform any first-run specific setup here
		}
	}
	return true
}
