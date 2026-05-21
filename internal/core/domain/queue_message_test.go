package domain

import (
	"testing"
	"time"
)

// newTestQueueMessage creates a valid QueueMessage with mode defaulting to "Global".
func newTestQueueMessage(t *testing.T) *QueueMessage {
	t.Helper()
	msg, err := NewQueueMessage("MSG-001", "TEST_PROC", LogLevelInfo, "test payload", time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("NewQueueMessage() returned error: %v", err)
	}
	return msg
}

// ==========================================
// IsGlobalMessage Tests
// ==========================================

func TestQueueMessage_IsGlobalMessage_TrueByDefault(t *testing.T) {
	t.Parallel()

	msg := newTestQueueMessage(t)

	if !msg.IsGlobalMessage() {
		t.Fatalf("IsGlobalMessage() = false, want true for default-constructed message (mode=%q)", msg.Mode())
	}
}

func TestQueueMessage_IsGlobalMessage_FalseWhenModeIsSubscriber(t *testing.T) {
	t.Parallel()

	msg := newTestQueueMessage(t)
	msg.mode = "Subscriber"

	if msg.IsGlobalMessage() {
		t.Fatalf("IsGlobalMessage() = true, want false when mode=%q", msg.Mode())
	}
}

func TestQueueMessage_IsGlobalMessage_FalseWhenModeIsArbitrary(t *testing.T) {
	t.Parallel()

	msg := newTestQueueMessage(t)
	msg.mode = "unknown"

	if msg.IsGlobalMessage() {
		t.Fatalf("IsGlobalMessage() = true, want false when mode=%q", msg.Mode())
	}
}
