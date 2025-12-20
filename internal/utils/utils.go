package utils

import (
	"OmniView/internal/config"
	"OmniView/internal/database"
	"fmt"
)

func PrintTraceMessage() {
	// Connect to the database
	result, err := database.Fetch("SELECT 'HELLO WORLD' FROM DUAL")
	if err != nil {
		fmt.Printf("error fetching data: %v", err)
		return
	}
	for _, row := range result {
		fmt.Println(row)
	}
}

func CleanupResources() {
	database.CleanupDBConnection()
}

// StartupResources initializes necessary resources for the application.
// This will perform the below checks:
// 1. Check if the required database packages are installed and valid.
// 2. Initialize any other resources as needed.
// 3. Saves the state of database and permission status for future loads to optimize startup time.
//
// Return: error if any of the checks fail, nil otherwise.
func StartupResources() error {
	// Run system settings check
	config.SystemsSettingsCheck()
	// Check if the database permissions are valid to run the tracer appliation.
	return nil
}
