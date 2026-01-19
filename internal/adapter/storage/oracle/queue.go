package oracle

/*
#include "dpi.h"
#include "dpi_go_helpers.h"
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

	// Create queue handle
	queueHandle, err := oa.createQueueHandle(subscriber)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to create queue handle: %s", err)
	}
	defer C.dpiQueue_release(queueHandle)

	// Allocate array for message properties
	maxMessages := subscriber.BatchSize
	msgPropsArray := make([]*C.dpiMsgProps, maxMessages)
	numMessages := C.uint32_t(maxMessages)

	status := C.dpiQueue_deqMany(queueHandle, &numMessages, &msgPropsArray[0])

	if status != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)

		if errInfo.code == 25228 { // DPI-25228: No messages available
			return []string{}, [][]byte{}, 0, nil // No messages available
		}

		return nil, nil, 0, fmt.Errorf("failed to dequeue messages: %s", C.GoString(errInfo.message))
	}

	count := int(numMessages)
	fmt.Printf("[DEBUG] Dequeued %d messages for subscriber: %s\n", count, subscriber.Name)

	if count == 0 {
		return []string{}, [][]byte{}, 0, nil // No messages dequeued
	}

	// Extract messages
	messages := make([]string, count)
	msgIds := make([][]byte, count)
	var msgProps *C.dpiMsgProps

	for i := 0; i < count; i++ {
		msgProps = msgPropsArray[i]
		if msgProps == nil {
			return nil, nil, 0, fmt.Errorf("message properties at index %d is nil", i)
		}

		// Get message payload
		payload, err := oa.extractPayloadFromMsgProps(msgProps)
		if err != nil {
			C.dpiMsgProps_release(msgProps)
			return nil, nil, 0, fmt.Errorf("failed to extract payload from message properties: %s", err)
		}
		messages[i] = payload

		// Get message ID
		var msgIdPtr *C.char
		var msgIdLen C.uint32_t

		if C.dpiMsgProps_getMsgId(msgProps, &msgIdPtr, &msgIdLen) == C.DPI_SUCCESS {
			msgIds[i] = C.GoBytes(unsafe.Pointer(msgIdPtr), C.int(msgIdLen))
		} else {
			C.dpiMsgProps_release(msgProps)
			return nil, nil, 0, fmt.Errorf("failed to get message ID from message properties")
		}

		C.dpiMsgProps_release(msgProps)
	}
	return messages, msgIds, count, nil
}

// createQueueHandle creates and configures a queue handle for the given subscriber.
func (oa *OracleAdapter) createQueueHandle(subscriber domain.Subscriber) (*C.dpiQueue, error) {
	queueConfig := domain.NewQueueConfig()
	cQueueName := C.CString(queueConfig.Name())
	defer C.free(unsafe.Pointer(cQueueName))

	var queueHandle *C.dpiQueue

	// Get the payload object type if not already cached
	if oa.payloadType == nil {
		QueuePayloadType := queueConfig.PayloadType()
		if err := oa.getObjectType(oa.config.Username, QueuePayloadType, &oa.payloadType); err != nil {
			return nil, fmt.Errorf("failed to get payload object type: %s", err)
		}
	}

	// Create the queue handle
	if C.dpiConn_newQueue(
		oa.Connection,
		cQueueName,
		C.uint32_t(len(queueConfig.Name())),
		oa.payloadType,
		&queueHandle,
	) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return nil, fmt.Errorf("failed to create queue handle: %s", C.GoString(errInfo.message))
	}

	// Configure dequeue options
	if err := oa.configureDequeueOptions(queueHandle, subscriber); err != nil {
		C.dpiQueue_release(queueHandle)
		return nil, fmt.Errorf("failed to configure dequeue options: %s", err)
	}

	return queueHandle, nil
}

// configureDequeueOptions sets the dequeue options for the given queue and subscriber.
func (oa *OracleAdapter) configureDequeueOptions(queue *C.dpiQueue, subscriber domain.Subscriber) error {
	var dequeueOptions *C.dpiDeqOptions

	// Get dequeue options handle
	if C.dpiQueue_getDeqOptions(queue, &dequeueOptions) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to get dequeue options: %s", C.GoString(errInfo.message))
	}

	// Set consumer name
	cConsumerName := C.CString(subscriber.Name)
	defer C.free(unsafe.Pointer(cConsumerName))

	if C.dpiDeqOptions_setConsumerName(dequeueOptions, cConsumerName, C.uint32_t(len(subscriber.Name))) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to set consumer name: %s", C.GoString(errInfo.message))
	}

	// Set wait time
	if subscriber.WaitTime < 0 {
		return fmt.Errorf("invalid wait time, must be a positive integer: %d", subscriber.WaitTime)
	}

	if C.dpiDeqOptions_setWait(dequeueOptions, C.uint32_t(subscriber.WaitTime)) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to set wait time: %s (code: %d)", C.GoString(errInfo.message), errInfo.code)
	}

	// set visibility (Immediate - no transaction required)
	if C.dpiDeqOptions_setVisibility(dequeueOptions, C.DPI_VISIBILITY_IMMEDIATE) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to set visibility: %s (code: %d)", C.GoString(errInfo.message), errInfo.code)
	}

	// Set Dequeue mode to Remove once dequeued
	if C.dpiDeqOptions_setMode(dequeueOptions, C.DPI_MODE_DEQ_REMOVE) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to set dequeue mode: %s (code: %d)", C.GoString(errInfo.message), errInfo.code)
	}

	// Changed from FIRST_MSG to NEXT_MSG for sharded queue compatibility on Windows
	// FIRST_MSG has platform-specific issues with sharded queues
	if C.dpiDeqOptions_setNavigation(dequeueOptions, C.DPI_DEQ_NAV_NEXT_MSG) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to set navigation mode: %s (code: %d)", C.GoString(errInfo.message), errInfo.code)
	}

	return nil
}

// extractPayloadFromMsgProps extracts the JSON payload from the given message properties.
func (oa *OracleAdapter) extractPayloadFromMsgProps(msgProps *C.dpiMsgProps) (string, error) {
	var payloadObj *C.dpiObject
	var payloadPtr *C.char
	var payloadLen C.uint32_t

	// Get the payload object from message properties
	if C.dpiMsgProps_getPayload(msgProps, &payloadObj, &payloadPtr, &payloadLen) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return "", fmt.Errorf("failed to get payload from message properties: %s", C.GoString(errInfo.message))
	}

	if payloadObj == nil {
		return "", fmt.Errorf("payload object is nil")
	}

	// Get the JSON_DATA attribute from the payload object
	attrName := C.CString("JSON_DATA")
	defer C.free(unsafe.Pointer(attrName))

	var jsonDataAttr *C.dpiObjectAttr

	if C.getObjectAttributeByName(oa.payloadType, attrName, &jsonDataAttr) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return "", fmt.Errorf("failed to get JSON_DATA attribute from payload object: %s", C.GoString(errInfo.message))
	}
	defer C.dpiObjectAttr_release(jsonDataAttr)

	// Get the BLOB Value
	var blobObjData C.dpiData
	if C.dpiObject_getAttributeValue(payloadObj, jsonDataAttr, C.DPI_NATIVE_TYPE_LOB, &blobObjData) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return "", fmt.Errorf("failed to get JSON_DATA attribute value: %s", C.GoString(errInfo.message))
	}

	if blobObjData.isNull != 0 {
		return "", nil // JSON_DATA is NULL
	}

	// Read the BLOB data
	lobObject := C.getLobFromData(&blobObjData)

	var lobLength C.uint64_t
	if C.dpiLob_getSize(lobObject, &lobLength) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return "", fmt.Errorf("failed to get LOB size: %s", C.GoString(errInfo.message))
	}

	if lobLength == 0 {
		return "", nil // Empty LOB
	}

	// C.uint64_t(^uint(0) >> 1) is used to ensure the length fits into a Go slice (calculates the maximum value of a signed 64-bit integer)
	if lobLength > C.uint64_t(^uint(0)>>1) {
		return "", fmt.Errorf("LOB size %d exceeds maximum supported size", lobLength)
	}

	bufferLength := C.uint32_t(lobLength)
	buffer := make([]byte, bufferLength)
	bytesRead := lobLength

	if C.dpiLob_readBytes(lobObject, 1, lobLength, (*C.char)(unsafe.Pointer(&buffer[0])), &bytesRead) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return "", fmt.Errorf("failed to read LOB data: %s", C.GoString(errInfo.message))
	}

	return string(buffer[:int(bytesRead)]), nil
}

// getObjectType retrieves the object type for a given schema and type name.
func (oa *OracleAdapter) getObjectType(schema string, typeName string, objType **C.dpiObjectType) error {
	cSchema := C.CString(strings.ToUpper(schema))
	cTypeName := C.CString(typeName)
	defer C.free(unsafe.Pointer(cSchema))
	defer C.free(unsafe.Pointer(cTypeName))

	var localObjType *C.dpiObjectType

	if C.getObjectType(oa.Connection, cSchema, cTypeName, &localObjType) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to get object type for %s.%s: %s", schema, typeName, C.GoString(errInfo.message))
	}

	*objType = localObjType
	return nil
}
