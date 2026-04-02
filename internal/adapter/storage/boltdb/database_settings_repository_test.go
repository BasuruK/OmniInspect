package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"fmt"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// TestDatabaseSettingsStorageKey_HandlesRawAndStorageKeys verifies that raw IDs
// are escaped into storage keys while already-escaped storage keys are left unchanged.
func TestDatabaseSettingsStorageKey_HandlesRawAndStorageKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "raw id is escaped and prefixed",
			id:   "OPS/PRIMARY east",
			want: "cfg:OPS%2FPRIMARY%20east",
		},
		{
			name: "already escaped storage key is returned unchanged",
			id:   "cfg:OPS%2FPRIMARY%20east",
			want: "cfg:OPS%2FPRIMARY%20east",
		},
		{
			name: "raw id with literal cfg prefix is escaped as part of the raw value",
			id:   "cfg:team/database",
			want: "cfg:cfg:team%2Fdatabase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := databaseSettingsStorageKey(tt.id)
			if got != tt.want {
				t.Fatalf("databaseSettingsStorageKey(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// TestDatabaseSettingsRepository_GetByID_AcceptsEscapedStorageKey verifies that
// repository lookups accept the escaped storage key directly.
func TestDatabaseSettingsRepository_GetByID_AcceptsEscapedStorageKey(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)
	repo := NewDatabaseSettingsRepository(adapter)

	port, err := domain.NewPort(1521)
	if err != nil {
		t.Fatalf("NewPort: %v", err)
	}

	settings, err := domain.NewDatabaseSettings("OPS/PRIMARY east", "OMNI", "localhost", port, "system", "secret")
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}

	if err := repo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetByID(context.Background(), settings.StorageKey())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.StorageKey() != settings.StorageKey() {
		t.Fatalf("StorageKey() = %q, want %q", got.StorageKey(), settings.StorageKey())
	}
	if got.ID() != settings.ID() {
		t.Fatalf("ID() = %q, want %q", got.ID(), settings.ID())
	}
}

// TestDatabaseSettingsRepository_GetByID_RejectsUnescapedPrefixedRawID verifies that
// a malformed prefixed raw ID does not alias to the escaped storage key.
func TestDatabaseSettingsRepository_GetByID_RejectsUnescapedPrefixedRawID(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)
	repo := NewDatabaseSettingsRepository(adapter)

	port, err := domain.NewPort(1521)
	if err != nil {
		t.Fatalf("NewPort: %v", err)
	}

	settings, err := domain.NewDatabaseSettings("OPS/PRIMARY east", "OMNI", "localhost", port, "system", "secret")
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}

	if err := repo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err = repo.GetByID(context.Background(), "cfg:OPS/PRIMARY east")
	if err == nil {
		t.Fatal("expected GetByID to reject an unescaped prefixed raw ID")
	}
}

func TestDatabaseSettingsRepository_Save_PersistsPermissionValidationMarker(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)
	repo := NewDatabaseSettingsRepository(adapter)

	port, err := domain.NewPort(1521)
	if err != nil {
		t.Fatalf("NewPort: %v", err)
	}

	settings, err := domain.NewDatabaseSettings("OPS-PRIMARY", "OMNI", "localhost", port, "system", "secret")
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	settings.MarkPermissionsValidated()

	if err := repo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetByID(context.Background(), settings.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if !got.PermissionsValidated() {
		t.Fatal("expected persisted settings to retain the permission validation marker")
	}
}

// ==========================================
// Migration Tests
// ==========================================

// writeLegacyEntry writes a raw JSON payload under a legacy (non-"cfg:") key directly
// into the DatabaseConfigBucket, simulating data written by an older version of the app.
func writeLegacyEntry(t *testing.T, adapter *BoltAdapter, legacyKey string, rawJSON []byte) {
	t.Helper()
	err := adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put([]byte(legacyKey), rawJSON)
	})
	if err != nil {
		t.Fatalf("writeLegacyEntry: %v", err)
	}
}

// setDefaultPointer sets the DefaultDatabaseConfigKey to point at the given key,
// simulating the default pointer written by an older version of the app.
func setDefaultPointer(t *testing.T, adapter *BoltAdapter, key string) {
	t.Helper()
	err := adapter.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put([]byte(DefaultDatabaseConfigKey), []byte(key))
	})
	if err != nil {
		t.Fatalf("setDefaultPointer: %v", err)
	}
}

// readBucketKey reads the raw bytes for a key from the DatabaseConfigBucket.
// Returns nil if the key is absent.
func readBucketKey(t *testing.T, adapter *BoltAdapter, key string) []byte {
	t.Helper()
	var value []byte
	err := adapter.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DatabaseConfigBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		v := b.Get([]byte(key))
		if v != nil {
			value = make([]byte, len(v))
			copy(value, v)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("readBucketKey: %v", err)
	}
	return value
}

// TestMigrateLegacyDatabaseSettings_RekeysLegacyEntry verifies that a legacy entry
// (stored under "host:username") is moved to the "cfg:"-prefixed key scheme.
func TestMigrateLegacyDatabaseSettings_RekeysLegacyEntry(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)

	legacyKey := "localhost:system"
	legacyJSON := []byte(`{"database":"ORCL","host":"localhost","port":1521,"username":"system","password":"secret","isDefault":false}`)
	writeLegacyEntry(t, adapter, legacyKey, legacyJSON)

	if err := adapter.migrateLegacyDatabaseSettings(); err != nil {
		t.Fatalf("migrateLegacyDatabaseSettings: %v", err)
	}

	// Old key must be gone.
	if got := readBucketKey(t, adapter, legacyKey); got != nil {
		t.Errorf("expected legacy key %q to be removed after migration, but it still exists", legacyKey)
	}

	// New key must exist. Note: url.PathEscape does not encode ':' (valid path char),
	// so "localhost:system" → "cfg:localhost:system".
	newKey := "cfg:localhost%20ORCL%20system"
	newData := readBucketKey(t, adapter, newKey)
	if newData == nil {
		t.Fatalf("expected new key %q to exist after migration", newKey)
	}

	// The migrated record must be readable as DatabaseSettings with the dummy ID.
	repo := NewDatabaseSettingsRepository(adapter)
	settings, err := repo.GetByID(context.Background(), "cfg:localhost%20ORCL%20system")
	if err != nil {
		t.Fatalf("GetByID after migration: %v", err)
	}
	if settings.DatabaseID() != "localhost ORCL system" {
		t.Errorf("expected DatabaseID() = %q, got %q", "localhost ORCL system", settings.DatabaseID())
	}
}

// TestMigrateLegacyDatabaseSettings_UpdatesDefaultPointer verifies that when the
// default pointer referred to a legacy key, it is updated to the new "cfg:" key.
func TestMigrateLegacyDatabaseSettings_UpdatesDefaultPointer(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)

	legacyKey := "db-host:admin"
	legacyJSON := []byte(`{"database":"FREEDB","host":"db-host","port":1521,"username":"admin","password":"pass","isDefault":true}`)
	writeLegacyEntry(t, adapter, legacyKey, legacyJSON)
	setDefaultPointer(t, adapter, legacyKey)

	if err := adapter.migrateLegacyDatabaseSettings(); err != nil {
		t.Fatalf("migrateLegacyDatabaseSettings: %v", err)
	}

	// ':' is not encoded by url.PathEscape (valid path segment char).
	newKey := "cfg:db-host%20FREEDB%20admin"
	defaultPtr := readBucketKey(t, adapter, DefaultDatabaseConfigKey)
	if string(defaultPtr) != newKey {
		t.Errorf("expected default pointer to be updated to %q, got %q", newKey, string(defaultPtr))
	}
}

// TestMigrateLegacyDatabaseSettings_LeavesNewEntriesUntouched verifies that entries
// already using the "cfg:" scheme are not modified.
func TestMigrateLegacyDatabaseSettings_LeavesNewEntriesUntouched(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)
	repo := NewDatabaseSettingsRepository(adapter)

	port, err := domain.NewPort(1521)
	if err != nil {
		t.Fatalf("NewPort: %v", err)
	}
	settings, err := domain.NewDatabaseSettings("MY-DB", "ORCL", "localhost", port, "user", "pass")
	if err != nil {
		t.Fatalf("NewDatabaseSettings: %v", err)
	}
	if err := repo.Save(context.Background(), *settings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Running migration again must be a no-op.
	if err := adapter.migrateLegacyDatabaseSettings(); err != nil {
		t.Fatalf("migrateLegacyDatabaseSettings (second pass): %v", err)
	}

	// Entry must still be readable under the same key.
	got, err := repo.GetByID(context.Background(), settings.ID())
	if err != nil {
		t.Fatalf("GetByID after no-op migration: %v", err)
	}
	if got.ID() != settings.ID() {
		t.Errorf("ID() changed: got %q, want %q", got.ID(), settings.ID())
	}
}

// TestMigrateLegacyDatabaseSettings_PreservesExistingDatabaseId verifies that migration
// keeps an explicit databaseId and aligns the rewritten Bolt key to that same ID.
func TestMigrateLegacyDatabaseSettings_PreservesExistingDatabaseId(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)

	legacyKey := "some-host:user"
	legacyJSON := []byte(`{"databaseId":"MY-EXPLICIT-ID","database":"ORCL","host":"some-host","port":1521,"username":"user","password":"pass","isDefault":false}`)
	writeLegacyEntry(t, adapter, legacyKey, legacyJSON)

	if err := adapter.migrateLegacyDatabaseSettings(); err != nil {
		t.Fatalf("migrateLegacyDatabaseSettings: %v", err)
	}

	// Old key is gone.
	if got := readBucketKey(t, adapter, legacyKey); got != nil {
		t.Errorf("expected legacy key %q to be removed", legacyKey)
	}

	newKey := "cfg:MY-EXPLICIT-ID"
	if got := readBucketKey(t, adapter, newKey); got == nil {
		t.Fatalf("expected new key %q to exist after migration", newKey)
	}

	repo := NewDatabaseSettingsRepository(adapter)
	settings, err := repo.GetByID(context.Background(), "MY-EXPLICIT-ID")
	if err != nil {
		t.Fatalf("GetByID after migration: %v", err)
	}
	if settings.DatabaseID() != "MY-EXPLICIT-ID" {
		t.Errorf("DatabaseID() = %q, want %q", settings.DatabaseID(), "MY-EXPLICIT-ID")
	}
}

// TestMigrateLegacyDatabaseSettings_GeneratesNewDatabaseId verifies that migration
// backfills databaseId from connection fields when the legacy JSON does not include one.
func TestMigrateLegacyDatabaseSettings_GeneratesNewDatabaseId(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)

	legacyKey := "some-host:user"
	legacyJSON := []byte(`{"database":"ORCL","host":"some-host","port":1521,"username":"user","password":"pass","isDefault":false}`)
	writeLegacyEntry(t, adapter, legacyKey, legacyJSON)

	if err := adapter.migrateLegacyDatabaseSettings(); err != nil {
		t.Fatalf("migrateLegacyDatabaseSettings: %v", err)
	}

	newKey := "cfg:some-host%20ORCL%20user"
	if got := readBucketKey(t, adapter, newKey); got == nil {
		t.Fatalf("expected new key %q to exist after migration", newKey)
	}

	repo := NewDatabaseSettingsRepository(adapter)
	settings, err := repo.GetByID(context.Background(), "some-host ORCL user")
	if err != nil {
		t.Fatalf("GetByID after migration: %v", err)
	}
	if settings.DatabaseID() != "some-host ORCL user" {
		t.Errorf("DatabaseID() = %q, want %q", settings.DatabaseID(), "some-host ORCL user")
	}
}

// TestMigrateLegacyDatabaseSettings_IsIdempotent verifies that running the migration
// twice produces the same result as running it once.
func TestMigrateLegacyDatabaseSettings_IsIdempotent(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)

	legacyKey := "idempotent-host:sa"
	legacyJSON := []byte(`{"database":"TESTDB","host":"idempotent-host","port":1521,"username":"sa","password":"pw","isDefault":false}`)
	writeLegacyEntry(t, adapter, legacyKey, legacyJSON)

	for i := range 2 {
		if err := adapter.migrateLegacyDatabaseSettings(); err != nil {
			t.Fatalf("migrateLegacyDatabaseSettings (pass %d): %v", i+1, err)
		}
	}

	// ':' is not encoded by url.PathEscape (valid path segment char).
	newKey := "cfg:idempotent-host%20TESTDB%20sa"
	if got := readBucketKey(t, adapter, newKey); got == nil {
		t.Fatalf("expected new key %q to exist after idempotent migration", newKey)
	}
	if got := readBucketKey(t, adapter, legacyKey); got != nil {
		t.Errorf("expected legacy key %q to remain absent after second migration pass", legacyKey)
	}
}

// newTestBoltAdapter creates an isolated BoltDB adapter with the database
// settings bucket initialized for repository tests.
func newTestBoltAdapter(t *testing.T) *BoltAdapter {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.bolt")
	adapter := &BoltAdapter{dbPath: dbPath}
	if err := adapter.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	t.Cleanup(func() {
		if err := adapter.db.Close(); err != nil {
			// errorf here instead of fatalf to allow cleanup to continue and remove temp files
			t.Errorf("db.Close: %v", err)
		}
	})

	return adapter
}
