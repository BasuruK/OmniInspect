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

int DequeueManyAndExtract(dpiConn* conn, dpiContext* ctx, const char* schemaName, const char* subName, uint32_t batchSize, uint32_t waitTime, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount);

void FreeDequeueResults(TraceMessage* messages, TraceId* ids, uint32_t count);
