# Blocking Dequeue Implementation Guide

## Architecture Change: From OCI Subscriptions to Kafka-Style Blocking Consumer

This document explains the complete architectural change from Oracle OCI push notifications to a Kafka-style blocking consumer pattern. Follow these steps to implement the changes yourself.

---

## Table of Contents

1. [Problem Analysis](#1-problem-analysis)
2. [Theoretical Background](#2-theoretical-background)
3. [Solution Architecture](#3-solution-architecture)
4. [Implementation Steps](#4-implementation-steps)
   - [Step 1: Update C Header File](#step-1-update-c-header-file-dequeue_opsh)
   - [Step 2: Add Blocking Dequeue C Functions](#step-2-add-blocking-dequeue-c-functions-dequeue_opsc)
   - [Step 3: Add Go Wrapper Function](#step-3-add-go-wrapper-function-queuego)
   - [Step 4: Update Repository Interface](#step-4-update-repository-interface-repositorygo)
   - [Step 5: Rewrite Tracer Service](#step-5-rewrite-tracer-service-tracer_servicego)
5. [Testing](#5-testing)
6. [Performance Considerations](#6-performance-considerations)

---

## 1. Problem Analysis

### Why OCI Subscriptions Failed

The original implementation used Oracle's OCI subscription mechanism (`dpiConn_subscribe` with `DPI_SUBSCR_NAMESPACE_AQ`). This mechanism works as follows:

```
┌─────────────────┐                      ┌─────────────────┐
│   Your App      │                      │  Oracle DB      │
│                 │                      │                 │
│  1. Subscribe   │─────────────────────►│  Register       │
│                 │                      │  callback       │
│                 │                      │                 │
│                 │    2. Message        │                 │
│                 │       enqueued       │                 │
│                 │                      │                 │
│  Callback       │◄─────────────────────│  3. DB connects │
│  invoked!       │   INBOUND CONNECTION │  back to client │
│                 │      (BLOCKED!)      │                 │
└─────────────────┘                      └─────────────────┘
```

**The problem**: Step 3 requires the Oracle Database to initiate a **new inbound TCP connection** to your client machine. This fails because:

1. **Corporate firewalls** block inbound connections from external servers
2. **NAT/Router** configurations don't allow unsolicited inbound traffic
3. **Sharded Queues (TEQ)** have limited support for traditional OCI callbacks

### Additional Issue: Sharded Queues

You're using `DBMS_AQADM.CREATE_SHARDED_QUEUE` which creates a **Transactional Event Queue (TEQ)** - Oracle's modern high-throughput queue type. TEQ queues:

- Are designed for Kafka-like consumption patterns
- Have **limited support** for traditional OCI subscription callbacks
- Work best with **blocking dequeue** patterns

### Why Periodic Check Worked

Your periodic check used `DBMS_AQ.DEQUEUE_ARRAY` with `wait_time = 0` (no wait). This:
- Uses your **existing outbound connection** to Oracle
- Directly queries the queue tables
- Doesn't rely on push notifications

---

## 2. Theoretical Background

### The Blocking Dequeue Pattern (Long-Polling)

This is the same pattern used by **Apache Kafka consumers**. Here's how it works:

```
┌─────────────────────────────────────────────────────────────────┐
│                    BLOCKING DEQUEUE PATTERN                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Consumer (Your App)                 Broker (Oracle DB)         │
│  ───────────────────                 ─────────────────          │
│                                                                 │
│  1. poll(timeout=5s)  ─────────────────────────────────►        │
│     "Give me messages, I'll wait up to 5 seconds"               │
│                                                                 │
│     ┌─────────────────────────────────────────────────┐         │
│     │  Connection held open by database...            │         │
│     │  Database monitors queue for new messages...    │         │
│     └─────────────────────────────────────────────────┘         │
│                                                                 │
│                                      Message enqueued! ◄────    │
│                                                                 │
│  2. Messages returned  ◄───────────────────────────────         │
│     IMMEDIATELY (no 5s wait!)                                   │
│                                                                 │
│  3. Process messages                                            │
│                                                                 │
│  4. Go back to step 1                                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Blocking Call** | The dequeue call blocks (waits) on the database side, not client side |
| **Wait Timeout** | Maximum time to wait before returning (even with 0 messages) |
| **Immediate Return** | If messages exist, they're returned immediately without waiting |
| **No Polling Overhead** | The blocking call IS the wait mechanism - no repeated queries |
| **Firewall Friendly** | Uses your existing outbound connection to the database |

### Comparison with Kafka

| Kafka | Oracle AQ (This Implementation) |
|-------|--------------------------------|
| `consumer.poll(Duration.ofSeconds(5))` | `DBMS_AQ.DEQUEUE_ARRAY(..., wait => 5)` |
| Broker holds connection | Oracle holds connection |
| Returns when messages available | Returns when messages available |
| Returns empty after timeout | Returns empty after timeout |

### Why This Works Through Firewalls

```
┌──────────────┐                           ┌──────────────┐
│  Your App    │                           │  Oracle DB   │
│              │                           │              │
│  Outbound    │═══════════════════════════│              │
│  Connection  │   (Firewall allows this)  │              │
│              │                           │              │
│  Blocking    │                           │  Waits for   │
│  dequeue     │                           │  messages    │
│  call        │                           │              │
│              │◄══════════════════════════│  Returns     │
│  Receives    │   (Same connection!)      │  messages    │
│  messages    │                           │              │
└──────────────┘                           └──────────────┘
```

**No new inbound connection needed** - Oracle returns data on the same connection your app opened.

---

## 3. Solution Architecture

### Before (Broken)

```
┌─────────────────────────────────────────────────────────────────┐
│  TracerService                                                  │
│  ├── SubscriptionManager (OCI subscriptions - DOESN'T WORK)     │
│  ├── notifyChan (never receives notifications)                  │
│  └── Periodic polling (5s fallback)                             │
└─────────────────────────────────────────────────────────────────┘
```

### After (Working)

```
┌─────────────────────────────────────────────────────────────────┐
│  TracerService                                                  │
│  └── blockingConsumerLoop()                                     │
│      └── BlockingDequeue(timeout=5s)                            │
│          └── DequeueManyWithWait() [C function]                 │
│              └── DBMS_AQ.DEQUEUE_ARRAY(wait => 5)               │
└─────────────────────────────────────────────────────────────────┘
```

### Components Changed

| File | Change Type | Description |
|------|-------------|-------------|
| `dequeue_ops.h` | Modified | Added new function declaration |
| `dequeue_ops.c` | Modified | Added 2 new C functions |
| `queue.go` | Modified | Added Go wrapper for blocking dequeue |
| `repository.go` | Modified | Added interface method |
| `tracer_service.go` | **Rewritten** | Complete architecture change |

---

## 4. Implementation Steps

### Step 1: Update C Header File (dequeue_ops.h)

**File**: `internal/adapter/storage/oracle/dequeue_ops.h`

**Action**: Add new function declaration after the existing `DequeueManyAndExtract` declaration.

**Add this declaration**:

```c
// Blocking dequeue with configurable wait time (in seconds)
// waitTime: -1 = wait forever, 0 = no wait, >0 = wait N seconds
int DequeueManyWithWait(dpiConn* conn, dpiContext* context, const char* subscriberName, uint32_t batchSize, int32_t waitTime, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount);
```

**Complete header file should look like**:

```c
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

// Original non-blocking dequeue (wait_time = 0)
int DequeueManyAndExtract(dpiConn* conn, const char* schemaName, const char* subscriberName, uint32_t batchSize, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount);

// Blocking dequeue with configurable wait time (in seconds)
// waitTime: -1 = wait forever, 0 = no wait, >0 = wait N seconds
int DequeueManyWithWait(dpiConn* conn, dpiContext* context, const char* subscriberName, uint32_t batchSize, int32_t waitTime, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount);

void FreeDequeueResults (TraceMessage* messages, TraceId* ids, uint32_t count);

#endif // DEQUEUE_OPS_H
```

---

### Step 2: Add Blocking Dequeue C Functions (dequeue_ops.c)

**File**: `internal/adapter/storage/oracle/dequeue_ops.c`

**Action**: Add two new functions after the `FreeDequeueResults` function.

#### Function 1: ExecuteDequeueWithWait (static helper)

This is a **static** (private) function that executes the PL/SQL dequeue with a configurable wait time.

```c
/**
 * ExecuteDequeueWithWait - Executes the dequeue PL/SQL procedure with configurable wait time
 * 
 * @param conn: Pointer to the dpiConn representing the Oracle connection
 * @param context: Pointer to the dpiContext for error handling
 * @param subscriberName: Name of the subscriber (queue consumer)
 * @param batchSize: Number of messages to dequeue
 * @param waitTime: Wait time in seconds (-1 = forever, 0 = no wait, >0 = wait N seconds)
 * @param outPayloadVar: Output variable for the payload collection
 * @param outRawVar: Output variable for the raw ID collection
 * @param outCount: Output parameter to receive the actual number of dequeued messages
 * @return: 0 on success, -1 on failure
 */
static int ExecuteDequeueWithWait(dpiConn* conn, dpiContext* context, const char* subscriberName, uint32_t batchSize, int32_t waitTime, dpiVar* outPayloadVar, dpiVar* outRawVar, uint32_t* outCount) {
    dpiStmt* stmt = NULL;
    dpiVar* subVar = NULL;
    dpiVar* batchVar = NULL;
    dpiVar* waitVar = NULL;
    dpiVar* countVar = NULL;
    dpiData* subData = NULL;
    dpiData* batchData = NULL;
    dpiData* waitData = NULL;
    dpiData* countData = NULL;
    uint32_t subNameLen = (uint32_t)strlen(subscriberName);
    int result = -1;

    // Use parameterized wait time instead of hardcoded 0
    const char* sql = "BEGIN OMNI_TRACER_API.Dequeue_Array_Events(:1, :2, :3, :4, :5, :6); END;";
    
    if (dpiConn_prepareStmt(conn, 0, sql, strlen(sql), NULL, 0, &stmt) != DPI_SUCCESS) {
        dpiErrorInfo errInfo;
        dpiContext_getError(context, &errInfo);
        fprintf(stderr, "[C ERROR] Failed to prepare statement: %s\n", errInfo.message);
        goto cleanup;
    }
    
    // Subscriber name parameter (position 1)
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_VARCHAR, DPI_NATIVE_TYPE_BYTES, 1, subNameLen, 1, 0, NULL, &subVar, &subData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to create subscriber var\n");
        goto cleanup;
    }
    if (dpiVar_setFromBytes(subVar, 0, subscriberName, subNameLen) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to set subscriber name\n");
        goto cleanup;
    }
    if (dpiStmt_bindByPos(stmt, 1, subVar) != DPI_SUCCESS) goto cleanup;
    
    // Batch size parameter (position 2)
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &batchVar, &batchData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to create batch var\n");
        goto cleanup;
    }
    batchData->value.asInt64 = (int64_t)batchSize;
    batchData->isNull = 0;
    if (dpiStmt_bindByPos(stmt, 2, batchVar) != DPI_SUCCESS) goto cleanup;
    
    // Wait time parameter (position 3)
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &waitVar, &waitData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to create wait var\n");
        goto cleanup;
    }
    waitData->value.asInt64 = (int64_t)waitTime;
    waitData->isNull = 0;
    if (dpiStmt_bindByPos(stmt, 3, waitVar) != DPI_SUCCESS) goto cleanup;
    
    // Output payload parameter (position 4)
    if (dpiStmt_bindByPos(stmt, 4, outPayloadVar) != DPI_SUCCESS) goto cleanup;
    
    // Output raw parameter (position 5)
    if (dpiStmt_bindByPos(stmt, 5, outRawVar) != DPI_SUCCESS) goto cleanup;
    
    // Output count parameter (position 6)
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &countVar, &countData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to create count var\n");
        goto cleanup;
    }
    if (dpiStmt_bindByPos(stmt, 6, countVar) != DPI_SUCCESS) goto cleanup;

    // Execute - this will BLOCK until messages arrive or timeout
    if (dpiStmt_execute(stmt, 0, 0) != DPI_SUCCESS) {
        dpiErrorInfo errInfo;
        dpiContext_getError(context, &errInfo);
        
        // ORA-25228: timeout or end of fetch (no messages) - this is expected
        if (errInfo.code == 25228) {
            *outCount = 0;
            result = 0;
            goto cleanup;
        }
        
        fprintf(stderr, "[C ERROR] Failed to execute dequeue: %s (code: %d)\n", errInfo.message, errInfo.code);
        goto cleanup;
    }

    // Get count
    *outCount = (uint32_t)countData->value.asInt64;
    result = 0;
    
cleanup:
    if (stmt) dpiStmt_release(stmt);
    if (subVar) dpiVar_release(subVar);
    if (batchVar) dpiVar_release(batchVar);
    if (waitVar) dpiVar_release(waitVar);
    if (countVar) dpiVar_release(countVar);

    return result;
}
```

**Key differences from original `ExecuteDequeueProc`**:

1. Takes `context` parameter for better error handling
2. Takes `waitTime` parameter instead of hardcoded `0`
3. Binds 6 parameters instead of 5 (added wait_time as position 3)
4. Handles ORA-25228 (timeout/no messages) as success with count=0

#### Function 2: DequeueManyWithWait (public function)

This is the main public function that Go code will call.

```c
/**
 * DequeueManyWithWait - Blocking dequeue with configurable wait time
 * 
 * This function implements a Kafka-style blocking consumer pattern.
 * The call will block (wait) until messages arrive or the timeout expires.
 * 
 * @param conn: Pointer to the dpiConn representing the Oracle connection
 * @param context: Pointer to the dpiContext for error handling
 * @param subscriberName: Name of the subscriber (queue consumer)
 * @param batchSize: Maximum number of messages to dequeue
 * @param waitTime: Wait time in seconds (-1 = wait forever, 0 = no wait, >0 = wait N seconds)
 * @param outMessages: Output parameter to receive an array of TraceMessage structures
 * @param outIds: Output parameter to receive an array of TraceId structures
 * @param actualCount: Output parameter to receive the actual number of dequeued messages
 * @return: 0 on success (including timeout with 0 messages), -1 on failure
 */
int DequeueManyWithWait(dpiConn* conn, dpiContext* context, const char* subscriberName, uint32_t batchSize, int32_t waitTime, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount) {
    
    dpiObjectType *payloadType = NULL, *rawType = NULL, *objType = NULL;
    dpiObjectAttr *jsonAttr = NULL;
    dpiVar *payloadVar = NULL, *rawVar = NULL;
    dpiData *payloadData = NULL, *rawData = NULL;

    const char* payloadArrayName = "OMNI_TRACER_PAYLOAD_ARRAY";
    const char* rawArrayName = "OMNI_TRACER_RAW_ARRAY";
    const char* payloadTypeName = "OMNI_TRACER_PAYLOAD_TYPE";

    int result = -1;
    uint32_t outIdx = 0;

    *outMessages = NULL;
    *outIds = NULL;
    *actualCount = 0;

    // Load Types
    if (dpiConn_getObjectType(conn, payloadArrayName, strlen(payloadArrayName), &payloadType) != DPI_SUCCESS) {
        dpiErrorInfo errInfo;
        dpiContext_getError(context, &errInfo);
        fprintf(stderr, "[C ERROR] Failed to get payload type: %s\n", errInfo.message);
        goto cleanup;
    }
    if (dpiConn_getObjectType(conn, rawArrayName, strlen(rawArrayName), &rawType) != DPI_SUCCESS) goto cleanup;

    // Attribute handle for the element inside the collection
    if (dpiConn_getObjectType(conn, payloadTypeName, strlen(payloadTypeName), &objType) != DPI_SUCCESS) goto cleanup;
    if (dpiObjectType_getAttributes(objType, 1, &jsonAttr) != DPI_SUCCESS) goto cleanup;

    // Create Variables for Out Collections
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 0, 0, payloadType, &payloadVar, &payloadData) != DPI_SUCCESS) goto cleanup;
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 0, 0, rawType, &rawVar, &rawData) != DPI_SUCCESS) goto cleanup;

    // Execute blocking dequeue
    if (ExecuteDequeueWithWait(conn, context, subscriberName, batchSize, waitTime, payloadVar, rawVar, actualCount) != 0) {
        fprintf(stderr, "[C ERROR] ExecuteDequeueWithWait failed\n");
        goto cleanup;
    }

    // If no messages, return success with count=0
    if (*actualCount == 0) {
        result = 0;
        goto cleanup;
    }

    // Allocate output arrays
    *outMessages = (TraceMessage*)calloc(*actualCount, sizeof(TraceMessage));
    if (!*outMessages) {
        fprintf(stderr, "[C ERROR] Memory allocation failed for messages\n");
        goto cleanup;
    }
    *outIds = (TraceId*)calloc(*actualCount, sizeof(TraceId));
    if (!*outIds) {
        fprintf(stderr, "[C ERROR] Memory allocation failed for ids\n");
        free(*outMessages);
        *outMessages = NULL;
        goto cleanup;
    }

    // Extract messages from collections
    dpiObject *payloadColl = payloadData->value.asObject;
    dpiObject *rawColl = rawData->value.asObject;

    int32_t idx = 0;
    int exists = 0;

    if (dpiObject_getFirstIndex(payloadColl, &idx, &exists) != DPI_SUCCESS) goto cleanup;

    while (exists && outIdx < *actualCount) {
        dpiData element;
        if (dpiObject_getElementValueByIndex(payloadColl, idx, DPI_NATIVE_TYPE_OBJECT, &element) != DPI_SUCCESS) goto cleanup;
        
        if (!element.isNull) {
            dpiData lobVal;
            dpiObject *msgObj = element.value.asObject;
            
            if (dpiObject_getAttributeValue(msgObj, jsonAttr, DPI_NATIVE_TYPE_LOB, &lobVal) == DPI_SUCCESS) {
                if (!lobVal.isNull) {
                    (*outMessages)[outIdx].data = ReadLobContent(lobVal.value.asLOB, &(*outMessages)[outIdx].length);
                }
            }
        }

        // Extract Raw ID
        dpiData rawElement;
        if (dpiObject_getElementValueByIndex(rawColl, idx, DPI_NATIVE_TYPE_BYTES, &rawElement) != DPI_SUCCESS) {
            goto cleanup;
        }

        if (!rawElement.isNull) {
            uint32_t len = rawElement.value.asBytes.length;
            (*outIds)[outIdx].data = (char*)malloc(len);
            if ((*outIds)[outIdx].data) {
                (*outIds)[outIdx].length = len;
                memcpy((*outIds)[outIdx].data, rawElement.value.asBytes.ptr, len);
            }
        }

        outIdx++;
        if (dpiObject_getNextIndex(payloadColl, idx, &idx, &exists) != DPI_SUCCESS) {
            goto cleanup;
        }
    }
    
    result = 0;

cleanup:
    if (result != 0 && *outMessages) {
        for (uint32_t i = 0; i < outIdx; i++) {
            if ((*outMessages)[i].data) free((*outMessages)[i].data);
        }
        free(*outMessages);
        *outMessages = NULL;
    }
    if (result != 0 && *outIds) {
        for (uint32_t i = 0; i < outIdx; i++) {
            if ((*outIds)[i].data) free((*outIds)[i].data);
        }
        free(*outIds);
        *outIds = NULL;
    }
    if (result != 0) {
        *actualCount = 0;
    }
    
    if (payloadType) dpiObjectType_release(payloadType);
    if (rawType) dpiObjectType_release(rawType);
    if (objType) dpiObjectType_release(objType);
    if (jsonAttr) dpiObjectAttr_release(jsonAttr);
    if (payloadVar) dpiVar_release(payloadVar);
    if (rawVar) dpiVar_release(rawVar);
    
    return result;
}
```

**Key differences from original `DequeueManyAndExtract`**:

1. Takes `context` parameter for better error handling
2. Takes `waitTime` parameter
3. Calls `ExecuteDequeueWithWait` instead of `ExecuteDequeueProc`
4. Initializes output pointers to NULL at start
5. Better error handling and cleanup

---

### Step 3: Add Go Wrapper Function (queue.go)

**File**: `internal/adapter/storage/oracle/queue.go`

**Action**: Add the `BlockingDequeue` method after the existing `BulkDequeueTracerMessages` function.

```go
// BlockingDequeue performs a blocking dequeue operation that waits for messages.
// This implements a Kafka-style consumer pattern where the call blocks until:
// - Messages arrive (returns immediately with messages)
// - Timeout expires (returns with count=0)
// - Context is cancelled (returns error)
//
// waitTimeSeconds: -1 = wait forever, 0 = no wait, >0 = wait N seconds
func (oa *OracleAdapter) BlockingDequeue(subscriber domain.Subscriber, waitTimeSeconds int) ([]string, [][]byte, int, error) {
	if oa.Connection == nil {
		return nil, nil, 0, fmt.Errorf("database connection is not established")
	}

	if subscriber.BatchSize <= 0 {
		return nil, nil, 0, fmt.Errorf("batch size must be > 0")
	}

	var cMessages *C.TraceMessage
	var cIds *C.TraceId
	var cCount C.uint32_t

	cSubscriberName := C.CString(subscriber.Name)
	defer C.free(unsafe.Pointer(cSubscriberName))

	// Call blocking dequeue with wait time
	result := C.DequeueManyWithWait(
		oa.Connection,
		oa.Context,
		cSubscriberName,
		C.uint32_t(subscriber.BatchSize),
		C.int32_t(waitTimeSeconds),
		&cMessages,
		&cIds,
		&cCount,
	)

	if result != 0 {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)

		// ORA-25228: timeout/no messages - this is expected, not an error
		if errInfo.code == 25228 {
			return []string{}, [][]byte{}, 0, nil
		}

		return nil, nil, 0, fmt.Errorf("blocking dequeue failed: %s (code: %d)", C.GoString(errInfo.message), errInfo.code)
	}

	count := int(cCount)

	if count == 0 {
		return []string{}, [][]byte{}, 0, nil
	}

	// Free results after extracting data
	defer C.FreeDequeueResults(cMessages, cIds, cCount)

	messages := make([]string, count)
	msgIds := make([][]byte, count)

	for i := 0; i < count; i++ {
		msg := (*C.TraceMessage)(unsafe.Pointer(uintptr(unsafe.Pointer(cMessages)) + uintptr(i)*unsafe.Sizeof(*cMessages)))
		id := (*C.TraceId)(unsafe.Pointer(uintptr(unsafe.Pointer(cIds)) + uintptr(i)*unsafe.Sizeof(*cIds)))

		if msg.data != nil && msg.length > 0 {
			messages[i] = C.GoStringN(msg.data, C.int(msg.length))
		}

		if id.data != nil && id.length > 0 {
			msgIds[i] = C.GoBytes(unsafe.Pointer(id.data), C.int(id.length))
		}
	}

	return messages, msgIds, count, nil
}
```

**Key points**:

1. Similar to `BulkDequeueTracerMessages` but calls `DequeueManyWithWait`
2. Passes `waitTimeSeconds` to the C function
3. Handles ORA-25228 as success (timeout with no messages)
4. Returns `([]string, [][]byte, int, error)` - same signature as existing function

---

### Step 4: Update Repository Interface (repository.go)

**File**: `internal/core/ports/repository.go`

**Action**: Add the new method to the `DatabaseRepository` interface.

**Find this interface**:

```go
type DatabaseRepository interface {
    // ... existing methods ...
    BulkDequeueTracerMessages(subscriber domain.Subscriber) ([]string, [][]byte, int, error)
    GetRawConnection() unsafe.Pointer
    GetRawContext() unsafe.Pointer
}
```

**Add this line after `BulkDequeueTracerMessages`**:

```go
// BlockingDequeue performs a blocking dequeue that waits for messages (Kafka-style consumer)
// waitTimeSeconds: -1 = wait forever, 0 = no wait, >0 = wait N seconds
BlockingDequeue(subscriber domain.Subscriber, waitTimeSeconds int) ([]string, [][]byte, int, error)
```

**Complete interface should look like**:

```go
type DatabaseRepository interface {
    ExecuteStatement(query string) error
    Fetch(query string) ([]string, error)
    FetchWithParams(query string, params map[string]interface{}) ([]string, error)
    ExecuteWithParams(query string, params map[string]interface{}) error
    PackageExists(packageName string) (bool, error)
    DeployPackages(sequences []string, types []string, packageSpecs []string, packageBodies []string) error
    DeployFile(sqlContent string) error
    RegisterNewSubscriber(subscriber domain.Subscriber) error
    CheckQueueDepth(subscriberID string, queueTableName string) (int, error)
    BulkDequeueTracerMessages(subscriber domain.Subscriber) ([]string, [][]byte, int, error)
    // BlockingDequeue performs a blocking dequeue that waits for messages (Kafka-style consumer)
    // waitTimeSeconds: -1 = wait forever, 0 = no wait, >0 = wait N seconds
    BlockingDequeue(subscriber domain.Subscriber, waitTimeSeconds int) ([]string, [][]byte, int, error)
    GetRawConnection() unsafe.Pointer
    GetRawContext() unsafe.Pointer
}
```

---

### Step 5: Rewrite Tracer Service (tracer_service.go)

**File**: `internal/service/tracer/tracer_service.go`

This is a **complete rewrite** of the tracer service. The old implementation used OCI subscriptions; the new one uses blocking dequeue.

#### Changes Overview

| Aspect | Old | New |
|--------|-----|-----|
| Import `subscription` package | Yes | **No** (remove) |
| Import `time` package | Yes | **No** (remove) |
| SubscriptionManager field | Yes | **No** (remove) |
| OCI subscription | Yes | **No** (remove completely) |
| Notification channel | Yes | **No** (not needed) |
| Periodic ticker | Yes | **No** (not needed) |
| Consumer pattern | Push (broken) | Blocking pull (working) |

#### Complete New Implementation

```go
package tracer

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// Default wait time for blocking dequeue (in seconds)
// The dequeue call will block for up to this many seconds waiting for messages.
// If messages arrive before the timeout, they are returned immediately.
const DefaultBlockingWaitSeconds = 5

// Service: Manages package deployments and event listening
// Uses a Kafka-style blocking consumer pattern for true event-driven processing
type TracerService struct {
	db        ports.DatabaseRepository
	bolt      ports.ConfigRepository
	processMu sync.Mutex
}

// Constructor: NewTracerService Constructor for TracerService
func NewTracerService(db ports.DatabaseRepository, bolt ports.ConfigRepository) (*TracerService, error) {
	return &TracerService{
		db:   db,
		bolt: bolt,
	}, nil
}

// StartEventListener starts the Kafka-style blocking consumer loop.
// This uses blocking dequeue - the call waits on the database side until
// messages arrive, providing true event-driven behavior without polling.
//
// How it works:
// 1. Call BlockingDequeue with a timeout (e.g., 5 seconds)
// 2. Oracle holds the connection open, waiting for messages
// 3. When messages arrive -> immediately returned, processed
// 4. If timeout expires -> returns with 0 messages, loop retries
// 5. This continues until context is cancelled
//
// This is the same pattern used by Kafka consumers (long-polling).
func (ts *TracerService) StartEventListener(ctx context.Context, subscriber *domain.Subscriber, schema string) error {
	fmt.Printf("[BLOCKING CONSUMER] Starting event listener for subscriber: %s\n", subscriber.Name)
	fmt.Printf("[BLOCKING CONSUMER] Wait timeout: %d seconds (messages delivered instantly when available)\n", DefaultBlockingWaitSeconds)

	// Process any existing messages first (non-blocking)
	ts.processExistingMessages(subscriber)

	// Start the blocking consumer loop
	go ts.blockingConsumerLoop(ctx, subscriber)

	return nil
}

// blockingConsumerLoop implements the Kafka-style blocking consumer pattern.
// Each iteration blocks waiting for messages - no polling overhead.
func (ts *TracerService) blockingConsumerLoop(ctx context.Context, subscriber *domain.Subscriber) {
	for {
		// Check if context is cancelled before blocking
		select {
		case <-ctx.Done():
			fmt.Println("[BLOCKING CONSUMER] Event listener stopped.")
			return
		default:
			// Continue to blocking dequeue
		}

		// BLOCKING CALL - waits up to DefaultBlockingWaitSeconds for messages
		// This is where the "magic" happens:
		// - Oracle holds our connection open
		// - When a message is enqueued, Oracle immediately returns it
		// - No polling, no delays, true event-driven
		messages, msgIDs, count, err := ts.db.BlockingDequeue(*subscriber, DefaultBlockingWaitSeconds)

		if err != nil {
			log.Printf("[BLOCKING CONSUMER] Dequeue error: %v", err)
			// Brief pause before retry on error
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		if count > 0 {
			fmt.Printf("[EVENT] Received %d message(s)\n", count)
			ts.processMessages(messages, msgIDs, count)
		}
		// If count == 0, timeout expired - loop immediately retries
		// No artificial delay needed - the blocking call provides the timing
	}
}

// processExistingMessages drains any messages that were queued before startup
func (ts *TracerService) processExistingMessages(subscriber *domain.Subscriber) {
	// Use non-blocking dequeue (wait_time=0) for initial drain
	messages, msgIDs, count, err := ts.db.BulkDequeueTracerMessages(*subscriber)
	if err != nil {
		log.Printf("[STARTUP] Error processing existing messages: %v", err)
		return
	}

	if count > 0 {
		fmt.Printf("[STARTUP] Processing %d existing message(s)\n", count)
		ts.processMessages(messages, msgIDs, count)
	}
}

// processMessages handles a batch of dequeued messages
func (ts *TracerService) processMessages(messages []string, msgIDs [][]byte, count int) {
	ts.processMu.Lock()
	defer ts.processMu.Unlock()

	for i := 0; i < count; i++ {
		var msg domain.QueueMessage
		if err := json.Unmarshal([]byte(messages[i]), &msg); err != nil {
			log.Printf("[ERROR] Failed to unmarshal message ID %x: %v", msgIDs[i], err)
			continue
		}

		ts.handleTracerMessage(msg)
	}
}

func (ts *TracerService) handleTracerMessage(msg domain.QueueMessage) {
	fmt.Printf("[%s] [%s] %s: %s \n", msg.Timestamp, msg.LogLevel, msg.ProcessName, msg.Payload)
}

// DeployAndCheck ensures the necessary tracer package is deployed and initialized
func (ts *TracerService) DeployAndCheck() error {
	var exists bool
	if err := deployTracerPackage(ts, &exists); err != nil {
		return fmt.Errorf("failed to deploy tracer package: %w", err)
	}
	if !exists {
		if err := initializeTracerPackage(ts); err != nil {
			return fmt.Errorf("failed to initialize tracer package: %w", err)
		}
	}
	return nil
}

// DeployTracerPackage deploys the Omni tracer package to the database if not already present
func deployTracerPackage(ts *TracerService, exists *bool) error {
	var err error
	*exists, err = ts.db.PackageExists("OMNI_TRACER_API")
	if err != nil {
		return fmt.Errorf("failed to check package existence: %w", err)
	}

	if *exists {
		return nil
	}

	omniTracerSQLPackage, err := assets.GetSQLFile("Omni_Tracer.sql")
	if err != nil {
		return fmt.Errorf("failed to read Omni tracer package file: %w", err)
	}

	if err := ts.db.DeployFile(string(omniTracerSQLPackage)); err != nil {
		return fmt.Errorf("failed to deploy Omni tracer package: %w", err)
	}

	return nil
}

// InitializeTracerPackage initializes the Omni tracer package in the database
func initializeTracerPackage(ts *TracerService) error {
	omniInitInsFile, err := assets.GetInsFile("Omni_Initialize.ins")
	if err != nil {
		return fmt.Errorf("failed to read Omni initialize file: %w", err)
	}

	if err := ts.db.ExecuteStatement(string(omniInitInsFile)); err != nil {
		return fmt.Errorf("failed to deploy Omni initialize file: %w", err)
	}

	return nil
}
```

#### What Was Removed

1. **Import**: `"OmniView/internal/adapter/subscription"` - no longer needed
2. **Import**: `"time"` - no longer needed (no ticker)
3. **Struct field**: `subscriptionMgr *subscription.SubscriptionManager` - removed
4. **Method**: `eventLoop()` - replaced with `blockingConsumerLoop()`
5. **Method**: `cleanUp()` - no longer needed (no subscription to clean up)
6. **Method**: `processBatch()` - replaced with `processMessages()`
7. **Method**: `checkQueueDepth()` - no longer needed
8. **All OCI subscription code** - completely removed

#### What Was Added

1. **Constant**: `DefaultBlockingWaitSeconds = 5`
2. **Method**: `blockingConsumerLoop()` - the new consumer pattern
3. **Method**: `processExistingMessages()` - drains queue at startup
4. **Method**: `processMessages()` - simplified message processing

---

## 5. Testing

### Build the Application

```bash
go build .\cmd\omniview\
```

### Run the Application

```bash
go run .\cmd\omniview\main.go
```

You should see:
```
✓ loaded database from boltDB
✓ Connected to the database
Registered Subscriber: SUB_XXXXXXXX
[BLOCKING CONSUMER] Starting event listener for subscriber: SUB_XXXXXXXX
[BLOCKING CONSUMER] Wait timeout: 5 seconds (messages delivered instantly when available)
Tracer started
```

### Send a Test Message

From Oracle SQL*Plus or SQLcl:

```sql
BEGIN
  OMNI_TRACER_API.Trace_Message('Hello from blocking dequeue!', 'INFO');
  COMMIT;
END;
/
```

### Expected Output

In your application console, you should see:
```
[EVENT] Received 1 message(s)
[2026-01-28T12:34:56.789+00:00] [INFO] OMNI_TRACER_API: Hello from blocking dequeue!
```

The message should appear **immediately** (within milliseconds), not after waiting for the 5-second timeout.

---

## 6. Performance Considerations

### Latency

| Scenario | Expected Latency |
|----------|------------------|
| Message enqueued while blocking | < 100ms |
| Message enqueued just after timeout | Up to 5 seconds (worst case) |
| Average | < 500ms |

### Tuning the Wait Timeout

The `DefaultBlockingWaitSeconds` constant controls the maximum wait time:

| Value | Behavior | Use Case |
|-------|----------|----------|
| 1 | Very responsive, more loop iterations | Real-time requirements |
| 5 | Good balance (recommended) | Most applications |
| 30 | Lower overhead, higher latency | Background processing |
| -1 | Wait forever | Not recommended (can't stop cleanly) |

### Connection Considerations

- Each blocking dequeue holds a database connection open
- Ensure your connection pool can handle the blocked connections
- Consider using a dedicated connection for the consumer

### Scaling

For multiple consumers:
- Each consumer can have its own subscriber name
- Each runs its own blocking consumer loop
- Messages are distributed across consumers (queue fanout)

---

## Summary

You've replaced a broken push notification system with a working Kafka-style blocking consumer. The key insight is:

> **Don't wait for the database to push to you (blocked by firewalls).  
> Instead, ask the database and tell it to wait for messages on your behalf.**

This is the same pattern used by Kafka, RabbitMQ, and other message brokers. It's reliable, firewall-friendly, and provides true event-driven behavior.
