package data

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// strCol builds a string-typed column from literal cell values; "␀" markers
// are not used — pass nil directly via anySlice.
func strCol(name string, cells ...any) Column {
	return Column{Name: name, Type: TypeString, Cells: cells}
}

func frameOf(cols ...Column) *Frame { return &Frame{Columns: cols} }

func TestInferFrameSafe(t *testing.T) {
	tests := []struct {
		name      string
		col       Column
		types     []string
		wantType  DType
		wantCells []any
	}{
		{
			name:      "ints with signs and empties",
			col:       strCol("a", "1", "+2", "-3", "", nil),
			wantType:  TypeInt,
			wantCells: []any{int64(1), int64(2), int64(-3), nil, nil},
		},
		{
			name:      "floats win over ints when mixed",
			col:       strCol("a", "1", "2.5"),
			wantType:  TypeFloat,
			wantCells: []any{float64(1), 2.5},
		},
		{
			name:      "one bad cell blocks safe conversion",
			col:       strCol("a", "1", "2", "x"),
			wantType:  TypeString,
			wantCells: []any{"1", "2", "x"},
		},
		{
			name:      "booleans case-insensitive",
			col:       strCol("a", "true", "FALSE", "True"),
			types:     []string{"boolean"},
			wantType:  TypeBool,
			wantCells: []any{true, false, true},
		},
		{
			name:      "boolean not enabled by default",
			col:       strCol("a", "true", "false"),
			wantType:  TypeString,
			wantCells: []any{"true", "false"},
		},
		{
			name:      "dates in both layouts",
			col:       strCol("a", "1990-01-02", "1991/03/04"),
			types:     []string{"all"},
			wantType:  TypeDate,
			wantCells: []any{date(1990, 1, 2), date(1991, 3, 4)},
		},
		{
			name:     "datetimes rfc3339 and space layout with fraction",
			col:      strCol("a", "2024-05-06T07:08:09Z", "2024-05-06 07:08:09.5"),
			types:    []string{"datetime"},
			wantType: TypeDatetime,
			wantCells: []any{
				time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
				time.Date(2024, 5, 6, 7, 8, 9, 500000000, time.UTC),
			},
		},
		{
			name:      "all-empty column stays string",
			col:       strCol("a", "", nil, ""),
			wantType:  TypeString,
			wantCells: []any{"", nil, ""},
		},
		{
			name:      "whitespace trimmed before parse",
			col:       strCol("a", " 7 ", "8"),
			wantType:  TypeInt,
			wantCells: []any{int64(7), int64(8)},
		},
		{
			name:      "unknown type names ignored",
			col:       strCol("a", "1", "2"),
			types:     []string{"bogus"},
			wantType:  TypeString,
			wantCells: []any{"1", "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := frameOf(tt.col)
			out := InferFrame(in, "safe", tt.types)
			c := out.Columns[0]
			if c.Type != tt.wantType {
				t.Fatalf("type = %v, want %v", c.Type, tt.wantType)
			}
			if len(c.Cells) != len(tt.wantCells) {
				t.Fatalf("len = %d, want %d", len(c.Cells), len(tt.wantCells))
			}
			for i := range c.Cells {
				if !cellEqual(c.Cells[i], tt.wantCells[i]) {
					t.Errorf("cell[%d] = %#v, want %#v", i, c.Cells[i], tt.wantCells[i])
				}
			}
			// input must be untouched
			if in.Columns[0].Type != TypeString {
				t.Error("input frame was mutated")
			}
		})
	}
}

func TestInferFrameFastNullOnFailure(t *testing.T) {
	// First 128 rows are ints; row 128 (outside the sample) is garbage and
	// must become null rather than blocking the conversion.
	cells := make([]any, 0, 130)
	for i := 0; i < inferSampleRows; i++ {
		cells = append(cells, strconv.Itoa(i))
	}
	cells = append(cells, "not-a-number", "42")
	out := InferFrame(frameOf(strCol("n", cells...)), "fast", nil)
	c := out.Columns[0]
	if c.Type != TypeInt {
		t.Fatalf("type = %v, want TypeInt", c.Type)
	}
	if c.Cells[inferSampleRows] != nil {
		t.Errorf("unparsable cell = %#v, want nil", c.Cells[inferSampleRows])
	}
	if c.Cells[inferSampleRows+1] != int64(42) {
		t.Errorf("last cell = %#v, want 42", c.Cells[inferSampleRows+1])
	}
}

func TestInferFrameSafeVsFastDisagree(t *testing.T) {
	// Garbage beyond the sample: fast converts (null-on-failure), safe refuses.
	cells := make([]any, 0, 130)
	for i := 0; i < inferSampleRows; i++ {
		cells = append(cells, "1")
	}
	cells = append(cells, "oops")

	fast := InferFrame(frameOf(strCol("n", cells...)), "fast", nil)
	if fast.Columns[0].Type != TypeInt {
		t.Errorf("fast type = %v, want TypeInt", fast.Columns[0].Type)
	}
	safe := InferFrame(frameOf(strCol("n", cells...)), "safe", nil)
	if safe.Columns[0].Type != TypeString {
		t.Errorf("safe type = %v, want TypeString", safe.Columns[0].Type)
	}
}

func TestInferFrameNoMode(t *testing.T) {
	f := frameOf(strCol("a", "1", "2"))
	if out := InferFrame(f, "no", nil); out != f {
		t.Error("mode no should return the frame unchanged")
	}
}

func TestInferFrameSkipsTypedColumns(t *testing.T) {
	col := Column{Name: "n", Type: TypeInt, Cells: []any{int64(1)}}
	out := InferFrame(frameOf(col), "safe", []string{"all"})
	if out.Columns[0].Type != TypeInt {
		t.Errorf("typed column changed to %v", out.Columns[0].Type)
	}
}

func TestInferFrameFixture(t *testing.T) {
	f := loadCSVFixture(t, filepath.Join("testdata", "mixed.csv"))
	out := InferFrame(f, "safe", []string{"all"})
	want := map[string]DType{
		"id":     TypeInt,
		"price":  TypeFloat,
		"active": TypeBool,
		"born":   TypeDate,
		"seen":   TypeDatetime,
		"note":   TypeString,
	}
	for name, wt := range want {
		i := out.ColumnIndex(name)
		if i < 0 {
			t.Fatalf("column %q missing", name)
		}
		if got := out.Columns[i].Type; got != wt {
			t.Errorf("column %q type = %v, want %v", name, got, wt)
		}
	}
	// the all-empty row must be null everywhere
	for c := range out.Columns {
		if out.Cell(2, c) != nil && out.Columns[c].Type != TypeString {
			t.Errorf("row 2 col %d = %#v, want nil", c, out.Cell(2, c))
		}
	}
}

func loadCSVFixture(t *testing.T, path string) *Frame {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	records, err := csv.NewReader(fh).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	f := New(records[0]...)
	for _, rec := range records[1:] {
		row := make([]any, len(rec))
		for i, s := range rec {
			row[i] = s
		}
		f.AppendRow(row)
	}
	return f
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func cellEqual(a, b any) bool {
	if ta, ok := a.(time.Time); ok {
		tb, ok := b.(time.Time)
		return ok && ta.Equal(tb)
	}
	return a == b
}
