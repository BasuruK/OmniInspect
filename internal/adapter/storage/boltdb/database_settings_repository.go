package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	bolt "go.etcd.io/bbolt"
)

var ErrAdapterNotInitialized = errors.New("boltAdapter not initialized")

// DatabaseSettingsRepository implements ports.DatabaseSettingsRepository
type DatabaseSettingsRepository struct {
	adapter *BoltAdapter
}

// NewDatabaseSettingsRepository creates a new DatabaseSettingsRepository
func NewDatabaseSettingsRepository(adapter *BoltAdapter) *DatabaseSettingsRepository {
	return &DatabaseSettingsRepository{
		adapter: adapter,
	}
}

// Save stores database settings
func (dsr *DatabaseSettingsRepository) Save(ctx context.Context, settings domain.DatabaseSettings) error {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate adapter before accessing the database
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return ErrAdapterNotInitialized
	}

	return dsr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		jsonData, err := json.Marshal(&settings)
		if err != nil {
			return fmt.Errorf("failed to marshal database settings: %w", err)
		}

		key := settings.StorageKey()
		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save database settings: %w", err)
		}

		// Persist default pointer key when this settings object is marked as default.
		if settings.IsDefault() {
			if err := b.Put([]byte(DefaultDatabaseConfigKey), []byte(key)); err != nil {
				return fmt.Errorf("failed to save default database settings key: %w", err)
			}
		} else {
			// Clear default pointer if this key was previously the default
			currentDefault := b.Get([]byte(DefaultDatabaseConfigKey))
			if currentDefault != nil && string(currentDefault) == key {
				if err := b.Delete([]byte(DefaultDatabaseConfigKey)); err != nil {
					return fmt.Errorf("failed to clear default database settings key: %w", err)
				}
			}
		}
		return nil
	})
}

// GetByID retrieves database settings by ID
func (dsr *DatabaseSettingsRepository) GetByID(ctx context.Context, id string) (*domain.DatabaseSettings, error) {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate adapter before accessing the database
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return nil, ErrAdapterNotInitialized
	}

	var settings *domain.DatabaseSettings
	err := dsr.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		data := b.Get([]byte(databaseSettingsStorageKey(id)))
		if data == nil {
			return fmt.Errorf("database settings not found for id: %s", id)
		}

		return json.Unmarshal(data, &settings)
	})
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// GetDefault retrieves the default database settings
func (dsr *DatabaseSettingsRepository) GetDefault(ctx context.Context) (*domain.DatabaseSettings, error) {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate adapter before accessing the database
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return nil, ErrAdapterNotInitialized
	}

	var settings *domain.DatabaseSettings
	err := dsr.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		// Get the default key
		defaultKey := b.Get([]byte(DefaultDatabaseConfigKey))
		if defaultKey == nil {
			return domain.ErrDefaultSettingsNotFound
		}

		data := b.Get(defaultKey)
		if data == nil {
			return fmt.Errorf("database settings not found for key: %s", string(defaultKey))
		}

		return json.Unmarshal(data, &settings)
	})
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// GetAll retrieves all stored database settings
func (dsr *DatabaseSettingsRepository) GetAll(ctx context.Context) ([]domain.DatabaseSettings, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return nil, ErrAdapterNotInitialized
	}

	var results []domain.DatabaseSettings
	err := dsr.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		return b.ForEach(func(k, v []byte) error {
			key := string(k)
			if key == DefaultDatabaseConfigKey {
				return nil // skip the default pointer key
			}
			var settings domain.DatabaseSettings
			if err := json.Unmarshal(v, &settings); err != nil {
				log.Printf("[DatabaseSettings] Warning: failed to unmarshal database settings entry with key %q: %v", key, err)
				return nil // skip malformed entries
			}
			results = append(results, settings)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// Delete removes database settings by ID
func (dsr *DatabaseSettingsRepository) Delete(ctx context.Context, id string) error {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate adapter before accessing the database
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return ErrAdapterNotInitialized
	}

	return dsr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		storageKey := databaseSettingsStorageKey(id)
		if err := b.Delete([]byte(storageKey)); err != nil {
			return err
		}

		// If the deleted config was the default, clear the default pointer key
		defaultKey := b.Get([]byte(DefaultDatabaseConfigKey))
		if defaultKey != nil && string(defaultKey) == storageKey {
			if err := b.Delete([]byte(DefaultDatabaseConfigKey)); err != nil {
				return fmt.Errorf("failed to clear default database settings key: %w", err)
			}
		}
		return nil
	})
}

// databaseSettingsStorageKey converts a user-facing database ID to a storage key.
// It is idempotent: if the input is already a valid escaped storage key, it is returned unchanged.
func databaseSettingsStorageKey(id string) string {
	// Check if input appears to be an already-escaped storage key
	if strings.HasPrefix(id, "cfg:") {
		rawPart := strings.TrimPrefix(id, "cfg:")
		unescaped, err := url.PathUnescape(rawPart)
		if err == nil && url.PathEscape(unescaped) == rawPart {
			// Already a valid storage key (properly escaped)
			return id
		}
	}

	// Treat the entire input as a raw ID that needs escaping
	return "cfg:" + url.PathEscape(id)
}
