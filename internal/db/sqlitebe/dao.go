package sqlitebe

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	// DQL
	SELECT  = "select"
	PRAGMA  = "pragma"
	WITH    = "with"
	EXPLAIN = "explain"
	VALUES  = "values"

	// DDL & DML & DCL & TCL ....

	// max rows fetched when browsing a table from the tree view
	FetchLimit = 200
)

var (
	DbClinet *DB
)

type DB struct {
	*sql.DB
	File string
}

func NewDB(file string) (*DB, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return nil, fmt.Errorf("sqlite file path is empty")
	}

	// reuse the cached client only if it points to the same file
	if DbClinet != nil && DbClinet.File == file {
		return DbClinet, nil
	}

	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	dbc, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}

	if err := dbc.Ping(); err != nil {
		dbc.Close()
		return nil, err
	}

	// replace the previously cached client (re-login with another file)
	if DbClinet != nil {
		DbClinet.DB.Close()
	}

	DbClinet = &DB{
		DB:   dbc,
		File: file,
	}
	return DbClinet, nil
}

func GetDB() *DB {
	return DbClinet
}

func GetDbFile() string {
	if DbClinet == nil {
		return ""
	}
	return DbClinet.File
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
	case SELECT, PRAGMA, WITH, EXPLAIN, VALUES:
		rawCmdResult.IsDQL = true
	default:
		// INSERT/UPDATE/DELETE ... RETURNING (SQLite >= 3.35) also produces
		// a result set that db.Exec would silently discard.
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

	fields, err = rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	records, err = readRecords(rows)
	if err != nil {
		return nil, nil, err
	}

	return fields, records, nil
}

func (db *DB) RawExec(query string) (sql.Result, error) {
	res, err := db.Exec(query)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListTables lists all user tables of the sqlite database file.
func (db *DB) ListTables() ([]string, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	rows, err := db.Query(query)
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

// FetchTableFields returns the column names of a table via PRAGMA table_info.
func (db *DB) FetchTableFields(table string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", quoteIdentifier(table))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := readRecords(rows)
	if err != nil {
		return nil, err
	}

	// PRAGMA table_info columns: cid, name, type, notnull, dflt_value, pk
	fields := make([]string, 0, len(records))
	for _, record := range records {
		if len(record) > 1 {
			fields = append(fields, record[1])
		}
	}
	return fields, nil
}

func (db *DB) FetchTableRecords(table string) ([][]string, error) {
	query := fmt.Sprintf("select * from %s limit %d", quoteIdentifier(table), FetchLimit)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return readRecords(rows)
}

// ListPrimaryKeys lists the primary-key column names of table, in pk-ordinal
// order. Empty when the table has no primary key. The namespace argument is
// ignored: sqlite files have no schema level.
func (db *DB) ListPrimaryKeys(table string) ([]string, error) {
	query := fmt.Sprintf("SELECT name FROM pragma_table_info(%s) WHERE pk > 0 ORDER BY pk", quoteString(table))
	_, records, err := db.RawQuery(query)
	if err != nil {
		return nil, err
	}
	pks := make([]string, 0, len(records))
	for _, record := range records {
		if len(record) > 0 {
			pks = append(pks, record[0])
		}
	}
	return pks, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

// quoteIdentifier quotes a sqlite identifier with double quotes, doubling
// any embedded double quote.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// quoteString wraps name in a single-quoted SQL string literal, doubling any
// embedded single quote. Used for the table argument of pragma_table_info.
func quoteString(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func readRecords(rows *sql.Rows) ([][]string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var records [][]string
	for rows.Next() {
		record := make([]any, len(columns))
		for i := range record {
			record[i] = &sql.RawBytes{}
		}

		if err := rows.Scan(record...); err != nil {
			return nil, err
		}
		currentRow := make([]string, 0, len(columns))
		for _, rawValue := range record {
			currentRow = append(currentRow, string(*rawValue.(*sql.RawBytes)))
		}
		records = append(records, currentRow)
	}
	return records, rows.Err()
}
