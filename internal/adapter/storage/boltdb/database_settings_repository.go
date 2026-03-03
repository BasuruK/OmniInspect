package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

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
		return fmt.Errorf("boltAdapter not initialized")
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

		key := settings.Username() + ":" + settings.Database()
		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save database settings: %w", err)
		}

		// Persist default pointer key when this settings object is marked as default.
		if settings.IsDefault() {
			if err := b.Put([]byte(DefaultDatabaseConfigKey), []byte(key)); err != nil {
				return fmt.Errorf("failed to save default database settings key: %w", err)
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
		return nil, fmt.Errorf("boltAdapter not initialized")
	}

	var settings *domain.DatabaseSettings
	err := dsr.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		data := b.Get([]byte(id))
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
		return nil, fmt.Errorf("boltAdapter not initialized")
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
			return fmt.Errorf("default database settings not found")
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

// Delete removes database settings by ID
func (dsr *DatabaseSettingsRepository) Delete(ctx context.Context, id string) error {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate adapter before accessing the database
	if dsr == nil || dsr.adapter == nil || dsr.adapter.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	return dsr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", DatabaseConfigBucket)
		}

		if err := b.Delete([]byte(id)); err != nil {
			return err
		}

		// If the deleted config was the default, clear the default pointer key
		defaultKey := b.Get([]byte(DefaultDatabaseConfigKey))
		if defaultKey != nil && string(defaultKey) == id {
			if err := b.Delete([]byte(DefaultDatabaseConfigKey)); err != nil {
				return fmt.Errorf("failed to clear default database settings key: %w", err)
			}
		}
		return nil
	})
}
