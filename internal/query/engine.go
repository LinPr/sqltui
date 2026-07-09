// Package query implements the embedded SQL engine that file mode runs
// against: loaded frames are registered as tables in an in-memory SQLite
// database and query results come back as new frames. It also provides
// SQL autocompletion for the query editor (see complete.go).
package query

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/LinPr/sqltui/internal/data"
)

// Engine wraps a single in-memory SQLite database. Frames are registered as
// tables (Register), queried with plain SQL (Query) and dropped again
// (Unregister). The zero value is not usable; construct with NewEngine.
type Engine struct {
	db *sql.DB

	mu     sync.Mutex
	tables map[string]struct{}
}

// NewEngine opens a fresh in-memory database.
func NewEngine() (*Engine, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}
	// database/sql pools connections and every new connection to ":memory:"
	// would see its own, empty database. Pin the pool to one connection so
	// all statements share the same in-memory database.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}
	return &Engine{db: db, tables: make(map[string]struct{})}, nil
}

// Close releases the underlying database.
func (e *Engine) Close() error {
	return e.db.Close()
}

// Register (re)creates a table named name holding the contents of f. An
// existing table with the same name is replaced. Any name is accepted,
// including "_" and names containing spaces or punctuation. Duplicate column
// names within the frame are de-duplicated (a, a -> a, a_2) so the table can
// be created.
func (e *Engine) Register(name string, f *data.Frame) error {
	if f == nil {
		return fmt.Errorf("register %q: nil frame", name)
	}
	if f.NumCols() == 0 {
		return fmt.Errorf("register %q: frame has no columns", name)
	}

	colNames := dedupeNames(f.ColumnNames())

	var create strings.Builder
	create.WriteString("CREATE TABLE ")
	create.WriteString(quoteIdent(name))
	create.WriteString(" (")
	for i, c := range f.Columns {
		if i > 0 {
			create.WriteString(", ")
		}
		create.WriteString(quoteIdent(colNames[i]))
		create.WriteByte(' ')
		create.WriteString(affinity(c.Type))
	}
	create.WriteString(")")

	placeholders := strings.Repeat(", ?", f.NumCols())[2:]
	insert := "INSERT INTO " + quoteIdent(name) + " VALUES (" + placeholders + ")"

	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("register %q: %w", name, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec("DROP TABLE IF EXISTS " + quoteIdent(name)); err != nil {
		return fmt.Errorf("register %q: drop: %w", name, err)
	}
	if _, err := tx.Exec(create.String()); err != nil {
		return fmt.Errorf("register %q: create: %w", name, err)
	}

	stmt, err := tx.Prepare(insert)
	if err != nil {
		return fmt.Errorf("register %q: prepare insert: %w", name, err)
	}
	defer stmt.Close()

	args := make([]any, f.NumCols())
	for row := 0; row < f.NumRows(); row++ {
		for col := range f.Columns {
			args[col] = bindValue(f.Columns[col].Cells[row], f.Columns[col].Type)
		}
		if _, err := stmt.Exec(args...); err != nil {
			return fmt.Errorf("register %q: insert row %d: %w", name, row, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("register %q: %w", name, err)
	}

	e.mu.Lock()
	e.tables[name] = struct{}{}
	e.mu.Unlock()
	return nil
}

// Unregister drops the named table. Dropping a table that was never
// registered is not an error.
func (e *Engine) Unregister(name string) error {
	if _, err := e.db.Exec("DROP TABLE IF EXISTS " + quoteIdent(name)); err != nil {
		return fmt.Errorf("unregister %q: %w", name, err)
	}
	e.mu.Lock()
	delete(e.tables, name)
	e.mu.Unlock()
	return nil
}

// Tables returns the names of all registered tables, sorted.
func (e *Engine) Tables() ([]string, error) {
	e.mu.Lock()
	names := make([]string, 0, len(e.tables))
	for n := range e.tables {
		names = append(names, n)
	}
	e.mu.Unlock()
	sort.Strings(names)
	return names, nil
}

// TableColumns returns the column names of every registered table, keyed by
// table name, in table-declaration order (via PRAGMA table_info).
func (e *Engine) TableColumns() (map[string][]string, error) {
	names, err := e.Tables()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(names))
	for _, n := range names {
		cols, err := e.tableInfo(n)
		if err != nil {
			return nil, fmt.Errorf("columns of %q: %w", n, err)
		}
		out[n] = cols
	}
	return out, nil
}

// tableInfo lists the column names of one table.
func (e *Engine) tableInfo(name string) ([]string, error) {
	rows, err := e.db.Query("PRAGMA table_info(" + quoteIdent(name) + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var (
			cid, notnull, pk int
			col, typ         string
			dflt             any
		)
		if err := rows.Scan(&cid, &col, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// Query runs q against the engine and materializes the result as a frame.
// Storage classes map back to the canonical cell set: INTEGER -> int64,
// REAL -> float64, TEXT -> string, BLOB -> string, NULL -> nil; columns
// declared BOOLEAN map their 0/1 values back to bool. Duplicate
// result column names are de-duplicated (a, a -> a, a_2) because frame
// columns are addressed by name.
func (e *Engine) Query(q string) (*data.Frame, error) {
	rows, err := e.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	names := dedupeNames(colNames)
	f := &data.Frame{Columns: make([]data.Column, len(names))}
	// typed[i] is true once column i has a definite dtype — either from the
	// declared column type or, failing that (computed expressions have no
	// declared type, since SQLite types values, not columns), from the first
	// non-nil value scanned.
	typed := make([]bool, len(names))
	for i, n := range names {
		f.Columns[i] = data.Column{Name: n, Type: data.TypeString}
		if i < len(colTypes) {
			if t, ok := dtypeFromDecl(colTypes[i].DatabaseTypeName()); ok {
				f.Columns[i].Type = t
				typed[i] = true
			}
		}
	}

	vals := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i := range vals {
			v := normalizeCell(vals[i])
			if !typed[i] && v != nil {
				f.Columns[i].Type = dtypeOf(v)
				typed[i] = true
			}
			// Columns declared BOOLEAN store 0/1; keep cells consistent
			// with the reported dtype.
			if n, ok := v.(int64); ok && f.Columns[i].Type == data.TypeBool {
				v = n != 0
			}
			f.Columns[i].Cells = append(f.Columns[i].Cells, v)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return f, nil
}

// quoteIdent wraps an identifier in double quotes, doubling any embedded
// quote characters, so arbitrary names (including "_", spaces, punctuation)
// are safe in DDL and DML.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// affinity maps a frame dtype to a SQLite column type. Bool columns are
// declared BOOLEAN (INTEGER affinity in SQLite) so Query's dtypeFromDecl can
// map them back to bool instead of degrading to int64.
func affinity(t data.DType) string {
	switch t {
	case data.TypeInt:
		return "INTEGER"
	case data.TypeBool:
		return "BOOLEAN"
	case data.TypeFloat:
		return "REAL"
	default: // TypeString, TypeDate, TypeDatetime
		return "TEXT"
	}
}

// bindValue converts a canonical cell value into a driver-bindable value.
// nil stays NULL, bools become 0/1 and times are rendered as RFC 3339 text
// (date-only for date columns).
func bindValue(v any, t data.DType) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string, int64, float64:
		return x
	case bool:
		if x {
			return int64(1)
		}
		return int64(0)
	case time.Time:
		if t == data.TypeDate {
			return x.Format("2006-01-02")
		}
		return x.Format(time.RFC3339)
	default:
		return fmt.Sprint(x)
	}
}

// normalizeCell coerces a scanned value into the canonical cell set:
// nil | string | int64 | float64 | bool | time.Time.
func normalizeCell(v any) any {
	switch x := v.(type) {
	case nil, string, int64, float64, bool, time.Time:
		return x
	case []byte:
		return string(x)
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float32:
		return float64(x)
	default:
		return fmt.Sprint(x)
	}
}

// dtypeFromDecl maps a declared SQLite column type (per DatabaseTypeName)
// back to a frame dtype. The empty string — computed expressions have no
// declared type — reports !ok so the caller can infer from values instead.
func dtypeFromDecl(decl string) (data.DType, bool) {
	d := strings.ToUpper(decl)
	switch {
	case d == "":
		return data.TypeString, false
	case strings.Contains(d, "BOOL"):
		return data.TypeBool, true
	case strings.Contains(d, "INT"):
		return data.TypeInt, true
	case strings.Contains(d, "REAL"), strings.Contains(d, "FLOA"), strings.Contains(d, "DOUB"), strings.Contains(d, "NUMERIC"), strings.Contains(d, "DECIMAL"):
		return data.TypeFloat, true
	case strings.Contains(d, "CHAR"), strings.Contains(d, "TEXT"), strings.Contains(d, "CLOB"), strings.Contains(d, "BLOB"):
		return data.TypeString, true
	default:
		return data.TypeString, false
	}
}

// dtypeOf reports the frame dtype of a canonical cell value.
func dtypeOf(v any) data.DType {
	switch v.(type) {
	case int64:
		return data.TypeInt
	case float64:
		return data.TypeFloat
	case bool:
		return data.TypeBool
	case time.Time:
		return data.TypeDatetime
	default:
		return data.TypeString
	}
}

// dedupeNames makes every name unique by suffixing repeats with _2, _3, ...
// while never colliding with a name that already exists in the list.
func dedupeNames(in []string) []string {
	out := make([]string, len(in))
	seen := make(map[string]bool, len(in))
	for i, n := range in {
		name := n
		for k := 2; seen[name]; k++ {
			name = fmt.Sprintf("%s_%d", n, k)
		}
		seen[name] = true
		out[i] = name
	}
	return out
}
