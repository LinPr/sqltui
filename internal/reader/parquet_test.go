package reader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	pq "github.com/parquet-go/parquet-go"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/reader"
)

// TestParquetPhysicalTypeMapping checks reader conversions the app's own
// writer never produces: int32, float32, and millisecond/nanosecond
// timestamps on required (non-optional) columns.
func TestParquetPhysicalTypeMapping(t *testing.T) {
	// Field order is alphabetical (a, b, c, d) because pq.Group sorts by name.
	schema := pq.NewSchema("rec", pq.Group{
		"a": pq.Int(32),
		"b": pq.Leaf(pq.FloatType),
		"c": pq.Timestamp(pq.Millisecond),
		"d": pq.Timestamp(pq.Nanosecond),
	})

	ms := time.Date(2024, 5, 6, 7, 8, 9, 123000000, time.UTC)
	ns := time.Date(2024, 5, 6, 7, 8, 9, 123456789, time.UTC)

	path := filepath.Join(t.TempDir(), "typed.parquet")
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	pw := pq.NewGenericWriter[any](fh, schema)
	row := pq.Row{
		pq.Int32Value(-7).Level(0, 0, 0),
		pq.FloatValue(1.5).Level(0, 0, 1),
		pq.Int64Value(ms.UnixMilli()).Level(0, 0, 2),
		pq.Int64Value(ns.UnixNano()).Level(0, 0, 3),
	}
	if _, err := pw.WriteRows([]pq.Row{row}); err != nil {
		t.Fatal(err)
	}
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}

	src, err := reader.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := reader.For(reader.FormatParquet)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := r.Read(src, reader.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	f := frames[0].Frame

	want := []struct {
		name  string
		dtype data.DType
		cell  any
	}{
		{"a", data.TypeInt, int64(-7)},
		{"b", data.TypeFloat, float64(1.5)},
		{"c", data.TypeDatetime, ms},
		{"d", data.TypeDatetime, ns},
	}
	if f.NumRows() != 1 || f.NumCols() != len(want) {
		t.Fatalf("shape = %dx%d, want 1x%d", f.NumRows(), f.NumCols(), len(want))
	}
	for i, w := range want {
		if f.Columns[i].Name != w.name {
			t.Errorf("column %d name = %q, want %q", i, f.Columns[i].Name, w.name)
		}
		if f.Columns[i].Type != w.dtype {
			t.Errorf("column %q type = %v, want %v", w.name, f.Columns[i].Type, w.dtype)
		}
		if got := f.Columns[i].Cells[0]; !reflect.DeepEqual(got, w.cell) {
			t.Errorf("column %q cell = %#v, want %#v", w.name, got, w.cell)
		}
	}
}
