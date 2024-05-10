package mysql

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"strings"
)

const (
	// DQL
	SELECT = "select"
	SHOW   = "show"

	// DDL & DML & DCL & TCL ....
)

var (
	DbClinet *DB
)

type DB struct {
	*sql.DB
	dsn string
}

func NewDB(dsn string) (*DB, error) {
	if DbClinet != nil {
		return DbClinet, nil
	}
	dbc, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := dbc.Ping(); err != nil {
		return nil, err
	}

	DbClinet = &DB{
		DB:  dbc,
		dsn: dsn,
	}
	return DbClinet, nil
}

func GetDB() *DB {
	return DbClinet
}

func GetDbName() string {
	dbName := strings.Split(
		strings.Split(DbClinet.dsn, "/")[1],
		"?")[0]
	return dbName
}

type RawCommandResult struct {
	Fields  []string
	Records [][]string
	Result  sql.Result
	IsDQL   bool
}

func (db *DB) RawSqlCommand(query string) (rawCmdResult RawCommandResult, err error) {
	cmd := strings.Split(strings.Trim(query, " "), " ")
	if len(cmd) == 0 {
		return rawCmdResult, fmt.Errorf("empty query")
	}

	switch strings.ToLower(cmd[0]) {
	case SELECT, SHOW:
		rawCmdResult.IsDQL = true
		rawCmdResult.Fields, rawCmdResult.Records, err = db.RawQuery(query)
		return rawCmdResult, err
	default:
		rawCmdResult.IsDQL = false
		rawCmdResult.Result, err = db.RawExec(query)
		return rawCmdResult, err
	}
}

func (db *DB) RawQuery(query string) (fields []string, records [][]string, err error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}

	records, err = readRecords(rows)
	if err != nil {
		return nil, nil, err
	}

	query = strings.ToLower(strings.TrimSuffix(query, ";"))
	words := strings.Split(query, " ")
	var tableName string
	for i, word := range words {
		switch word {
		case "from":
			tableName = words[i+1]
			break
		case "show":
			// TODO: 处理一些特殊情况
		}
	}

	if tableName != "" {
		fields, err = db.FetchTableFields(tableName)
		if err != nil {
			return nil, nil, err
		}
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

	var databases []string
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}

	return databases, nil
}

func (db *DB) ShowDatabaseTables(database string) ([]string, error) {
	query := "show tables"
	if database != "" {
		query = fmt.Sprintf("show tables from %s", database)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *DB) ShowCurrentDatabaseTables() ([]string, error) {
	return db.ShowDatabaseTables("")
}

func (db *DB) FetchTableFields(table string) ([]string, error) {
	query := fmt.Sprintf("describe %s", table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

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
	query := fmt.Sprintf("select * from %s", table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	records, err := readRecords(rows)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (db *DB) Close() {
	db.Close()
}

func readRecords(rows *sql.Rows) ([][]string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var records [][]string
	for rows.Next() {
		record := make([]any, len(columns), len(columns))
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
		log.Printf("------ currentRow: %+v", currentRow)
		records = append(records, currentRow)
	}
	log.Printf("------ records: %+v", records)
	return records, nil
}
