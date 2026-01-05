package subscription

/*
#cgo darwin CFLAGS: -I${SRCDIR}/../../../third_party/odpi/include -I${SRCDIR}
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../third_party/odpi/lib -lodpi -Wl,-rpath,${SRCDIR}/../../../third_party/odpi/lib -Wl,-rpath,/opt/oracle/instantclient_23_7

#cgo windows CFLAGS: -I${SRCDIR}/../../../third_party/odpi/include -I${SRCDIR}
#cgo windows LDFLAGS: -L${SRCDIR}/../../../third_party/odpi/lib -lodpi -LC:/oracle_inst/instantclient_23_7 -loci

#include "dpi.h"
#include "queue_callback.h"
#include <stdio.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// SubscriptionHandle represents a subscription handle in ODPI-C
type SubscriptionHandle struct {
	subscr         *C.dpiSubscr
	goHandle       cgo.Handle
	subscriberName string
}

// SubscriptionManager manages database subscriptions for AQ
type SubscriptionManager struct {
	connection          *C.dpiConn
	context             *C.dpiContext
	activeSubscriptions map[string]*SubscriptionHandle
}

// NewSubscriptionManager creates a new SubscriptionManager instance
// Accepts unsafe.Pointer to allow cross-package CGO type compatibility
func NewSubscriptionManager(ctx unsafe.Pointer, conn unsafe.Pointer) *SubscriptionManager {
	return &SubscriptionManager{
		connection:          (*C.dpiConn)(conn),
		context:             (*C.dpiContext)(ctx),
		activeSubscriptions: make(map[string]*SubscriptionHandle),
	}
}

// Subscribe registers a new subscription for the given subscriber name
func (sm *SubscriptionManager) Subscribe(subscriberName string, notifyChan chan<- struct{}) (string, error) {
	if sm.connection == nil || sm.context == nil {
		return "", fmt.Errorf("invalid database connection or context")
	}

	// Check if already subscribed TODO: remove if its causing regular errors
	if _, exists := sm.activeSubscriptions[subscriberName]; exists {
		return "", fmt.Errorf("subscription already exists for: %s", subscriberName) // Already subscribed
	}

	handle := cgo.NewHandle(notifyChan)

	cSubscriberName := C.CString(subscriberName)
	defer C.free(unsafe.Pointer(cSubscriberName))

	var subscr *C.dpiSubscr

	result := C.RegisterOracleSubscription(sm.connection, sm.context, cSubscriberName, C.uintptr_t(handle), &subscr)

	if result != C.DPI_SUCCESS {
		handle.Delete()
		return "", fmt.Errorf("failed to register oracle subscription for: %s", subscriberName)
	}

	sm.activeSubscriptions[subscriberName] = &SubscriptionHandle{
		subscr:         subscr,
		goHandle:       handle,
		subscriberName: subscriberName,
	}

	return subscriberName, nil
}

// Unsubscribe removes an existing subscription for the given subscriber name
func (sm *SubscriptionManager) Unsubscribe(subscriberName string) error {
	subscription, exists := sm.activeSubscriptions[subscriberName]
	if !exists {
		return fmt.Errorf("no active subscription found for: %s", subscriberName)
	}

	// Unregister the subscription in ODPI-C and release
	C.UnregisterOracleSubscription(subscription.subscr)

	// Release the Go handle
	subscription.goHandle.Delete()

	// Remove from active subscriptions map
	delete(sm.activeSubscriptions, subscriberName)

	return nil
}

// UnsubscribeAll removes all active subscriptions
func (sm *SubscriptionManager) UnsubscribeAll() {
	for subscriberName := range sm.activeSubscriptions {
		_ = sm.Unsubscribe(subscriberName)
	}
}

// GetActiveSubscriptions returns a list of active subscription names
func (sm *SubscriptionManager) GetActiveSubscriptions() []string {
	subs := make([]string, 0, len(sm.activeSubscriptions))
	for name := range sm.activeSubscriptions {
		subs = append(subs, name)
	}

	return subs
}
