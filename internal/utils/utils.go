package utils

import "OmniView/internal/database"

func PrintTraceMessage() {
	// Connect to the database
	database.Fetch("SELECT 'HELLO WORLD' FROM DUAL")
}
