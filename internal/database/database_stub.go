//go:build no_oracle
// +build no_oracle

/*
This file contains stub implementations for database functions when Oracle support is not available.
*/
package database

import (
	"errors"
	"sync"
)

var ErrOracleNotSupported = errors.New("Oracle support not compiled in (built with no_oracle tag)")

type Database struct {
	Connection interface{}
	Context    interface{}
}

var (
	dbConn  *Database
	dbMutex sync.Mutex
)

func getDBInstance() (*Database, error) {
	return nil, ErrOracleNotSupported
}

func CleanupDBConnection() {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	dbConn = nil
}

func newDatabaseConnection() (*Database, error) {
	return nil, ErrOracleNotSupported
}

func setContext() (interface{}, error) {
	return nil, ErrOracleNotSupported
}

func prepareStatement(db *Database, query string) (interface{}, error) {
	return nil, ErrOracleNotSupported
}

func ExecuteStatement(query string) error {
	return ErrOracleNotSupported
}

func executeAndReturnStatement(query string, stmt interface{}) (interface{}, error) {
	return nil, ErrOracleNotSupported
}

func Fetch(query string) ([]string, error) {
	return nil, ErrOracleNotSupported
}

func createConnection(username string, password string, connectionString string, context interface{}) (interface{}, error) {
	return nil, ErrOracleNotSupported
}
