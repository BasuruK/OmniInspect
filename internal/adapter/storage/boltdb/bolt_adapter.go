package boltdb

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	// Buckets
	DatabaseConfigBucket = "DatabaseConfigurations"
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
func NewBoltAdapter(dbPath string) (*BoltAdapter, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("NewBoltAdapter: dbPath cannot be empty")
	}
	return &BoltAdapter{
		dbPath: dbPath,
	}, nil
}

func (ba *BoltAdapter) Initialize() error {
	if ba.db != nil {
		return fmt.Errorf("boltAdapter already initialized")
	}

	var err error
	if ba.db, err = bolt.Open(ba.dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second}); err != nil {
		return fmt.Errorf("failed to open BoltDB: %w", err)
	}

	// Initialize buckets
	if err := ba.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(DatabaseConfigBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(ClientConfigBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(SubscriberBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(PermissionsBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(WebhookConfigBucket)); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	// Migrate any legacy database settings (pre-ID-column records) to the new key scheme.
	return ba.migrateLegacyDatabaseSettings()
}

// migrateLegacyDatabaseSettings rewrites database config entries stored under the
// old "cfg:<host>:<database>" key scheme to the new "cfg:<databaseID>" scheme.
// Since the user-provided databaseID is unknown during migration, a dummy ID is
// generated in the format "cfg:<host> <database> <username>" which users can edit later.
// This is a one-time idempotent migration; properly escaped keys are left untouched.
func (ba *BoltAdapter) migrateLegacyDatabaseSettings() error {
	return ba.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return nil // bucket doesn't exist yet — nothing to migrate
		}

		// Collect legacy entries in a separate pass so we can safely mutate inside the transaction.
		type legacyEntry struct {
			oldKey  string
			rawJSON []byte
		}
		var toMigrate []legacyEntry

		if err := b.ForEach(func(k, v []byte) error {
			key := string(k)
			if key == DefaultDatabaseConfigKey {
				return nil // skip the default pointer entry
			}

			// A properly migrated key has properly escaped values.
			// Old keys like "cfg:localhost:ORCL" have unescaped colons.
			// New keys would have colons escaped as %3A, or no colons if databaseID has none.
			if isNewStyleStorageKey(key) {
				return nil // already on the new scheme
			}

			// Copy value so we own it outside the iteration.
			copied := make([]byte, len(v))
			copy(copied, v)
			toMigrate = append(toMigrate, legacyEntry{oldKey: key, rawJSON: copied})
			return nil
		}); err != nil {
			return fmt.Errorf("migrateLegacyDatabaseSettings: scan failed: %w", err)
		}

		if len(toMigrate) == 0 {
			return nil
		}

		// Read the current default pointer so we can update it if needed.
		defaultPtr := string(b.Get([]byte(DefaultDatabaseConfigKey)))

		for _, entry := range toMigrate {
			// Parse the raw JSON to extract fields for dummy ID generation.
			var rawMap map[string]interface{}
			if err := json.Unmarshal(entry.rawJSON, &rawMap); err != nil {
				log.Printf("[BoltDB] Migration: skipping entry %q — failed to parse JSON: %v", entry.oldKey, err)
				continue
			}

			// Generate databaseId from JSON fields (host:database:username format).
			// This is the new standard - user can reconfigure later.
			var databaseId string
			parts := []string{}
			if host, ok := rawMap["host"].(string); ok && host != "" {
				parts = append(parts, host)
			}
			if db, ok := rawMap["database"].(string); ok && db != "" {
				parts = append(parts, db)
			}
			if username, ok := rawMap["username"].(string); ok && username != "" {
				parts = append(parts, username)
			}
			if len(parts) > 0 {
				databaseId = strings.Join(parts, " ")
			}
			if databaseId == "" {
				databaseId = entry.oldKey
			}
			rawMap["databaseId"] = databaseId

			newJSON, err := json.Marshal(rawMap)
			if err != nil {
				log.Printf("[BoltDB] Migration: skipping entry %q — failed to re-marshal JSON: %v", entry.oldKey, err)
				continue
			}

			// New key format: cfg:<dummyID> where dummyID uses spaces (no special chars needing escaping)
			newKey := "cfg:" + url.PathEscape(databaseId)

			if err := b.Put([]byte(newKey), newJSON); err != nil {
				return fmt.Errorf("migrateLegacyDatabaseSettings: write new key %q: %w", newKey, err)
			}
			if err := b.Delete([]byte(entry.oldKey)); err != nil {
				return fmt.Errorf("migrateLegacyDatabaseSettings: delete old key %q: %w", entry.oldKey, err)
			}

			// Update the default pointer if it was pointing at the old key.
			if defaultPtr == entry.oldKey {
				if err := b.Put([]byte(DefaultDatabaseConfigKey), []byte(newKey)); err != nil {
					return fmt.Errorf("migrateLegacyDatabaseSettings: update default pointer: %w", err)
				}
				defaultPtr = newKey
			}

			log.Printf("[BoltDB] Migration: re-keyed database config %q → %q", entry.oldKey, newKey)
		}

		return nil
	})
}

// isNewStyleStorageKey returns true if the key is a properly migrated storage key.
// A properly migrated key can be reconstructed by unescaping and re-escaping: 
// if makeSettingsID(url.PathUnescape(raw)) == key, then the key is new-style.
// Old-style keys like "cfg:localhost:ORCL" don't match because re-escaping 
// "localhost:ORCL" gives "cfg:localhost%3AORCL", not the original key.
func isNewStyleStorageKey(key string) bool {
	if !strings.HasPrefix(key, "cfg:") {
		return false
	}
	raw := strings.TrimPrefix(key, "cfg:")
	unescaped, err := url.PathUnescape(raw)
	if err != nil {
		return false
	}
	// Reconstruct the canonical storage key and compare to the original.
	canonicalKey := "cfg:" + url.PathEscape(unescaped)
	return canonicalKey == key
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

	key := config.StorageKey()

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
		if config.ID == domain.DefaultWebhookID {
			if err := b.Put([]byte(DefaultWebhookKey), []byte(config.ID)); err != nil {
				return fmt.Errorf("failed to set default webhook: %v", err)
			}
		}

		return nil
	})
}

// GetWebhookConfig retrieves the webhook configuration from BoltDB (uses default key)
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

		// First get the default key (which points to the actual config ID)
		defaultKey := b.Get([]byte(DefaultWebhookKey))
		configKey := string(defaultKey)
		if configKey == "" {
			// Fallback to default ID if no pointer exists
			configKey = domain.DefaultWebhookID
		}

		// Read the config using the resolved key
		configData := b.Get([]byte(configKey))
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

		// Check if DefaultWebhookKey points to this webhook and clear it
		defaultKey := b.Get([]byte(DefaultWebhookKey))
		if string(defaultKey) == id {
			if err := b.Delete([]byte(DefaultWebhookKey)); err != nil {
				return fmt.Errorf("failed to clear default webhook key: %v", err)
			}
		}

		return b.Delete([]byte(id))
	})
}
