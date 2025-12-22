//go:build !no_oracle

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
	"strings"
	"sync"
	"unsafe"
)

// Database represents an Oracle database connection using ODPI-C. It holds the connection handle and context for database operations.
type Database struct {
	Connection *C.dpiConn
	Context    *C.dpiContext
}

// Global variable to hold the database connection
var (
	dbConn  *Database
	dbOnce  sync.Once
	dbMutex sync.Mutex
	dbErr   error
)

// getDBInstance returns the singleton database connection instance.
// It creates a new connection if one doesn't exist. This function is thread-safe and ensures only one database connection is maintained.
func getDBInstance() (*Database, error) {
	dbOnce.Do(func() {
		dbConn, dbErr = newDatabaseConnection()
	})
	if dbErr != nil {
		return nil, dbErr
	}
	if dbConn == nil || dbConn.Connection == nil {
		return nil, fmt.Errorf("failed to establish database connection")
	}
	return dbConn, nil
}

// CleanupDBConnection releases the database connection and context resources.
// It should be called when the application shuts down to ensure proper cleanup of Oracle resources. This function is thread-safe.
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
	dbErr = nil
	dbOnce = sync.Once{} // Reset sync.Once for potential future use
}

// newDatabaseConnection creates a new Oracle database connection using
// configuration settings. It initializes the DPI context and establishes a connection to the Oracle database.
func newDatabaseConnection() (*Database, error) {
	// Load the configurations
	dbConfigs, err := config.GetDefaultDatabaseConfigurations()
	if err != nil {
		return nil, err
	}

	// Set connection parameters
	username := dbConfigs.DatabaseSettings.Username
	password := dbConfigs.DatabaseSettings.Password
	connectionString := fmt.Sprintf("%s:%s/%s", dbConfigs.DatabaseSettings.Host, fmt.Sprint(dbConfigs.DatabaseSettings.Port), dbConfigs.DatabaseSettings.Database)

	// Set Context for the connection
	context, err := setContext()
	if err != nil {
		return nil, err
	}

	// Connect to the database := implicitly declare *C.dpiConn type
	conn, err := createConnection(username, password, connectionString, context)
	if err != nil {
		C.dpiContext_destroy(context)
		return nil, err
	}

	// Return the database connection instance
	return &Database{
		Connection: conn,
		Context:    context,
	}, nil
}

// setContext creates and initializes a new DPI context with the current
// ODPI library version. The context must be destroyed when no longer needed.
func setContext() (*C.dpiContext, error) {
	var context *C.dpiContext
	var contextError C.dpiErrorInfo

	if C.dpiContext_createWithParams(C.DPI_MAJOR_VERSION, C.DPI_MINOR_VERSION, nil, &context, &contextError) != C.DPI_SUCCESS {
		return nil, fmt.Errorf("failed to create DPI Context: %s", C.GoString(contextError.message))
	}
	return context, nil
}

// prepareStatement prepares an SQL statement for execution.
// It returns a statement handle that must be released after use.
func prepareStatement(db *Database, query string) (*C.dpiStmt, error) {
	var stmt *C.dpiStmt
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

// ExecuteStatement executes the given SQL statement without returning results.
// It's suitable for INSERT, UPDATE, DELETE, or DDL statements.
func ExecuteStatement(query string) error {
	var stmt *C.dpiStmt

	// Get the database instance
	db, err := getDBInstance()
	if err != nil {
		return err
	}

	stmt, err = prepareStatement(db, query)
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

// executeAndReturnStatement executes the given SQL query and returns
// the statement object for further processing.
// IMPORTANT: The caller is responsible for releasing the statement.
func executeAndReturnStatement(query string) (*C.dpiStmt, error) {
	db, err := getDBInstance()
	if err != nil {
		return nil, err
	}

	stmt, err := prepareStatement(db, query)
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

// Fetch executes a SELECT query and returns all results as a slice of strings.
// It fetches only the first column from each row. The statement is automatically
// released after fetching completes.
func Fetch(query string) ([]string, error) {
	var err error
	var results []string

	stmt, err := executeAndReturnStatement(query)
	if err != nil {
		return nil, err
	}
	defer C.dpiStmt_release(stmt) // Release the statement after execution

	// Fetch the result
	for {
		var found C.int
		var bufferRowIndex C.uint32_t

		fetch := C.dpiStmt_fetch(stmt, &found, &bufferRowIndex)
		if fetch != C.DPI_SUCCESS {
			var errInfo C.dpiErrorInfo
			db, err := getDBInstance()
			if err != nil {
				return results, fmt.Errorf("failed to fetch data: %s", err)
			}
			C.dpiContext_getError(db.Context, &errInfo)
			return results, fmt.Errorf("failed to fetch data: %s", C.GoString(errInfo.message))
		}
		if found == 0 {
			break // No more rows
		}
		var data *C.dpiData
		var nativeTypeNum C.dpiNativeTypeNum

		if C.dpiStmt_getQueryValue(stmt, 1, &nativeTypeNum, &data) == C.DPI_SUCCESS {
			ptr := C.getAsBytesPtr(data)
			length := C.getAsBytesLength(data)
			str := C.GoStringN(ptr, C.int(length))

			results = append(results, str)
		} else {
			return results, fmt.Errorf("failed to get query value")
		}
	}
	return results, nil
}

// createConnection establishes a connection to an Oracle database using the provided credentials and connection string. It returns a connection
// handle that must be released when no longer needed.
func createConnection(username string, password string, connectionString string, context *C.dpiContext) (*C.dpiConn, error) {
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
		C.uint32_t(len(username)),
		c_password,
		C.uint32_t(len(password)),
		c_connectionString,
		C.uint32_t(len(connectionString)),
		nil, // dpiCommonParams
		nil, // dpiConnCreateParams
		&conn) == C.DPI_SUCCESS {
		fmt.Println("Connected to the database")
	} else {
		C.dpiContext_getError(context, &errInfo)
		return nil, fmt.Errorf("failed to create database connection: %s", C.GoString(errInfo.message))
	}
	return conn, nil
}

// PackageExists checks if a specific package exists and is valid in the connected Oracle database.
func PackageExists(packageName string) (bool, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) 
						  FROM user_objects 
						  WHERE object_type = 'PACKAGE' 
						  AND object_name = UPPER('%s') 
						  AND status = 'VALID'`, strings.ToUpper(packageName))
	results, err := Fetch(query)
	if err != nil {
		return false, err
	}
	if len(results) > 0 {
		return true, nil
	}
	return false, nil
}

// DeployPackage deploys the given SQL package to the connected Oracle database.
// Package structure should contain Package Specification and Package Body as a single string in the correct order.
func DeployPackages(sequences []string, packageSpec []string, packageBody []string) error {
	// Execution Order: Sequence -> Package Specification -> Package Body
	// Step 1: Deploy Sequences
	if sequences != nil {
		for _, seq := range sequences {
			if err := ExecuteStatement(seq); err != nil {
				return fmt.Errorf("failed to deploy sequence: %s", err)
			}
		}
	}

	// Step 2: Deploy Package Specifications
	if packageSpec != nil {
		for _, spec := range packageSpec {
			if err := ExecuteStatement(spec); err != nil {
				return fmt.Errorf("failed to deploy package specification: %s", err)
			}
		}
	}

	// Step 3: Deploy Package Body
	if packageBody != nil {
		for _, body := range packageBody {
			if err := ExecuteStatement(body); err != nil {
				return fmt.Errorf("failed to deploy package body: %s", err)
			}
		}
	}

	return nil
}
