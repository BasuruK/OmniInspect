package database

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
	"OmniView/internal/config"
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
func getDBInstance() (*Database, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if dbConn == nil {
		dbConn = newDatabaseConnection()
	}
	if dbConn == nil || dbConn.Connection == nil {
		return nil, fmt.Errorf("failed to establish database connection")
	}
	return dbConn, nil
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

func newDatabaseConnection() *Database {
	// Load the configurations
	dbConfigs := config.LoadConfigurations().DatabaseSettings

	// Set connection parameters
	username := dbConfigs.Username
	password := dbConfigs.Password
	connectionString := fmt.Sprintf("%s:%s/%s", dbConfigs.Host, fmt.Sprint(dbConfigs.Port), dbConfigs.Database)

	// Set Context for the connection
	context := setContext()

	// Connect to the database := implicitly declare *C.dpiConn type
	conn := createConnection(username, password, connectionString, context)

	// Return the database connection instance
	db := &Database{
		Connection: conn,
		Context:    context,
	}

	return db
}

func setContext() *C.dpiContext {
	var context *C.dpiContext
	var contextError C.dpiErrorInfo

	if C.dpiContext_createWithParams(C.DPI_MAJOR_VERSION, C.DPI_MINOR_VERSION, nil, &context, &contextError) != C.DPI_SUCCESS {
		fmt.Printf("Failed to create DPI Context: %s", C.GoString(contextError.message))
	}
	return context
}

func prepareStatement(db *Database, query string, stmt *C.dpiStmt) (*C.dpiStmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	// Prepare the statement
	if C.dpiConn_prepareStmt(db.Connection, 0, cQuery, C.uint32_t(len(query)), nil, 0, &stmt) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(db.Context, &errInfo)
		return nil, fmt.Errorf("failed to prepare statement: %s", C.GoString(errInfo.message))
	}
	return stmt, nil
}

func ExecuteStatement(query string) error {
	var stmt *C.dpiStmt

	// Get the database instance
	db, err := getDBInstance()
	if err != nil {
		return err
	}

	stmt, err = prepareStatement(db, query, stmt)
	if err != nil {
		return err
	}
	defer C.dpiStmt_release(stmt)

	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(db.Context, &errInfo)
		return fmt.Errorf("failed to execute statement: %s", C.GoString(errInfo.message))
	}

	return nil
}

// ExecuteAndReturnStatement executes the given SQL statement and returns the statement object
func executeAndReturnStatement(query string, stmt *C.dpiStmt) (*C.dpiStmt, error) {
	db, err := getDBInstance()
	if err != nil {
		return nil, err
	}

	stmt, err = prepareStatement(db, query, stmt)
	if err != nil {
		return nil, err
	}

	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(db.Context, &errInfo)
		return nil, fmt.Errorf("failed to execute statement: %s", C.GoString(errInfo.message))
	}

	return stmt, nil
}

func Fetch(query string) ([]string, error) {
	var stmt *C.dpiStmt
	var err error
	var results []string

	stmt, err = executeAndReturnStatement(query, stmt)
	if err != nil {
		return nil, err
	}
	defer C.dpiStmt_release(stmt) // Release the statement after execution

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

func createConnection(username string, password string, connectionString string, context *C.dpiContext) *C.dpiConn {
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
