package utils

/*
#cgo CFLAGS: -I${SRCDIR}/../lib/odpi/include
#cgo LDFLAGS: -L${SRCDIR}/../lib/odpi/lib -lodpi

#include "dpi.h"
#include "dpi_go_helpers.h"
#include <stdio.h>
#include <stdlib.h>

*/
import "C"
import (
	"fmt"
	"unsafe"
)

type Database struct {
	Connection *C.dpiConn
}

func NewDBConnection() *Database {
	// Load the configurations
	dbConfigs := LoadConfigurations().DatabaseSettings

	// Set connection parameters
	username := dbConfigs.Username
	password := dbConfigs.Password
	connectionString := fmt.Sprintf("%s:%s/%s", dbConfigs.Database, dbConfigs.Port, dbConfigs.Host)

	// Set Context for the connection
	context := SetContext()

	// Connect to the database
	var conn *C.dpiConn
	conn = CreateConnection(username, password, connectionString, context)

	var stmt *C.dpiStmt
	query := C.CString("SELECT 'Hellow from DB weeeeee!' FROM DUAL")
	defer C.free(unsafe.Pointer(query))

	// Prepare the statement
	if C.dpiConn_prepareStmt(conn, 0, query, C.uint32_t(len(C.GoString(query))), nil, 0, &stmt) != C.DPI_SUCCESS {
		fmt.Println("Failed to prepare statement")
	}

	//defer C.dpiStmt_release(stmt) // Release the statement when done

	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		fmt.Println("Failed to execute statement")
	}

	// Fetch the result
	for {
		var found C.int
		var bufferRowIndex C.uint32_t

		if C.dpiStmt_fetch(stmt, &found, &bufferRowIndex) != C.DPI_SUCCESS || found == 0 {
			break
		}
		var data *C.dpiData
		var nativeTypeNum C.dpiNativeTypeNum

		if C.dpiStmt_getQueryValue(stmt, 1, &nativeTypeNum, &data) == C.DPI_SUCCESS {
			ptr := C.getAsBytesPtr(data)
			length := C.getAsBytesLength(data)
			str := C.GoStringN(ptr, C.int(length))

			fmt.Println("Result : ", str)

		} else {
			fmt.Println("Failed to get query value")
		}
	}

	db := &Database{
		Connection: conn}

	defer C.dpiConn_release(conn)       // Release the connection when done
	defer C.dpiContext_destroy(context) // Release the context when done
	return db
}

func SetContext() *C.dpiContext {
	var context *C.dpiContext
	var contextError C.dpiErrorInfo

	if C.dpiContext_createWithParams(C.DPI_MAJOR_VERSION, C.DPI_MINOR_VERSION, nil, &context, &contextError) != C.DPI_SUCCESS {
		fmt.Printf("Failed to create DPI Context: %s", C.GoString(contextError.message))
	}
	return context
}

func CreateConnection(username string, password string, connectionString string, context *C.dpiContext) *C.dpiConn {
	var conn *C.dpiConn
	var errInfo C.dpiErrorInfo

	c_username := C.CString(username)
	c_password := C.CString(password)
	c_connectionString := C.CString(connectionString)

	if C.dpiConn_create(context,
		c_username,
		C.uint32_t(len(C.GoString(c_username))),
		c_password,
		C.uint32_t(len(C.GoString(c_password))),
		c_connectionString,
		C.uint32_t(len(C.GoString(c_connectionString))),
		nil, // dpiCommonParams
		nil, // dpiConnCreateParams
		&conn) == C.DPI_SUCCESS {
		fmt.Println("Connected to the database")
	} else {
		fmt.Printf("Failed to create database connection Connection: %s", C.GoString(errInfo.message))
		return nil
	}
	return conn
}
