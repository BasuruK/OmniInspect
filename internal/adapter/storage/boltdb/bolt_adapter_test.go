package boltdb

import (
	"errors"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestBoltAdapter_HasEncryptedCredentials(t *testing.T) {
	t.Parallel()

	// Case 1: legacy plaintext only -> false, nil
	t.Run("plaintext only", func(t *testing.T) {
		t.Parallel()
		adapter := newTestBoltAdapter(t)

		err := adapter.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(DatabaseConfigBucket))
			if b == nil {
				return errors.New("bucket not found")
			}
			return b.Put([]byte("DBconfig:test_plain"), []byte(`{"password": "plain_password"}`))
		})
		if err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}

		hasEncrypted, err := adapter.HasEncryptedCredentials()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasEncrypted {
			t.Fatalf("expected hasEncrypted to be false, got true")
		}
	})

	// Case 2: plaintext + encrypted -> true, nil
	t.Run("plaintext plus encrypted", func(t *testing.T) {
		t.Parallel()
		adapter := newTestBoltAdapter(t)

		err := adapter.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(DatabaseConfigBucket))
			if b == nil {
				return errors.New("bucket not found")
			}
			if err := b.Put([]byte("DBconfig:test_plain"), []byte(`{"password": "plain_password"}`)); err != nil {
				return err
			}
			return b.Put([]byte("DBconfig:test_enc"), []byte(`{"password": "enc:v1:encrypted_password"}`))
		})
		if err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}

		hasEncrypted, err := adapter.HasEncryptedCredentials()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasEncrypted {
			t.Fatalf("expected hasEncrypted to be true, got false")
		}
	})

	// Case 3: corrupt path / missing and unreadable -> false, error
	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()
		// Using a path that cannot be written/opened because it lies in a non-existent parent directory
		invalidPath := filepath.Join(t.TempDir(), "nonexistent_dir", "test.bolt")
		adapter := &BoltAdapter{dbPath: invalidPath}

		hasEncrypted, err := adapter.HasEncryptedCredentials()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if hasEncrypted {
			t.Fatalf("expected hasEncrypted to be false on error, got true")
		}
	})
}
