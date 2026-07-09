package writer

import (
	"encoding/csv"
	"io"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatCSV, csvWriter{})
	Register(FormatTSV, csvWriter{tab: true})
}

// csvWriter serializes frames as delimiter-separated values.
type csvWriter struct {
	tab bool // force tab separator (tsv)
}

func (cw csvWriter) Write(w io.Writer, f *data.Frame, opt Options) error {
	enc := csv.NewWriter(w)
	sep := opt.Separator
	if sep == 0 {
		sep = ','
	}
	if cw.tab {
		sep = '\t'
	}
	enc.Comma = sep

	if opt.Header {
		if err := enc.Write(f.ColumnNames()); err != nil {
			return err
		}
	}
	record := make([]string, f.NumCols())
	for r := 0; r < f.NumRows(); r++ {
		for c := range f.Columns {
			record[c] = data.FormatValue(f.Columns[c].Cells[r])
		}
		if err := enc.Write(record); err != nil {
			return err
		}
	}
	enc.Flush()
	return enc.Error()
}
