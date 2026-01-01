package assets

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

//go:embed ins/*.ins
var insFiles embed.FS

// GetSQLFile reads an embedded SQL file by name.
// fileName should be just the base filename (e.g., "Permission_Checks.sql") without the "sql/" directory prefix.
// Returns the file contents or an error if the file cannot be read.
func GetSQLFile(fileName string) ([]byte, error) {
	// Prevent path traversal attempts
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		return nil, fmt.Errorf("invalid SQL filename: %s", fileName)
	}

	data, err := sqlFiles.ReadFile("sql/" + fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read SQL file %s: %w", fileName, err)
	}

	return data, nil
}

// GetInsFile reads an embedded INS file by name.
// fileName should be just the base filename (e.g., "config.ins") without the "ins/" directory prefix.
// Returns the file contents or an error if the file cannot be read.
func GetInsFile(fileName string) ([]byte, error) {
	// Prevent path traversal attempts
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		return nil, fmt.Errorf("invalid INS filename: %s", fileName)
	}

	data, err := insFiles.ReadFile("ins/" + fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read INS file %s: %w", fileName, err)
	}

	return data, nil
}
