#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "dpi.h"
#include "queue_callback.h"

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
    dpiSubscr *subscr;

     if (conn == NULL || context == NULL || queueName == NULL || queueName[0] == '\0' || subscriberName == NULL || subscriberName[0] == '\0' || outSubscr == NULL) {
        fprintf(stderr, "[C ERROR] Queue name, subscriber name, conn, context or output pointer is missing\n");
        fflush(stderr);
        return DPI_FAILURE;
    }

    // 1. Initialize subscription parameters
    if (dpiContext_initSubscrCreateParams(context, &createParams) != DPI_SUCCESS) {
        dpiContext_getError(context, &errorInfo);
        fprintf(stderr, "[C ERROR] Failed to init params: %s\n", errorInfo.message);
        fflush(stderr);
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
    char *subscriptionName = NULL;

    if (subscriberName != NULL && strlen(subscriberName) > 0) {
        size_t subscriptionNameLen = strlen(queueName) + 1 + strlen(subscriberName) + 1;
        subscriptionName = (char*)malloc(subscriptionNameLen);

        if (subscriptionName == NULL) {
            fprintf(stderr, "[C ERROR] Failed to allocate memory for subscription name\n");
            fflush(stderr);
            return DPI_FAILURE;
        }

        // Format: "QUEUE:CONSUMER"
        snprintf(subscriptionName, subscriptionNameLen, "%s:%s", queueName, subscriberName);

        createParams.name = subscriptionName;
        createParams.nameLength = (uint32_t)strlen(subscriptionName);
    
        int result = dpiConn_subscribe(conn, &createParams, &subscr);
        if (result != DPI_SUCCESS) {
            dpiContext_getError(context, &errorInfo);
            fprintf(stderr, "[C ERROR] Failed to create subscription: %s\n", errorInfo.message);
            fflush(stderr);
            free(subscriptionName);
            return DPI_FAILURE;
        }
        *outSubscr = subscr;
    
        // Clean up
        free(subscriptionName);
        return DPI_SUCCESS;
    } else {
        fprintf(stderr, "[C ERROR] Subscription name or subscriber name is missing\n");
        fflush(stderr);
        return DPI_FAILURE;
    }
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
    }
}