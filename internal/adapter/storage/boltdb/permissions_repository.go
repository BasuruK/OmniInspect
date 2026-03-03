package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const (
	PermissionsBucket = "Permissions"
)

// PermissionsRepository implements ports.PermissionsRepository
type PermissionsRepository struct {
	adapter *BoltAdapter
}

// NewPermissionsRepository creates a new PermissionsRepository
func NewPermissionsRepository(adapter *BoltAdapter) *PermissionsRepository {
	return &PermissionsRepository{
		adapter: adapter,
	}
}

// Save stores database permissions for a schema
func (r *PermissionsRepository) Save(ctx context.Context, perms *domain.DatabasePermissions) error {
	if r.adapter.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	return r.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PermissionsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", PermissionsBucket)
		}

		jsonData, err := json.Marshal(perms)
		if err != nil {
			return fmt.Errorf("failed to marshal permissions: %w", err)
		}

		key := perms.Schema()
		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save permissions: %w", err)
		}

		return nil
	})
}

// Get retrieves database permissions for a schema
func (r *PermissionsRepository) Get(ctx context.Context, schema string) (*domain.DatabasePermissions, error) {
	if r.adapter.db == nil {
		return nil, fmt.Errorf("boltAdapter not initialized")
	}

	var perms domain.DatabasePermissions
	err := r.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PermissionsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", PermissionsBucket)
		}

		data := b.Get([]byte(schema))
		if data == nil {
			return domain.ErrMissingPermissions
		}

		return json.Unmarshal(data, &perms)
	})
	if err != nil {
		return nil, err
	}

	return &perms, nil
}

// Exists checks if permissions exist for a schema
func (r *PermissionsRepository) Exists(ctx context.Context, schema string) (bool, error) {
	if r.adapter.db == nil {
		return false, fmt.Errorf("boltAdapter not initialized")
	}

	var exists bool
	err := r.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PermissionsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", PermissionsBucket)
		}

		exists = b.Get([]byte(schema)) != nil
		return nil
	})
	return exists, err
}
