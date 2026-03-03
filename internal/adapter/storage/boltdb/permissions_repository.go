package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
func (pr *PermissionsRepository) Save(ctx context.Context, perms *domain.DatabasePermissions) error {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate adapter and permissions before accessing the database
	if pr == nil || pr.adapter == nil || pr.adapter.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}
	// Validate permissions object
	if perms == nil {
		return fmt.Errorf("permissions cannot be nil")
	}
	// Validate schema name
	key := strings.TrimSpace(perms.Schema())
	if key == "" {
		return fmt.Errorf("schema cannot be empty")
	}
	// Save the permissions to BoltDB
	return pr.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PermissionsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", PermissionsBucket)
		}

		jsonData, err := json.Marshal(perms)
		if err != nil {
			return fmt.Errorf("failed to marshal permissions: %w", err)
		}

		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save permissions: %w", err)
		}

		return nil
	})
}

// Get retrieves database permissions for a schema
func (pr *PermissionsRepository) Get(ctx context.Context, schema string) (*domain.DatabasePermissions, error) {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate adapter before accessing the database
	if pr == nil || pr.adapter == nil || pr.adapter.db == nil {
		return nil, fmt.Errorf("boltAdapter not initialized")
	}
	// Validate schema name
	key := strings.TrimSpace(schema)
	if key == "" {
		return nil, fmt.Errorf("schema cannot be empty")
	}

	var perms domain.DatabasePermissions
	err := pr.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PermissionsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", PermissionsBucket)
		}

		data := b.Get([]byte(key))
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
func (pr *PermissionsRepository) Exists(ctx context.Context, schema string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	if pr == nil || pr.adapter == nil || pr.adapter.db == nil {
		return false, fmt.Errorf("boltAdapter not initialized")
	}

	key := strings.TrimSpace(schema)
	if key == "" {
		return false, fmt.Errorf("schema cannot be empty")
	}

	var exists bool
	err := pr.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PermissionsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", PermissionsBucket)
		}

		exists = b.Get([]byte(key)) != nil
		return nil
	})
	return exists, err
}
