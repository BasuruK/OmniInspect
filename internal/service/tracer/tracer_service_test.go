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

	// Simulate the shutdown-owner path by calling the dispatcher's Stop method
	dispatcher.Stop()

	if !dispatcher.stopped {
		t.Fatal("expected dispatcher.Stop() to stop the global webhook dispatcher")
	}
}

func TestWebhookDispatcherStop_IsIdempotent(t *testing.T) {
	dispatcher := newWebhookDispatcher()
	dispatcher.Stop()
	dispatcher.Stop()
}