package subscription

/*
#include <stdint.h>
*/
import "C"
import (
	"fmt"
	"runtime/cgo"
)

//export notifyGoChannel
func notifyGoChannel(handle C.uintptr_t) {
	fmt.Println("[GO] notifyGoChannel Called.")
	h := cgo.Handle(handle)

	// Retrieve the Go channel from the handle
	ch := h.Value().(chan<- struct{})

	// Non-blocking send to the channel, drop if the channel is full
	select {
	case ch <- struct{}{}:
		// Notification sent successfully
	default:
		// Channel is full, skip sending to avoid blocking
		fmt.Println("[GO] Channel is Full, Skipping sending to avoid blocking")
	}
}
