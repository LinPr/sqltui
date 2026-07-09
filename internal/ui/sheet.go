package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
)

// sheetSanitize normalizes carriage returns so a raw \r never reaches the
// terminal (it would return the cursor to column 0 and overwrite the line)
// and so ansi.Wrap's width accounting stays correct. sheetBody and
// sheetLineCount must agree on this for scroll clamping.
func sheetSanitize(val string) string {
	val = strings.ReplaceAll(val, "\r\n", "\n")
	return strings.ReplaceAll(val, "\r", "\n")
}

// sheetBody builds the scrollable lines of the sheet view for one row:
// per column, the column name followed by the wrapped value.
func sheetBody(f *data.Frame, row, width int, th *theme.Theme) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for c := range f.Columns {
		name := f.Columns[c].Name
		lines = append(lines, th.Header.Render(padLine(name, width)))
		val := sheetSanitize(f.CellString(row, c))
		if val == "" {
			val = " "
		}
		for _, l := range strings.Split(ansi.Wrap(val, width, ""), "\n") {
			lines = append(lines, th.Text.Render(padLine(l, width)))
		}
		lines = append(lines, th.Text.Render(strings.Repeat(" ", width)))
	}
	return lines
}

// sheetLineCount reports how many body lines the sheet has (for scroll
// clamping without rendering).
func sheetLineCount(f *data.Frame, row, width int) int {
	if f == nil || row < 0 || row >= f.NumRows() {
		return 0
	}
	if width < 1 {
		width = 1
	}
	n := 0
	for c := range f.Columns {
		val := sheetSanitize(f.CellString(row, c))
		if val == "" {
			val = " "
		}
		n += 2 + strings.Count(ansi.Wrap(val, width, ""), "\n") + 1
	}
	return n
}

// renderSheet draws the row-detail sheet into a width x height area.
// off is the scroll offset in body lines (pre-clamped by the caller, but
// clamped again defensively).
func renderSheet(f *data.Frame, row, off, width, height int, th *theme.Theme) string {
	if f == nil || f.NumRows() == 0 || width <= 0 || height <= 0 {
		return blankLines(max(1, width), max(1, height), th)
	}
	row = clamp(row, 0, f.NumRows()-1)

	head := fmt.Sprintf("row %d of %d", row+1, f.NumRows())
	out := []string{th.Subtle.Render(padLine(head, width))}

	body := sheetBody(f, row, width, th)
	visible := height - 1
	off = clamp(off, 0, max(0, len(body)-visible))
	for i := 0; i < visible; i++ {
		if off+i < len(body) {
			out = append(out, body[off+i])
		} else {
			out = append(out, th.Text.Render(strings.Repeat(" ", width)))
		}
	}
	return strings.Join(out, "\n")
}

// rowClipboardText renders one row as tab-separated values for copying.
func rowClipboardText(f *data.Frame, row int) string {
	if f == nil || row < 0 || row >= f.NumRows() {
		return ""
	}
	parts := make([]string, f.NumCols())
	for c := range f.Columns {
		parts[c] = f.CellString(row, c)
	}
	return strings.Join(parts, "\t")
}
