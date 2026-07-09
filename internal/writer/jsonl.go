package writer

import (
	"bufio"
	"io"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatJSONL, jsonlWriter{})
}

// jsonlWriter serializes a frame as one JSON object per line.
type jsonlWriter struct{}

func (jsonlWriter) Write(w io.Writer, f *data.Frame, opt Options) error {
	bw := bufio.NewWriter(w)
	keys := jsonKeys(f)
	for r := 0; r < f.NumRows(); r++ {
		if _, err := bw.Write(rowObject(f, keys, r)); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}
