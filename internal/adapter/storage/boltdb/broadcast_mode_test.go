package boltdb

import (
	"OmniView/internal/core/domain"
	"testing"
)

// TestBoltAdapter_GetBroadcastMode_ReturnsDefaultWhenKeyAbsent verifies that
// GetBroadcastMode returns BroadcastModeGlobal when no mode has been stored yet.
func TestBoltAdapter_GetBroadcastMode_ReturnsDefaultWhenKeyAbsent(t *testing.T) {
	t.Parallel()

	adapter := newTestBoltAdapter(t)

	got, err := adapter.GetBroadcastMode()
	if err != nil {
		t.Fatalf("GetBroadcastMode: %v", err)
	}
	if got != domain.BroadcastModeGlobal {
		t.Fatalf("GetBroadcastMode() = %v, want %v", got, domain.BroadcastModeGlobal)
	}
}

// TestBoltAdapter_SetAndGetBroadcastMode_RoundTrip verifies that each BroadcastMode
// value is correctly persisted in the ClientConfigurations bucket and retrieved unchanged.
func TestBoltAdapter_SetAndGetBroadcastMode_RoundTrip(t *testing.T) {
	t.Parallel()

	modes := []domain.BroadcastMode{
		domain.BroadcastModeGlobal,
		domain.BroadcastModeSubscriber,
		domain.BroadcastModeBroadcast,
	}

	for _, mode := range modes {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()

			adapter := newTestBoltAdapter(t)

			if err := adapter.SetBroadcastMode(mode); err != nil {
				t.Fatalf("SetBroadcastMode(%v): %v", mode, err)
			}

			got, err := adapter.GetBroadcastMode()
			if err != nil {
				t.Fatalf("GetBroadcastMode: %v", err)
			}
			if got != mode {
				t.Fatalf("GetBroadcastMode() = %v, want %v", got, mode)
			}
		})
	}
}

// TestBoltAdapter_GetBroadcastMode_UninitializedDB verifies that GetBroadcastMode
// returns an error when the BoltAdapter has not been initialized.
func TestBoltAdapter_GetBroadcastMode_UninitializedDB(t *testing.T) {
	t.Parallel()

	adapter := &BoltAdapter{dbPath: "unused.bolt"}

	if _, err := adapter.GetBroadcastMode(); err == nil {
		t.Fatal("expected error from GetBroadcastMode on uninitialized adapter, got nil")
	}
}

// TestBoltAdapter_SetBroadcastMode_UninitializedDB verifies that SetBroadcastMode
// returns an error when the BoltAdapter has not been initialized.
func TestBoltAdapter_SetBroadcastMode_UninitializedDB(t *testing.T) {
	t.Parallel()

	adapter := &BoltAdapter{dbPath: "unused.bolt"}

	if err := adapter.SetBroadcastMode(domain.BroadcastModeSubscriber); err == nil {
		t.Fatal("expected error from SetBroadcastMode on uninitialized adapter, got nil")
	}
}
