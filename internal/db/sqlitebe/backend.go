package sqlitebe

import (
	"fmt"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
)

// Config holds the connection parameters for a sqlite database file.
type Config struct {
	FilePath string
}

// Backend adapts the sqlite data-access layer to the db.Backend contract.
type Backend struct {
	db *DB
}

var _ db.Backend = (*Backend)(nil)

// Connect opens the sqlite file and pings it before returning. The file
// must already exist.
func Connect(cfg Config) (*Backend, error) {
	conn, err := NewDB(cfg.FilePath)
	if err != nil {
		return nil, err
	}
	return &Backend{db: conn}, nil
}

func (b *Backend) Kind() string { return "sqlite" }

func (b *Backend) Title() string {
	return "sqlite://" + b.db.File
}

// Run executes a statement, routing queries vs. mutations the same way the
// underlying data-access layer does. Query results come back as an
// all-string frame.
func (b *Backend) Run(stmt string) (db.Result, error) {
	raw, err := b.db.RawSqlCommand(stmt)
	if err != nil {
		return db.Result{}, err
	}
	if raw.IsDQL {
		return db.Result{Frame: db.StringFrame(raw.Fields, raw.Records)}, nil
	}

	exec := &db.ExecResult{HasLastInsert: true}
	if raw.Result != nil {
		if n, err := raw.Result.RowsAffected(); err == nil {
			exec.RowsAffected = n
		}
		if id, err := raw.Result.LastInsertId(); err == nil {
			exec.LastInsertID = id
		}
	}
	return db.Result{Exec: exec}, nil
}

// Namespaces returns a single empty namespace: sqlite files have no
// database/schema level.
func (b *Backend) Namespaces() ([]string, error) {
	return []string{""}, nil
}

// Tables lists all user tables; the namespace argument is ignored.
func (b *Backend) Tables(string) ([]string, error) {
	return b.db.ListTables()
}

// FetchTable loads up to limit rows of a table; the namespace argument is
// ignored.
func (b *Backend) FetchTable(_ string, table string, limit int) (*data.Frame, error) {
	query := fmt.Sprintf("select * from %s limit %d", quoteIdentifier(table), limit)
	fields, records, err := b.db.RawQuery(query)
	if err != nil {
		return nil, err
	}
	return db.StringFrame(fields, records), nil
}

func (b *Backend) Close() error {
	return b.db.Close()
}
