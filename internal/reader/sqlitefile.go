package reader

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatSQLite, sqliteFileReader{})
}

type sqliteFileReader struct{}

const sqliteMagic = "SQLite format 3\x00"

func (sqliteFileReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	if opt.Key != "" {
		return nil, fmt.Errorf("encrypted databases are not supported")
	}
	if err := checkSQLiteHeader(src.LocalPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", "file:"+src.LocalPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	tables, err := listTables(db)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("database has no tables")
	}

	out := make([]NamedFrame, 0, len(tables))
	for _, table := range tables {
		frame, err := readTable(db, table)
		if err != nil {
			if opt.IgnoreErrors {
				continue
			}
			return nil, fmt.Errorf("table %q: %w", table, err)
		}
		out = append(out, NamedFrame{Name: table, Frame: frame})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("database has no readable tables")
	}
	return out, nil
}

// checkSQLiteHeader verifies the file starts with the standard magic bytes;
// files that do not are either corrupt, another format, or encrypted.
func checkSQLiteHeader(path string) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	buf := make([]byte, len(sqliteMagic))
	n, _ := fh.Read(buf)
	if n == 0 {
		return nil // empty file: driver treats it as an empty database
	}
	if string(buf[:n]) != sqliteMagic {
		return fmt.Errorf("%s is not a database file (or it is encrypted; encrypted databases are not supported)", path)
	}
	return nil
}

func listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func readTable(db *sql.DB, table string) (*data.Frame, error) {
	quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
	rows, err := db.Query(`SELECT * FROM ` + quoted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	frame := &data.Frame{Columns: make([]data.Column, len(names))}
	for i, name := range names {
		dt := data.TypeString
		if i < len(colTypes) {
			dt = sqliteDeclType(colTypes[i].DatabaseTypeName())
		}
		frame.Columns[i] = data.Column{Name: name, Type: dt}
	}

	holders := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range holders {
		ptrs[i] = &holders[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		cells := make([]any, len(names))
		for i, v := range holders {
			cells[i] = normalizeSQLiteValue(v)
		}
		frame.AppendRow(cells)
	}
	return frame, rows.Err()
}

// sqliteDeclType maps a declared column type to a frame column type using
// the standard type-affinity rules.
func sqliteDeclType(decl string) data.DType {
	d := strings.ToUpper(decl)
	switch {
	case strings.Contains(d, "INT"):
		return data.TypeInt
	case strings.Contains(d, "REAL"), strings.Contains(d, "FLOA"),
		strings.Contains(d, "DOUB"), strings.Contains(d, "NUMERIC"),
		strings.Contains(d, "DECIMAL"):
		return data.TypeFloat
	case strings.Contains(d, "BOOL"):
		return data.TypeBool
	default:
		return data.TypeString
	}
}

// normalizeSQLiteValue maps driver values onto the frame cell vocabulary:
// nil | string | int64 | float64 | bool | time.Time.
func normalizeSQLiteValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case int64:
		return x
	case float64:
		return x
	case string:
		return x
	case bool:
		return x
	case time.Time:
		return x
	case []byte:
		if utf8.Valid(x) {
			return string(x)
		}
		return "0x" + hex.EncodeToString(x)
	default:
		return fmt.Sprint(x)
	}
}
