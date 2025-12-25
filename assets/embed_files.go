package assets

import (
	"embed"
	"fmt"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func GetSQLFile(fileName string) ([]byte, error) {
	data, err := sqlFiles.ReadFile("sql/" + fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read SQL file %s: %w", fileName, err)
	}
	return data, nil
}
