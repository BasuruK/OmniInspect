package oracle

/*
#include "dpi.h"
#include "dpi_go_helpers.h"
#include "dequeue_ops.h"
#include <stdio.h>
#include <stdlib.h>
*/
import "C"

import (
	"OmniView/internal/core/domain"
	"fmt"
	"strings"
	"unsafe"
)

// CheckQueueDepth checks the queue depth for the given subscriber ID
func (oa *OracleAdapter) CheckQueueDepth(subscriberID string, queueTableName string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) 
			  FROM %s
			  WHERE QUEUE = :queueName
			  AND CONSUMER_NAME = :subscriberID
			  AND MSG_STATE = 'READY'`, queueTableName)

	results, err := oa.FetchWithParams(query, map[string]interface{}{
		"queueName":    domain.QueueName,
		"subscriberID": subscriberID,
	})
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, fmt.Errorf("no results returned from queue depth query")
	}

	count, err := parseCountResult(results)
	if err != nil {
		return 0, fmt.Errorf("failed to parse count result: %v", err)
	}

	return count, nil
}

func (oa *OracleAdapter) BulkDequeueTracerMessages(subscriber domain.Subscriber) ([]string, [][]byte, int, error) {
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

	cSchemaName := C.CString(strings.ToUpper(oa.config.Username))
	defer C.free(unsafe.Pointer(cSchemaName))

	if C.DequeueManyAndExtract(oa.Connection, oa.Context, cSchemaName, cSubscriberName, C.uint32_t(subscriber.BatchSize), C.int32_t(subscriber.WaitTime), &cMessages, &cIds, &cCount) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)

		if errInfo.code == 25228 { // DPI-25228: No messages available
			return []string{}, [][]byte{}, 0, nil
		}

		return nil, nil, 0, fmt.Errorf("failed to dequeue messages: %s", C.GoString(errInfo.message))
	}
	defer C.FreeDequeueResults(cMessages, cIds, cCount)

	count := int(cCount)

	if count == 0 {
		return []string{}, [][]byte{}, 0, nil
	}

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
