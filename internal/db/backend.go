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

// ColumnMeta describes one column of a live table, for the info overlay
// and the row sheet type display. DataType is the engine-native type name
// (e.g. "int", "varchar(255)", "text"); IsNullable is "YES"/"NO" or "" when
// unknown; Default is the column default literal or ""; Comment is the
// column comment or "".
type ColumnMeta struct {
	Name       string
	DataType   string
	IsNullable string
	Default    string
	Comment    string
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
	// PrimaryKeys lists the primary-key column names of namespace.table
	// (empty when the table has no primary key). Engines without namespaces
	// ignore namespace.
	PrimaryKeys(namespace, table string) ([]string, error)
	// ColumnsMeta returns column metadata for namespace.table, in ordinal
	// order. Engines without namespaces ignore namespace.
	ColumnsMeta(namespace, table string) ([]ColumnMeta, error)
	// ColumnIndexTypes returns a map of column name → index type label for each
	// indexed column in namespace.table. Labels are "PK", "UNIQUE", or "INDEX"
	// (highest priority wins when a column appears in multiple indexes).
	// Returns an empty map (not an error) when the table has no indexes.
	ColumnIndexTypes(namespace, table string) (map[string]string, error)
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
