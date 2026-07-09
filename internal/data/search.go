package data

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// SearchExact returns the indices of rows where at least one cell's display
// string contains needle (case-sensitive). An empty needle matches every row.
func SearchExact(f *Frame, needle string) []int {
	rows := f.NumRows()
	out := make([]int, 0, rows)
	for r := 0; r < rows; r++ {
		for c := range f.Columns {
			if strings.Contains(f.CellString(r, c), needle) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// SearchFuzzy returns the indices of rows whose joined display string fuzzily
// matches needle, in frame order. An empty needle matches every row.
func SearchFuzzy(f *Frame, needle string) []int {
	rows := f.NumRows()
	if needle == "" {
		all := make([]int, rows)
		for i := range all {
			all[i] = i
		}
		return all
	}
	haystack := make([]string, rows)
	parts := make([]string, len(f.Columns))
	for r := 0; r < rows; r++ {
		for c := range f.Columns {
			parts[c] = f.CellString(r, c)
		}
		haystack[r] = strings.Join(parts, " ")
	}
	matches := fuzzy.Find(needle, haystack)
	out := make([]int, len(matches))
	for i, m := range matches {
		out[i] = m.Index
	}
	sort.Ints(out) // fuzzy.Find sorts by score; restore frame order
	return out
}
