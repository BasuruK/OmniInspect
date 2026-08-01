package mcpserver

import "context"

// NotifyStoppedUnexpected fires onStopped without blocking the caller.
// Intentional cancel (ctx already done) skips notification. Unexpected Serve
// exit notifies asynchronously so teardown can finish even if Program.Send
// stalls before tea.Program.Run starts.
func NotifyStoppedUnexpected(ctx context.Context, onStopped func()) {
	if ctx.Err() != nil || onStopped == nil {
		return
	}
	go onStopped()
}
