package data

import (
	"reflect"
	"testing"
)

func TestWithCell(t *testing.T) {
	f := frameOf(
		Column{Name: "id", Type: TypeInt, Cells: []any{int64(1), int64(2), int64(3)}},
		Column{Name: "name", Type: TypeString, Cells: []any{"ann", "bob", "cyd"}},
	)

	got := f.WithCell(1, 1, "BOB")

	// The target cell is replaced.
	if got.CellString(1, 1) != "BOB" {
		t.Errorf("target cell = %q, want BOB", got.CellString(1, 1))
	}
	// Siblings in the same row are untouched.
	if got.CellString(1, 0) != "2" {
		t.Errorf("sibling cell (1,0) = %q, want 2", got.CellString(1, 0))
	}
	// Cells in the same column but other rows are untouched.
	if got.CellString(0, 1) != "ann" || got.CellString(2, 1) != "cyd" {
		t.Errorf("same-column other rows changed: 0=%q 2=%q", got.CellString(0, 1), got.CellString(2, 1))
	}
	// Column type is preserved.
	if got.Columns[1].Type != TypeString {
		t.Errorf("col type = %v, want TypeString", got.Columns[1].Type)
	}

	// The original frame is left unchanged.
	if f.CellString(1, 1) != "bob" {
		t.Errorf("original frame mutated: cell(1,1) = %q, want bob", f.CellString(1, 1))
	}
	if !reflect.DeepEqual(f.Columns[1].Cells, []any{"ann", "bob", "cyd"}) {
		t.Errorf("original column cells mutated: %#v", f.Columns[1].Cells)
	}

	// WithCell on a typed column keeps the column type even when the new
	// value's Go type differs.
	got2 := f.WithCell(0, 0, "9")
	if got2.Columns[0].Type != TypeInt {
		t.Errorf("typed column type changed: %v", got2.Columns[0].Type)
	}
	if got2.Cell(0, 0) != "9" {
		t.Errorf("typed col cell = %v, want 9", got2.Cell(0, 0))
	}
}

func TestWithoutRows(t *testing.T) {
	f := frameOf(
		Column{Name: "id", Type: TypeInt, Cells: []any{int64(1), int64(2), int64(3), int64(4)}},
		Column{Name: "name", Type: TypeString, Cells: []any{"ann", "bob", "cyd", "dan"}},
	)

	// Removing a middle row drops only it.
	got := f.WithoutRows([]int{1})
	if got.NumRows() != 3 {
		t.Fatalf("middle drop NumRows = %d, want 3", got.NumRows())
	}
	if got.CellString(0, 0) != "1" || got.CellString(0, 1) != "ann" {
		t.Errorf("middle drop row 0 = (%q,%q), want (1,ann)", got.CellString(0, 0), got.CellString(0, 1))
	}
	if got.CellString(1, 0) != "3" || got.CellString(1, 1) != "cyd" {
		t.Errorf("middle drop row 1 = (%q,%q), want (3,cyd)", got.CellString(1, 0), got.CellString(1, 1))
	}
	if got.CellString(2, 0) != "4" || got.CellString(2, 1) != "dan" {
		t.Errorf("middle drop row 2 = (%q,%q), want (4,dan)", got.CellString(2, 0), got.CellString(2, 1))
	}

	// Removing multiple drops exactly those rows, order preserved.
	got = f.WithoutRows([]int{0, 2})
	if got.NumRows() != 2 {
		t.Fatalf("multi drop NumRows = %d, want 2", got.NumRows())
	}
	if got.CellString(0, 0) != "2" || got.CellString(0, 1) != "bob" {
		t.Errorf("multi drop row 0 = (%q,%q), want (2,bob)", got.CellString(0, 0), got.CellString(0, 1))
	}
	if got.CellString(1, 0) != "4" || got.CellString(1, 1) != "dan" {
		t.Errorf("multi drop row 1 = (%q,%q), want (4,dan)", got.CellString(1, 0), got.CellString(1, 1))
	}

	// Out-of-range indices are ignored (no panic, no effect).
	got = f.WithoutRows([]int{-1, 4, 100})
	if got.NumRows() != 4 {
		t.Errorf("out-of-range NumRows = %d, want 4", got.NumRows())
	}

	// Duplicate indices collapse to a single drop.
	got = f.WithoutRows([]int{1, 1})
	if got.NumRows() != 3 {
		t.Errorf("duplicate drop NumRows = %d, want 3", got.NumRows())
	}

	// Original frame is left unchanged.
	if f.NumRows() != 4 {
		t.Errorf("original NumRows = %d, want 4", f.NumRows())
	}
	if !reflect.DeepEqual(f.Columns[0].Cells, []any{int64(1), int64(2), int64(3), int64(4)}) {
		t.Errorf("original col 0 mutated: %#v", f.Columns[0].Cells)
	}
	if !reflect.DeepEqual(f.Columns[1].Cells, []any{"ann", "bob", "cyd", "dan"}) {
		t.Errorf("original col 1 mutated: %#v", f.Columns[1].Cells)
	}

	// Column types are preserved.
	for i, want := range []DType{TypeInt, TypeString} {
		if got.Columns[i].Type != want {
			t.Errorf("col %d type = %v, want %v", i, got.Columns[i].Type, want)
		}
	}

	// Removing the only row of a 1-row frame yields 0 rows.
	one := frameOf(
		Column{Name: "id", Type: TypeInt, Cells: []any{int64(7)}},
		Column{Name: "name", Type: TypeString, Cells: []any{"solo"}},
	)
	got = one.WithoutRows([]int{0})
	if got.NumRows() != 0 {
		t.Errorf("1-row drop NumRows = %d, want 0", got.NumRows())
	}
	if got.NumCols() != 2 {
		t.Errorf("1-row drop NumCols = %d, want 2", got.NumCols())
	}
	if got.Columns[0].Type != TypeInt || got.Columns[1].Type != TypeString {
		t.Errorf("1-row drop types changed: %v %v", got.Columns[0].Type, got.Columns[1].Type)
	}

	// An empty input slice is a copy.
	got = f.WithoutRows(nil)
	if got.NumRows() != 4 || got.NumCols() != 2 {
		t.Errorf("nil drop = %d rows %d cols, want 4x2", got.NumRows(), got.NumCols())
	}

	// A frame with no columns returns an empty-frame copy.
	empty := &Frame{Columns: nil}
	got = empty.WithoutRows([]int{0})
	if got.NumCols() != 0 || got.NumRows() != 0 {
		t.Errorf("no-col frame = %d cols %d rows, want 0x0", got.NumCols(), got.NumRows())
	}
}
