package reader_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/reader"
)

// writeWorkbook creates an xlsx fixture. Sheet1 has a header, typed values
// and ragged rows; when multi is true a second data sheet and an empty sheet
// are added.
func writeWorkbook(t *testing.T, path string, multi bool) {
	t.Helper()
	wb := excelize.NewFile()
	defer wb.Close()

	cells := map[string]any{
		"A1": "name", "B1": "age", "C1": "score",
		"A2": "alice", "B2": 30, "C2": 1.5,
		"A3": "bob", "B3": 7, // C3 missing: ragged
		"A4": "carol", // B4/C4 missing: ragged
	}
	for ref, v := range cells {
		if err := wb.SetCellValue("Sheet1", ref, v); err != nil {
			t.Fatal(err)
		}
	}
	if multi {
		if _, err := wb.NewSheet("Extra"); err != nil {
			t.Fatal(err)
		}
		if err := wb.SetCellValue("Extra", "A1", "k"); err != nil {
			t.Fatal(err)
		}
		if err := wb.SetCellValue("Extra", "A2", "v"); err != nil {
			t.Fatal(err)
		}
		if _, err := wb.NewSheet("Empty"); err != nil {
			t.Fatal(err)
		}
	}
	if err := wb.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func readExcel(t *testing.T, path string, opt reader.Options) []reader.NamedFrame {
	t.Helper()
	src, err := reader.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := reader.For(reader.FormatExcel)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := r.Read(src, opt)
	if err != nil {
		t.Fatal(err)
	}
	return frames
}

func TestExcelSingleSheet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "people.xlsx")
	writeWorkbook(t, path, false)

	frames := readExcel(t, path, reader.DefaultOptions())
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	// Single data sheet: named after the file, not the sheet.
	if frames[0].Name != "people" {
		t.Errorf("frame name = %q, want %q", frames[0].Name, "people")
	}
	f := frames[0].Frame
	if want := []string{"name", "age", "score"}; !reflect.DeepEqual(f.ColumnNames(), want) {
		t.Fatalf("columns = %v, want %v", f.ColumnNames(), want)
	}

	wantCols := []struct {
		dtype data.DType
		cells []any
	}{
		{data.TypeString, []any{"alice", "bob", "carol"}},
		{data.TypeInt, []any{int64(30), int64(7), nil}}, // ragged rows padded with nil
		{data.TypeFloat, []any{float64(1.5), nil, nil}}, // inferred float
	}
	for i, want := range wantCols {
		if f.Columns[i].Type != want.dtype {
			t.Errorf("column %d type = %v, want %v", i, f.Columns[i].Type, want.dtype)
		}
		if !reflect.DeepEqual(f.Columns[i].Cells, want.cells) {
			t.Errorf("column %d cells = %#v, want %#v", i, f.Columns[i].Cells, want.cells)
		}
	}
}

func TestExcelMultiSheet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	writeWorkbook(t, path, true)

	frames := readExcel(t, path, reader.DefaultOptions())
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2 (empty sheet must be skipped)", len(frames))
	}
	// Multiple data sheets keep their sheet names.
	if frames[0].Name != "Sheet1" || frames[1].Name != "Extra" {
		t.Errorf("frame names = %q, %q, want Sheet1, Extra", frames[0].Name, frames[1].Name)
	}
	extra := frames[1].Frame
	if want := []string{"k"}; !reflect.DeepEqual(extra.ColumnNames(), want) {
		t.Errorf("Extra columns = %v, want %v", extra.ColumnNames(), want)
	}
	if want := []any{"v"}; !reflect.DeepEqual(extra.Columns[0].Cells, want) {
		t.Errorf("Extra cells = %#v, want %#v", extra.Columns[0].Cells, want)
	}
}

func TestExcelNoHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "people.xlsx")
	writeWorkbook(t, path, false)

	opt := reader.DefaultOptions()
	opt.NoHeader = true
	frames := readExcel(t, path, opt)
	f := frames[0].Frame
	if want := []string{"column_1", "column_2", "column_3"}; !reflect.DeepEqual(f.ColumnNames(), want) {
		t.Fatalf("columns = %v, want %v", f.ColumnNames(), want)
	}
	if f.NumRows() != 4 {
		t.Fatalf("rows = %d, want 4 (header row is data)", f.NumRows())
	}
	// Mixed header+numeric column cannot be inferred as int: stays string.
	if f.Columns[1].Type != data.TypeString {
		t.Errorf("column 2 type = %v, want string", f.Columns[1].Type)
	}
	if got := f.Columns[1].Cells[0]; got != "age" {
		t.Errorf("first cell of column 2 = %#v, want \"age\"", got)
	}
}
