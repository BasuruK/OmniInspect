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
        const char errorInfoMsg; \
        uint32_t errorInfoMsgLength; \
        dpiContext_getError((ctx), NULL, &errorInfoMsg, &errorInfoMsgLength); \
        fprintf(stderr, "[C ERROR] %s: %.*s\n", errorInfoMsgLength, errorInfoMsg); \
        return -1; \
    }

static char* ReadLobContent(dpiLob* lob, uint64_t* outLength) {
    dpiContext *ctx = NULL;
    uint64_t size = 0;

    // 
    if(dpiLob_getSize(lob, &size) != DPI_SUCCESS) {
        return NULL;
    }

    if (size == 0) {
        *outLength = 0;
        return NULL;
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

static int ExecuteDequeuProc(dpiConn* conn, const char* subscriber_name, uint32_t batchSize, dpiVar* outPayloadVar, dpiVar* outRawVar, uint32_t* outCount) {
    dpiStmt* stmt = NULL;
    dpiVar* subVar = NULL;
    dpiVar* batchVar = NULL;
    dpiVar* countVar = NULL;
    dpiData* countData = NULL;

    const char* sql = "BEGIN OMNI_TRACER_API.Dequeue_Array_Events(:1, :2, 1, :3, :4, :5); END;";

    if (dpiConn_prepareStmt(conn, 0, sql, strlen(sql), NULL, 0, &stmt) != DPI_SUCCESS) return -1;

    // Subscriber name parameter
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_VARCHAR, DPI_NATIVE_TYPE_BYTES, 1, 0, 0, 0, NULL, &subVar, NULL) != DPI_SUCCESS) {
        dpiStmt_release(stmt);
        return -1;
    }
    dpiVar_setFromBytes(subVar, 0, subscriber_name, strlen(subscriber_name));
    if (dpiStmt_bindByPos(stmt, 1, subVar) != DPI_SUCCESS) return -1;
    
    // Batch size parameter
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &batchVar, NULL) != DPI_SUCCESS) {
        dpiStmt_release(stmt);
        dpiVar_release(subVar);
        return -1;
    }
    dpiVar_setFromInt64(batchVar, 0, (int64_t)batchSize);
    if (dpiStmt_bindByPos(stmt, 2, batchVar) != DPI_SUCCESS) return -1;

    // Output payload parameter
    if (dpiStmt_bindByPos(stmt, 3, outPayloadVar) != DPI_SUCCESS) return -1;

    // Output raw parameter
    if (dpiStmt_bindByPos(stmt, 4, outRawVar) != DPI_SUCCESS) return -1;

    // Output count parameter
    if (dpiConn_newVar(conn, DPI_ORACLE_TYPE_NUMBER, DPI_NATIVE_TYPE_INT64, 1, 0, 0, 0, NULL, &countVar, &countData) != DPI_SUCCESS) {
        dpiStmt_release(stmt);
        dpiVar_release(subVar);
        dpiVar_release(batchVar);
        return -1;
    }

    // Execute
    if (dpiStmt_execute(stmt, 0,0) != DPI_SUCCESS) {
        fprintf(stderr, "[C ERROR] Failed to execute statement\n");
        dpiStmt_release(stmt);
        dpiVar_release(subVar);
        dpiVar_release(batchVar);
        dpiVar_release(countVar);
        return -1;
    }

    // Get count
    *outCount = (uint32_t)countData->value.asInt64;

    // Cleanup
    dpiStmt_release(stmt);
    dpiVar_release(subVar);
    dpiVar_release(batchVar);
    dpiVar_release(countVar);
    return 0;
}