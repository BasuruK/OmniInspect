package tracer

import (
	"sync"
	"testing"
)

func TestStopAll_DoesNotStopGlobalWebhookDispatcher(t *testing.T) {
	previousDispatcher := globalWebhookDispatcher

	dispatcher := newWebhookDispatcher()
	globalWebhookDispatcher = dispatcher

	t.Cleanup(func() {
		dispatcher.Stop()
		globalWebhookDispatcher = previousDispatcher
		dispatcherOnce = sync.Once{}
	})

	StopAll(&TracerService{})

	if dispatcher.stopped {
		t.Fatal("expected StopAll to leave the global webhook dispatcher running")
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

func TestWebhookDispatcherStop_IsIdempotent(t *testing.T) {
	dispatcher := newWebhookDispatcher()
	dispatcher.Stop()
	dispatcher.Stop()
}
