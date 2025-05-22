package utils

import (
	"encoding/json"
	"os"
)

type Configurations struct {
	DatabaseSettings struct {
		Database string
		Host     string
		Username string
		Password string
	}
	ClientSettings struct {
		EnableUtf8 bool
	}
}

/*
This is the constructor for Configurations structure

	Return: DatabaseSetting structure, ClientSetting Structure
*/
func NewConfigFile() *Configurations {
	configurations := LoadConfigurations()
	return &Configurations{
		DatabaseSettings: configurations.DatabaseSettings,
		ClientSettings:   configurations.ClientSettings,
	}
}

// GetDatabaseSettings returns the database settings as a struct
func (c *Configurations) GetDatabaseSettingsStruct() struct {
	Database string
	Host     string
	Username string
	Password string
} {
	return c.DatabaseSettings
}

// GetClientSettings returns the client settings as a struct
func (c *Configurations) GetClientSettingsStruct() struct {
	EnableUtf8 bool
} {
	return c.ClientSettings
}

func LoadConfigurations() Configurations {
	var configStruct Configurations

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
	return Configurations{
		DatabaseSettings: configStruct.DatabaseSettings,
		ClientSettings:   configStruct.ClientSettings}
}
