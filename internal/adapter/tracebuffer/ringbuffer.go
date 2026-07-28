package tracebuffer

import (
	"context"
	"sync"

	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
)

// Compile-time check.
var _ ports.TraceAppender = (*RingBuffer)(nil)

// ==========================================
// Ring Buffer
// ==========================================

// ringEntry pairs a stored message with a monotonically increasing sequence number. The sequence number — not the message ID — is what since_id
// filtering keys off of, so ID formats that don't sort lexicographically in insertion order (UUIDs, unpadded counters, etc.) still filter correctly.
type ringEntry struct {
	msg   *domain.QueueMessage
	seq   uint64
	bytes int64 // payload size, drives maxBytes eviction
}

// RingBuffer is a bounded, FIFO-evicting trace store backed by a fixed-size circular array. It is safe for concurrent use by multiple goroutines.
//
// The buffer holds at most `capacity` messages; once full, Append overwrites the oldest entry. When maxBytes > 0, the oldest entries are additionally
// evicted whenever total payload bytes exceed maxBytes — a count ceiling alone cannot stop a few multi-MB payloads from exhausting memory. The backing slice is allocated once at construction so Append never reallocates.
type RingBuffer struct {
	mu         sync.RWMutex
	buf        []ringEntry
	head       int // index of the oldest entry
	size       int // current number of entries (0 <= size <= capacity)
	capacity   int
	maxBytes   int64 // payload byte ceiling; <= 0 disables byte-based eviction
	totalBytes int64
	nextSeq    uint64 // monotonically increasing, assigned on Append
	evicted    uint64 // count of entries overwritten by FIFO eviction
}

// New returns a RingBuffer with the given capacity. A non-positive capacity is treated as 1. maxBytes <= 0 disables byte-based eviction.
func New(capacity int, maxBytes int64) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		buf:      make([]ringEntry, capacity),
		capacity: capacity,
		maxBytes: maxBytes,
	}
}

// Append adds a message to the buffer. On a full buffer, the oldest entry is overwritten. Nil messages are ignored. ctx is honored only for cancellation; the underlying storage is in-memory and performs no I/O.
func (r *RingBuffer) Append(ctx context.Context, msg *domain.QueueMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	entryBytes := int64(len(msg.Payload()))
	if r.maxBytes > 0 && entryBytes > r.maxBytes {
		// A single payload over the ceiling can never fit even after evicting every other entry, so drop it instead of wiping out retained history for no benefit.
		r.evicted++
		return nil
	}

	r.nextSeq++
	entry := ringEntry{msg: msg, seq: r.nextSeq, bytes: entryBytes}

	if r.size == r.capacity {
		// Buffer is full: overwrite the oldest slot and advance head.
		r.totalBytes -= r.buf[r.head].bytes
		r.buf[r.head] = entry
		r.head = (r.head + 1) % r.capacity
		r.evicted++
	} else {
		// Tail = position of the next free slot = (head + size) mod cap.
		tail := (r.head + r.size) % r.capacity
		r.buf[tail] = entry
		r.size++
	}
	r.totalBytes += entry.bytes

	// Byte ceiling: evict oldest until back under maxBytes. Since entries over the ceiling are rejected above, this always converges to <= maxBytes (the last entry standing is the newest, which fits alone).
	for r.maxBytes > 0 && r.totalBytes > r.maxBytes && r.size > 1 {
		r.totalBytes -= r.buf[r.head].bytes
		r.buf[r.head] = ringEntry{}
		r.head = (r.head + 1) % r.capacity
		r.size--
		r.evicted++
	}
	return nil
}

// List returns up to `limit` messages in newest-first order. When sinceID is non-empty, only messages appended strictly after the message with that ID
// are returned (by insertion sequence, not string comparison of the ID itself). If sinceID is non-empty but no longer present in the buffer
// (already evicted, or unknown), List returns domain.ErrTraceCursorExpired so the caller can distinguish "nothing new" from "your cursor expired". A non-positive limit returns no entries.
func (r *RingBuffer) List(ctx context.Context, limit int, sinceID string) ([]*domain.QueueMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []*domain.QueueMessage{}, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var sinceSeq uint64
	if sinceID != "" {
		found := false
		for i := 0; i < r.size; i++ {
			idx := (r.head + i) % r.capacity
			if r.buf[idx].msg.MessageID() == sinceID {
				sinceSeq = r.buf[idx].seq
				found = true
				break
			}
		}
		if !found {
			return nil, domain.ErrTraceCursorExpired
		}
	}

	// Walk newest → oldest.
	out := make([]*domain.QueueMessage, 0, limit)
	for i := r.size - 1; i >= 0 && len(out) < limit; i-- {
		// Map logical index i (0 = oldest, size-1 = newest) to physical index.
		idx := (r.head + i) % r.capacity
		e := r.buf[idx]
		if sinceID != "" && e.seq <= sinceSeq {
			break
		}
		out = append(out, e.msg)
	}
	return out, nil
}

// Clear removes all messages and releases the references held by the backing array so the GC can reclaim trace payloads. It does not reset the
// eviction counter or sequence counter — those track lifetime activity, not current contents.
func (r *RingBuffer) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < r.size; i++ {
		idx := (r.head + i) % r.capacity
		r.buf[idx] = ringEntry{}
	}
	r.head = 0
	r.size = 0
	r.totalBytes = 0
	return nil
}

// Len returns the current number of stored messages.
func (r *RingBuffer) Len(ctx context.Context) int {
	if err := ctx.Err(); err != nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// Evicted returns the lifetime count of messages lost: entries overwritten
// at capacity, entries dropped to stay under maxBytes, and payloads rejected for exceeding maxBytes on their own. Callers can surface this to detect silent trace loss under
// sustained load — Len alone cannot distinguish "buffer full" from "buffer has been overwriting for a while."
func (r *RingBuffer) Evicted(ctx context.Context) int {
	if err := ctx.Err(); err != nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int(r.evicted)
}

