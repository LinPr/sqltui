package postgresbe

import (
	"fmt"
	"strings"
	"time"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
)

// Config holds the connection parameters for a PostgreSQL server.
type Config struct {
	UserName string
	Password string
	Host     string
	Port     string
	DbName   string
	SslMode  string
}

// Dsn renders the config as a driver connection string:
// postgres://user:pass@host:port/dbname?sslmode=<sslmode>
func (c Config) Dsn() string {
	return BuildDsn(c.UserName, c.Password, c.Host, c.Port, c.DbName, c.SslMode)
}

// Backend adapts the postgres data-access layer to the db.Backend contract.
type Backend struct {
	db   *DB
	host string
}

var _ db.Backend = (*Backend)(nil)

// Connect opens a postgres connection and pings it before returning.
func Connect(cfg Config) (*Backend, error) {
	conn, err := NewDB(cfg.Dsn(), cfg.DbName)
	if err != nil {
		return nil, err
	}
	return &Backend{db: conn, host: cfg.Host}, nil
}

func (b *Backend) Kind() string { return "postgres" }

func (b *Backend) Title() string {
	return fmt.Sprintf("postgres://%s/%s", b.host, b.db.dbName)
}

// Run executes a statement with the same DQL / RETURNING routing as the
// underlying data-access layer, but scans query results into typed frame
// cells instead of pre-rendered strings.
func (b *Backend) Run(stmt string) (db.Result, error) {
	words := strings.Fields(strings.TrimSpace(stmt))
	if len(words) == 0 {
		return db.Result{}, fmt.Errorf("empty query")
	}

	isDQL := false
	switch strings.ToLower(words[0]) {
	case SELECT, WITH, SHOW, EXPLAIN, VALUES, TABLE:
		isDQL = true
	default:
		// INSERT/UPDATE/DELETE ... RETURNING also produces a result set
		isDQL = hasReturningClause(words)
	}

	if isDQL {
		frame, err := b.queryFrame(stmt)
		if err != nil {
			return db.Result{}, err
		}
		return db.Result{Frame: frame}, nil
	}

	res, err := b.db.RawExec(stmt)
	if err != nil {
		return db.Result{}, err
	}
	exec := &db.ExecResult{HasLastInsert: false}
	if res != nil {
		if n, err := res.RowsAffected(); err == nil {
			exec.RowsAffected = n
		}
	}
	return db.Result{Exec: exec}, nil
}

// Namespaces lists the user schemas of the database.
func (b *Backend) Namespaces() ([]string, error) {
	return b.db.ListSchemas()
}

// CurrentNamespace reports the schema new objects resolve to on this
// connection (usually "public"), so browsers can scope their listing to it
// (optional interface, not part of db.Backend).
func (b *Backend) CurrentNamespace() string {
	if b.db != nil && b.db.DB != nil {
		var schema string
		if err := b.db.QueryRow("SELECT current_schema()").Scan(&schema); err == nil && schema != "" {
			return schema
		}
	}
	return "public"
}

// Tables lists the tables of one schema.
func (b *Backend) Tables(namespace string) ([]string, error) {
	return b.db.ListTables(namespace)
}

// FetchTable loads up to limit rows of namespace.table.
func (b *Backend) FetchTable(namespace, table string, limit int) (*data.Frame, error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d",
		quoteIdentifier(namespace), quoteIdentifier(table), limit)
	return b.queryFrame(query)
}

// PrimaryKeys lists the primary-key column names of namespace.table.
func (b *Backend) PrimaryKeys(namespace, table string) ([]string, error) {
	return b.db.ListPrimaryKeys(namespace, table)
}

func (b *Backend) Close() error {
	return b.db.Close()
}

// queryFrame runs a query and scans the result set into a frame, keeping
// driver-native values where the frame model allows them.
func (b *Backend) queryFrame(query string) (*data.Frame, error) {
	rows, err := b.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	frame := data.New(fields...)
	for rows.Next() {
		values := make([]any, len(fields))
		for i := range values {
			values[i] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return nil, err
		}
		row := make([]any, len(fields))
		for i, value := range values {
			row[i] = cellValue(*(value.(*any)))
		}
		frame.AppendRow(row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return frame, nil
}

// cellValue converts a scanned driver value into a frame cell: nil stays
// nil, []byte becomes string, time.Time is kept, the other native scalar
// types pass through, and everything else is rendered like the legacy
// string formatter did.
func cellValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v
	case string, int64, float64, bool:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
