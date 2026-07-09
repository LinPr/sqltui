package data

import (
	"errors"
	"fmt"
)

// Concat vertically concatenates frames. The result's columns are the union
// of the input column names in first-seen order; rows from frames lacking a
// column get null cells there. When the same column name carries different
// types across frames, the unified column becomes TypeString and every
// non-null value is rendered with FormatValue.
func Concat(frames ...*Frame) (*Frame, error) {
	if len(frames) == 0 {
		return nil, errors.New("concat: no frames given")
	}
	for i, f := range frames {
		if f == nil {
			return nil, fmt.Errorf("concat: frame %d is nil", i)
		}
	}

	// Union of column names in first-seen order, with unified types.
	var order []string
	unified := map[string]DType{}
	for _, f := range frames {
		for _, c := range f.Columns {
			t, seen := unified[c.Name]
			if !seen {
				order = append(order, c.Name)
				unified[c.Name] = c.Type
			} else if t != c.Type {
				unified[c.Name] = TypeString
			}
		}
	}

	totalRows := 0
	for _, f := range frames {
		totalRows += f.NumRows()
	}

	out := &Frame{Columns: make([]Column, len(order))}
	for i, name := range order {
		col := Column{Name: name, Type: unified[name], Cells: make([]any, 0, totalRows)}
		for _, f := range frames {
			n := f.NumRows()
			idx := f.ColumnIndex(name)
			if idx < 0 {
				col.Cells = append(col.Cells, make([]any, n)...) // nulls
				continue
			}
			src := &f.Columns[idx]
			if col.Type == TypeString && src.Type != TypeString {
				for _, v := range src.Cells {
					if v == nil {
						col.Cells = append(col.Cells, nil)
					} else {
						col.Cells = append(col.Cells, FormatValue(v))
					}
				}
			} else {
				col.Cells = append(col.Cells, src.Cells...)
			}
		}
		out.Columns[i] = col
	}
	return out, nil
}
