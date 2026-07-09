// Package data provides the in-memory columnar table model shared by every
// part of the application: file readers, the SQL engine, database backends
// and the UI all speak *data.Frame.
package data

import (
	"fmt"
	"strconv"
	"time"
)

// DType identifies the logical type of a column.
type DType uint8

const (
	TypeString DType = iota
	TypeInt
	TypeFloat
	TypeBool
	TypeDate
	TypeDatetime
)

func (t DType) String() string {
	switch t {
	case TypeInt:
		return "i64"
	case TypeFloat:
		return "f64"
	case TypeBool:
		return "bool"
	case TypeDate:
		return "date"
	case TypeDatetime:
		return "datetime"
	default:
		return "str"
	}
}

// Cell values are one of: nil (null), string, int64, float64, bool, time.Time.
// Readers must normalize to exactly these representations.
type Column struct {
	Name  string
	Type  DType
	Cells []any
}

// Frame is an immutable-by-convention table. Operations that transform a
// frame (filter, sort, query, cast) produce a new Frame.
type Frame struct {
	Columns []Column
}

// New creates an empty frame with the given column names, all string-typed.
func New(names ...string) *Frame {
	cols := make([]Column, len(names))
	for i, n := range names {
		cols[i] = Column{Name: n, Type: TypeString}
	}
	return &Frame{Columns: cols}
}

func (f *Frame) NumCols() int {
	return len(f.Columns)
}

func (f *Frame) NumRows() int {
	if len(f.Columns) == 0 {
		return 0
	}
	return len(f.Columns[0].Cells)
}

func (f *Frame) ColumnNames() []string {
	names := make([]string, len(f.Columns))
	for i, c := range f.Columns {
		names[i] = c.Name
	}
	return names
}

// ColumnIndex returns the index of the named column, or -1.
func (f *Frame) ColumnIndex(name string) int {
	for i, c := range f.Columns {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// Cell returns the raw value at (row, col); nil means null.
func (f *Frame) Cell(row, col int) any {
	return f.Columns[col].Cells[row]
}

// CellString renders the value at (row, col) for display.
func (f *Frame) CellString(row, col int) string {
	return FormatValue(f.Columns[col].Cells[row])
}

// Row returns the raw values of one row.
func (f *Frame) Row(row int) []any {
	vals := make([]any, len(f.Columns))
	for i := range f.Columns {
		vals[i] = f.Columns[i].Cells[row]
	}
	return vals
}

// AppendRow appends one row; vals must have NumCols entries.
func (f *Frame) AppendRow(vals []any) {
	for i := range f.Columns {
		f.Columns[i].Cells = append(f.Columns[i].Cells, vals[i])
	}
}

// Select returns a new frame containing the given rows (in order).
func (f *Frame) Select(rows []int) *Frame {
	out := &Frame{Columns: make([]Column, len(f.Columns))}
	for i, c := range f.Columns {
		cells := make([]any, len(rows))
		for j, r := range rows {
			cells[j] = c.Cells[r]
		}
		out.Columns[i] = Column{Name: c.Name, Type: c.Type, Cells: cells}
	}
	return out
}

// FormatValue renders a cell value for display.
func FormatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 && x.Nanosecond() == 0 {
			return x.Format("2006-01-02")
		}
		return x.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprint(x)
	}
}
