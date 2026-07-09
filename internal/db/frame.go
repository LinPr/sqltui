package db

import (
	"github.com/LinPr/sqltui/internal/data"
)

// StringFrame builds an all-string frame from a result-set header and rows,
// as produced by the raw query helpers of the SQL backends. Missing trailing
// cells become nil (null).
func StringFrame(fields []string, records [][]string) *data.Frame {
	f := data.New(fields...)
	for _, rec := range records {
		row := make([]any, len(fields))
		for i := range fields {
			if i < len(rec) {
				row[i] = rec[i]
			}
		}
		f.AppendRow(row)
	}
	return f
}
