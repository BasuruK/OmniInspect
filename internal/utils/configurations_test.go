package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"OmniView/internal/utils" // Updated import path
)

// TestLoadConfigurations tests the LoadConfigurations function.
func TestLoadConfigurations(t *testing.T) {
	// 1. Test with no configuration file (should load defaults)
	t.Run("DefaultValues", func(t *testing.T) {
		config, err := utils.LoadConfigurations("") // Call with empty path for defaults
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "default_db", config.DatabaseSettings.Database)
		assert.Equal(t, "localhost", config.DatabaseSettings.Host)
		assert.Equal(t, 5432, config.DatabaseSettings.Port)
		assert.Equal(t, "user", config.DatabaseSettings.Username)
		assert.Equal(t, "password", config.DatabaseSettings.Password)
		assert.Equal(t, true, config.ClientSettings.EnableUtf8)
	})

	// Create a temporary directory for test configuration files
	tempDir, err := os.MkdirTemp("", "test_configs_") // Added underscore for clarity
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Test with a non-existent configuration file
	t.Run("FileNotFound", func(t *testing.T) {
		_, err := utils.LoadConfigurations(filepath.Join(tempDir, "non_existent_config.json"))
		assert.Error(t, err)
		// Optionally, check for a specific error message if desired
		// Example: assert.Contains(t, err.Error(), "error opening configuration file")
	})

	// 3. Test with an invalid JSON file
	t.Run("InvalidJSON", func(t *testing.T) {
		invalidJSONPath := filepath.Join(tempDir, "invalid.json")
		err := os.WriteFile(invalidJSONPath, []byte(`{"database_settings": {"host": "localhost", "port": "not_an_int"}}`), 0644) // malformed port
		if err != nil {
			t.Fatalf("Failed to write invalid JSON file: %v", err)
		}
		_, err = utils.LoadConfigurations(invalidJSONPath)
		assert.Error(t, err)
		// Optionally, check for a specific error message
		// Example: assert.Contains(t, err.Error(), "error decoding JSON")
	})

	// 4. Test with a valid configuration file
	t.Run("ValidConfiguration", func(t *testing.T) {
		validJSON := `{
			"database_settings": {
				"database": "prod_db",
				"host": "remotehost",
				"port": 5433,
				"username": "prod_user",
				"password": "prod_password"
			},
			"client_settings": {
				"enable_utf8": false
			}
		}`
		validJSONPath := filepath.Join(tempDir, "valid.json")
		err := os.WriteFile(validJSONPath, []byte(validJSON), 0644)
		if err != nil {
			t.Fatalf("Failed to write valid JSON file: %v", err)
		}

		config, err := utils.LoadConfigurations(validJSONPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "prod_db", config.DatabaseSettings.Database)
		assert.Equal(t, "remotehost", config.DatabaseSettings.Host)
		assert.Equal(t, 5433, config.DatabaseSettings.Port)
		assert.Equal(t, "prod_user", config.DatabaseSettings.Username)
		assert.Equal(t, "prod_password", config.DatabaseSettings.Password)
		assert.Equal(t, false, config.ClientSettings.EnableUtf8)
	})

	// 5. Test with a file that has missing fields (should use Go's default zero values for missing fields)
	t.Run("PartialConfiguration", func(t *testing.T) {
		partialJSON := `{
			"database_settings": {
				"host": "partial_host"
			}
		}` // Port, Username, Password, ClientSettings are missing
		partialJSONPath := filepath.Join(tempDir, "partial.json")
		err := os.WriteFile(partialJSONPath, []byte(partialJSON), 0644)
		if err != nil {
			t.Fatalf("Failed to write partial JSON file: %v", err)
		}

		config, err := utils.LoadConfigurations(partialJSONPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "", config.DatabaseSettings.Database) // Zero value for string
		assert.Equal(t, "partial_host", config.DatabaseSettings.Host)
		assert.Equal(t, 0, config.DatabaseSettings.Port) // Zero value for int
		assert.Equal(t, "", config.DatabaseSettings.Username) // Zero value for string
		assert.Equal(t, "", config.DatabaseSettings.Password) // Zero value for string
		assert.Equal(t, false, config.ClientSettings.EnableUtf8) // Zero value for bool
	})

	// 6. Test with an empty JSON file (should result in a decoding error)
	t.Run("EmptyFile", func(t *testing.T) {
		emptyJSONPath := filepath.Join(tempDir, "empty.json")
		err := os.WriteFile(emptyJSONPath, []byte(""), 0644) // Create an empty file
		if err != nil {
			t.Fatalf("Failed to write empty JSON file: %v", err)
		}

		_, err = utils.LoadConfigurations(emptyJSONPath)
		assert.Error(t, err)
		// Optionally, check for a specific error message related to EOF or empty input
		// Example: assert.Contains(t, err.Error(), "EOF") or similar depending on JSON parser behavior for empty files
	})
}

// TestGetSettingsStructs tests the GetDatabaseSettingsStruct and GetClientSettingsStruct methods.
func TestGetSettingsStructs(t *testing.T) {
	// 1. Test Case: GetDatabaseSettingsStruct with populated data
	t.Run("GetDatabaseSettingsStruct_Populated", func(t *testing.T) {
		config := utils.Configurations{
			DatabaseSettings: struct {
				Database string `json:"database"`
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Username string `json:"username"`
				Password string `json:"password"`
			}{
				Database: "test_db",
				Host:     "test_host",
				Port:     1234,
				Username: "test_user",
				Password: "test_password",
			},
			ClientSettings: struct {
				EnableUtf8 bool `json:"enable_utf8"`
			}{
				EnableUtf8: true, // Arbitrary value for this part of test
			},
		}

		dbSettings := config.GetDatabaseSettingsStruct()
		assert.Equal(t, "test_db", dbSettings.Database)
		assert.Equal(t, "test_host", dbSettings.Host)
		assert.Equal(t, 1234, dbSettings.Port)
		assert.Equal(t, "test_user", dbSettings.Username)
		assert.Equal(t, "test_password", dbSettings.Password)
	})

	// 2. Test Case: GetClientSettingsStruct with populated data
	t.Run("GetClientSettingsStruct_Populated", func(t *testing.T) {
		config := utils.Configurations{
			DatabaseSettings: struct { // Arbitrary values for this part of test
				Database string `json:"database"`
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Username string `json:"username"`
				Password string `json:"password"`
			}{},
			ClientSettings: struct {
				EnableUtf8 bool `json:"enable_utf8"`
			}{
				EnableUtf8: true,
			},
		}
		clientSettings := config.GetClientSettingsStruct()
		assert.Equal(t, true, clientSettings.EnableUtf8)

		// Test with false as well
		config.ClientSettings.EnableUtf8 = false
		clientSettings = config.GetClientSettingsStruct()
		assert.Equal(t, false, clientSettings.EnableUtf8)
	})

	// 3. Test Case: GetSettingsStructs with Zero Values
	t.Run("GetSettingsStructs_ZeroValues", func(t *testing.T) {
		var config utils.Configurations // Zero-value instance

		dbSettings := config.GetDatabaseSettingsStruct()
		assert.Equal(t, "", dbSettings.Database, "Default Database should be empty string")
		assert.Equal(t, "", dbSettings.Host, "Default Host should be empty string")
		assert.Equal(t, 0, dbSettings.Port, "Default Port should be 0")
		assert.Equal(t, "", dbSettings.Username, "Default Username should be empty string")
		assert.Equal(t, "", dbSettings.Password, "Default Password should be empty string")

		clientSettings := config.GetClientSettingsStruct()
		assert.Equal(t, false, clientSettings.EnableUtf8, "Default EnableUtf8 should be false")
	})
}

// TestNewConfigFile tests the NewConfigFile function.
// NewConfigFile is expected to load "settings.json" from the current working directory
// of the test execution, which is typically the package directory.
func TestNewConfigFile(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	// It's common for tests to be run with the package directory as the working directory.
	// Let's ensure settings.json is created there.
	packageDir := filepath.Join(originalWd) // Assuming tests run from package dir.

	settingsFilePath := filepath.Join(packageDir, "settings.json")

	// Helper function to remove settings.json, ignoring errors if it doesn't exist
	removeSettingsFile := func() {
		_ = os.Remove(settingsFilePath)
	}

	// 1. Test Case: Successful Load
	t.Run("SuccessfulLoad", func(t *testing.T) {
		defer removeSettingsFile() // Clean up afterwards

		validJSON := `{
			"database_settings": {
				"database": "test_db_new",
				"host": "testhost_new",
				"port": 1234,
				"username": "testuser_new",
				"password": "testpassword_new"
			},
			"client_settings": {
				"enable_utf8": true
			}
		}`
		err := os.WriteFile(settingsFilePath, []byte(validJSON), 0644)
		if err != nil {
			t.Fatalf("Failed to write temporary settings.json: %v", err)
		}

		var config *utils.Configurations
		assert.NotPanics(t, func() {
			config = utils.NewConfigFile()
		}, "NewConfigFile panicked with a valid settings.json")

		assert.NotNil(t, config)
		if config != nil { // Proceed only if config is not nil to avoid panic on dereference
			assert.Equal(t, "test_db_new", config.DatabaseSettings.Database)
			assert.Equal(t, "testhost_new", config.DatabaseSettings.Host)
			assert.Equal(t, 1234, config.DatabaseSettings.Port)
			assert.Equal(t, "testuser_new", config.DatabaseSettings.Username)
			assert.Equal(t, "testpassword_new", config.DatabaseSettings.Password)
			assert.Equal(t, true, config.ClientSettings.EnableUtf8)
		}
	})

	// 2. Test Case: File Not Found (Panic)
	t.Run("FileNotFoundPanic", func(t *testing.T) {
		removeSettingsFile() // Ensure it's not there

		assert.Panics(t, func() {
			utils.NewConfigFile()
		}, "NewConfigFile did not panic when settings.json was missing.")
	})

	// 3. Test Case: Invalid JSON (Panic)
	t.Run("InvalidJSONPanic", func(t *testing.T) {
		defer removeSettingsFile() // Clean up afterwards

		invalidJSON := `{"database_settings": {"host": "localhost", "port": "not_an_int"}}` // Malformed port
		err := os.WriteFile(settingsFilePath, []byte(invalidJSON), 0644)
		if err != nil {
			t.Fatalf("Failed to write temporary invalid settings.json: %v", err)
		}

		assert.Panics(t, func() {
			utils.NewConfigFile()
		}, "NewConfigFile did not panic with invalid JSON in settings.json.")
	})
}
