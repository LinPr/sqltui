package mysqlbe

import (
	"fmt"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
)

// Config holds the connection parameters for a MySQL server.
type Config struct {
	UserName string
	Password string
	Host     string
	Port     string
	DbName   string
}

// Dsn renders the config as a driver connection string:
// user:pass@tcp(host:port)/db?charset=utf8&parseTime=true
func (c Config) Dsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=true",
		c.UserName, c.Password, c.Host, c.Port, c.DbName)
}

// Backend adapts the mysql data-access layer to the db.Backend contract.
type Backend struct {
	db   *DB
	host string
}

var _ db.Backend = (*Backend)(nil)

// Connect opens a mysql connection and pings it before returning.
func Connect(cfg Config) (*Backend, error) {
	conn, err := NewDB(cfg.Dsn(), cfg.DbName)
	if err != nil {
		return nil, err
	}
	return &Backend{db: conn, host: cfg.Host}, nil
}

func (b *Backend) Kind() string { return "mysql" }

func (b *Backend) Title() string {
	return fmt.Sprintf("mysql://%s/%s", b.host, b.db.dbName)
}

// Run executes a statement, routing queries vs. mutations the same way the
// underlying data-access layer does. Query results come back as an
// all-string frame (NULL is not distinguishable from the empty string in the
// raw byte scan, so cells stay strings as-is).
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

// Namespaces lists the databases of the server.
func (b *Backend) Namespaces() ([]string, error) {
	return b.db.ShowDatabases()
}

// CurrentNamespace reports the database this connection was opened against,
// so browsers can scope their listing to it (optional interface, not part of
// db.Backend).
func (b *Backend) CurrentNamespace() string {
	if b.db == nil {
		return ""
	}
	return b.db.dbName
}

// Tables lists the tables of one database.
func (b *Backend) Tables(namespace string) ([]string, error) {
	return b.db.ShowDatabaseTables(namespace)
}

// FetchTable loads up to limit rows of namespace.table.
func (b *Backend) FetchTable(namespace, table string, limit int) (*data.Frame, error) {
	ident := quoteIdent(table)
	if namespace != "" {
		ident = quoteIdent(namespace) + "." + ident
	}
	query := fmt.Sprintf("select * from %s limit %d", ident, limit)
	fields, records, err := b.db.RawQuery(query)
	if err != nil {
		return nil, err
	}
	return db.StringFrame(fields, records), nil
}

// PrimaryKeys lists the primary-key column names of namespace.table.
func (b *Backend) PrimaryKeys(namespace, table string) ([]string, error) {
	return b.db.PrimaryKeys(namespace, table)
}

func (b *Backend) Close() error {
	return b.db.Close()
}
