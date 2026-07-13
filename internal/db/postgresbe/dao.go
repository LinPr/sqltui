package postgresbe

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	// DQL keywords
	SELECT  = "select"
	WITH    = "with"
	SHOW    = "show"
	EXPLAIN = "explain"
	VALUES  = "values"
	TABLE   = "table"

	// max rows fetched when browsing a table from the tree view
	FetchLimit = 200
)

var (
	DbClient *DB
	// clientMu serializes the check-close-assign swap of DbClient: dial
	// commands run in goroutines and two overlapping reconnects must not
	// race on the global client.
	clientMu sync.Mutex
)

type DB struct {
	*sql.DB
	dsn    string
	dbName string
}

// BuildDsn builds a postgres connection string of the form
// postgres://user:pass@host:port/dbname?sslmode=<sslMode>
func BuildDsn(userName, password, host, port, dbName, sslMode string) string {
	if sslMode == "" {
		sslMode = "disable"
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(userName, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + dbName,
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
	}
	return u.String()
}

func NewDB(dsn string, dbName string) (*DB, error) {
	dbc, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	if err := dbc.Ping(); err != nil {
		dbc.Close()
		return nil, err
	}

	clientMu.Lock()
	defer clientMu.Unlock()
	// close the previous connection when re-connecting
	if DbClient != nil {
		DbClient.DB.Close()
	}

	DbClient = &DB{
		DB:     dbc,
		dsn:    dsn,
		dbName: dbName,
	}
	return DbClient, nil
}

func GetDB() *DB {
	return DbClient
}

func GetDbName() string {
	if DbClient == nil {
		return ""
	}
	return DbClient.dbName
}

type RawCommandResult struct {
	Fields  []string
	Records [][]string
	Result  sql.Result
	IsDQL   bool
}

func (db *DB) RawSqlCommand(query string) (rawCmdResult RawCommandResult, err error) {
	words := strings.Fields(strings.TrimSpace(query))
	if len(words) == 0 {
		return rawCmdResult, fmt.Errorf("empty query")
	}

	switch strings.ToLower(words[0]) {
	case SELECT, WITH, SHOW, EXPLAIN, VALUES, TABLE:
		rawCmdResult.IsDQL = true
	default:
		// INSERT/UPDATE/DELETE ... RETURNING also produces a result set
		rawCmdResult.IsDQL = hasReturningClause(words)
	}

	if rawCmdResult.IsDQL {
		rawCmdResult.Fields, rawCmdResult.Records, err = db.RawQuery(query)
		return rawCmdResult, err
	}
	rawCmdResult.Result, err = db.RawExec(query)
	return rawCmdResult, err
}

// hasReturningClause reports whether the statement contains a RETURNING
// clause, whose result set would be silently discarded by db.Exec.
func hasReturningClause(words []string) bool {
	for _, word := range words[1:] {
		if strings.EqualFold(word, "returning") {
			return true
		}
	}
	return false
}

func (db *DB) RawQuery(query string) (fields []string, records [][]string, err error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	return readRecords(rows)
}

func (db *DB) RawExec(query string) (sql.Result, error) {
	res, err := db.Exec(query)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (db *DB) ListSchemas() ([]string, error) {
	query := `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schema_name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	return schemas, rows.Err()
}

func (db *DB) ListTables(schema string) ([]string, error) {
	query := `SELECT table_name FROM information_schema.tables
		WHERE table_schema = $1
		ORDER BY table_name`
	rows, err := db.Query(query, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (db *DB) FetchTableRecords(schema, table string) (fields []string, records [][]string, err error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d",
		quoteIdentifier(schema), quoteIdentifier(table), FetchLimit)
	return db.RawQuery(query)
}

// ListPrimaryKeys lists the primary-key column names of schema.table, in
// index-ordinal order. Empty when the table has no primary key.
func (db *DB) ListPrimaryKeys(schema, table string) ([]string, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("postgres connection is not open")
	}
	regclass := quoteIdentifier(schema) + "." + quoteIdentifier(table)
	query := `SELECT a.attname FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indisprimary AND i.indrelid = $1::regclass
		ORDER BY array_position(i.indkey, a.attnum)`
	rows, err := db.Query(query, regclass)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pks []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		pks = append(pks, col)
	}
	return pks, rows.Err()
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func readRecords(rows *sql.Rows) (fields []string, records [][]string, err error) {
	fields, err = rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	for rows.Next() {
		values := make([]any, len(fields))
		for i := range values {
			values[i] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return nil, nil, err
		}

		var currentRow []string
		for _, value := range values {
			currentRow = append(currentRow, formatValue(*(value.(*any))))
		}
		records = append(records, currentRow)
	}
	return fields, records, rows.Err()
}

func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}
