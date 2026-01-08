#include "queue_callback.h"
#include "dpi.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Function to notify Go channel with the given handle
// This is a forward declaration; the actual implementation is in Go via cgo.Handle
extern void notifyGoChannel(uintptr_t handle);

/**
 * onQueueNotification - ODPI-C callback invoked when queue notification arrives
 * 
 * This function is called by Oracle when a message is enqueued to a subscribed queue.
 * It receives the notification and signals the Go layer via a channel.
 * 
 * @param context: User-provided context (contains Go channel handle)
 * @param message: Contains notification metadata (queue name, event type, etc.)
 */
static void onQueueNotification(void* context, dpiSubscrMessage* message) {
    if (context == NULL) {
        fprintf(stderr, "[OCI CALLBACK] Error: Context is NULL in onQueueNotification\n");
        return;
    }

    // Convert context to uintptr_t to pass to Go
    uintptr_t goHandle = (uintptr_t)context;

    // Debug Information
    #ifdef DEBUG_NOTIFICATIONS
    printf("[OCI CALLBACK] Notification received\n");
    printf("  Event Type: %d\n", message->eventType);
    printf("  Queue Name: %.*s\n", message->queueNameLength, message->queueName);
    if (message->consumerName) {
        printf("  Consumer Name: %.*s\n", message->consumerNameLength, message->consumerName);
    }
    #endif

    // Notify the Go channel
    notifyGoChannel(goHandle);
}

/**
 * RegisterOracleSubscription - Registers a subscription for Oracle AQ notifications
 * 
 * This function sets up a subscription on the given connection to listen for
 * notifications on the specified queue. It configures the subscription parameters,
 * including the callback function and context.
 * 
 * @param conn: Pointer to the dpiConn representing the Oracle connection
 * @param context: Pointer to the dpiContext for error handling
 * @param queueName: Name of the queue to subscribe to
 * @param subscriberName: Name of the subscriber (queue consumer)
 * @param goChannelHandle: Handle to the Go channel for notifications
 * @param outSubscr: Output parameter to receive the created subscription object
 * 
 * @return: DPI_SUCCESS on success, DPI_FAILURE on error
 */
int RegisterOracleSubscription(dpiConn* conn, dpiContext* context, const char* queueName, const char* subscriberName, uintptr_t goChannelHandle, dpiSubscr** outSubscr) {
    dpiSubscrCreateParams createParams;
    dpiErrorInfo errorInfo;

    // 1. Initialize subscription parameters
    if (dpiContext_initSubscrCreateParams(context, &createParams) != DPI_SUCCESS) {
        dpiContext_getError(context, &errorInfo);
        fprintf(stderr, "[OCI ERROR] Failed to initialize subscription parameters: %s\n", errorInfo.message);
        return DPI_FAILURE;
    }

    // 2. Configure subscribtion for AQ namespace
    createParams.subscrNamespace = DPI_SUBSCR_NAMESPACE_AQ;
    createParams.protocol = DPI_SUBSCR_PROTO_CALLBACK; // Callback protocol

    // Quality of Service
    createParams.qos = DPI_SUBSCR_QOS_RELIABLE; // Reliable delivery

    // 3. Set callbacks
    createParams.callback = onQueueNotification; // C callback function
    createParams.callbackContext = (void*)goChannelHandle; // Pass Go channel handle

    // 4. Set queue name and subscriber name
    // For Multi-Consumer queues, OCI expects consumer name. 
    // While recipientName is supported, some OCI configurations require the name to be formatted as "QUEUE:CONSUMER"
    // We will set both for maximum compatibility.
    
    static char subscriptionName[512]; // Static buffer to ensure validity during call (though stack usually fine for sync call)
    memset(subscriptionName, 0, sizeof(subscriptionName));

    if (subscriberName != NULL && strlen(subscriberName) > 0) {
        // Format: "QUEUE:CONSUMER"
        snprintf(subscriptionName, sizeof(subscriptionName), "%s:%s", queueName, subscriberName);
        createParams.name = subscriptionName;
        createParams.nameLength = (uint32_t)strlen(subscriptionName);
        
        createParams.recipientName = subscriberName;
        createParams.recipientNameLength = (uint32_t)strlen(subscriberName);
    } 

    // 5. Create the subscription
    if (dpiConn_subscribe(conn, &createParams, outSubscr) != DPI_SUCCESS) {
        dpiContext_getError(context, &errorInfo);
        fprintf(stderr, "[OCI ERROR] Failed to create subscription: %s\n", errorInfo.message);
        return DPI_FAILURE;
    }

    printf("[OCI INFO] Subscription to queue '%s' for subscriber '%s' registered successfully.\n", queueName, subscriberName ? subscriberName : "ANY");
    return DPI_SUCCESS;
}

/**
 * UnregisterOracleSubscription - Unregisters an Oracle AQ subscription
 * 
 * This function releases the resources associated with the given subscription.
 * 
 * @param subscr: Pointer to the dpiSubscr representing the subscription to be released
 */
void UnregisterOracleSubscription(dpiSubscr* subscr) {
    if (subscr != NULL) {
        dpiSubscr_release(subscr);
        printf("[OCI INFO] Subscription unregistered successfully.\n");
    }
}