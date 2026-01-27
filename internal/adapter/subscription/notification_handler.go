package subscription

/*
#include <stdint.h>
*/
import "C"
import (
	"runtime/cgo"
)

//export notifyGoChannel
func notifyGoChannel(handle C.uintptr_t) {
	h := cgo.Handle(handle)

	// Retrieve the Go channel from the handle
	ch := h.Value().(chan<- struct{})

	// Non-blocking send to the channel, drop if the channel is full
	select {
	case ch <- struct{}{}:
		// Notification sent successfully
	default:
		// Channel is full, skip sending to avoid blocking
	}
}
