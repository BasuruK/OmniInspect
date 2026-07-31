package domain

import (
	"testing"
	"time"
)

// ==========================================
// NewBroadcastMode Tests
// ==========================================

func TestNewBroadcastMode_ParsesKnownModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  BroadcastMode
	}{
		{"Global", BroadcastModeGlobal},
		{"Subscriber", BroadcastModeSubscriber},
		{"Only Subscriber", BroadcastModeSubscriber},
		{"Broadcast", BroadcastModeBroadcast},
		{"Only Broadcast", BroadcastModeBroadcast},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := NewBroadcastMode(tt.input); got != tt.want {
				t.Fatalf("NewBroadcastMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewBroadcastMode_DefaultsToGlobalForUnknown(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "unknown", "global", "GLOBAL", "subscriber", "BROADCAST"} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if got := NewBroadcastMode(input); got != BroadcastModeGlobal {
				t.Fatalf("NewBroadcastMode(%q) = %v, want BroadcastModeGlobal", input, got)
			}
		})
	}
}

// ==========================================
// BroadcastMode.String Tests
// ==========================================

func TestBroadcastMode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode BroadcastMode
		want string
	}{
		{BroadcastModeGlobal, "Global"},
		{BroadcastModeSubscriber, "Only Subscriber"},
		{BroadcastModeBroadcast, "Only Broadcast"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.mode.String(); got != tt.want {
				t.Fatalf("BroadcastMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// ==========================================
// BroadcastMode.Next Tests
// ==========================================

func TestBroadcastMode_Next_CyclesGlobalSubscriberBroadcastGlobal(t *testing.T) {
	t.Parallel()

	cycle := []BroadcastMode{
		BroadcastModeGlobal,
		BroadcastModeSubscriber,
		BroadcastModeBroadcast,
		BroadcastModeGlobal, // wraps back
	}
	for i := 0; i < len(cycle)-1; i++ {
		got := cycle[i].Next()
		if got != cycle[i+1] {
			t.Fatalf("BroadcastMode(%d).Next() = %v, want %v", cycle[i], got, cycle[i+1])
		}
	}
}

// ==========================================
// BroadcastMode.Includes Tests
// ==========================================

func newIncludesTestMessage(t *testing.T, mode string) *QueueMessage {
	t.Helper()
	msg, err := NewQueueMessage("id-1", "proc", LogLevelInfo, "payload", time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewQueueMessage: %v", err)
	}
	msg.mode = mode
	return msg
}

func TestBroadcastMode_Includes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    BroadcastMode
		msgMode string
		want    bool
	}{
		{"global mode shows broadcast message", BroadcastModeGlobal, "Global", true},
		{"global mode shows subscriber message", BroadcastModeGlobal, "Only Subscriber", true},
		{"subscriber mode hides broadcast message", BroadcastModeSubscriber, "Global", false},
		{"subscriber mode shows subscriber message", BroadcastModeSubscriber, "Only Subscriber", true},
		{"broadcast mode shows broadcast message", BroadcastModeBroadcast, "Global", true},
		{"broadcast mode hides subscriber message", BroadcastModeBroadcast, "Only Subscriber", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.mode.Includes(newIncludesTestMessage(t, tt.msgMode)); got != tt.want {
				t.Fatalf("BroadcastMode(%d).Includes(mode=%q) = %v, want %v", tt.mode, tt.msgMode, got, tt.want)
			}
		})
	}
}

func TestBroadcastMode_Includes_NilMessage(t *testing.T) {
	t.Parallel()

	for _, mode := range []BroadcastMode{BroadcastModeGlobal, BroadcastModeSubscriber, BroadcastModeBroadcast} {
		if mode.Includes(nil) {
			t.Fatalf("BroadcastMode(%d).Includes(nil) = true, want false", mode)
		}
	}
}
