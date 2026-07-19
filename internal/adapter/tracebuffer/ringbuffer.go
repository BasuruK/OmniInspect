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

// RingBuffer is a bounded, FIFO-evicting trace store backed by a slice.
// It is safe for concurrent use by multiple goroutines.
//
// The buffer holds at most `capacity` messages; once full, the oldest entry
// is evicted on Append.
type RingBuffer struct {
	mu       sync.RWMutex
	items    []*domain.QueueMessage
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
		items:    make([]*domain.QueueMessage, 0, capacity),
		capacity: capacity,
	}
}

// Append adds a message to the buffer, evicting the oldest entry when at
// capacity. Nil messages are ignored. ctx is honored only for cancellation;
// the underlying storage is in-memory and performs no I/O.
func (r *RingBuffer) Append(ctx context.Context, msg *domain.QueueMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.items) >= r.capacity {
		// FIFO eviction: drop the oldest.
		r.items = r.items[1:]
	}
	r.items = append(r.items, msg)
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
	for i := len(r.items) - 1; i >= 0 && len(out) < limit; i-- {
		m := r.items[i]
		if sinceID != "" && m.MessageID() <= sinceID {
			break
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
	for _, m := range r.items {
		if m.MessageID() == id {
			return m, nil
		}
	}
	return nil, nil
}

// Clear removes all messages.
func (r *RingBuffer) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = r.items[:0]
	return nil
}

// Len returns the current number of stored messages.
func (r *RingBuffer) Len(ctx context.Context) int {
	if err := ctx.Err(); err != nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
