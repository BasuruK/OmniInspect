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

// SwitchDefault stores the previous and new default settings in a single transaction.
func (dsr *DatabaseSettingsRepository) SwitchDefault(ctx context.Context, previousDefault *domain.DatabaseSettings, newDefault domain.DatabaseSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return ErrAdapterNotInitialized
	}

	return dsr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		if previousDefault != nil && previousDefault.StorageKey() != newDefault.StorageKey() {
			previousJSON, err := json.Marshal(previousDefault)
			if err != nil {
				return fmt.Errorf("failed to marshal previous default database settings: %w", err)
			}
			if err := b.Put([]byte(previousDefault.StorageKey()), previousJSON); err != nil {
				return fmt.Errorf("failed to save previous default database settings: %w", err)
			}
		}

		newJSON, err := json.Marshal(&newDefault)
		if err != nil {
			return fmt.Errorf("failed to marshal new default database settings: %w", err)
		}
		if err := b.Put([]byte(newDefault.StorageKey()), newJSON); err != nil {
			return fmt.Errorf("failed to save new default database settings: %w", err)
		}
		if err := b.Put([]byte(DefaultDatabaseConfigKey), []byte(newDefault.StorageKey())); err != nil {
			return fmt.Errorf("failed to save default database settings key: %w", err)
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
				return nil
			}
			var settings domain.DatabaseSettings
			if err := json.Unmarshal(v, &settings); err != nil {
				// Log warning but continue iteration to return valid entries
				log.Printf("warning: failed to unmarshal database settings for key %q: %v", key, err)
				return nil
			}
			settings.SetPersistedKey(key)
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return ErrAdapterNotInitialized
	}

	storageKey := databaseSettingsStorageKey(id)

	return dsr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		if err := b.Delete([]byte(storageKey)); err != nil {
			return fmt.Errorf("delete database settings %q: %w", storageKey, err)
		}

		defaultKey := b.Get([]byte(DefaultDatabaseConfigKey))
		if defaultKey != nil && string(defaultKey) == storageKey {
			if err := b.Delete([]byte(DefaultDatabaseConfigKey)); err != nil {
				return fmt.Errorf("failed to clear default database settings key: %w", err)
			}
		}
		return nil
	})
}

// Replace atomically removes the record at oldKey and writes newRecord in a
// single BoltDB transaction. When oldKey and newRecord.StorageKey() are the
// same the delete is skipped and only the save is performed.
func (dsr *DatabaseSettingsRepository) Replace(ctx context.Context, oldKey string, newRecord domain.DatabaseSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return ErrAdapterNotInitialized
	}

	normalizedOld := databaseSettingsStorageKey(oldKey)
	newKey := newRecord.StorageKey()

	return dsr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		// Remove the old entry only when the key actually changed.
		if normalizedOld != newKey {
			if err := b.Delete([]byte(normalizedOld)); err != nil {
				return fmt.Errorf("replace: delete old key %q: %w", normalizedOld, err)
			}
			// Clear the default pointer if it was pointing at the old key.
			defaultKey := b.Get([]byte(DefaultDatabaseConfigKey))
			if defaultKey != nil && string(defaultKey) == normalizedOld {
				if err := b.Delete([]byte(DefaultDatabaseConfigKey)); err != nil {
					return fmt.Errorf("replace: clear default pointer: %w", err)
				}
			}
		}

		jsonData, err := json.Marshal(&newRecord)
		if err != nil {
			return fmt.Errorf("replace: marshal new record: %w", err)
		}
		if err := b.Put([]byte(newKey), jsonData); err != nil {
			return fmt.Errorf("replace: save new record %q: %w", newKey, err)
		}
		return nil
	})
}

// databaseSettingsStorageKey converts a user-facing database ID to a storage key.
// It always normalizes the input by prefixing DATABASE_CONFIG_KEY_PREFIX and applying
// url.PathEscape to the resolved database ID, rather than returning previously
// escaped keys verbatim. This produces a consistent storage key for both raw and
// previously-escaped inputs.
func databaseSettingsStorageKey(id string) string {
	unescapedID := id
	if strings.HasPrefix(id, DATABASE_CONFIG_KEY_PREFIX) {
		unescapedPart := strings.TrimPrefix(id, DATABASE_CONFIG_KEY_PREFIX)
		if unescaped, err := url.PathUnescape(unescapedPart); err == nil {
			unescapedID = unescaped
		} else {
			// Unescape failed; use the trimmed part as-is to avoid double-prefixing
			unescapedID = unescapedPart
		}
	} else if strings.HasPrefix(id, LEGACY_CONFIG_KEY_PREFIX) {
		unescapedPart := strings.TrimPrefix(id, LEGACY_CONFIG_KEY_PREFIX)
		if unescaped, err := url.PathUnescape(unescapedPart); err == nil {
			unescapedID = unescaped
		} else {
			// Unescape failed; use the trimmed part as-is to avoid double-prefixing
			unescapedID = unescapedPart
		}
	}

	return DATABASE_CONFIG_KEY_PREFIX + url.PathEscape(unescapedID)
}
