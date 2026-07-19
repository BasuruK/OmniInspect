package tracebuffer

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"sync"
)

// ==========================================
// Ring Buffer
// ==========================================

// RingBuffer is a bounded, FIFO-evicting trace store backed by a fixed-size
// circular array. It is safe for concurrent use by multiple goroutines.
//
// The buffer holds at most `capacity` messages; once full, Append overwrites
// the oldest entry. The backing slice is allocated once at construction so
// Append never reallocates.
type RingBuffer struct {
	mu       sync.RWMutex
	buf      []*domain.QueueMessage
	head     int // index of the oldest entry
	size     int // current number of entries (0 <= size <= capacity)
	capacity int
}

// Compile-time check that RingBuffer satisfies the TraceAppender port.
var _ ports.TraceAppender = (*RingBuffer)(nil)

// New returns a RingBuffer with the given capacity. A non-positive capacity
// is treated as 1.
func New(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		buf:      make([]*domain.QueueMessage, capacity),
		capacity: capacity,
	}
}

// Append adds a message to the buffer. On a full buffer, the oldest entry
// is overwritten. Nil messages are ignored. ctx is honored only for
// cancellation; the underlying storage is in-memory and performs no I/O.
func (r *RingBuffer) Append(ctx context.Context, msg *domain.QueueMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	var drop *domain.QueueMessage
	if r.size == r.capacity {
		// Buffer is full: overwrite the oldest slot and advance head.
		drop = r.buf[r.head]
		r.buf[r.head] = msg
		r.head = (r.head + 1) % r.capacity
	} else {
		// Tail = position of the next free slot = (head + size) mod cap.
		tail := (r.head + r.size) % r.capacity
		r.buf[tail] = msg
		r.size++
	}
	_ = drop // help GC; we already replaced the slot, drop is just a local.
	return nil
}

// List returns up to `limit` messages in newest-first order. When sinceID is
// non-empty, only messages whose ID is strictly greater (lexicographically)
// than sinceID are returned. A non-positive limit returns no entries.
func (r *RingBuffer) List(ctx context.Context, limit int, sinceID string) ([]*domain.QueueMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []*domain.QueueMessage{}, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Walk newest → oldest.
	out := make([]*domain.QueueMessage, 0, limit)
	for i := r.size - 1; i >= 0 && len(out) < limit; i-- {
		// Map logical index i (0 = oldest, size-1 = newest) to physical index.
		idx := (r.head + i) % r.capacity
		m := r.buf[idx]
		if sinceID != "" && m.MessageID() <= sinceID {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// GetByID returns the message with the given ID, or nil if not found.
func (r *RingBuffer) GetByID(ctx context.Context, id string) (*domain.QueueMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := 0; i < r.size; i++ {
		idx := (r.head + i) % r.capacity
		m := r.buf[idx]
		if m.MessageID() == id {
			return m, nil
		}
	}
	return nil, nil
}

// Clear removes all messages and releases the references held by the
// backing array so the GC can reclaim trace payloads.
func (r *RingBuffer) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < r.size; i++ {
		idx := (r.head + i) % r.capacity
		r.buf[idx] = nil
	}
	r.head = 0
	r.size = 0
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
