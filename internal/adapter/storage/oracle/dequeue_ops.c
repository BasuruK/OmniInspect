#ifndef DEQUEUE_OPS_H
#define DEQUEUE_OPS_H

#include "dpi.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include "dpi_go_helpers.h"
#include "dequeue_ops.h"

#define LOG_FAIL(step) fprintf(stderr, "[C DEQUEUE] %s failed\n", step)
#define CHECK_OK(call, step) do { if ((call) != DPI_SUCCESS) { LOG_FAIL(step); return -1; } } while (0)

static char* ReadFullBlob(dpiLob* lob, uint64_t* outLength) {
    uint64_t sizeInBytes = 0;
    char* buffer = NULL;

    if (lob == NULL) {
        if (outLength) *outLength = 0;
        return NULL;
    }

    if (dpiLob_getSize(lob, &sizeInBytes) != DPI_SUCCESS) return NULL;
    if (sizeInBytes == 0) {
        *outLength = 0;
        return NULL;
    }

    buffer = (char*)malloc((size_t)sizeInBytes + 1);
    if (!buffer) return NULL;

    if (dpiLob_readBytes(lob, 1, sizeInBytes, buffer, &sizeInBytes) != DPI_SUCCESS) {
        free(buffer);
        return NULL;
    }

    buffer[sizeInBytes] = '\0';
    *outLength = sizeInBytes;
    return buffer;
}

static int ExtractBulkTraces(uint32_t count, dpiObject* payloadArrayObj, dpiObject* rawArrayObj, dpiObjectAttr* jsonAttr, TraceMessage* outMessages, TraceId* outIds){
    int32_t index = 0;
    int exists = 0;
    uint32_t outIndex = 0;

    CHECK_OK(dpiObject_getFirstIndex(payloadArrayObj, &index, &exists), "dpiObject_getFirstIndex(payloadArrayObj)");

    while (exists && outIndex < count) {
        fprintf(stderr, "[C DEQUEUE] Extract index=%d outIndex=%u\n", index, outIndex);
        dpiData payloadElement;
        CHECK_OK(dpiObject_getElementValueByIndex(payloadArrayObj, index, DPI_NATIVE_TYPE_OBJECT, &payloadElement), "dpiObject_getElementValueByIndex(payloadArrayObj)");

        if (payloadElement.isNull || payloadElement.value.asObject == NULL) {
            fprintf(stderr, "[C DEQUEUE] payloadElement NULL at index=%d\n", index);
            outMessages[outIndex].data = NULL;
            outMessages[outIndex].length = 0;
        } else {
            dpiData lobData;
            CHECK_OK(dpiObject_getAttributeValue(payloadElement.value.asObject, jsonAttr, DPI_NATIVE_TYPE_LOB, &lobData), "dpiObject_getAttributeValue(JSON_DATA)");

            if (lobData.isNull || lobData.value.asLOB == NULL) {
                fprintf(stderr, "[C DEQUEUE] JSON_DATA LOB NULL at index=%d\n", index);
                outMessages[outIndex].data = NULL;
                outMessages[outIndex].length = 0;
            } else {
                outMessages[outIndex].data = ReadFullBlob(lobData.value.asLOB, &outMessages[outIndex].length);
                if (lobData.value.asLOB != NULL) {
                    dpiLob_release(lobData.value.asLOB);
                }
            }

            dpiObject_release(payloadElement.value.asObject);
        }

        dpiData rawElement;
        CHECK_OK(dpiObject_getElementValueByIndex(rawArrayObj, index, DPI_NATIVE_TYPE_BYTES, &rawElement), "dpiObject_getElementValueByIndex(rawArrayObj)");

        if (!rawElement.isNull) {
            outIds[outIndex].length = rawElement.value.asBytes.length;
            outIds[outIndex].data = (char*)malloc(outIds[outIndex].length);
            memcpy(outIds[outIndex].data, rawElement.value.asBytes.ptr, outIds[outIndex].length);
        } else {
            fprintf(stderr, "[C DEQUEUE] msg_id RAW NULL at index=%d\n", index);
            outIds[outIndex].data = NULL;
            outIds[outIndex].length = 0;
        }

        CHECK_OK(dpiObject_getNextIndex(payloadArrayObj, index, &index, &exists), "dpiObject_getNextIndex(payloadArrayObj)");
        outIndex++;
    }

    return 0;
}

static int DequeueManyProxy(dpiConn* conn, dpiContext* ctx, const char* subName, uint32_t batchSize, uint32_t waitTime, dpiVar* outMessages, dpiVar* outIds, uint32_t* actualCount){
    dpiStmt* stmt = NULL;
    const char* sql = "BEGIN OMNI_TRACER_API.Dequeue_Array_Events(:1, :2, :3, :4, :5, :6); END;";

    dpiVar* subVar = NULL;
    dpiData* subData = NULL;
    dpiVar* batchVar = NULL;
    dpiVar* waitVar = NULL;
    dpiVar* countVar = NULL;
    dpiData* batchData = NULL;
    dpiData* waitData = NULL;
    dpiData* countData = NULL;

    // Prepare
    CHECK_OK(dpiConn_prepareStmt(conn, 0, sql, (uint32_t)strlen(sql), NULL, 0, &stmt), "dpiConn_prepareStmt");

    // :1 Subscriber
    CHECK_OK(dpiConn_newVar(conn, DPI_ORACLE_TYPE_VARCHAR, DPI_NATIVE_TYPE_BYTES, 1, 0, 1, 0, NULL, &subVar, &subData), "dpiConn_newVar(subVar)");
    dpiVar_setFromBytes(subVar, 0, subName, (uint32_t)strlen(subName));
    CHECK_OK(dpiStmt_bindByPos(stmt, 1, subVar), "dpiStmt_bindByPos(:1)");

    // :2 Batch size
    CHECK_OK(dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &batchVar, &batchData), "dpiConn_newVar(batchVar)");
    batchData->value.asInt64 = (int64_t)batchSize;
    CHECK_OK(dpiStmt_bindByPos(stmt, 2, batchVar), "dpiStmt_bindByPos(:2)");

    // :3 Wait time
    CHECK_OK(dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &waitVar, &waitData), "dpiConn_newVar(waitVar)");
    waitData->value.asInt64 = (int64_t)waitTime;
    CHECK_OK(dpiStmt_bindByPos(stmt, 3, waitVar), "dpiStmt_bindByPos(:3)");

    // :4 Messages (CLOB array) and :5 IDs (RAW array) are created by caller
    CHECK_OK(dpiStmt_bindByPos(stmt, 4, outMessages), "dpiStmt_bindByPos(:4)");
    CHECK_OK(dpiStmt_bindByPos(stmt, 5, outIds), "dpiStmt_bindByPos(:5)");

    // :6 Actual count
    // Allow NULL for count (defensive)
    CHECK_OK(dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 1, 0, NULL, &countVar, &countData), "dpiConn_newVar(countVar)");
    CHECK_OK(dpiStmt_bindByPos(stmt, 6, countVar), "dpiStmt_bindByPos(:6)");

    // Execute
    if (dpiStmt_execute(stmt, 0, NULL) != DPI_SUCCESS) {
        fprintf(stderr, "[C DEQUEUE] dpiStmt_execute failed\n");
        if (ctx) {
            dpiErrorInfo errInfo;
            dpiContext_getError(ctx, &errInfo);
            fprintf(stderr, "[C DEQUEUE] ORA-%d: %s\n", errInfo.code, errInfo.message);
        }
        return -1;
    }

    // Get count
    if (countData->isNull) {
        fprintf(stderr, "[C DEQUEUE] msg_count_ is NULL; defaulting to 0\n");
        *actualCount = 0;
    } else {
        *actualCount = (uint32_t)countData->value.asInt64;
    }

    // Cleanup
    dpiVar_release(subVar);
    dpiVar_release(batchVar);
    dpiVar_release(waitVar);
    dpiVar_release(countVar);
    dpiStmt_release(stmt);
    return 0;
}

int DequeueManyAndExtract(dpiConn* conn, dpiContext* ctx, const char* schemaName, const char* subName, uint32_t batchSize, uint32_t waitTime, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount){
    dpiVar* payloadVar = NULL;
    dpiVar* rawVar = NULL;
    dpiData* payloadData = NULL;
    dpiData* rawData = NULL;
    dpiObjectType* payloadArrayType = NULL;
    dpiObjectType* rawArrayType = NULL;
    dpiObjectType* payloadObjType = NULL;
    dpiObjectAttr* jsonAttr = NULL;

    CHECK_OK(getObjectType(conn, schemaName, "OMNI_TRACER_PAYLOAD_ARRAY", &payloadArrayType), "getObjectType(OMNI_TRACER_PAYLOAD_ARRAY)");
    CHECK_OK(getObjectType(conn, schemaName, "OMNI_TRACER_RAW_ARRAY", &rawArrayType), "getObjectType(OMNI_TRACER_RAW_ARRAY)");
    CHECK_OK(getObjectType(conn, schemaName, "OMNI_TRACER_PAYLOAD_TYPE", &payloadObjType), "getObjectType(OMNI_TRACER_PAYLOAD_TYPE)");
    CHECK_OK(getObjectAttributeByName(payloadObjType, "JSON_DATA", &jsonAttr), "getObjectAttributeByName(JSON_DATA)");

    // Allow NULL OUT collections by setting nullable=1
    CHECK_OK(dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 1, 0, payloadArrayType, &payloadVar, &payloadData), "dpiConn_newVar(payloadVar)");
    CHECK_OK(dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 1, 0, rawArrayType, &rawVar, &rawData), "dpiConn_newVar(rawVar)");

    if (DequeueManyProxy(conn, ctx, subName, batchSize, waitTime, payloadVar, rawVar, actualCount) != 0) return -1;

    if (payloadData->isNull || rawData->isNull) {
        fprintf(stderr, "[C DEQUEUE] OUT collections are NULL (payload=%d, raw=%d)\n", payloadData->isNull, rawData->isNull);
        *actualCount = 0;
    }

    *outMessages = (TraceMessage*)calloc(*actualCount, sizeof(TraceMessage));
    *outIds = (TraceId*)calloc(*actualCount, sizeof(TraceId));
    if (*actualCount > 0 && (!*outMessages || !*outIds)) return -1;

    if (*actualCount > 0 && !payloadData->isNull && !rawData->isNull) {
        dpiObject* payloadArrayObj = payloadData->value.asObject;
        dpiObject* rawArrayObj = rawData->value.asObject;

        if (payloadArrayObj == NULL || rawArrayObj == NULL) {
            LOG_FAIL("OUT collections are NULL");
            return -1;
        }
        if (ExtractBulkTraces(*actualCount, payloadArrayObj, rawArrayObj, jsonAttr, *outMessages, *outIds) != 0) return -1;
    }

    if (jsonAttr) {
        dpiObjectAttr_release(jsonAttr);
    }
    if (payloadObjType) {
        dpiObjectType_release(payloadObjType);
    }
    if (payloadArrayType) {
        dpiObjectType_release(payloadArrayType);
    }
    if (rawArrayType) {
        dpiObjectType_release(rawArrayType);
    }

    if (payloadData && payloadData->value.asObject) {
        dpiObject_release(payloadData->value.asObject);
    }
    if (rawData && rawData->value.asObject) {
        dpiObject_release(rawData->value.asObject);
    }

    dpiVar_release(payloadVar);
    dpiVar_release(rawVar);
    return 0;
}

void FreeDequeueResults(TraceMessage* messages, TraceId* ids, uint32_t count){
    if (messages) {
        for (uint32_t i = 0; i < count; i++) {
            free(messages[i].data);
        }
        free(messages);
    }

    if (ids) {
        for (uint32_t i = 0; i < count; i++) {
            free(ids[i].data);
        }
        free(ids);
    }
}

#endif