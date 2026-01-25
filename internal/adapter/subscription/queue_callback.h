#ifndef QUEUE_CALLBACKS_H
#define QUEUE_CALLBACKS_H

#include "dpi.h"
#include <stdint.h>
#include <string.h>

/**
 * RegisterOracleSubscription - Registers a subscription for Oracle AQ notifications
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
int RegisterOracleSubscription(dpiConn* conn, dpiContext* context, const char* queueName, const char* subscriberName, uintptr_t goChannelHandle, dpiSubscr** outSubscr);

/**
 * UnregisterOracleSubscription - Unregisters an existing Oracle AQ subscription
 * 
 * @param subscr: Pointer to the dpiSubscr representing the subscription to be unregistered
 */
void UnregisterOracleSubscription(dpiSubscr* subscr);

#endif