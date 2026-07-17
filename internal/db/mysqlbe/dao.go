package mysqlbe

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"

	dbapi "github.com/LinPr/sqltui/internal/db"
)

const (
	// DQL
	SELECT  = "select"
	SHOW    = "show"
	DESC    = "desc"
	DESCRB  = "describe"
	WITH    = "with"
	EXPLAIN = "explain"
	TABLE   = "table"  // MySQL 8: TABLE t
	VALUES  = "values" // MySQL 8: VALUES ROW(...)

	// DDL & DML & DCL & TCL ....

	// max rows fetched when browsing a table from the tree view
	FetchLimit = 200
)

var (
	DbClinet *DB
	// clientMu serializes the check-close-assign swap of DbClinet: dial
	// commands run in goroutines and two overlapping reconnects must not
	// race on the global client.
	clientMu sync.Mutex
)

type DB struct {
	*sql.DB
	dsn    string
	dbName string
}

func NewDB(dsn string, dbName string) (*DB, error) {
	dbc, err := sql.Open("mysql", dsn)
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
	if DbClinet != nil {
		DbClinet.DB.Close()
	}

	DbClinet = &DB{
		DB:     dbc,
		dsn:    dsn,
		dbName: dbName,
	}
	return DbClinet, nil
}

func GetDB() *DB {
	return DbClinet
}

func GetDbName() string {
	if DbClinet == nil {
		return ""
	}
	return DbClinet.dbName
}

type RawCommandResult struct {
	Fields  []string
	Records [][]string
	Result  sql.Result
	IsDQL   bool
}

func (db *DB) RawSqlCommand(query string) (rawCmdResult RawCommandResult, err error) {
	cmd := strings.Fields(strings.TrimSpace(query))
	if len(cmd) == 0 {
		return rawCmdResult, fmt.Errorf("empty query")
	}

	first := strings.ToLower(cmd[0])
	switch first {
	case SELECT, SHOW, DESC, DESCRB, WITH, EXPLAIN, TABLE, VALUES:
		rawCmdResult.IsDQL = true
	default:
		switch {
		case strings.HasPrefix(first, "("):
			// parenthesized set queries, e.g. (SELECT 1) UNION (SELECT 2)
			rawCmdResult.IsDQL = true
		default:
			// INSERT/UPDATE/DELETE ... RETURNING (MariaDB) also produces a
			// result set that db.Exec would silently discard.
			rawCmdResult.IsDQL = hasReturningClause(cmd)
		}
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

	// use the result set column names as the table header
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

func (db *DB) ShowDatabases() ([]string, error) {
	query := "show databases"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}

	return databases, rows.Err()
}

func (db *DB) ShowDatabaseTables(database string) ([]string, error) {
	query := "show tables"
	if database != "" {
		query = "show tables from " + quoteIdent(database)
	}

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

func (db *DB) ShowCurrentDatabaseTables() ([]string, error) {
	return db.ShowDatabaseTables("")
}

func (db *DB) FetchTableFields(table string) ([]string, error) {
	query := fmt.Sprintf("describe %s", quoteIdent(table))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := readRecords(rows)
	if err != nil {
		return nil, err
	}

	fields := []string{}
	for _, record := range records {
		fields = append(fields, record[0])
	}
	return fields, nil
}

func (db *DB) FetchTableRecords(table string) ([][]string, error) {
	query := fmt.Sprintf("select * from %s limit %d", quoteIdent(table), FetchLimit)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := readRecords(rows)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

// PrimaryKeys lists the primary-key column names of database.table, in
// key-ordinal order. If database is empty the connection's current database
// is used.
func (db *DB) PrimaryKeys(database, table string) ([]string, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("mysql connection is not open")
	}
	if database == "" {
		database = db.dbName
	}
	query := `SELECT column_name FROM information_schema.key_column_usage
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`
	rows, err := db.Query(query, database, table)
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

// ColumnsMeta returns column metadata for database.table, in ordinal order.
// If database is empty the connection's current database is used.
func (db *DB) ColumnsMeta(database, table string) ([]dbapi.ColumnMeta, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("mysql connection is not open")
	}
	if database == "" {
		database = db.dbName
	}
	query := `SELECT column_name, data_type, is_nullable, column_default, column_comment
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`
	rows, err := db.Query(query, database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []dbapi.ColumnMeta
	for rows.Next() {
		var c dbapi.ColumnMeta
		var defaultVal sql.NullString
		if err := rows.Scan(&c.Name, &c.DataType, &c.IsNullable, &defaultVal, &c.Comment); err != nil {
			return nil, err
		}
		if defaultVal.Valid {
			c.Default = defaultVal.String
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// ColumnIndexTypes returns a column→index-type map for database.table.
// Priority: PK > UNIQUE > INDEX. Columns not in any index are absent from the map.
func (db *DB) ColumnIndexTypes(database, table string) (map[string]string, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("mysql connection is not open")
	}
	if database == "" {
		database = db.dbName
	}
	query := `SELECT column_name, index_name, non_unique
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		ORDER BY CASE WHEN index_name = 'PRIMARY' THEN 0 WHEN non_unique = 0 THEN 1 ELSE 2 END, seq_in_index`
	rows, err := db.Query(query, database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var colName, indexName string
		var nonUnique int
		if err := rows.Scan(&colName, &indexName, &nonUnique); err != nil {
			return nil, err
		}
		label := "INDEX"
		if indexName == "PRIMARY" {
			label = "PK"
		} else if nonUnique == 0 {
			label = "UNIQUE"
		}
		// Only set if not already set by a higher-priority index
		if existing, ok := result[colName]; !ok || indexPriority(existing) < indexPriority(label) {
			result[colName] = label
		}
	}
	return result, rows.Err()
}

func indexPriority(label string) int {
	switch label {
	case "PK":
		return 3
	case "UNIQUE":
		return 2
	case "INDEX":
		return 1
	}
	return 0
}

// quoteIdent quotes a mysql identifier with backticks, doubling any
// embedded backtick.
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func readRecords(rows *sql.Rows) ([][]string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var records [][]string
	for rows.Next() {
		record := make([]any, len(columns))
		for i := range columns {
			record[i] = &sql.RawBytes{}
		}

		if err = rows.Scan(record...); err != nil {
			return nil, err
		}
		var currentRow []string
		for _, rawValue := range record {
			field := string(*rawValue.(*sql.RawBytes))
			currentRow = append(currentRow, field)
		}
		records = append(records, currentRow)
	}
	return records, rows.Err()
}
