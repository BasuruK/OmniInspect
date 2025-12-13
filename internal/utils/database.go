package utils

/*
#cgo darwin CFLAGS: -I${SRCDIR}/../lib/odpi/include
#cgo darwin LDFLAGS: -L${SRCDIR}/../../lib -lodpi -Wl,-rpath,${SRCDIR}/../../lib -Wl,-rpath,/opt/oracle/instantclient_23_7
#cgo windows CFLAGS: -I${SRCDIR}/../lib/odpi/include
#cgo windows LDFLAGS: -L${SRCDIR}/../.. -lodpi -LC:/oracle_inst/instantclient_23_7 -loci

#include "dpi.h"
#include "dpi_go_helpers.h"
#include <stdio.h>
#include <stdlib.h>

*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"
)

type Database struct {
	Connection *C.dpiConn
	Context    *C.dpiContext
}

// Global variable to hold the database connection
var (
	dbConn  *Database
	dbMutex sync.Mutex
)

// GetDBInstance returns the singleton database connection instance
func GetDBInstance() *Database {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if dbConn == nil {
		dbConn = NewDatabaseConnection()
	}
	return dbConn
}

// CleanupDBConnection releases the database connection and context
func CleanupDBConnection() {

	dbMutex.Lock()
	defer dbMutex.Unlock() // Ensure thread-safe cleanup

	if dbConn != nil && dbConn.Connection != nil {
		C.dpiConn_release(dbConn.Connection)
		dbConn.Connection = nil
	}
	if dbConn != nil && dbConn.Context != nil {
		C.dpiContext_destroy(dbConn.Context)
		dbConn.Context = nil
	}

	dbConn = nil
}

func NewDatabaseConnection() *Database {
	// Load the configurations
	dbConfigs := LoadConfigurations().DatabaseSettings

	// Set connection parameters
	username := dbConfigs.Username
	password := dbConfigs.Password
	connectionString := fmt.Sprintf("%s:%s/%s", dbConfigs.Host, fmt.Sprint(dbConfigs.Port), dbConfigs.Database)

	// Set Context for the connection
	context := SetContext()

	// Connect to the database := implicitly declare *C.dpiConn type
	conn := CreateConnection(username, password, connectionString, context)

	// Return the database connection instance
	db := &Database{
		Connection: conn,
		Context:    context,
	}

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

func PrepareStatement(db *Database, query string, stmt *C.dpiStmt) *C.dpiStmt {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	// Prepare the statement
	if C.dpiConn_prepareStmt(db.Connection, 0, cQuery, C.uint32_t(len(query)), nil, 0, &stmt) != C.DPI_SUCCESS {
		fmt.Println("Failed to prepare statement")
	}
	defer C.dpiStmt_release(stmt) // Release the statement when done

	return stmt
}

func ExecuteStatement(query string) error {
	var stmt *C.dpiStmt
	db := GetDBInstance()
	stmt = PrepareStatement(db, query, stmt)

	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		fmt.Println("Failed to execute statement")
	}
	return nil
}

func FetchData() ([]string, error) {
	db := GetDBInstance()
	if db == nil || db.Connection == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}

	var stmt *C.dpiStmt
	var results []string
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

			results = append(results, str)
			fmt.Println("Result : ", str)
		} else {
			return results, fmt.Errorf("failed to get query value")
		}
	}
	return results, nil
}

func CreateConnection(username string, password string, connectionString string, context *C.dpiContext) *C.dpiConn {
	var conn *C.dpiConn
	var errInfo C.dpiErrorInfo

	c_username := C.CString(username)
	c_password := C.CString(password)
	c_connectionString := C.CString(connectionString)

	defer C.free(unsafe.Pointer(c_username))
	defer C.free(unsafe.Pointer(c_password))
	defer C.free(unsafe.Pointer(c_connectionString))

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
		C.dpiContext_getError(context, &errInfo)
		fmt.Printf("Failed to create database connection Connection: %s", C.GoString(errInfo.message))
		return nil
	}
	return conn
}
