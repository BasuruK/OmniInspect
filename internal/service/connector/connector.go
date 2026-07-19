package connector

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"fmt"
	"strings"
	"sync"
)

// ==========================================
// Connector Service
// ==========================================

// Connector is the single source of truth for the currently active database.
// It holds an in-memory cache that is hydrated from BoltDB on first use and
// updated on every SetActive. Safe for concurrent use.
type Connector struct {
	bolt     ports.ConfigRepository
	mu       sync.RWMutex
	id       string
	hydrated bool
}

// New constructs a Connector backed by the given BoltDB config repository.
// The active id is loaded lazily on first call to Active(); pass an empty
// bolt to defer persistence entirely (in-memory only).
func New(bolt ports.ConfigRepository) *Connector {
	return &Connector{bolt: bolt}
}

// Active returns the storage key (storage-prefixed id) of the currently
// active database. Returns an empty string when no active database has
// been recorded, either because the BoltDB pointer is unset or because the
// call was made before initialization completed.
func (c *Connector) Active() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id != "" || c.bolt == nil {
		return id
	}
	// Cache miss: hydrate from BoltDB. We re-take the write lock because the
	// read lock cannot be upgraded mid-flight in Go.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hydrated {
		return c.id
	}
	stored, err := c.bolt.GetActiveDatabaseID()
	if err != nil {
		// Persistence failures must not block reads; surface via empty string
		// so callers can fall back to "no active database".
		return ""
	}
	c.id = stored
	c.hydrated = true
	return c.id
}

// SetActive records the given storage key as the active database and
// persists it to BoltDB. An empty id clears the active pointer. Errors
// from BoltDB are returned to the caller and the in-memory cache is left
// untouched on failure (so callers can retry without an inconsistent
// "in-memory says X, disk says Y" state).
func (c *Connector) SetActive(ctx context.Context, id string) error {
	if c == nil {
		return domain.ErrNilConnector
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)

	if c.bolt == nil {
		// In-memory only mode (used by tests); still record the change.
		c.mu.Lock()
		c.id = id
		c.hydrated = true
		c.mu.Unlock()
		return nil
	}

	// BoltDB write and cache update must happen under the same lock so
	// concurrent Active() readers cannot observe BoltDB committed while the
	// in-memory cache still holds the previous value.
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.bolt.SetActiveDatabaseID(id); err != nil {
		return fmt.Errorf("connector: persist active id: %w", err)
	}
	c.id = id
	c.hydrated = true
	return nil
}
