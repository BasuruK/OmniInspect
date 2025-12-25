package oracle

/*
#cgo darwin CFLAGS: -I${SRCDIR}/../../../../third_party/odpi/include
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../../third_party/odpi/lib -lodpi -Wl,-rpath,${SRCDIR}/../../../../third_party/odpi/lib -Wl,-rpath,/opt/oracle/instantclient_23_7

#cgo windows CFLAGS: -I${SRCDIR}/../../../../third_party/odpi/include
#cgo windows LDFLAGS: -L${SRCDIR}/../../../../third_party/odpi/lib -lodpi -LC:/oracle_inst/instantclient_23_7 -loci

#include "dpi.h"
#include "dpi_go_helpers.h"
#include <stdio.h>
#include <stdlib.h>
*/
import "C"

import (
	"OmniView/internal/core/domain"
	"fmt"
	"strconv"
	"unsafe"
)

// Adapter: Implements ports.DatabaseRepository for Oracle database
type OracleAdapter struct {
	Connection *C.dpiConn
	Context    *C.dpiContext
	config     domain.DatabaseSettings
}

// Constructor: Creates a new OracleAdapter and injects configuratios
func NewOracleAdapter(cfg domain.DatabaseSettings) *OracleAdapter {
	return &OracleAdapter{
		config: cfg,
	}
}

func (oa *OracleAdapter) Fetch(query string) ([]string, error) {
	var err error

	stmt, err := oa.ExecuteAndReturnStatement(query)
	if err != nil {
		return nil, err
	}
	defer C.dpiStmt_release(stmt) // Release the statement after execution
	// Fetch the result using helper
	fetched, err := oa.FetchData(stmt)
	if err != nil {
		return nil, err
	}

	return fetched, nil
}

// executeAndReturnStatement executes the given SQL query and returns
// the statement object for further processing.
// IMPORTANT: The caller is responsible for releasing the statement.
func (oa *OracleAdapter) ExecuteAndReturnStatement(query string) (*C.dpiStmt, error) {
	stmt, err := oa.PrepareStatement(query)
	if err != nil {
		return nil, err
	}

	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return nil, fmt.Errorf("failed to execute statement: %s", C.GoString(errInfo.message))
	}

	return stmt, nil
}

// executeWithStatement executes the given SQL statement which is already prepared and returns
// the statement object for further processing.
// IMPORTANT: The caller is responsible for releasing the statement.
func (oa *OracleAdapter) ExecuteWithStatement(stmt *C.dpiStmt) error {
	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to execute statement: %s", C.GoString(errInfo.message))
	}

	return nil
}

// fetchData reads all rows from the provided statement and returns the
// first column values as a slice of strings. The caller is responsible
// for releasing the statement handle.
func (oa *OracleAdapter) FetchData(stmt *C.dpiStmt) ([]string, error) {
	var results []string
	for {
		var found C.int
		var bufferRowIndex C.uint32_t

		fetch := C.dpiStmt_fetch(stmt, &found, &bufferRowIndex)
		if fetch != C.DPI_SUCCESS {
			var errInfo C.dpiErrorInfo
			C.dpiContext_getError(oa.Context, &errInfo)
			return nil, fmt.Errorf("failed to fetch data: %s", C.GoString(errInfo.message))
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

// prepareStatement prepares an SQL statement for execution.
// It returns a statement handle that must be released after use.
func (oa *OracleAdapter) PrepareStatement(query string) (*C.dpiStmt, error) {
	var stmt *C.dpiStmt
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	// Prepare the statement
	if C.dpiConn_prepareStmt(oa.Connection, 0, cQuery, C.uint32_t(len(query)), nil, 0, &stmt) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return nil, fmt.Errorf("failed to prepare statement: %s", C.GoString(errInfo.message))
	}
	return stmt, nil
}

// ExecuteStatement executes the given SQL statement without returning results.
// It's suitable for INSERT, UPDATE, DELETE, or DDL statements.
func (oa *OracleAdapter) ExecuteStatement(query string) error {
	stmt, err := oa.PrepareStatement(query)
	if err != nil {
		return err
	}
	defer C.dpiStmt_release(stmt)

	// Execute the statement
	if C.dpiStmt_execute(stmt, C.DPI_MODE_EXEC_DEFAULT, nil) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to execute statement: %s", C.GoString(errInfo.message))
	}

	return nil
}

// Connect establishes the connection to the Oracle database using the injected configuration.
// It initializes the DPI context and the connection handle.
func (oa *OracleAdapter) Connect() error {
	// 1. Initialize the DPI context
	if err := oa.InitContext(); err != nil {
		return err
	}
	// 2. Create the connection string
	connectionString := fmt.Sprintf("%s:%s/%s", oa.config.Host, fmt.Sprint(oa.config.Port), oa.config.Database)
	// 3. Create the connection
	if err := oa.CreateConnection(oa.config.Username, oa.config.Password, connectionString); err != nil {
		// Cleanup context if connection fails
		C.dpiContext_destroy(oa.Context)
		oa.Context = nil
		return err
	}
	fmt.Println("Connected to the database")

	return nil
}

// Close releases the database connection and context resources.
func (oa *OracleAdapter) Close() error {
	if oa.Connection != nil {
		C.dpiConn_release(oa.Connection)
		oa.Connection = nil
	}
	if oa.Context != nil {
		C.dpiContext_destroy(oa.Context)
		oa.Context = nil
	}

	return nil
}

// setContext creates and initializes a new DPI context with the current
// ODPI library version. The context must be destroyed when no longer needed.
func (oa *OracleAdapter) InitContext() error {
	var contextError C.dpiErrorInfo

	if C.dpiContext_createWithParams(C.DPI_MAJOR_VERSION, C.DPI_MINOR_VERSION, nil, &oa.Context, &contextError) != C.DPI_SUCCESS {
		return fmt.Errorf("failed to create DPI Context: %s", C.GoString(contextError.message))
	}

	return nil
}

// createConnection establishes a connection to an Oracle database using the provided credentials and connection string. It returns a connection
// handle that must be released when no longer needed.
func (oa *OracleAdapter) CreateConnection(username string, password string, connectionString string) error {
	// Convert Go strings to C strings
	c_username := C.CString(username)
	c_password := C.CString(password)
	c_connectionString := C.CString(connectionString)

	// Ensure C strings are freed after the function exits
	defer C.free(unsafe.Pointer(c_username))
	defer C.free(unsafe.Pointer(c_password))
	defer C.free(unsafe.Pointer(c_connectionString))

	// Call ODPI-C function
	if C.dpiConn_create(oa.Context,
		c_username,
		C.uint32_t(len(username)),
		c_password,
		C.uint32_t(len(password)),
		c_connectionString,
		C.uint32_t(len(connectionString)),
		nil, // dpiCommonParams
		nil, // dpiConnCreateParams
		&oa.Connection) != C.DPI_SUCCESS {
		var errInfo C.dpiErrorInfo
		C.dpiContext_getError(oa.Context, &errInfo)
		return fmt.Errorf("failed to create database connection: %s", C.GoString(errInfo.message))
	}

	return nil
}

// bindParamsToQuery binds parameters to a prepared statement.
// It supports string, int, and float64 parameter types.
func (oa *OracleAdapter) BindParamsToQuery(stmt *C.dpiStmt, params map[string]interface{}) error {
	for name, value := range params {
		cName := C.CString(name)

		var dpiData C.dpiData
		var nativeType C.dpiNativeTypeNum

		switch v := value.(type) {
		case string:
			nativeType = C.DPI_NATIVE_TYPE_BYTES
			cValue := C.CString(v)
			C.initDPIDataAsBytes(&dpiData, cValue, C.uint32_t(len(v)))

			// Bind the parameter
			// For string values, the binding is done here to ensure proper memory management, and the C string is freed after binding.
			if C.dpiStmt_bindValueByName(stmt, cName, C.uint32_t(len(name)), nativeType, &dpiData) != C.DPI_SUCCESS {
				C.free(unsafe.Pointer(cName))
				C.free(unsafe.Pointer(cValue))
				return fmt.Errorf("failed to bind parameter %s with type %d", name, nativeType)
			}
			C.free(unsafe.Pointer(cName))
			C.free(unsafe.Pointer(cValue))
			continue
		case int:
			nativeType = C.DPI_NATIVE_TYPE_INT64
			C.initDPIDataAsInt64(&dpiData, C.int64_t(v))
		case float64:
			nativeType = C.DPI_NATIVE_TYPE_DOUBLE
			C.initDPIDataAsDouble(&dpiData, C.double(v))
		default:
			C.free(unsafe.Pointer(cName))
			return fmt.Errorf("unsupported parameter type for %s", name)
		}

		if C.dpiStmt_bindValueByName(stmt, cName, C.uint32_t(len(name)), nativeType, &dpiData) != C.DPI_SUCCESS {
			C.free(unsafe.Pointer(cName))

			var errInfo C.dpiErrorInfo
			C.dpiContext_getError(oa.Context, &errInfo)
			return fmt.Errorf("failed to bind parameter %s: %s with type %d", name, C.GoString(errInfo.message), nativeType)
		}
		C.free(unsafe.Pointer(cName))
	}
	return nil
}

// PackageExists checks if a specific package exists and is valid in the connected Oracle database.
func (oa *OracleAdapter) PackageExists(packageName string) (bool, error) {
	query := `SELECT COUNT(1) 
			FROM user_objects 
			WHERE object_type = 'PACKAGE' 
			AND object_name = UPPER(:packageName) 
			AND status = 'VALID'`

	results, err := oa.FetchWithParams(query, map[string]interface{}{
		"packageName": packageName,
	})

	if err != nil {
		return false, err
	}
	if len(results) == 0 {
		return false, fmt.Errorf("no results returned from package existence query")
	}

	count, err := strconv.Atoi(results[0])
	if err != nil {
		return false, fmt.Errorf("failed to parse count result: %v", err)
	}

	return count > 0, nil
}

// FetchWithParams executes a SELECT query with parameters and returns all results as a slice of strings.
func (oa *OracleAdapter) FetchWithParams(query string, params map[string]interface{}) ([]string, error) {
	stmt, err := oa.PrepareStatement(query)
	if err != nil {
		return nil, err
	}
	defer C.dpiStmt_release(stmt) // Release the statement after execution

	// Bind parameters to the statement
	if err = oa.BindParamsToQuery(stmt, params); err != nil {
		return nil, err
	}

	// Execute the statement after binding parameters
	err = oa.ExecuteWithStatement(stmt)
	if err != nil {
		return nil, err
	}

	// After binding params, fetch using helper
	fetched, err := oa.FetchData(stmt)
	if err != nil {
		return nil, err
	}

	return fetched, nil
}

// DeployPackages deploys the given SQL package to the connected Oracle database.
// Package structure should contain Package Specification and Package Body as a single string in the correct order.
func (oa *OracleAdapter) DeployPackages(sequences []string, packageSpec []string, packageBody []string) error {
	// Execution Order: Sequence -> Package Specification -> Package Body
	// The loops are nil safe, so empty slices will be skipped.
	// Step 1: Deploy Sequences
	for _, seq := range sequences {
		if err := oa.ExecuteStatement(seq); err != nil {
			return fmt.Errorf("failed to deploy sequence: %s", err)
		}
	}

	// Step 2: Deploy Package Specifications
	for _, spec := range packageSpec {
		if err := oa.ExecuteStatement(spec); err != nil {
			return fmt.Errorf("failed to deploy package specification: %s", err)
		}
	}

	// Step 3: Deploy Package Body
	for _, body := range packageBody {
		if err := oa.ExecuteStatement(body); err != nil {
			return fmt.Errorf("failed to deploy package body: %s", err)
		}
	}

	return nil
}

// DeployFile deploys a SQL file content to the connected Oracle database.
func (oa *OracleAdapter) DeployFile(sqlContent string) error {
	sequences, packageSpecs, packageBodies, err := Extract(sqlContent)
	if err != nil {
		return fmt.Errorf("failed to extract SQL content: %s", err)
	}

	if err := oa.DeployPackages(sequences, packageSpecs, packageBodies); err != nil {
		return fmt.Errorf("failed to deploy SQL content: %s", err)
	}

	return nil
}
