package reader

import (
	"reflect"
	"testing"
)

func TestJSONArray(t *testing.T) {
	nf := readOne(t, "array.json", noInferOpts(FormatJSON))
	f := nf.Frame
	wantCols := []string{"id", "name", "ok", "score", "note", "tags", "meta"}
	if got := f.ColumnNames(); !reflect.DeepEqual(got, wantCols) {
		t.Fatalf("columns = %v, want %v", got, wantCols)
	}
	wantRows := [][]any{
		{int64(1), "alice", true, 3.5, nil, nil, nil},
		{int64(2), "bob", nil, int64(4), nil, `["x","y"]`, `{"a":1}`},
	}
	if f.NumRows() != len(wantRows) {
		t.Fatalf("rows = %d, want %d", f.NumRows(), len(wantRows))
	}
	for i, want := range wantRows {
		if got := f.Row(i); !reflect.DeepEqual(got, want) {
			t.Errorf("row %d = %#v, want %#v", i, got, want)
		}
	}
}

func TestJSONSingleObject(t *testing.T) {
	nf := readOne(t, "single.json", noInferOpts(FormatJSON))
	checkGrid(t, nf.Frame,
		[]string{"a", "b", "c"},
		[][]any{{int64(1), "x", false}})
}

func TestJSONCellValue(t *testing.T) {
	tests := []struct {
		raw  string
		want any
	}{
		{`null`, nil},
		{`true`, true},
		{`false`, false},
		{`"hi"`, "hi"},
		{`42`, int64(42)},
		{`-7`, int64(-7)},
		{`3.0`, int64(3)}, // integral float becomes int64
		{`3.25`, 3.25},
		{`1e300`, 1e300},
		{`9223372036854775807`, int64(9223372036854775807)},
		// values at or just above 2^63 must not sign-flip into int64
		{`9223372036854775808`, 9223372036854775808.0},
		{`9223372036854776000`, 9223372036854776000.0},
		{`-9223372036854775808`, int64(-9223372036854775808)},
		{`[1, 2]`, `[1,2]`},
		{`{"a": [1] }`, `{"a":[1]}`},
	}
	for _, tt := range tests {
		got, err := jsonCellValue([]byte(tt.raw))
		if err != nil {
			t.Errorf("jsonCellValue(%s): %v", tt.raw, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("jsonCellValue(%s) = %#v, want %#v", tt.raw, got, tt.want)
		}
	}
}
