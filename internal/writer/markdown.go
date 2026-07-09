package writer

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatMarkdown, markdownWriter{})
}

// markdownWriter serializes a frame as a pipe table with a header row and an
// alignment row (numeric columns right-aligned).
type markdownWriter struct{}

// mdEscaper escapes backslashes before pipes so a cell containing `\|`
// roundtrips (the reader decodes `\\` back to a single backslash).
var mdEscaper = strings.NewReplacer(`\`, `\\`, "|", `\|`, "\r\n", " ", "\n", " ", "\r", " ")

func (markdownWriter) Write(w io.Writer, f *data.Frame, opt Options) error {
	nCols := f.NumCols()
	if nCols == 0 {
		return nil
	}

	// Render every cell up front so columns can be padded to equal width.
	header := make([]string, nCols)
	widths := make([]int, nCols)
	for c, col := range f.Columns {
		header[c] = mdEscaper.Replace(col.Name)
		widths[c] = max(utf8.RuneCountInString(header[c]), 3)
	}
	body := make([][]string, f.NumRows())
	for r := range body {
		row := make([]string, nCols)
		for c := range f.Columns {
			row[c] = mdEscaper.Replace(data.FormatValue(f.Columns[c].Cells[r]))
			if n := utf8.RuneCountInString(row[c]); n > widths[c] {
				widths[c] = n
			}
		}
		body[r] = row
	}

	bw := bufio.NewWriter(w)
	writeRow := func(cells []string) {
		bw.WriteByte('|')
		for c, cell := range cells {
			bw.WriteByte(' ')
			bw.WriteString(cell)
			bw.WriteString(strings.Repeat(" ", widths[c]-utf8.RuneCountInString(cell)))
			bw.WriteString(" |")
		}
		bw.WriteByte('\n')
	}

	writeRow(header)
	align := make([]string, nCols)
	for c, col := range f.Columns {
		if col.Type == data.TypeInt || col.Type == data.TypeFloat {
			align[c] = strings.Repeat("-", widths[c]-1) + ":"
		} else {
			align[c] = strings.Repeat("-", widths[c])
		}
	}
	writeRow(align)
	for _, row := range body {
		writeRow(row)
	}
	return bw.Flush()
}
