package boltdb

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	// Buckets
	DatabaseConfigBucket  = "DatabaseConfigurations"
	ClientConfigBucket   = "ClientConfigurations"
	WebhookConfigBucket  = "WebhookConfigurations"
	// Bucket Keys
	DefaultDatabaseConfigKey = "db:default"
	RunCycleStatusKey        = "run:status"
	DatabaseConfigKeyPrefix  = "db:config:"
	DefaultWebhookKey        = "webhook:default"
)

// BoltAdapter implements the ports.ConfigRepository
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
		return fmt.Errorf("boltAdapter already initialized")
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
		if _, err := tx.CreateBucketIfNotExists([]byte(SubscriberBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(PermissionsBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(WebhookConfigBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}

		return nil
	})
}

// Close closes the BoltDB database.
func (ba *BoltAdapter) Close() error {
	if ba.db != nil {
		err := ba.db.Close()
		ba.db = nil
		return err
	}

	return nil
}

func (ba *BoltAdapter) SaveDatabaseConfig(config *domain.DatabaseSettings) error {
	if ba.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	if config == nil {
		return fmt.Errorf("database config cannot be nil")
	}

	key := fmt.Sprintf("%s%s:%s", DatabaseConfigKeyPrefix, config.Username(), config.Database())

	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))

		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

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
		if config.IsDefault() {
			if err := b.Put([]byte(DefaultDatabaseConfigKey), []byte(key)); err != nil {
				return fmt.Errorf("failed to set default database config: %v", err)
			}
		}

		return nil
	})
}

func (ba *BoltAdapter) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	if ba.db == nil {
		return nil, fmt.Errorf("boltAdapter not initialized")
	}

	var config *domain.DatabaseSettings

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

	return config, nil
}

// DatabaseConfigExists checks if a database configuration exists for the given key.
func (ba *BoltAdapter) DatabaseConfigExists(key string) (bool, error) {
	return exists(ba, []byte(DatabaseConfigBucket), key)
}

// Exists checks if a key exists in the specified bucket.
func exists(ba *BoltAdapter, bucket []byte, key string) (bool, error) {
	if ba.db == nil {
		return false, fmt.Errorf("boltAdapter not initialized")
	}

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
	if ba.db == nil {
		return false, fmt.Errorf("boltAdapter not initialized")
	}

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
func (ba *BoltAdapter) SetFirstRunCycleStatus(status ports.RunCycleStatus) error {
	if ba.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ClientConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", ClientConfigBucket)
		}

		// Save the first run status as a simple boolean
		firstRun := status.IsFirstRun()
		var data []byte
		if firstRun {
			data = []byte("true")
		} else {
			data = []byte("false")
		}
		if err := b.Put([]byte(RunCycleStatusKey), data); err != nil {
			return fmt.Errorf("failed to save run cycle status: %v", err)
		}

		return nil
	})
}

// SaveWebhookConfig saves a webhook configuration to BoltDB
func (ba *BoltAdapter) SaveWebhookConfig(config *domain.WebhookConfig) error {
	if ba.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	if config == nil {
		return fmt.Errorf("webhook config cannot be nil")
	}

	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(WebhookConfigBucket))

		if b == nil {
			return fmt.Errorf("bucket %s not found", WebhookConfigBucket)
		}

		// Marshal the config to JSON
		jsonData, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal webhook config: %v", err)
		}

		// Save Config with the ID as key
		if err := b.Put([]byte(config.ID), jsonData); err != nil {
			return fmt.Errorf("failed to save webhook config: %v", err)
		}

		// If this is the default webhook, update default key
		if config.ID == "default" {
			if err := b.Put([]byte(DefaultWebhookKey), []byte(config.ID)); err != nil {
				return fmt.Errorf("failed to set default webhook: %v", err)
			}
		}

		return nil
	})
}

// GetWebhookConfig retrieves the webhook configuration from BoltDB (uses default ID)
func (ba *BoltAdapter) GetWebhookConfig() (*domain.WebhookConfig, error) {
	if ba.db == nil {
		return nil, fmt.Errorf("boltAdapter not initialized")
	}

	var config *domain.WebhookConfig

	err := ba.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(WebhookConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", WebhookConfigBucket)
		}

		// Use default webhook ID
		configData := b.Get([]byte(domain.DefaultWebhookID))
		if configData == nil {
			return fmt.Errorf("webhook config not found")
		}

		return json.Unmarshal(configData, &config)
	})
	if err != nil {
		return nil, err
	}

	return config, nil
}

// DeleteWebhookConfig deletes a webhook configuration from BoltDB
func (ba *BoltAdapter) DeleteWebhookConfig(id string) error {
	if ba.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(WebhookConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", WebhookConfigBucket)
		}

		return b.Delete([]byte(id))
	})
}
