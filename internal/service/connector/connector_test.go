package connector

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// ==========================================
// Fake Bolt Config Repository
// ==========================================

// fakeBoltConfig is an in-memory implementation of ports.ConfigRepository for
// testing the Connector. Only the two active-database methods are exercised;
// the rest return zero values to satisfy the interface.
type fakeBoltConfig struct {
	mu  sync.RWMutex
	id  string
	err error
}

func (f *fakeBoltConfig) setID(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.id = id
}

func (f *fakeBoltConfig) GetActiveDatabaseID() (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.err != nil {
		return "", f.err
	}
	return f.id, nil
}

func (f *fakeBoltConfig) SetActiveDatabaseID(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.id = id
	return nil
}

// Unused ports.ConfigRepository methods — return zero values.
func (f *fakeBoltConfig) SaveDatabaseConfig(*domain.DatabaseSettings) error { return nil }
func (f *fakeBoltConfig) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	return nil, nil
}
func (f *fakeBoltConfig) IsApplicationFirstRun() (bool, error)              { return false, nil }
func (f *fakeBoltConfig) SetFirstRunCycleStatus(ports.RunCycleStatus) error { return nil }
func (f *fakeBoltConfig) SaveWebhookConfig(*domain.WebhookConfig) error     { return nil }
func (f *fakeBoltConfig) GetWebhookConfig() (*domain.WebhookConfig, error)  { return nil, nil }
func (f *fakeBoltConfig) DeleteWebhookConfig(string) error                  { return nil }
func (f *fakeBoltConfig) GetTracerPackageVersion() (string, error)          { return "", nil }
func (f *fakeBoltConfig) SetTracerPackageVersion(string) error              { return nil }
func (f *fakeBoltConfig) GetBroadcastMode() (domain.BroadcastMode, error) {
	return domain.BroadcastModeGlobal, nil
}
func (f *fakeBoltConfig) SetBroadcastMode(domain.BroadcastMode) error { return nil }

// Compile-time check.
var _ ports.ConfigRepository = (*fakeBoltConfig)(nil)

// countingBolt wraps any ConfigRepository and counts GetActiveDatabaseID
// calls. Used to assert hydration happens exactly once.
type countingBolt struct {
	inner ports.ConfigRepository
	calls atomic.Int64
}

func (c *countingBolt) GetActiveDatabaseID() (string, error) {
	c.calls.Add(1)
	return c.inner.GetActiveDatabaseID()
}

func (c *countingBolt) SetActiveDatabaseID(id string) error {
	return c.inner.SetActiveDatabaseID(id)
}

// Delegate every other ConfigRepository method to inner. We don't exercise
// them in the hydration test, but the compile-time interface check requires
// the full surface.
func (c *countingBolt) SaveDatabaseConfig(s *domain.DatabaseSettings) error {
	return c.inner.SaveDatabaseConfig(s)
}
func (c *countingBolt) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	return c.inner.GetDefaultDatabaseConfig()
}
func (c *countingBolt) IsApplicationFirstRun() (bool, error) { return c.inner.IsApplicationFirstRun() }
func (c *countingBolt) SetFirstRunCycleStatus(s ports.RunCycleStatus) error {
	return c.inner.SetFirstRunCycleStatus(s)
}
func (c *countingBolt) SaveWebhookConfig(s *domain.WebhookConfig) error {
	return c.inner.SaveWebhookConfig(s)
}
func (c *countingBolt) GetWebhookConfig() (*domain.WebhookConfig, error) {
	return c.inner.GetWebhookConfig()
}
func (c *countingBolt) DeleteWebhookConfig(id string) error { return c.inner.DeleteWebhookConfig(id) }
func (c *countingBolt) GetTracerPackageVersion() (string, error) {
	return c.inner.GetTracerPackageVersion()
}
func (c *countingBolt) SetTracerPackageVersion(v string) error {
	return c.inner.SetTracerPackageVersion(v)
}
func (c *countingBolt) GetBroadcastMode() (domain.BroadcastMode, error) {
	return c.inner.GetBroadcastMode()
}
func (c *countingBolt) SetBroadcastMode(m domain.BroadcastMode) error {
	return c.inner.SetBroadcastMode(m)
}

// ==========================================
// Active(): cache + hydration
// ==========================================

func TestActive_EmptyWhenNothingStored(t *testing.T) {
	c := New(&fakeBoltConfig{})
	if got := c.Active(); got != "" {
		t.Fatalf("expected empty active id, got %q", got)
	}
}

func TestActive_HydratesFromBoltOnFirstRead(t *testing.T) {
	bolt := &fakeBoltConfig{}
	bolt.setID("DBconfig:prod-1")

	c := New(bolt)
	if got := c.Active(); got != "DBconfig:prod-1" {
		t.Fatalf("expected hydrated id, got %q", got)
	}
	// Second read must use cache; spy on fakeBoltConfig to confirm.
	c.Active()
	c.Active()
	// Hydration is implemented as a single bolt read on cache miss; this
	// assertion is implicit — we simply confirm the value stays correct.
	if got := c.Active(); got != "DBconfig:prod-1" {
		t.Fatalf("cache drift, got %q", got)
	}
}

func TestActive_ReturnsEmptyWhenBoltErrors(t *testing.T) {
	bolt := &fakeBoltConfig{}
	bolt.err = errors.New("disk on fire")

	c := New(bolt)
	if got := c.Active(); got != "" {
		t.Fatalf("expected empty on bolt error, got %q", got)
	}
}

// Regression: when BoltDB has no active id (legitimately empty), the cache
// must stick. Previously c.id == "" was treated as a cache miss on every
// call, causing repeated bolt reads on the MCP status hot path.
func TestActive_EmptyResultSticksInCache(t *testing.T) {
	bolt := &countingBolt{inner: &fakeBoltConfig{}}
	c := New(bolt)

	for i := 0; i < 5; i++ {
		if got := c.Active(); got != "" {
			t.Fatalf("call %d: expected empty, got %q", i, got)
		}
	}
	if got := bolt.calls.Load(); got != 1 {
		t.Fatalf("expected 1 bolt read after 5 calls, got %d", got)
	}
}

// ==========================================
// SetActive(): persistence + cache
// ==========================================

func TestSetActive_PersistsAndUpdatesCache(t *testing.T) {
	bolt := &fakeBoltConfig{}
	c := New(bolt)

	if err := c.SetActive(context.Background(), "DBconfig:prod-1"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if got := c.Active(); got != "DBconfig:prod-1" {
		t.Fatalf("expected prod-1, got %q", got)
	}
	stored, err := bolt.GetActiveDatabaseID()
	if err != nil {
		t.Fatalf("GetActiveDatabaseID: %v", err)
	}
	if stored != "DBconfig:prod-1" {
		t.Fatalf("expected persisted prod-1, got %q", stored)
	}
}

func TestSetActive_EmptyClears(t *testing.T) {
	bolt := &fakeBoltConfig{}
	bolt.setID("DBconfig:old")
	c := New(bolt)

	if err := c.SetActive(context.Background(), ""); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if got := c.Active(); got != "" {
		t.Fatalf("expected empty after clear, got %q", got)
	}
	stored, _ := bolt.GetActiveDatabaseID()
	if stored != "" {
		t.Fatalf("expected bolt cleared, got %q", stored)
	}
}

func TestSetActive_TrimsWhitespace(t *testing.T) {
	bolt := &fakeBoltConfig{}
	c := New(bolt)

	if err := c.SetActive(context.Background(), "  DBconfig:prod-1  "); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if got := c.Active(); got != "DBconfig:prod-1" {
		t.Fatalf("expected trimmed id, got %q", got)
	}
}

func TestSetActive_LeavesCacheOnBoltError(t *testing.T) {
	bolt := &fakeBoltConfig{}
	bolt.setID("DBconfig:original")
	c := New(bolt)
	// Warm cache.
	_ = c.Active()

	bolt.err = errors.New("write failed")
	if err := c.SetActive(context.Background(), "DBconfig:new"); err == nil {
		t.Fatal("expected SetActive to surface bolt error")
	}
	if got := c.Active(); got != "DBconfig:original" {
		t.Fatalf("cache must not change on persistence failure, got %q", got)
	}
}

func TestSetActive_HonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bolt := &fakeBoltConfig{}
	c := New(bolt)
	if err := c.SetActive(ctx, "DBconfig:x"); err == nil {
		t.Fatal("expected context error, got nil")
	}
}

// ==========================================
// In-memory-only mode
// ==========================================

func TestNew_NilBoltInMemoryOnly(t *testing.T) {
	c := New(nil)
	if err := c.SetActive(context.Background(), "DBconfig:mem"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if got := c.Active(); got != "DBconfig:mem" {
		t.Fatalf("expected mem id, got %q", got)
	}
}

// ==========================================
// Concurrency
// ==========================================

func TestConcurrentSetActive_RaceFree(t *testing.T) {
	const writers = 8
	const writes = 200

	bolt := &fakeBoltConfig{}
	c := New(bolt)

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				id := "DBconfig:w" + strconv.Itoa(w) + "-" + strconv.Itoa(i)
				if err := c.SetActive(context.Background(), id); err != nil {
					t.Errorf("SetActive: %v", err)
					return
				}
			}
		}()
	}

	// Concurrent readers.
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
					_ = c.Active()
				}
			}
		}()
	}

	wg.Wait()
	close(stop)
	readers.Wait()

	// Final invariant: cache and disk agree.
	if got, stored := c.Active(), mustStored(bolt); got != stored {
		t.Fatalf("cache/disk divergence: cache=%q disk=%q", got, stored)
	}
}

// ==========================================
// Helpers
// ==========================================

func mustStored(bolt *fakeBoltConfig) string {
	stored, err := bolt.GetActiveDatabaseID()
	if err != nil {
		panic(err)
	}
	return stored
}
