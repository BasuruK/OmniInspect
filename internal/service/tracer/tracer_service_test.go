package tracer

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"sync"
	"testing"
)

type stubConfigRepository struct{}

func (stubConfigRepository) SaveDatabaseConfig(*domain.DatabaseSettings) error { return nil }
func (stubConfigRepository) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	return nil, nil
}
func (stubConfigRepository) IsApplicationFirstRun() (bool, error)               { return false, nil }
func (stubConfigRepository) SetFirstRunCycleStatus(domain.RunCycleStatus) error { return nil }
func (stubConfigRepository) SaveWebhookConfig(*domain.WebhookConfig) error      { return nil }
func (stubConfigRepository) GetWebhookConfig() (*domain.WebhookConfig, error)   { return nil, nil }
func (stubConfigRepository) DeleteWebhookConfig(string) error                   { return nil }

type stubDatabaseRepository struct{}

func (stubDatabaseRepository) Connect(context.Context) error { return nil }
func (stubDatabaseRepository) Close(context.Context) error   { return nil }
func (stubDatabaseRepository) RegisterNewSubscriber(context.Context, domain.Subscriber) error {
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
func (stubDatabaseRepository) DeployPackages(context.Context, []string, []string, []string, []string) error {
	return nil
}
func (stubDatabaseRepository) DeployFile(context.Context, string) error { return nil }

func TestNewTracerService_RejectsNilDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewTracerService(nil, stubConfigRepository{}, make(chan *domain.QueueMessage, 1))
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
