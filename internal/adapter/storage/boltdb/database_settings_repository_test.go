package boltdb

import (
	"OmniView/internal/core/domain"
	"context"
	"path/filepath"
	"testing"
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
