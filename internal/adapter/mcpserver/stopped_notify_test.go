package mcpserver

import (
	"context"
	"testing"
	"time"
)

func TestNotifyStoppedUnexpected_DoesNotBlockWhenCallbackHangs(t *testing.T) {
	t.Parallel()

	// Simulates Program.Send stalling because tea.Program.Run never started
	// (early ServeStreamableHTTP exit / failed TUI startup).
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })

	done := make(chan struct{})
	go func() {
		defer close(done)
		NotifyStoppedUnexpected(context.Background(), func() { <-hang })
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("NotifyStoppedUnexpected blocked on OnStopped; stopMCP would hang on done")
	}
}

func TestNotifyStoppedUnexpected_SkipsOnCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := make(chan struct{}, 1)
	NotifyStoppedUnexpected(ctx, func() { called <- struct{}{} })

	select {
	case <-called:
		t.Fatal("OnStopped must not run after intentional cancel")
	case <-time.After(50 * time.Millisecond):
	}
}
