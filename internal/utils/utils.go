package utils

import (
	"OmniView/internal/database"
	"fmt"
)

func PrintTraceMessage() {
	// Connect to the database
	result, err := database.Fetch("SELECT 'HELLO WORLD' FROM DUAL")
	if err != nil {
		fmt.Println(fmt.Errorf("error fetching data: %v", err))
		return
	}
	for _, row := range result {
		fmt.Println(row)
	}
}

func CleanupResources() {
	database.CleanupDBConnection()
}
