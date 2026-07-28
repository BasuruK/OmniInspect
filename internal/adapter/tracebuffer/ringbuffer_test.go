package tracebuffer

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"strconv"
	"strings"
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

func makeMessageWithPayload(t *testing.T, id string, payload string) *domain.QueueMessage {
	t.Helper()
	ts := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	msg, err := domain.NewQueueMessage(id, "proc", domain.LogLevelInfo, payload, ts)
	if err != nil {
		t.Fatalf("NewQueueMessage(%q) failed: %v", id, err)
	}
	return msg
}

// ==========================================
// Construction
// ==========================================

func TestNew_DefaultsZeroCapacityToOne(t *testing.T) {
	rb := New(0, 0)
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
	m, err := rb.List(context.Background(), 1, "")
	if err != nil || len(m) != 1 || m[0].MessageID() != "b" {
		t.Fatalf("expected newest entry 'b', got %+v err=%v", m, err)
	}
}

// ==========================================
// Append / eviction
// ==========================================

func TestAppend_EvictsFIFOAtCapacity(t *testing.T) {
	rb := New(3, 0)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := rb.Append(context.Background(), makeMessage(t, id)); err != nil {
			t.Fatalf("Append %s: %v", id, err)
		}
	}

	if got := rb.Len(context.Background()); got != 3 {
		t.Fatalf("expected len=3, got %d", got)
	}
	// Oldest two (a, b) must be evicted; what remains is e, d, c newest-first.
	got, err := rb.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"e", "d", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d (%+v)", len(want), len(got), got)
	}
	for i, m := range got {
		if m.MessageID() != want[i] {
			t.Fatalf("position %d: got %s want %s", i, m.MessageID(), want[i])
		}
	}
}

func TestAppend_NilIsIgnored(t *testing.T) {
	rb := New(2, 0)
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
	rb := New(10, 0)
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
	rb := New(10, 0)
	for _, id := range []string{"a", "b", "c", "d"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}

	got, err := rb.List(context.Background(), 10, "b")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Appended after "b" → c, d (newest-first).
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

func TestList_SinceIDWorksWithNonSortableIDs(t *testing.T) {
	rb := New(10, 0)
	// IDs that do NOT sort lexicographically in insertion order — a
	// lexicographic comparison would silently misfilter here (e.g. "9" > "10").
	ids := []string{"msg-9", "msg-10", "msg-2", "msg-100"}
	for _, id := range ids {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}

	// since_id="msg-10" is the 2nd inserted; only entries appended AFTER it
	// (by insertion sequence) should be returned, regardless of string order.
	got, err := rb.List(context.Background(), 10, "msg-10")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"msg-100", "msg-2"} // newest-first, by insertion sequence
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d (%+v)", len(want), len(got), got)
	}
	for i, m := range got {
		if m.MessageID() != want[i] {
			t.Fatalf("position %d: got %s want %s", i, m.MessageID(), want[i])
		}
	}
}

func TestList_SinceIDNotFoundReturnsCursorExpired(t *testing.T) {
	rb := New(10, 0)
	for _, id := range []string{"a", "b", "c"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}

	// Unknown/evicted sinceID: the caller must be able to tell "your cursor
	// expired" apart from "nothing new".
	_, err := rb.List(context.Background(), 10, "never-existed")
	if !errors.Is(err, domain.ErrTraceCursorExpired) {
		t.Fatalf("expected ErrTraceCursorExpired for unknown sinceID, got %v", err)
	}
}

func TestList_NonPositiveLimitReturnsEmpty(t *testing.T) {
	rb := New(5, 0)
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
// Clear
// ==========================================

func TestClear_RemovesAll(t *testing.T) {
	rb := New(5, 0)
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
// Eviction counter
// ==========================================

func TestEvicted_CountsOverwrittenEntries(t *testing.T) {
	rb := New(3, 0)
	if got := rb.Evicted(context.Background()); got != 0 {
		t.Fatalf("expected 0 evicted before any overwrite, got %d", got)
	}
	for _, id := range []string{"a", "b", "c"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}
	if got := rb.Evicted(context.Background()); got != 0 {
		t.Fatalf("expected 0 evicted while under capacity, got %d", got)
	}
	for _, id := range []string{"d", "e"} {
		_ = rb.Append(context.Background(), makeMessage(t, id))
	}
	if got := rb.Evicted(context.Background()); got != 2 {
		t.Fatalf("expected 2 evicted after 2 overwrites, got %d", got)
	}
}

// ==========================================
// Byte ceiling
// ==========================================

func TestAppend_MaxBytesEvictsOldestUntilUnderCeiling(t *testing.T) {
	rb := New(10, 15)
	payload := strings.Repeat("a", 5) // 5 bytes per message
	for _, id := range []string{"a", "b", "c"} {
		if err := rb.Append(context.Background(), makeMessageWithPayload(t, id, payload)); err != nil {
			t.Fatalf("Append(%q): %v", id, err)
		}
	}
	if got := rb.Len(context.Background()); got != 3 {
		t.Fatalf("expected 3 entries at the ceiling, got %d", got)
	}
	if got := rb.Evicted(context.Background()); got != 0 {
		t.Fatalf("expected 0 evicted at the ceiling, got %d", got)
	}

	if err := rb.Append(context.Background(), makeMessageWithPayload(t, "d", payload)); err != nil {
		t.Fatalf("Append(d): %v", err)
	}
	if got := rb.Len(context.Background()); got != 3 {
		t.Fatalf("expected oldest entry evicted to stay under the byte ceiling, got len=%d", got)
	}
	if got := rb.Evicted(context.Background()); got != 1 {
		t.Fatalf("expected 1 evicted after exceeding the byte ceiling, got %d", got)
	}
	remaining, err := rb.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, m := range remaining {
		if m.MessageID() == "a" {
			t.Fatalf("expected oldest entry %q to have been evicted", "a")
		}
	}
}

func TestAppend_DropsEntryOverByteCeiling(t *testing.T) {
	rb := New(10, 10)
	if err := rb.Append(context.Background(), makeMessageWithPayload(t, "a", "small")); err != nil {
		t.Fatalf("Append(a): %v", err)
	}

	oversized := strings.Repeat("x", 20)
	if err := rb.Append(context.Background(), makeMessageWithPayload(t, "b", oversized)); err != nil {
		t.Fatalf("Append(b): %v", err)
	}

	if got := rb.Len(context.Background()); got != 1 {
		t.Fatalf("expected oversized entry to be dropped without touching retained history, got len=%d", got)
	}
	if got := rb.Evicted(context.Background()); got != 1 {
		t.Fatalf("expected oversized entry to count as evicted, got %d", got)
	}
	remaining, err := rb.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(remaining) != 1 || remaining[0].MessageID() != "a" {
		t.Fatalf("expected only retained entry %q, got %+v", "a", remaining)
	}
}

// ==========================================
// Concurrency
// ==========================================

func TestConcurrentAppendReadClear(t *testing.T) {
	const writers = 8
	const appends = 200
	const capacity = 500

	rb := New(capacity, 0)

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

