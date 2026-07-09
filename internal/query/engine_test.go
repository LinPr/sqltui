package query

import (
	"reflect"
	"testing"
	"time"

	"github.com/LinPr/sqltui/internal/data"
)

// fixtureFrame builds a frame exercising all six dtypes, nulls in every
// column, and a column name containing an embedded double quote.
func fixtureFrame() *data.Frame {
	d1 := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	dt1 := time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC)
	return &data.Frame{Columns: []data.Column{
		{Name: "s", Type: data.TypeString, Cells: []any{"hello", nil, "wörld"}},
		{Name: "i", Type: data.TypeInt, Cells: []any{int64(1), nil, int64(-42)}},
		{Name: "f", Type: data.TypeFloat, Cells: []any{1.5, nil, -0.25}},
		{Name: "b", Type: data.TypeBool, Cells: []any{true, nil, false}},
		{Name: "d", Type: data.TypeDate, Cells: []any{d1, nil, d1}},
		{Name: "dt", Type: data.TypeDatetime, Cells: []any{dt1, nil, dt1}},
		{Name: `weird "name`, Type: data.TypeString, Cells: []any{"x", nil, `y"z`}},
	}}
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

func TestRegisterRoundtrip(t *testing.T) {
	e := newTestEngine(t)
	if err := e.Register("_", fixtureFrame()); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := e.Query(`SELECT * FROM "_"`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	wantNames := []string{"s", "i", "f", "b", "d", "dt", `weird "name`}
	if !reflect.DeepEqual(got.ColumnNames(), wantNames) {
		t.Fatalf("column names = %v, want %v", got.ColumnNames(), wantNames)
	}
	if got.NumRows() != 3 {
		t.Fatalf("NumRows = %d, want 3", got.NumRows())
	}

	// Bools survive the round trip (BOOLEAN-declared column), dates and
	// datetimes come back as RFC text.
	wantTypes := []data.DType{
		data.TypeString, data.TypeInt, data.TypeFloat, data.TypeBool,
		data.TypeString, data.TypeString, data.TypeString,
	}
	wantCells := [][]any{
		{"hello", nil, "wörld"},
		{int64(1), nil, int64(-42)},
		{1.5, nil, -0.25},
		{true, nil, false},
		{"2024-03-15", nil, "2024-03-15"},
		{"2024-03-15T10:30:45Z", nil, "2024-03-15T10:30:45Z"},
		{"x", nil, `y"z`},
	}
	for i, c := range got.Columns {
		if c.Type != wantTypes[i] {
			t.Errorf("column %q type = %v, want %v", c.Name, c.Type, wantTypes[i])
		}
		if !reflect.DeepEqual(c.Cells, wantCells[i]) {
			t.Errorf("column %q cells = %#v, want %#v", c.Name, c.Cells, wantCells[i])
		}
	}
}

func TestRegisterReplacesAndWeirdTableNames(t *testing.T) {
	e := newTestEngine(t)
	name := `my "table", yes!`

	f1 := &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(1), int64(2)}},
	}}
	if err := e.Register(name, f1); err != nil {
		t.Fatalf("Register #1: %v", err)
	}

	// Re-registering must replace, not append.
	f2 := &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(9)}},
	}}
	if err := e.Register(name, f2); err != nil {
		t.Fatalf("Register #2: %v", err)
	}

	got, err := e.Query(`SELECT a FROM "my ""table"", yes!"`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.NumRows() != 1 || got.Cell(0, 0) != int64(9) {
		t.Fatalf("got %d rows, cell(0,0)=%v; want 1 row with 9", got.NumRows(), got.Cell(0, 0))
	}
}

func TestQueryDuplicateColumnNames(t *testing.T) {
	e := newTestEngine(t)
	f := &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(7)}},
	}}
	if err := e.Register("t", f); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := e.Query(`SELECT a, a, a FROM "t"`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"a", "a_2", "a_3"}
	if !reflect.DeepEqual(got.ColumnNames(), want) {
		t.Fatalf("column names = %v, want %v", got.ColumnNames(), want)
	}
	for i := range want {
		if got.Cell(0, i) != int64(7) {
			t.Errorf("cell(0,%d) = %v, want 7", i, got.Cell(0, i))
		}
	}
}

func TestQueryExpressionTypes(t *testing.T) {
	e := newTestEngine(t)
	f := &data.Frame{Columns: []data.Column{
		{Name: "n", Type: data.TypeInt, Cells: []any{int64(1), int64(2), int64(3)}},
	}}
	if err := e.Register("t", f); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := e.Query(`SELECT COUNT(*) AS c, AVG(n) AS m, 'hi' AS s, NULL AS z FROM "t"`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	tests := []struct {
		col  int
		typ  data.DType
		cell any
	}{
		{0, data.TypeInt, int64(3)},
		{1, data.TypeFloat, 2.0},
		{2, data.TypeString, "hi"},
		{3, data.TypeString, nil}, // all-NULL column defaults to string
	}
	for _, tc := range tests {
		if got.Columns[tc.col].Type != tc.typ {
			t.Errorf("col %d type = %v, want %v", tc.col, got.Columns[tc.col].Type, tc.typ)
		}
		if got.Cell(0, tc.col) != tc.cell {
			t.Errorf("cell(0,%d) = %v, want %v", tc.col, got.Cell(0, tc.col), tc.cell)
		}
	}
}

func TestQueryWhereOnRegisteredValues(t *testing.T) {
	e := newTestEngine(t)
	if err := e.Register("_", fixtureFrame()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := e.Query(`SELECT s FROM "_" WHERE b = 1 AND i = 1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.NumRows() != 1 || got.Cell(0, 0) != "hello" {
		t.Fatalf("got %d rows, cell=%v; want 1 row \"hello\"", got.NumRows(), got.Cell(0, 0))
	}
}

func TestTablesAndUnregister(t *testing.T) {
	e := newTestEngine(t)
	f := &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeString, Cells: nil},
	}}
	for _, n := range []string{"zebra", "_", "with space"} {
		if err := e.Register(n, f); err != nil {
			t.Fatalf("Register %q: %v", n, err)
		}
	}
	got, err := e.Tables()
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	want := []string{"_", "with space", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tables = %v, want %v", got, want)
	}

	if err := e.Unregister("with space"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	got, _ = e.Tables()
	want = []string{"_", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tables after Unregister = %v, want %v", got, want)
	}
	if _, err := e.Query(`SELECT * FROM "with space"`); err == nil {
		t.Fatal("query against dropped table succeeded, want error")
	}
	// Unregistering an unknown table is not an error.
	if err := e.Unregister("nope"); err != nil {
		t.Fatalf("Unregister unknown: %v", err)
	}
}

func TestRegisterErrors(t *testing.T) {
	e := newTestEngine(t)
	if err := e.Register("t", nil); err == nil {
		t.Error("Register(nil) succeeded, want error")
	}
	if err := e.Register("t", &data.Frame{}); err == nil {
		t.Error("Register(zero columns) succeeded, want error")
	}
}

func TestRegisterDuplicateFrameColumns(t *testing.T) {
	e := newTestEngine(t)
	f := &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(1)}},
		{Name: "a", Type: data.TypeString, Cells: []any{"x"}},
	}}
	if err := e.Register("t", f); err != nil {
		t.Fatalf("Register with duplicate columns: %v", err)
	}
	got, err := e.Query(`SELECT "a_2", "a" FROM "t"`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Cell(0, 0) != "x" || got.Cell(0, 1) != int64(1) {
		t.Fatalf("cells = %v, %v; want x, 1", got.Cell(0, 0), got.Cell(0, 1))
	}
}

func TestQueryBooleanDecltype(t *testing.T) {
	e := newTestEngine(t)
	// A table declared with BOOLEAN (created outside Register) reads back
	// as real bools.
	if _, err := e.db.Exec(`CREATE TABLE flags (ok BOOLEAN); INSERT INTO flags VALUES (1), (0), (NULL)`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := e.Query(`SELECT ok FROM flags`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Columns[0].Type != data.TypeBool {
		t.Errorf("type = %v, want bool", got.Columns[0].Type)
	}
	want := []any{true, false, nil}
	if !reflect.DeepEqual(got.Columns[0].Cells, want) {
		t.Errorf("cells = %#v, want %#v", got.Columns[0].Cells, want)
	}
}

func TestQueryBadSQL(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.Query("SELEC nonsense"); err == nil {
		t.Fatal("bad SQL succeeded, want error")
	}
}

func TestDedupeNames(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"no dupes", []string{"a", "b"}, []string{"a", "b"}},
		{"simple", []string{"a", "a", "a"}, []string{"a", "a_2", "a_3"}},
		{"collision with existing suffix", []string{"a", "a", "a_2"}, []string{"a", "a_2", "a_2_2"}},
		{"empty names", []string{"", ""}, []string{"", "_2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dedupeNames(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dedupeNames(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a", `"a"`},
		{"_", `"_"`},
		{`we"ird`, `"we""ird"`},
		{"with space!", `"with space!"`},
	}
	for _, tc := range tests {
		if got := quoteIdent(tc.in); got != tc.want {
			t.Errorf("quoteIdent(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
