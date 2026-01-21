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

int DequeueManyAndExtract(dpiConn* conn, const char* schemaName, const char* subscriberName, uint32_t batchSize, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount);

void FreeDequeueResults (TraceMessage* messages, TraceId* ids, uint32_t count);

static int ExecuteDequeuProc(dpiConn* conn, const char* subscriber_name, uint32_t batchSize, dpiVar* outPayloadVar, dpiVar* outRawVar, uint32_t* outCount);

static char* ReadLobContent(dpiLob* lob, uint64_t* outLength);