package data

import (
	"strconv"
	"strings"
	"time"
)

// inferSampleRows is how many leading rows the "fast" mode inspects when
// deciding a column's type.
const inferSampleRows = 128

// inferCandidates is the priority order in which types are tried. The first
// enabled candidate that matches every inspected cell wins, so integer-looking
// columns become ints (not floats), and "true"/"false" columns become bools.
var inferCandidates = []DType{TypeInt, TypeFloat, TypeBool, TypeDate, TypeDatetime}

// typeNameToDType maps the option-level type names to DTypes.
var typeNameToDType = map[string]DType{
	"int":      TypeInt,
	"float":    TypeFloat,
	"boolean":  TypeBool,
	"date":     TypeDate,
	"datetime": TypeDatetime,
}

// InferFrame converts string-typed columns produced by text readers to
// concrete types. It never mutates f; a new frame is returned (unconverted
// columns share their cell slices with f).
//
// mode is one of:
//
//	"no"    return f unchanged.
//	"fast"  decide each column's type by sampling its first 128 rows, then
//	        convert every row. Values outside the sample that fail to parse
//	        become null: converting with null-on-failure keeps the column
//	        homogeneous, at the cost of dropping the odd unparsable cell.
//	"safe"  scan all rows; a column is converted only when every non-empty
//	        cell parses as the candidate type.
//
// types selects which candidate types are considered: any subset of
// "int", "float", "boolean", "date", "datetime", or ["all"] for all five.
// An empty list defaults to int+float.
//
// Empty-string cells (after trimming ASCII space) count as null candidates:
// they never veto a conversion and become nil in converted columns. Columns
// containing non-string, non-nil cells are left untouched.
func InferFrame(f *Frame, mode string, types []string) *Frame {
	if f == nil || mode == "no" || mode == "" || f.NumRows() == 0 {
		return f
	}
	enabled := enabledTypes(types)
	if len(enabled) == 0 {
		return f
	}

	out := &Frame{Columns: make([]Column, len(f.Columns))}
	copy(out.Columns, f.Columns)
	for i := range out.Columns {
		c := &out.Columns[i]
		if c.Type != TypeString {
			continue
		}
		sample := c.Cells
		if mode == "fast" && len(sample) > inferSampleRows {
			sample = sample[:inferSampleRows]
		}
		t, ok := detectColumnType(sample, enabled)
		if !ok {
			continue
		}
		c.Type = t
		c.Cells = convertCells(c.Cells, t)
	}
	return out
}

// enabledTypes resolves the option-level type list to a DType set in
// candidate priority order.
func enabledTypes(types []string) []DType {
	if len(types) == 0 {
		return []DType{TypeInt, TypeFloat}
	}
	set := map[DType]bool{}
	for _, name := range types {
		if strings.EqualFold(name, "all") {
			return inferCandidates
		}
		if t, ok := typeNameToDType[strings.ToLower(name)]; ok {
			set[t] = true
		}
	}
	out := make([]DType, 0, len(set))
	for _, t := range inferCandidates {
		if set[t] {
			out = append(out, t)
		}
	}
	return out
}

// detectColumnType returns the highest-priority enabled type that every
// non-empty cell in cells parses as. It reports false when no type matches
// or when the cells hold no evidence at all (all null/empty), or when any
// cell is a non-string, non-nil value.
func detectColumnType(cells []any, enabled []DType) (DType, bool) {
next:
	for _, t := range enabled {
		evidence := false
		for _, v := range cells {
			if v == nil {
				continue
			}
			s, ok := v.(string)
			if !ok {
				return TypeString, false // not a text column, leave alone
			}
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if _, ok := parseAs(s, t); !ok {
				continue next
			}
			evidence = true
		}
		if evidence {
			return t, true
		}
	}
	return TypeString, false
}

// convertCells converts every cell to type t. Nulls and empty strings become
// nil; values that fail to parse (possible in fast mode, whose sample may not
// have seen them) also become nil.
func convertCells(cells []any, t DType) []any {
	out := make([]any, len(cells))
	for i, v := range cells {
		s, ok := v.(string)
		if !ok {
			continue // nil (or foreign value) -> nil
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if parsed, ok := parseAs(s, t); ok {
			out[i] = parsed
		}
	}
	return out
}

var (
	dateLayouts     = []string{"2006-01-02", "2006/01/02"}
	datetimeLayouts = []string{time.RFC3339, "2006-01-02 15:04:05"}
	// Note: time.Parse accepts an optional fractional-seconds field after the
	// seconds element even when the layout omits it, so both datetime layouts
	// already cover fractional seconds.
)

// parseAs parses s (already trimmed, non-empty) as type t.
func parseAs(s string, t DType) (any, bool) {
	switch t {
	case TypeInt:
		n, err := strconv.ParseInt(s, 10, 64)
		return n, err == nil
	case TypeFloat:
		x, err := strconv.ParseFloat(s, 64)
		return x, err == nil
	case TypeBool:
		if strings.EqualFold(s, "true") {
			return true, true
		}
		if strings.EqualFold(s, "false") {
			return false, true
		}
		return nil, false
	case TypeDate:
		for _, layout := range dateLayouts {
			if ts, err := time.Parse(layout, s); err == nil {
				return ts, true
			}
		}
		return nil, false
	case TypeDatetime:
		for _, layout := range datetimeLayouts {
			if ts, err := time.Parse(layout, s); err == nil {
				return ts, true
			}
		}
		return nil, false
	}
	return nil, false
}
