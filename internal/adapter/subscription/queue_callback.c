#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "dpi.h"
#include "queue_callback.h"
#include "_cgo_export.h"

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
    fprintf(stderr, "=== [C CALLBACK] onQueueNotification CALLED ===\n");
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
    // For Multi-Consumer queues, OCI expects detailed specification.
    // We will construct "QUEUE:CONSUMER" format which is the standard fallback when explicit fields fail.

    size_t len = strlen(queueName) + strlen(subscriberName) + 10;
    char *subscriptionName = (char*)malloc(len);
    if (subscriptionName == NULL) {
         fprintf(stderr, "[C ERROR] Failed to allocate memory for subscription name\n");
         return DPI_FAILURE;
    }
    
    // Format: QUEUE_NAME:CONSUMER_NAME
    // We try WITHOUT extra quotes first, assuming the names are correct (usually uppercase).
    // If the user passes "SCHEMA.QUEUE", then it becomes "SCHEMA.QUEUE:CONSUMER"
    snprintf(subscriptionName, len, "%s:%s", queueName, subscriberName);
    
    createParams.name = subscriptionName;
    createParams.nameLength = (uint32_t)strlen(subscriptionName);
    
    // Explicitly NULL out recipientName to avoid confusion
    createParams.recipientName = NULL;
    createParams.recipientNameLength = 0;

    // Enable client-initiated connection for better firewall/NAT traversal
    // This allows the notification to use the existing connection/channel 
    // rather than the DB connecting back to the client.
    createParams.clientInitiated = 1;

    int result = dpiConn_subscribe(conn, &createParams, &subscr);
    
    // Clean up allocated memory immediately after call
    if (subscriptionName != NULL) {
        free(subscriptionName);
    }

    if (result != DPI_SUCCESS) {
        dpiContext_getError(context, &errorInfo);
        fprintf(stderr, "[C ERROR] Failed to create subscription: %s\n", errorInfo.message);
        fflush(stderr);
        return DPI_FAILURE;
    }
    *outSubscr = subscr;

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
    }
}