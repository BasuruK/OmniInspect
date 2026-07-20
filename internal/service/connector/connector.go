package connector

import (
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
	bolt    ports.ConfigRepository
	mu      sync.RWMutex
	id      string
	hydrate sync.Once
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
	if c.bolt != nil {
		c.hydrate.Do(func() {
			// Persistence failures must not block reads; leave c.id at its
			// zero value so callers fall back to "no active database".
			if stored, err := c.bolt.GetActiveDatabaseID(); err == nil {
				c.mu.Lock()
				c.id = stored
				c.mu.Unlock()
			}
		})
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// SetActive records the given storage key as the active database and
// persists it to BoltDB. An empty id clears the active pointer. Errors
// from BoltDB are returned to the caller and the in-memory cache is left
// untouched on failure (so callers can retry without an inconsistent
// "in-memory says X, disk says Y" state).
func (c *Connector) SetActive(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)

	// Mark hydration done so a later Active() never overwrites this value
	// with a stale read from BoltDB.
	c.hydrate.Do(func() {})

	if c.bolt == nil {
		// In-memory only mode (used by tests); still record the change.
		c.mu.Lock()
		c.id = id
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
	return nil
}
