package tracer

import "testing"

func TestStopAll_DoesNotStopGlobalWebhookDispatcher(t *testing.T) {
	previousDispatcher := globalWebhookDispatcher

	dispatcher := newWebhookDispatcher()
	globalWebhookDispatcher = dispatcher

	t.Cleanup(func() {
		dispatcher.Stop()
		globalWebhookDispatcher = previousDispatcher
	})

	StopAll(&TracerService{})

	if dispatcher.stopped {
		t.Fatal("expected StopAll to leave the global webhook dispatcher running")
	}
}

func TestWebhookDispatcherStop_IsIdempotent(t *testing.T) {
	dispatcher := newWebhookDispatcher()
	dispatcher.Stop()
	dispatcher.Stop()
}
