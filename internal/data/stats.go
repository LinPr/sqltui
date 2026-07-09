package data

import "time"

// NullCount returns how many cells in the column are null (nil).
func NullCount(c *Column) int {
	n := 0
	for _, v := range c.Cells {
		if v == nil {
			n++
		}
	}
	return n
}

// EstimatedSize returns a rough in-memory footprint of the frame in bytes:
// strings count as len+16 (bytes plus header), time.Time values as 24, and
// every other cell (int64, float64, bool, null) as 16.
func EstimatedSize(f *Frame) int64 {
	var total int64
	for i := range f.Columns {
		for _, v := range f.Columns[i].Cells {
			switch x := v.(type) {
			case string:
				total += int64(len(x)) + 16
			case time.Time:
				total += 24
			default:
				total += 16
			}
		}
	}
	return total
}

// Shape returns the frame's dimensions as (rows, cols).
func Shape(f *Frame) (rows, cols int) {
	return f.NumRows(), f.NumCols()
}
