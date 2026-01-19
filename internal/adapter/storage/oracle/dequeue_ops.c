#include <dpi.h>
#include <stdlib.h>
#include <string.h>
#include "dpi_go_helpers.h"
#include "dequeue_ops.h"

static char* ReadFullBlob(dpiLob* lob, uint64_t* outLength) {
    uint64_t sizeInBytes = 0;
    char* buffer = NULL;

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

    if (dpiObject_getFirstIndex(payloadArrayObj, &index, &exists) != DPI_SUCCESS) return -1;

    while (exists && outIndex < count) {
        dpiData payloadElement;
        if (dpiObject_getElementValueByIndex(payloadArrayObj, index, DPI_NATIVE_TYPE_OBJECT, &payloadElement) != DPI_SUCCESS) return -1;

        if (payloadElement.isNull || payloadElement.value.asObject == NULL) {
            outMessages[outIndex].data = NULL;
            outMessages[outIndex].length = 0;
        } else {
            dpiData lobData;
            if (dpiObject_getAttributeValue(payloadElement.value.asObject, jsonAttr, DPI_NATIVE_TYPE_LOB, &lobData) != DPI_SUCCESS) return -1;

            if (lobData.isNull) {
                outMessages[outIndex].data = NULL;
                outMessages[outIndex].length = 0;
            } else {
                outMessages[outIndex].data = ReadFullBlob(lobData.value.asLOB, &outMessages[outIndex].length);
                dpiLob_release(lobData.value.asLOB);
            }

            dpiObject_release(payloadElement.value.asObject);
        }

        dpiData rawElement;
        if (dpiObject_getElementValueByIndex(rawArrayObj, index, DPI_NATIVE_TYPE_BYTES, &rawElement) != DPI_SUCCESS) return -1;

        if (!rawElement.isNull) {
            outIds[outIndex].length = rawElement.value.asBytes.length;
            outIds[outIndex].data = (char*)malloc(outIds[outIndex].length);
            memcpy(outIds[outIndex].data, rawElement.value.asBytes.ptr, outIds[outIndex].length);
        } else {
            outIds[outIndex].data = NULL;
            outIds[outIndex].length = 0;
        }

        if (dpiObject_getNextIndex(payloadArrayObj, index, &index, &exists) != DPI_SUCCESS) return -1;
        outIndex++;
    }

    return 0;
}

static int DequeueManyProxy(dpiConn* conn, const char* subName, uint32_t batchSize, dpiVar* outMessages, dpiVar* outIds, uint32_t* actualCount){
    dpiStmt* stmt = NULL;
    const char* sql = "BEGIN OMNI_TRACER_API.Dequeue_Array_Events(:1, :2, 0, :3, :4, :5); END;";

    dpiVar* subVar = NULL;
    dpiVar* batchVar = NULL;
    dpiVar* countVar = NULL;
    dpiData* batchData = NULL;
    dpiData* countData = NULL;

    // Prepare
    if (dpiConn_prepareStmt(conn, 0, sql, (uint32_t)strlen(sql), NULL, 0, &stmt) != DPI_SUCCESS) return -1;

    // :1 Subscriber
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_VARCHAR, DPI_NATIVE_TYPE_BYTES, 1, 0, 1, 0, NULL, &subVar, NULL) != DPI_SUCCESS) return -1;
    dpiVar_setFromBytes(subVar, 0, subName, (uint32_t)strlen(subName));
    if (dpiStmt_bindByPos(stmt, 1, subVar) != DPI_SUCCESS) return -1;

    // :2 Batch size
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &batchVar, &batchData) != DPI_SUCCESS) return -1;
    batchData->value.asInt64 = (int64_t)batchSize;
    if (dpiStmt_bindByPos(stmt, 2, batchVar) != DPI_SUCCESS) return -1;

    // :3 Messages (CLOB array) and :4 IDs (RAW array) are created by caller
    if (dpiStmt_bindByPos(stmt, 3, outMessages) != DPI_SUCCESS) return -1;
    if (dpiStmt_bindByPos(stmt, 4, outIds) != DPI_SUCCESS) return -1;

    // :5 Actual count
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &countVar, &countData) != DPI_SUCCESS) return -1;
    if (dpiStmt_bindByPos(stmt, 5, countVar) != DPI_SUCCESS) return -1;

    // Execute
    if (dpiStmt_execute(stmt, 0, NULL) != DPI_SUCCESS) return -1;

    // Get count
    *actualCount = (uint32_t)countData->value.asInt64;

    // Cleanup
    dpiVar_release(subVar);
    dpiVar_release(batchVar);
    dpiVar_release(countVar);
    dpiStmt_release(stmt);
    return 0;
}

int DequeueManyAndExtract(dpiConn* conn, const char* schemaName, const char* subName, uint32_t batchSize, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount){
    dpiVar* payloadVar = NULL;
    dpiVar* rawVar = NULL;
    dpiData* payloadData = NULL;
    dpiData* rawData = NULL;
    dpiObjectType* payloadArrayType = NULL;
    dpiObjectType* rawArrayType = NULL;
    dpiObjectType* payloadObjType = NULL;
    dpiObjectAttr* jsonAttr = NULL;
    dpiObject* payloadArrayObj = NULL;
    dpiObject* rawArrayObj = NULL;

    if (getObjectType(conn, schemaName, "OMNI_TRACER_PAYLOAD_ARRAY", &payloadArrayType) != DPI_SUCCESS) return -1;
    if (getObjectType(conn, schemaName, "OMNI_TRACER_RAW_ARRAY", &rawArrayType) != DPI_SUCCESS) return -1;
    if (getObjectType(conn, schemaName, "OMNI_TRACER_PAYLOAD_TYPE", &payloadObjType) != DPI_SUCCESS) return -1;
    if (getObjectAttributeByName(payloadObjType, "JSON_DATA", &jsonAttr) != DPI_SUCCESS) return -1;

    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 0, 0, payloadArrayType, &payloadVar, &payloadData) != DPI_SUCCESS) return -1;
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 0, 0, rawArrayType, &rawVar, &rawData) != DPI_SUCCESS) return -1;

    if (createCollection(payloadArrayType, &payloadArrayObj) != DPI_SUCCESS) return -1;
    if (createCollection(rawArrayType, &rawArrayObj) != DPI_SUCCESS) return -1;

    if (dpiVar_setFromObject(payloadVar, 0, payloadArrayObj) != DPI_SUCCESS) return -1;
    if (dpiVar_setFromObject(rawVar, 0, rawArrayObj) != DPI_SUCCESS) return -1;

    if (DequeueManyProxy(conn, subName, batchSize, payloadVar, rawVar, actualCount) != 0) return -1;

    *outMessages = (TraceMessage*)calloc(*actualCount, sizeof(TraceMessage));
    *outIds = (TraceId*)calloc(*actualCount, sizeof(TraceId));
    if (*actualCount > 0 && (!*outMessages || !*outIds)) return -1;

    if (*actualCount > 0) {
        payloadArrayObj = payloadData->value.asObject;
        rawArrayObj = rawData->value.asObject;

        if (payloadArrayObj == NULL || rawArrayObj == NULL) return -1;
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
