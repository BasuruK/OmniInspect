package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const (
	SubscriberBucket = "Subscribers"
)

// SubscriberRepository implements ports.SubscriberRepository
type SubscriberRepository struct {
	adapter *BoltAdapter
}

// NewSubscriberRepository creates a new SubscriberRepository
func NewSubscriberRepository(adapter *BoltAdapter) *SubscriberRepository {
	return &SubscriberRepository{
		adapter: adapter,
	}
}

// Save stores a subscriber
func (r *SubscriberRepository) Save(ctx context.Context, subscriber domain.Subscriber) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate adapter before accessing the database
	if r == nil || r.adapter == nil || r.adapter.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	return r.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SubscriberBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", SubscriberBucket)
		}

		jsonData, err := json.Marshal(&subscriber)
		if err != nil {
			return fmt.Errorf("failed to marshal subscriber: %w", err)
		}

		key := subscriber.Name()
		if err := b.Put([]byte(key), jsonData); err != nil {
			return fmt.Errorf("failed to save subscriber: %w", err)
		}

		return nil
	})
}

// GetByName retrieves a subscriber by name
func (r *SubscriberRepository) GetByName(ctx context.Context, name string) (*domain.Subscriber, error) {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate adapter before accessing the database
	if r == nil || r.adapter == nil || r.adapter.db == nil {
		return nil, fmt.Errorf("boltAdapter not initialized")
	}

	var subscriber domain.Subscriber
	err := r.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SubscriberBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", SubscriberBucket)
		}

		data := b.Get([]byte(name))
		if data == nil {
			return domain.ErrSubscriberNotFound
		}

		return json.Unmarshal(data, &subscriber)
	})
	if err != nil {
		return nil, err
	}

	return &subscriber, nil
}

// Exists checks if a subscriber exists
func (r *SubscriberRepository) Exists(ctx context.Context, name string) (bool, error) {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// Validate adapter before accessing the database
	if r == nil || r.adapter == nil || r.adapter.db == nil {
		return false, fmt.Errorf("boltAdapter not initialized")
	}

	var exists bool
	err := r.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SubscriberBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", SubscriberBucket)
		}

		exists = b.Get([]byte(name)) != nil
		return nil
	})
	return exists, err
}

// Delete removes a subscriber
func (r *SubscriberRepository) Delete(ctx context.Context, name string) error {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate adapter before accessing the database
	if r == nil || r.adapter == nil || r.adapter.db == nil {
		return fmt.Errorf("boltAdapter not initialized")
	}

	return r.adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SubscriberBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", SubscriberBucket)
		}

		return b.Delete([]byte(name))
	})
}

// List returns all subscribers
func (r *SubscriberRepository) List(ctx context.Context) ([]domain.Subscriber, error) {
	// Check for context cancellation before proceeding
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate adapter before accessing the database
	if r == nil || r.adapter == nil || r.adapter.db == nil {
		return nil, fmt.Errorf("boltAdapter not initialized")
	}

	var subscribers []domain.Subscriber
	err := r.adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SubscriberBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", SubscriberBucket)
		}

		cursor := b.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var subscriber domain.Subscriber
			if err := json.Unmarshal(v, &subscriber); err != nil {
				return fmt.Errorf("failed to unmarshal subscriber: %w", err)
			}
			subscribers = append(subscribers, subscriber)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return subscribers, nil
}
