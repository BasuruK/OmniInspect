#ifndef DEQUEUE_OPS_H
#define DEQUEUE_OPS_H

#include <dpi.h>
#include <stdint.h>

typedef struct {
	char* data;
	uint64_t length;
} TraceMessage;

typedef struct {
	char* data;
	uint32_t length;
} TraceId;

// Blocking dequeue with wait time
// waitTime : -1 = wait indefinitely, 0 = no wait, >0 = wait time in seconds
int DequeueManyAndExtract(dpiConn* conn, dpiContext* context, const char* schemaName, const char* subscriberName, uint32_t batchSize, int32_t waitTime, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount);

void FreeDequeueResults (TraceMessage* messages, TraceId* ids, uint32_t count);


#endif // DEQUEUE_OPS_H