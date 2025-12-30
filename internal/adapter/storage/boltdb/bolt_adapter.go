package boltdb

import (
	"OmniView/internal/core/domain"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	// Buckets
	DatabaseConfigBucket = "DatabaseConfigurations"
	ClientConfigBucket   = "ClientConfigurations"
	// Bucket Defaults
	DefaultDatabaseConfigKey   = "db:default"
	DefaultClientConfigKey     = "client:default"
	DefaultPermissionConfigKey = "client:permissions"
	RunCycleStatusKey          = "run:status"
	// Bucket Key Signatures
	DatabaseConfigKeyPrefix = "db:config:"
)

type BoltAdapter struct {
	dbPath string
	db     *bolt.DB
}

// Constructor: NewBoltAdapter creates a new instance of BoltAdapter
func NewBoltAdapter(dbPath string) *BoltAdapter {
	return &BoltAdapter{
		dbPath: dbPath,
	}
}

func (ba *BoltAdapter) Initialize() error {
	if ba.db != nil {
		return fmt.Errorf("BoltAdapter already initialized")
	}

	var err error
	if ba.db, err = bolt.Open(ba.dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second}); err != nil {
		return fmt.Errorf("failed to open BoltDB: %v", err)
	}

	// Initialize buckets
	return ba.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(DatabaseConfigBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(ClientConfigBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}

		return nil
	})
}

// Close closes the BoltDB database.
func (ba *BoltAdapter) Close() error {
	if ba.db != nil {
		return ba.db.Close()
	}

	return nil
}

func (ba *BoltAdapter) SaveDatabaseConfig(config domain.DatabaseSettings) error {
	key := fmt.Sprintf("%s%s:%s", DatabaseConfigKeyPrefix, config.Username, config.Database)

	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))

		// Marshal the config to JSON
		jsonData, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal database config: %v", err)
		}
		// Save Config
		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save database config: %v", err)
		}
		// If default, update default key
		if config.Default {
			if err := b.Put([]byte(DefaultDatabaseConfigKey), []byte(key)); err != nil {
				return fmt.Errorf("failed to set default database config: %v", err)
			}
		}

		return nil
	})

}

func (ba *BoltAdapter) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	var config domain.DatabaseSettings

	// Get Default Key
	err := ba.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))

		defaultKey := b.Get([]byte(DefaultDatabaseConfigKey))
		if defaultKey == nil {
			return fmt.Errorf("default database config not found")
		}
		// Get Config JSON
		configData := b.Get(defaultKey)
		if configData == nil {
			return fmt.Errorf("database config not found for key: %s", string(defaultKey))
		}
		return json.Unmarshal(configData, &config)
	})
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (ba *BoltAdapter) SaveClientConfig(config domain.DatabasePermissions) error {
	key := DefaultPermissionConfigKey

	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ClientConfigBucket))

		// Marshal the config to JSON
		jsonData, err := json.Marshal(config.Permissions)
		if err != nil {
			return fmt.Errorf("failed to marshal client config: %v", err)
		}
		// Save Config
		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save client config: %v", err)
		}

		return nil
	})
}

// DatabaseConfigExists checks if a database configuration exists for the given key.
func (ba *BoltAdapter) DatabaseConfigExists(key string) (bool, error) {
	return exists(ba, []byte(DatabaseConfigBucket), key)
}

// Exists checks if a key exists in the specified bucket.
func exists(ba *BoltAdapter, bucket []byte, key string) (bool, error) {
	var found bool
	err := ba.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return fmt.Errorf("bucket %s not found", string(bucket))
		}
		v := b.Get([]byte(key))
		found = v != nil
		return nil
	})
	return found, err
}

// IsApplicationFirstRun checks if the application is running for the first time
// by verifying the presence of the run cycle status key in BoltDB.
// first run is to determine if initial setup tasks need to be performed.
func (ba *BoltAdapter) IsApplicationFirstRun() (bool, error) {
	var isFirstRun bool
	err := ba.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ClientConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", ClientConfigBucket)
		}
		v := b.Get([]byte(RunCycleStatusKey))
		isFirstRun = v == nil
		return nil
	})
	return isFirstRun, err
}

// SetFirstRunCycleStatus saves the current run cycle status to BoltDB.
func (ba *BoltAdapter) SetFirstRunCycleStatus(status domain.RunCycleStatus) error {
	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ClientConfigBucket))

		// Marshal the status to JSON
		jsonData, err := json.Marshal(status)
		if err != nil {
			return fmt.Errorf("failed to marshal run cycle status: %v", err)
		}
		// Save Status
		if err := b.Put([]byte(RunCycleStatusKey), jsonData); err != nil {
			return fmt.Errorf("failed to save run cycle status: %v", err)
		}

		return nil
	})
}
