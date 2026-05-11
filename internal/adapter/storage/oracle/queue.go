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
	"context"
	"fmt"
	"unsafe"
)

// CheckQueueDepth checks the queue depth for the given subscriber ID
func (oa *OracleAdapter) CheckQueueDepth(ctx context.Context, subscriberID string, queueTableName string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) 
			  FROM %s
			  WHERE QUEUE = :queueName
			  AND CONSUMER_NAME = :subscriberID
			  AND MSG_STATE = 'READY'`, queueTableName)

	results, err := oa.FetchWithParams(ctx, query, map[string]interface{}{
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

func (oa *OracleAdapter) BulkDequeueTracerMessages(ctx context.Context, subscriber domain.Subscriber) ([]string, [][]byte, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, 0, err
	}

	if oa.Context == nil {
		return nil, nil, 0, fmt.Errorf("dpi context is not initialized")
	}

	if oa.Connection == nil {
		return nil, nil, 0, fmt.Errorf("database connection is not established")
	}

	if subscriber.BatchSize().Int() <= 0 {
		return nil, nil, 0, fmt.Errorf("batch size must be > 0")
	}

	var cMessages *C.TraceMessage
	var cIds *C.TraceId
	var cCount C.uint32_t

	cSubscriberName := C.CString(subscriber.ConsumerName())
	defer C.free(unsafe.Pointer(cSubscriberName))

	if C.DequeueManyAndExtract(oa.Connection, oa.Context, cSubscriberName, C.uint32_t(subscriber.BatchSize().Int()), C.int32_t(subscriber.WaitTime().Int()), &cMessages, &cIds, &cCount) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)

		if errInfo.code == 25228 { // DPI-25228: No messages available
			return []string{}, [][]byte{}, 0, nil
		}

		return nil, nil, 0, fmt.Errorf("failed to dequeue messages: %s (code: %d)", C.GoString(errInfo.message), errInfo.code)
	}
	count := int(cCount)

	if count == 0 {
		return []string{}, [][]byte{}, 0, nil
	}

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
