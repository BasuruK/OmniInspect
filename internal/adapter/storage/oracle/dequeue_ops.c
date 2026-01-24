#include <dpi.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include "dpi_go_helpers.h"
#include "dequeue_ops.h"

// Debug function to print error messages
// This macro checks the status of a DPI operation and prints an error message if it fails.
#define CHECK_DPI(status, ctx, msg) \
    if (status != DPI_SUCCESS) { \
        dpiErrorInfo errorInfo; \
        dpiContext_getError((ctx), &errorInfo); \
        fprintf(stderr, "[C ERROR] %s: %.*s\n", (msg), (int)errorInfo.messageLength, errorInfo.message); \
        return -1; \
    }

/**
 * ReadLobContent - Reads the content of a LOB into a buffer
 * 
 * @param lob: Pointer to the dpiLob representing the LOB
 * @param outLength: Output parameter to receive the length of the read data
 * @return: Pointer to the allocated buffer containing the LOB data (must be freed by caller), or NULL on failure
 */
static char* ReadLobContent(dpiLob* lob, uint64_t* outLength) {
    uint64_t size = 0;

    if (!outLength) {
        return NULL;
    }
    *outLength = 0;

    // 
    if(dpiLob_getSize(lob, &size) != DPI_SUCCESS) {
        return NULL;
    }

    if (size == 0) {
        return NULL; // Empty LOB - outLength remains 0
    }
    char* buffer = (char*)malloc(size); // for Blobs size is in bytes
    if (!buffer) return NULL;
    uint64_t bytesRead = size;
    if (dpiLob_readBytes(lob, 1, size, buffer, &bytesRead) != DPI_SUCCESS) {
        free(buffer);
        return NULL;
    }

    *outLength = bytesRead;
    return buffer;
}

/**
 * ExecuteDequeuProc - Executes the dequeue PL/SQL procedure
 * 
 * @param conn: Pointer to the dpiConn representing the Oracle connection
 * @param subscriberName: Name of the subscriber (queue consumer)
 * @param batchSize: Number of messages to dequeue
 * @param outPayloadVar: Output variable for the payload collection
 * @param outRawVar: Output variable for the raw ID collection
 * @param outCount: Output parameter to receive the actual number of dequeued messages
 * @return: 0 on success, -1 on failure
 */
static int ExecuteDequeueProc(dpiConn* conn, const char* subscriberName, uint32_t batchSize, dpiVar* outPayloadVar, dpiVar* outRawVar, uint32_t* outCount) {
    dpiStmt* stmt = NULL;
    dpiVar* subVar = NULL;
    dpiVar* batchVar = NULL;
    dpiVar* countVar = NULL;
    dpiData* subData = NULL;
    dpiData* batchData = NULL;
    dpiData* countData = NULL;
    uint32_t subNameLen = (uint32_t)strlen(subscriberName);
    int result = -1;

    const char* sql = "BEGIN OMNI_TRACER_API.Dequeue_Array_Events(:1, :2, 1, :3, :4, :5); END;";

    if (dpiConn_prepareStmt(conn, 0, sql, strlen(sql), NULL, 0, &stmt) != DPI_SUCCESS) goto cleanup;

    // Subscriber name parameter
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_VARCHAR, DPI_NATIVE_TYPE_BYTES, 1, subNameLen, 1, 0, NULL, &subVar, &subData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to set new subvar\n");
        goto cleanup;
    }
    
    if (dpiVar_setFromBytes(subVar, 0, subscriberName, subNameLen) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to set subscriber name parameter\n");
        goto cleanup;
    }
    if (dpiStmt_bindByPos(stmt, 1, subVar) != DPI_SUCCESS) goto cleanup;
    
    // Batch size parameter
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &batchVar, &batchData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to bind batch size parameter\n");
        goto cleanup;
    }
    batchData->value.asInt64 = (int64_t)batchSize;
    batchData->isNull = 0;
    if (dpiStmt_bindByPos(stmt, 2, batchVar) != DPI_SUCCESS) goto cleanup;

    // Output payload parameter
    if (dpiStmt_bindByPos(stmt, 3, outPayloadVar) != DPI_SUCCESS) goto cleanup;

    // Output raw parameter
    if (dpiStmt_bindByPos(stmt, 4, outRawVar) != DPI_SUCCESS) goto cleanup;

    // Output count parameter
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &countVar, &countData) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to bind count parameter\n");
        goto cleanup;
    }
    if (dpiStmt_bindByPos(stmt, 5, countVar) != DPI_SUCCESS) goto cleanup;

    // Execute
    if (dpiStmt_execute(stmt, 0,0) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to execute statement\n");
        goto cleanup;
    }

    // Get count
    *outCount = (uint32_t)countData->value.asInt64;
    result = 0;
    goto cleanup;

    // Cleanup
    cleanup:
        if (stmt) dpiStmt_release(stmt);
        if (subVar) dpiVar_release(subVar);
        if (batchVar) dpiVar_release(batchVar);
        if (countVar) dpiVar_release(countVar);

    return result;
}

/**
 * DequeueManyAndExtract - Dequeues messages and extracts their content and IDs
 * 
 * @param conn: Pointer to the dpiConn representing the Oracle connection
 * @param schemaName: Schema name where the queue resides
 * @param subscriberName: Name of the subscriber (queue consumer)
 * @param batchSize: Number of messages to dequeue
 * @param outMessages: Output parameter to receive an array of TraceMessage structures
 * @param outIds: Output parameter to receive an array of TraceId structures
 * @param actualCount: Output parameter to receive the actual number of dequeued messages
 * @return: 0 on success, -1 on failure
 */
int DequeueManyAndExtract(dpiConn* conn, const char* schemaName, const char* subscriberName, uint32_t batchSize, TraceMessage** outMessages, TraceId** outIds, uint32_t* actualCount) {

    dpiObjectType *payloadType = NULL, *rawType = NULL, *objType = NULL;
    dpiObjectAttr *jsonAttr = NULL;
    dpiVar *payloadVar = NULL, *rawVar = NULL;
    dpiData *payloadData = NULL, *rawData = NULL;

    const char* payloadArrayName = "OMNI_TRACER_PAYLOAD_ARRAY";
    const char* rawArrayName = "OMNI_TRACER_RAW_ARRAY";
    const char* payloadTypeName = "OMNI_TRACER_PAYLOAD_TYPE";

    int result = -1;

    // Load Type
    if (dpiConn_getObjectType(conn, payloadArrayName, strlen(payloadArrayName), &payloadType) != DPI_SUCCESS) goto cleanup;
    if (dpiConn_getObjectType(conn, rawArrayName, strlen(rawArrayName), &rawType) != DPI_SUCCESS) goto cleanup;

    // Attribute handle for the element inside the collection
    if (dpiConn_getObjectType(conn, payloadTypeName, strlen(payloadTypeName), &objType) != DPI_SUCCESS) goto cleanup;
    if (dpiObjectType_getAttributes(objType, 1, &jsonAttr) != DPI_SUCCESS) goto cleanup;

    // Create Variables for Out Collections
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 0, 0, payloadType, &payloadVar, &payloadData) != DPI_SUCCESS) goto cleanup;
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_OBJECT, DPI_NATIVE_TYPE_OBJECT, 1, 0, 0, 0, rawType, &rawVar, &rawData) != DPI_SUCCESS) goto cleanup;

    // Execute Dequeue Procedure in PLSQL
    if (ExecuteDequeueProc(conn, subscriberName, batchSize, payloadVar, rawVar, actualCount) != 0) goto cleanup;

    // Allocate Go-C structs
    if (*actualCount == 0) {
        result = 0;
        goto cleanup;
    }

    *outMessages = (TraceMessage*)calloc(*actualCount, sizeof(TraceMessage));
    if (!*outMessages) goto cleanup;
    *outIds = (TraceId*)calloc(*actualCount, sizeof(TraceId));
    if (!*outIds) {
        free(*outMessages);
        *outMessages = NULL;
        goto cleanup;
    }

    // Iterate and Extract
    dpiObject *payloadColl = payloadData->value.asObject;
    dpiObject *rawColl = rawData->value.asObject;

    int32_t idx = 0;
    int exists = 0;
    uint32_t outIdx = 0;

    if (dpiObject_getFirstIndex(payloadColl, &idx, &exists) != DPI_SUCCESS) goto cleanup;

    while (exists && outIdx < *actualCount) {
        dpiData element;
        if (dpiObject_getElementValueByIndex(payloadColl, idx, DPI_NATIVE_TYPE_OBJECT, &element) != DPI_SUCCESS) goto cleanup;
        if (!element.isNull) {
            dpiData lobVal;
            dpiObject *msgObj = element.value.asObject;
            // Get JSON attribute
            if (dpiObject_getAttributeValue(msgObj, jsonAttr, DPI_NATIVE_TYPE_LOB, &lobVal) == DPI_SUCCESS) {
                if (!lobVal.isNull) {
                    // Read LOB content, extract data from LOB.
                    (*outMessages)[outIdx].data = ReadLobContent(lobVal.value.asLOB, &(*outMessages)[outIdx].length);
                }
            } else {
                goto cleanup;
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
            if (!(*outIds)[outIdx].data) { // malloc failed, clean up
                (*outIds)[outIdx].length = 0; 
                goto cleanup;
            }
            (*outIds)[outIdx].length = len;
            memcpy((*outIds)[outIdx].data, rawElement.value.asBytes.ptr, len);
        }

        outIdx++;
        if (dpiObject_getNextIndex(payloadColl, idx, &idx, &exists) != DPI_SUCCESS) {  
            goto cleanup;  
        }
    }
    result = 0;

cleanup:
    if (result != 0) {
        // Free partially allocated results on error
        if (*outMessages) {
            for (uint32_t i = 0; i < outIdx; i++) {
                if ((*outMessages)[i].data) free((*outMessages)[i].data);
            }
            free(*outMessages);
            *outMessages = NULL;
        }
        if (*outIds) {
            for (uint32_t i = 0; i < outIdx; i++) {
                if ((*outIds)[i].data) free((*outIds)[i].data);
            }
            free(*outIds);
            *outIds = NULL;
        }
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

/**
 * FreeDequeueResults - Frees the allocated memory for dequeued messages and IDs
 * 
 * @param messages: Pointer to the array of TraceMessage structures
 * @param ids: Pointer to the array of TraceId structures
 * @param count: Number of messages/IDs to free
 */
void FreeDequeueResults (TraceMessage* messages, TraceId* ids, uint32_t count) {
    if (messages) {
        for (uint32_t i = 0; i < count; i++) {
            if (messages[i].data) free(messages[i].data);
        }
        free(messages);
    }
    if (ids) {
        for (uint32_t i = 0; i < count; i++) {
            if (ids[i].data) free(ids[i].data);
        }
        free(ids);
    }
}