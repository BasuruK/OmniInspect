package tracer

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type stubConfigRepository struct{}

func (r *stubConfigRepository) SaveDatabaseConfig(*domain.DatabaseSettings) error { return nil }
func (r *stubConfigRepository) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	return nil, nil
}
func (r *stubConfigRepository) IsApplicationFirstRun() (bool, error)               { return false, nil }
func (r *stubConfigRepository) SetFirstRunCycleStatus(domain.RunCycleStatus) error { return nil }
func (r *stubConfigRepository) SaveWebhookConfig(*domain.WebhookConfig) error      { return nil }
func (r *stubConfigRepository) GetWebhookConfig() (*domain.WebhookConfig, error)   { return nil, nil }
func (r *stubConfigRepository) DeleteWebhookConfig(string) error                   { return nil }
func (r *stubConfigRepository) GetTracerPackageVersion() (string, error)           { return "", nil }
func (r *stubConfigRepository) SetTracerPackageVersion(string) error               { return nil }
func (r *stubConfigRepository) GetBroadcastMode() (domain.BroadcastMode, error) {
	return domain.BroadcastModeGlobal, nil
}
func (r *stubConfigRepository) SetBroadcastMode(domain.BroadcastMode) error { return nil }

type webhookConfigRepository struct {
	stubConfigRepository
	config *domain.WebhookConfig
}

func (r *webhookConfigRepository) GetWebhookConfig() (*domain.WebhookConfig, error) {
	return r.config, nil
}

type stubDatabaseRepository struct{}

func (stubDatabaseRepository) Connect(context.Context) error { return nil }
func (stubDatabaseRepository) Close(context.Context) error   { return nil }
func (stubDatabaseRepository) RegisterNewSubscriber(context.Context, domain.Subscriber) error {
	return nil
}
func (stubDatabaseRepository) UnregisterSubscriber(context.Context, domain.Subscriber) error {
	return nil
}
func (stubDatabaseRepository) BulkDequeueTracerMessages(context.Context, domain.Subscriber) ([]string, [][]byte, int, error) {
	return nil, nil, 0, nil
}
func (stubDatabaseRepository) CheckQueueDepth(context.Context, string, string) (int, error) {
	return 0, nil
}
func (stubDatabaseRepository) Fetch(context.Context, string) ([]string, error) { return nil, nil }
func (stubDatabaseRepository) ExecuteStatement(context.Context, string) error  { return nil }
func (stubDatabaseRepository) ExecuteWithParams(context.Context, string, map[string]interface{}) error {
	return nil
}
func (stubDatabaseRepository) FetchWithParams(context.Context, string, map[string]interface{}) ([]string, error) {
	return nil, nil
}
func (stubDatabaseRepository) PackageExists(context.Context, string) (bool, error) { return false, nil }
func (stubDatabaseRepository) ProcedureExists(context.Context, string, string) (bool, error) {
	return false, nil
}
func (stubDatabaseRepository) DeployPackages(context.Context, []string, []string, []string, []string) error {
	return nil
}
func (stubDatabaseRepository) DeployFile(context.Context, string) error { return nil }

func TestNewTracerService_RejectsNilDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewTracerService(nil, &stubConfigRepository{}, make(chan *domain.QueueMessage, 1))
	if !errors.Is(err, domain.ErrNilRepository) {
		t.Fatalf("expected ErrNilRepository, got %v", err)
	}

	_, err = NewTracerService(stubDatabaseRepository{}, nil, make(chan *domain.QueueMessage, 1))
	if !errors.Is(err, domain.ErrNilConfig) {
		t.Fatalf("expected ErrNilConfig, got %v", err)
	}
}

func TestStopConnectionListener_DoesNotStopGlobalWebhookDispatcher(t *testing.T) {
	previousDispatcher := globalWebhookDispatcher

	dispatcher := newWebhookDispatcher()
	globalWebhookDispatcher = dispatcher

	t.Cleanup(func() {
		dispatcher.Stop()
		globalWebhookDispatcher = previousDispatcher
		dispatcherOnce = sync.Once{}
	})

	(&TracerService{}).StopConnectionListener()

	if dispatcher.stopped {
		t.Fatal("expected StopConnectionListener to leave the global webhook dispatcher running")
	}
}

func TestStopAll_ForwardsToStopConnectionListener(t *testing.T) {
	service := &TracerService{eventChannel: make(chan *domain.QueueMessage, 2)}
	service.eventChannel <- &domain.QueueMessage{}

	StopAll(service)

	select {
	case <-service.eventChannel:
		t.Fatal("expected StopAll to forward to StopConnectionListener and drain queued events")
	default:
	}
}

func TestShutdownOwnerStopsGlobalWebhookDispatcher(t *testing.T) {
	previousDispatcher := globalWebhookDispatcher

	dispatcher := newWebhookDispatcher()
	globalWebhookDispatcher = dispatcher

	t.Cleanup(func() {
		globalWebhookDispatcher = previousDispatcher
		dispatcherOnce = sync.Once{}
	})

	StopWebhookDispatcher()

	if !dispatcher.stopped {
		t.Fatal("expected StopWebhookDispatcher to stop the global webhook dispatcher")
	}
}

func TestStopWebhookDispatcher_DoesNotInitializeDispatcher(t *testing.T) {
	previousDispatcher := globalWebhookDispatcher
	globalWebhookDispatcher = nil
	dispatcherOnce = sync.Once{}

	t.Cleanup(func() {
		globalWebhookDispatcher = previousDispatcher
		dispatcherOnce = sync.Once{}
	})

	StopWebhookDispatcher()

	if globalWebhookDispatcher != nil {
		t.Fatal("expected StopWebhookDispatcher to leave the global dispatcher nil when it was never initialized")
	}
}

// TestWebhookDispatcherStop_IsIdempotent verifies that calling Stop twice on a
// dispatcher created by newWebhookDispatcher does not panic because Stop uses
// sync.Once to guard the shutdown path.
func TestWebhookDispatcherStop_IsIdempotent(t *testing.T) {
	dispatcher := newWebhookDispatcher()
	dispatcher.Stop()
	dispatcher.Stop()
}

// spyDatabaseRepository is a controllable stub that tracks UnregisterSubscriber calls.
type spyDatabaseRepository struct {
	stubDatabaseRepository
	mu                   sync.Mutex
	unregisterCalled     bool
	unregisterCalledWith domain.Subscriber
	unregisterErr        error
}

func (s *spyDatabaseRepository) UnregisterSubscriber(_ context.Context, sub domain.Subscriber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregisterCalled = true
	s.unregisterCalledWith = sub
	return s.unregisterErr
}

func newTestSubscriber(t *testing.T) *domain.Subscriber {
	t.Helper()
	sub, err := domain.NewSubscriber("TESTNAME", domain.DefaultBatchSize, domain.DefaultWaitTime)
	if err != nil {
		t.Fatalf("failed to create test subscriber: %v", err)
	}
	return sub
}

func TestCancelConnectionListener_UnregistersActiveSubscriber(t *testing.T) {
	t.Parallel()
	spy := &spyDatabaseRepository{}
	sub := newTestSubscriber(t)

	ts := &TracerService{
		db:               spy,
		eventChannel:     make(chan *domain.QueueMessage, 1),
		activeSubscriber: sub,
	}

	ts.CancelConnectionListener()

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if !spy.unregisterCalled {
		t.Fatal("expected UnregisterSubscriber to be called when an active subscriber is present")
	}
	if spy.unregisterCalledWith.ConsumerName() != sub.ConsumerName() {
		t.Fatalf("expected UnregisterSubscriber called with %q, got %q",
			sub.ConsumerName(), spy.unregisterCalledWith.ConsumerName())
	}
	if ts.activeSubscriber != nil {
		t.Fatal("expected activeSubscriber to be cleared after unregistration")
	}
}

func TestCancelConnectionListener_SkipsUnregistrationWhenNoSubscriber(t *testing.T) {
	t.Parallel()
	spy := &spyDatabaseRepository{}

	ts := &TracerService{
		db:           spy,
		eventChannel: make(chan *domain.QueueMessage, 1),
		// activeSubscriber intentionally nil
	}

	ts.CancelConnectionListener()

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if spy.unregisterCalled {
		t.Fatal("expected UnregisterSubscriber NOT to be called when no active subscriber")
	}
}

func TestCancelConnectionListener_ContinuesOnUnregisterError(t *testing.T) {
	t.Parallel()
	spy := &spyDatabaseRepository{unregisterErr: errors.New("oracle unreachable")}
	sub := newTestSubscriber(t)

	ts := &TracerService{
		db:               spy,
		eventChannel:     make(chan *domain.QueueMessage, 1),
		activeSubscriber: sub,
	}

	// Must not panic or block — error is only logged
	ts.CancelConnectionListener()

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if !spy.unregisterCalled {
		t.Fatal("expected UnregisterSubscriber to be called even when an error is expected")
	}
	if ts.activeSubscriber != nil {
		t.Fatal("expected activeSubscriber to be cleared even after unregistration error")
	}
}

func TestHandleTracerMessage_QueuesWebhookOnlyWithOptIn(t *testing.T) {
	previousDispatcher := globalWebhookDispatcher

	t.Cleanup(func() {
		globalWebhookDispatcher = previousDispatcher
		dispatcherOnce = sync.Once{}
	})

	config, err := domain.NewWebhookConfig(domain.DefaultWebhookID, "https://example.com/webhook", true)
	if err != nil {
		t.Fatalf("failed to create webhook config: %v", err)
	}

	for _, tt := range []struct {
		name          string
		sendToWebhook bool
		wantQueued    int
	}{
		{name: "without opt-in", sendToWebhook: false, wantQueued: 0},
		{name: "with opt-in", sendToWebhook: true, wantQueued: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			injectedDispatcher := &webhookDispatcher{queue: make(chan webhookJob, 1)}
			globalWebhookDispatcher = injectedDispatcher
			dispatcherOnce = sync.Once{}
			dispatcherOnce.Do(func() {})

			msg, err := domain.NewQueueMessage(
				"message",
				"TEST_PROCESS",
				domain.LogLevelInfo,
				"payload",
				time.Unix(1700000000, 0),
				tt.sendToWebhook,
			)
			if err != nil {
				t.Fatalf("failed to create queue message: %v", err)
			}

			ts := &TracerService{bolt: &webhookConfigRepository{config: config}}
			if ok := ts.handleTracerMessage(context.Background(), msg); !ok {
				t.Fatal("expected handleTracerMessage to continue processing")
			}

			if queued := len(injectedDispatcher.queue); queued != tt.wantQueued {
				t.Fatalf("expected %d webhook jobs to be queued, got %d", tt.wantQueued, queued)
			}
		})
	}
}
