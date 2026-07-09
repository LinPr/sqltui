// Package db defines the interface between the UI and live database
// connections (MySQL, PostgreSQL, SQLite files, Redis).
package db

import (
	"github.com/LinPr/sqltui/internal/data"
)

// ExecResult reports the outcome of a non-query statement.
type ExecResult struct {
	RowsAffected  int64
	LastInsertID  int64
	HasLastInsert bool // false for engines without insert ids (e.g. postgres)
}

// Result is either a row set (Frame != nil) or an exec outcome.
type Result struct {
	Frame *data.Frame
	Exec  *ExecResult
}

// Backend is a live SQL database connection.
type Backend interface {
	// Kind identifies the engine: "mysql", "postgres" or "sqlite".
	Kind() string
	// Title is a short human label for the connection (e.g. "mysql://host/db").
	Title() string
	// Run executes a statement, routing queries vs. mutations automatically.
	Run(stmt string) (Result, error)
	// Namespaces lists databases (mysql) or schemas (postgres). Engines
	// without namespaces return a single empty string.
	Namespaces() ([]string, error)
	// Tables lists the tables in a namespace.
	Tables(namespace string) ([]string, error)
	// FetchTable loads up to limit rows of a table.
	FetchTable(namespace, table string, limit int) (*data.Frame, error)
	Close() error
}

// KVBackend is a live key-value store connection (Redis).
type KVBackend interface {
	// Title is a short human label for the connection.
	Title() string
	// Do runs a raw command and returns a rendered result.
	Do(args []string) (string, error)
	// ScanKeys lists keys of one type, up to a server-side cap.
	ScanKeys(keyType string) ([]string, error)
	// Value fetches and renders the value stored at key.
	Value(key string) (string, error)
	Close() error
}
