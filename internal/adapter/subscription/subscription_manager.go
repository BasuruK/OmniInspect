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
	"OmniView/internal/core/domain"
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// SubscriptionHandle represents a subscription handle in ODPI-C
type SubscriptionHandle struct {
	subscr     *C.dpiSubscr
	goHandle   cgo.Handle
	subscriber domain.Subscriber
}

// SubscriptionManager manages database subscriptions for AQ
type SubscriptionManager struct {
	connection          *C.dpiConn
	context             *C.dpiContext
	activeSubscriptions map[string]*SubscriptionHandle
	handles             map[string]cgo.Handle
}

// NewSubscriptionManager creates a new SubscriptionManager instance
// Accepts unsafe.Pointer to allow cross-package CGO type compatibility
func NewSubscriptionManager(connPtr unsafe.Pointer, ctxPtr unsafe.Pointer) *SubscriptionManager {
	return &SubscriptionManager{
		connection:          (*C.dpiConn)(connPtr),
		context:             (*C.dpiContext)(ctxPtr),
		activeSubscriptions: make(map[string]*SubscriptionHandle),
		handles:             make(map[string]cgo.Handle),
	}
}

// Subscribe registers a new subscription for the given subscriber name
func (sm *SubscriptionManager) Subscribe(subscriber domain.Subscriber, schema string, notifyChan chan<- struct{}) error {
	if sm.connection == nil || sm.context == nil {
		return fmt.Errorf("invalid database connection or context")
	}

	// Check if already subscribed TODO: remove if its causing regular errors
	if _, exists := sm.activeSubscriptions[subscriber.Name]; exists {
		return fmt.Errorf("subscription already exists for: %s", subscriber.Name) // Already subscribed
	}

	handle := cgo.NewHandle(notifyChan)

	cQueueName := C.CString(fmt.Sprintf("%s.%s", schema, domain.QueueName))
	//cQueueName := C.CString(domain.QueueName)
	defer C.free(unsafe.Pointer(cQueueName))

	cSubscriberName := C.CString(subscriber.Name)
	defer C.free(unsafe.Pointer(cSubscriberName))

	var subscr *C.dpiSubscr

	sm.handles[subscriber.Name] = handle // Store the handle to manage its lifecycle

	result := C.RegisterOracleSubscription(sm.connection, sm.context, cQueueName, cSubscriberName, C.uintptr_t(handle), &subscr)

	if result != C.DPI_SUCCESS {
		handle.Delete()
		return fmt.Errorf("failed to register oracle subscription for: %s", subscriber.Name)
	}

	sm.activeSubscriptions[subscriber.Name] = &SubscriptionHandle{
		subscr:     subscr,
		goHandle:   handle,
		subscriber: subscriber,
	}

	return nil
}

// Unsubscribe removes an existing subscription for the given subscriber name
func (sm *SubscriptionManager) Unsubscribe(subscriber domain.Subscriber) error {
	subscription, exists := sm.activeSubscriptions[subscriber.Name]
	if !exists {
		return fmt.Errorf("no active subscription found for: %s", subscriber.Name)
	}

	// Unregister the subscription in ODPI-C and release
	C.UnregisterOracleSubscription(subscription.subscr)

	// Release the Go handle
	subscription.goHandle.Delete()

	// Remove from active subscriptions map
	delete(sm.activeSubscriptions, subscriber.Name)

	return nil
}

// UnsubscribeAll removes all active subscriptions
func (sm *SubscriptionManager) UnsubscribeAll() {
	for subscriber := range sm.activeSubscriptions {
		_ = sm.Unsubscribe(sm.activeSubscriptions[subscriber].subscriber)
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
