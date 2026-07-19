package tracebuffer

import (
	"OmniView/internal/core/domain"
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

// ==========================================
// Helpers
// ==========================================

func makeMessage(t *testing.T, id string) *domain.QueueMessage {
	t.Helper()
	ts := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	msg, err := domain.NewQueueMessage(id, "proc", domain.LogLevelInfo, "payload", ts)
	if err != nil {
		t.Fatalf("NewQueueMessage(%q) failed: %v", id, err)
	}
	return msg
}

// ==========================================
// Construction
// ==========================================

func TestNew_DefaultsZeroCapacityToOne(t *testing.T) {
	rb := New(0)
	if rb.Len(context.Background()) != 0 {
		t.Fatalf("expected empty buffer, got len=%d", rb.Len(context.Background()))
	}
	if err := rb.Append(context.Background(), makeMessage(t, "a")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := rb.Append(context.Background(), makeMessage(t, "b")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// Capacity 1 → only the newest survives.
	if got := rb.Len(context.Background()); got != 1 {
		t.Fatalf("expected len=1, got %d", got)
	}
	m, err := rb.GetByID(context.Background(), "b")
	if err != nil || m == nil || m.MessageID() != "b" {
		t.Fatalf("expected newest entry 'b', got %+v err=%v", m, err)
	}
}

// ==========================================
// Append / eviction
// ==========================================

func TestAppend_EvictsFIFOAtCapacity(t *testing.T) {
	rb := New(3)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := rb.Append(context.Background(), makeMessage(t, id)); err != nil {
			t.Fatalf("Append %s: %v", id, err)
		}
	}

	if got := rb.Len(context.Background()); got != 3 {
		t.Fatalf("expected len=3, got %d", got)
	}
	// Oldest two must be evicted.
	for _, gone := range []string{"a", "b"} {
		m, err := rb.GetByID(context.Background(), gone)
		if err != nil {
			t.Fatalf("GetByID %s: %v", gone, err)
		}
		if m != nil {
			t.Fatalf("expected %s evicted, got %+v", gone, m)
		}
	}
	for _, kept := range []string{"c", "d", "e"} {
		m, err := rb.GetByID(context.Background(), kept)
		if err != nil {
			t.Fatalf("GetByID %s: %v", kept, err)
		}
		if m == nil || m.MessageID() != kept {
			t.Fatalf("expected %s kept, got %+v", kept, m)
		}
	}
}

func TestAppend_NilIsIgnored(t *testing.T) {
	rb := New(2)
	if err := rb.Append(context.Background(), nil); err != nil {
		t.Fatalf("Append nil: %v", err)
	}
	if got := rb.Len(context.Background()); got != 0 {
		t.Fatalf("expected len=0 after nil append, got %d", got)
	}
}

// ==========================================
// List ordering & sinceID filter
// ==========================================

func TestList_NewestFirstRespectsLimit(t *testing.T) {
	rb := New(10)
	for _, id := range []string{"a", "b", "c", "d"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}

	got, err := rb.List(context.Background(), 2, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	want := []string{"d", "c"}
	for i, m := range got {
		if m.MessageID() != want[i] {
			t.Fatalf("position %d: got %s want %s", i, m.MessageID(), want[i])
		}
	}
}

func TestList_SinceIDFiltersOutOlder(t *testing.T) {
	rb := New(10)
	for _, id := range []string{"a", "b", "c", "d"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}

	got, err := rb.List(context.Background(), 10, "b")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Lexicographically > "b" → c, d (newest-first).
	want := []string{"d", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d (%+v)", len(want), len(got), got)
	}
	for i, m := range got {
		if m.MessageID() != want[i] {
			t.Fatalf("position %d: got %s want %s", i, m.MessageID(), want[i])
		}
	}
}

func TestList_NonPositiveLimitReturnsEmpty(t *testing.T) {
	rb := New(5)
	_ = rb.Append(context.Background(), makeMessage(t, "x"))

	got, err := rb.List(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d entries", len(got))
	}
}

// ==========================================
// GetByID hit / miss
// ==========================================

func TestGetByID_HitAndMiss(t *testing.T) {
	rb := New(5)
	_ = rb.Append(context.Background(), makeMessage(t, "alpha"))
	_ = rb.Append(context.Background(), makeMessage(t, "beta"))

	m, err := rb.GetByID(context.Background(), "beta")
	if err != nil {
		t.Fatalf("GetByID hit: %v", err)
	}
	if m == nil || m.MessageID() != "beta" {
		t.Fatalf("expected beta, got %+v", m)
	}

	m, err = rb.GetByID(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetByID miss: %v", err)
	}
	if m != nil {
		t.Fatalf("expected nil for miss, got %+v", m)
	}
}

// ==========================================
// Clear
// ==========================================

func TestClear_RemovesAll(t *testing.T) {
	rb := New(5)
	for _, id := range []string{"a", "b", "c"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}
	if err := rb.Clear(context.Background()); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if got := rb.Len(context.Background()); got != 0 {
		t.Fatalf("expected len=0 after clear, got %d", got)
	}
}

// ==========================================
// Concurrency
// ==========================================

func TestConcurrentAppendReadClear(t *testing.T) {
	const writers = 8
	const appends = 200
	const capacity = 500

	rb := New(capacity)

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < appends; i++ {
				id := "w" + strconv.Itoa(w) + "-" + strconv.Itoa(i)
				_ = rb.Append(context.Background(), makeMessage(t, id))
			}
		}()
	}

	// Concurrent readers while writers run.
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for r := 0; r < 4; r++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = rb.List(context.Background(), 50, "")
					_ = rb.Len(context.Background())
				}
			}
		}()
	}

	wg.Wait()
	close(stop)
	readers.Wait()

	// Final invariants.
	if got := rb.Len(context.Background()); got > capacity {
		t.Fatalf("len %d exceeds capacity %d", got, capacity)
	}
}
