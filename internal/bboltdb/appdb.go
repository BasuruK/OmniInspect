package bboltdb

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

type DatabaseConfig struct {
	ID       string `json:"id"`
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Default  bool   `json:"default"`
}

type ClientConfig struct {
	EnableUtf8 bool `json:"enable_utf8"`
}

var (
	// BoltDB instance
	db *bolt.DB
	// Buckets
	DatabaseConfigBucket = []byte("DatabaseConfigurations")
	// Bucket Defaults
	DefaultDatabaseConfigKey = "db:default"
)

// Initialize opens the BoltDB database file and creates necessary buckets.
func Initialize(dbPath string) error {
	var err error
	if db, err = bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second}); err != nil {
		return fmt.Errorf("failed to open BoltDB: %v", err)
	}

	// Create necessary buckets if not exist
	return db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(DatabaseConfigBucket); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
		return nil
	})
}

// Close closes the BoltDB database.
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// Core CRUD operations
// Core operations for interacting with BoltDB

// Insert adds a key-value pair to the specified bucket.
func Insert(bucket []byte, key, value string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put([]byte(key), []byte(value))
	})
}

// Update modifies the value for a given key in the specified bucket.
func Update(bucket []byte, key, value string) error {
	return Insert(bucket, key, value) // Key will be overwritten if it exists
}

// Delete removes a key-value pair from the specified bucket.
func Delete(bucket []byte, key string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Delete([]byte(key))
	})
}

// Get retrieves the value for a given key from the specified bucket.
func Get(bucket []byte, key string) (string, error) {
	var value string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("key not found")
		}
		value = string(v)
		return nil
	})
	return value, err
}

// Exists checks if a key exists in the specified bucket.
func Exists(bucket []byte, key string) (bool, error) {
	var exists bool
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		v := b.Get([]byte(key))
		exists = v != nil
		return nil
	})
	return exists, err
}

// GetAll retrieves all keys from the specified bucket.
func GetAll(bucket []byte) (map[string]string, error) {
	results := make(map[string]string)
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.ForEach(func(k, v []byte) error {
			results[string(k)] = string(v)
			return nil
		})
	})
	return results, err
}

// InsertJSON inserts a JSON string into the specified bucket with the given key.
func InsertJSON(bucket []byte, key string, obj interface{}) error {
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object to JSON: %v", err)
	}
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put([]byte(key), jsonData)
	})
}

// GetJSON retrieves a JSON string from the specified bucket and unmarshals it into the provided object.
func GetJSON(bucket []byte, key string, jsonOut interface{}) error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("key not found")
		}
		return json.Unmarshal(v, jsonOut)
	})
}

// Bucket specific CRUD operations
// DatabaseConfig operations

// InsertDatabaseConfig adds a database configuration to the DatabaseConfig bucket.
// If the database is marked as default, it updates the default entry accordingly under db:default: key with the config ID.
func InsertDatabaseJSONConfig(config DatabaseConfig) error {
	key := fmt.Sprintf("db:config:%s", config.ID)
	if err := InsertJSON(DatabaseConfigBucket, key, config); err != nil {
		return err
	}
	if config.Default {
		// Also set as default
		if err := Insert(DatabaseConfigBucket, DefaultDatabaseConfigKey, config.ID); err != nil {
			return err
		}
	}

	return nil
}

// GetDatabaseConfig retrieves the default database configuration from the DatabaseConfig bucket.
func GetDefaultDatabaseJSONConfig() (DatabaseConfig, error) {
	var config DatabaseConfig
	defaultID, err := Get(DatabaseConfigBucket, DefaultDatabaseConfigKey)
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("failed to get default database config ID: %w", err)
	}

	key := fmt.Sprintf("db:config:%s", defaultID)
	if err := GetJSON(DatabaseConfigBucket, key, &config); err != nil {
		return DatabaseConfig{}, err
	}

	return config, nil
}
