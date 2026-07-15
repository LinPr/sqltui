package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
)

// sheetSepChar is the vertical bar drawn between the key and value columns.
const sheetSepChar = "│"

// sheetCursorMark prefixes the key cell of the cursored field.
const sheetCursorMark = "▶"

// sheetSanitize normalizes carriage returns so a raw \r never reaches the
// terminal (it would return the cursor to column 0 and overwrite the line)
// and so ansi.Wrap's width accounting stays correct. The wrap helpers below
// must agree on this for scroll clamping.
func sheetSanitize(val string) string {
	val = strings.ReplaceAll(val, "\r\n", "\n")
	return strings.ReplaceAll(val, "\r", "\n")
}

// sheetTableGeometry returns the key and value column widths for a sheet of
// the given inner width. keyW is clamped to [4, 20]: long key names wrap
// instead of widening the left column. valW takes the remainder minus the
// separator column. valW is guaranteed >= 1, shrinking keyW down to a
// minimum of 1 when inner is pathologically small.
func sheetTableGeometry(f *data.Frame, inner int) (keyW, valW int) {
	if inner < 1 {
		inner = 1
	}
	const hi = 20
	const markW = 2 // sheetCursorMark + space, or the two-space indent
	keyW = 4
	if f != nil {
		for _, c := range f.Columns {
			if w := ansi.StringWidth(c.Name) + markW; w > keyW {
				keyW = w
			}
		}
	}
	keyW = clamp(keyW, 4, hi)
	valW = inner - keyW - 1
	for valW < 1 && keyW > 1 {
		keyW--
		valW = inner - keyW - 1
	}
	if valW < 1 {
		valW = 1
		keyW = max(1, inner-2)
	}
	return keyW, valW
}

// sheetKeyEntryHeight reports the wrapped line count of one key entry in the
// left list. The key name is wrapped to keyW-markW (the space left after the
// two-cell marker prefix); it is always at least 1.
func sheetKeyEntryHeight(f *data.Frame, field, keyW int) int {
	if f == nil || field < 0 || field >= f.NumCols() {
		return 1
	}
	const markW = 2
	avail := keyW - markW
	if avail < 1 {
		avail = 1
	}
	wrapped := ansi.Wrap(sheetSanitize(f.Columns[field].Name), avail, "")
	return strings.Count(wrapped, "\n") + 1
}

// sheetKeyLineOffset returns the left-list line index where the given field's
// entry begins (the sum of entry heights of all preceding fields).
func sheetKeyLineOffset(f *data.Frame, field, keyW int) int {
	if f == nil || field <= 0 {
		return 0
	}
	n := 0
	for c := 0; c < field && c < f.NumCols(); c++ {
		n += sheetKeyEntryHeight(f, c, keyW)
	}
	return n
}

// sheetKeyLineCount reports the total left-list line count across all fields.
func sheetKeyLineCount(f *data.Frame, keyW int) int {
	if f == nil {
		return 0
	}
	n := 0
	for c := range f.Columns {
		n += sheetKeyEntryHeight(f, c, keyW)
	}
	return n
}

// sheetValueLineCount reports the wrapped line count of one field's value at
// the given row. An empty value occupies a single line.
func sheetValueLineCount(f *data.Frame, row, field, valW int) int {
	if f == nil || field < 0 || field >= f.NumCols() || row < 0 || row >= f.NumRows() {
		return 1
	}
	if valW < 1 {
		valW = 1
	}
	val := sheetSanitize(f.CellString(row, field))
	if val == "" {
		val = " "
	}
	return strings.Count(ansi.Wrap(val, valW, ""), "\n") + 1
}

// sheetMatchedCols returns the column indices whose name fuzzy-matches the
// trimmed pattern (case-insensitive). An empty pattern matches every column
// (0..NumCols-1). fuzzy.Find already ranks results; the returned slice is
// re-sorted by column index so the left list keeps its natural top-to-bottom
// order. A nil frame yields nil.
func sheetMatchedCols(f *data.Frame, pattern string) []int {
	if f == nil {
		return nil
	}
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	if pattern == "" {
		out := make([]int, f.NumCols())
		for i := range out {
			out[i] = i
		}
		return out
	}
	names := make([]string, f.NumCols())
	for i := range names {
		names[i] = strings.ToLower(f.Columns[i].Name)
	}
	matches := fuzzy.Find(pattern, names)
	out := make([]int, len(matches))
	for i, m := range matches {
		out[i] = m.Index
	}
	sort.Ints(out)
	return out
}

// sheetKeyEntryHeightCols reports the wrapped line count of one entry in a
// matched-column left list. cols is the matched index slice; pos is the index
// INTO cols (not a raw column index).
func sheetKeyEntryHeightCols(f *data.Frame, cols []int, pos, keyW int) int {
	if f == nil || pos < 0 || pos >= len(cols) {
		return 1
	}
	return sheetKeyEntryHeight(f, cols[pos], keyW)
}

// sheetKeyLineOffsetCols returns the left-list line index where the entry at
// position pos in cols begins (the sum of entry heights of all preceding
// matched positions).
func sheetKeyLineOffsetCols(f *data.Frame, cols []int, pos, keyW int) int {
	if f == nil || pos <= 0 {
		return 0
	}
	n := 0
	for c := 0; c < pos && c < len(cols); c++ {
		n += sheetKeyEntryHeight(f, cols[c], keyW)
	}
	return n
}

// sheetKeyLineCountCols reports the total left-list line count across every
// matched entry in cols.
func sheetKeyLineCountCols(f *data.Frame, cols []int, keyW int) int {
	if f == nil {
		return 0
	}
	n := 0
	for _, c := range cols {
		n += sheetKeyEntryHeight(f, c, keyW)
	}
	return n
}

// sheetKeyListCols renders the matched columns' key names as wrapped lines,
// highlighting the entry at position fieldCursor (an index INTO cols). Each
// rendered line is padded to keyW cells; the cursor marker prefixes the first
// line of the cursor entry.
func sheetKeyListCols(f *data.Frame, cols []int, keyW, fieldCursor int, th *theme.Theme) []string {
	const markW = 2
	avail := keyW - markW
	if avail < 1 {
		avail = 1
	}
	var lines []string
	for pos, c := range cols {
		name := sheetSanitize(f.Columns[c].Name)
		if name == "" {
			name = " "
		}
		wrapped := strings.Split(ansi.Wrap(name, avail, ""), "\n")
		cursor := pos == fieldCursor
		keyStyle := th.Header
		if cursor {
			keyStyle = th.RowSelected
		}
		for i, wl := range wrapped {
			mark := "  "
			if cursor && i == 0 {
				mark = sheetCursorMark + " "
			}
			cell := keyStyle.Render(padLine(mark+wl, keyW))
			lines = append(lines, cell)
		}
	}
	return lines
}

// sheetEditText renders the edit runes as plain text for wrapping. The block
// cursor styling is applied per visible line in sheetRenderValueLine.
func sheetEditText(runes []rune) string {
	if len(runes) == 0 {
		return " "
	}
	return string(runes)
}

// sheetEditLineOf reports which wrapped value line the edit cursor (rune
// index editCur) lands on, and the rune offset where that line begins. It
// mirrors the line breaking of ansi.Wrap over the edit runes: since the edit
// text has no word boundaries, wrapping falls back to hard breaks at valW.
func sheetEditLineOf(editRunes []rune, editCur, valW int) (lineIdx, lineStart int) {
	if valW < 1 {
		valW = 1
	}
	if editCur < 0 {
		editCur = 0
	}
	if editCur > len(editRunes) {
		editCur = len(editRunes)
	}
	line, start := 0, 0
	count := 0
	for i := 0; i < editCur; i++ {
		count++
		if count >= valW {
			start += count
			line++
			count = 0
		}
	}
	return line, start
}

// renderSheet draws the master-detail sheet into a width x height area: a
// subtle "row N of M" header line, a "Key | Value" column header, then the
// body. The LEFT column lists every field's key (wrapped, cursor
// highlighted). The RIGHT column shows ONLY the cursored field's value
// (wrapped, scrollable via valOff). When editing is true the right column
// shows the edit runes with a block cursor instead of the static value.
//
// off is the left-list scroll offset (Pane.SheetOff); valOff is the right
// value scroll offset (Pane.SheetValOff). renderSheet clamps both
// defensively; the edge-follow mutation stays in the caller.
//
// filter is the current field-filter pattern; filtering reports whether the
// filter input is active (a cursor is shown on the filter line). The left
// list narrows to the matched columns (sheetMatchedCols) and fieldCursor is
// interpreted as an index INTO the matched set, not a raw column. When the
// matched set is empty the body shows a subtle "no matching fields" line.
//
// fieldType is the cursored field's data-type token (e.g. "int",
// "varchar(255)", "i64"). When non-empty it is appended as a " (type)" suffix
// to the FIRST value line, sharing the value's styling. The tag is suppressed
// while editing so the block cursor stays clean. Empty means no tag (e.g. a
// column with no resolvable type).
func renderSheet(f *data.Frame, row, off, width, height, fieldCursor int, editing bool, editRunes []rune, editCur, valOff int, filter string, filtering bool, fieldType string, th *theme.Theme) string {
	if f == nil || f.NumRows() == 0 || width <= 0 || height <= 0 {
		return blankLines(max(1, width), max(1, height), th)
	}
	row = clamp(row, 0, f.NumRows()-1)
	inner := width
	if inner < 1 {
		inner = 1
	}
	keyW, valW := sheetTableGeometry(f, inner)

	matched := sheetMatchedCols(f, filter)
	// fieldCursor is an index into matched; resolve the real column for the
	// right pane. Empty matched set -> no field.
	actualCol := -1
	if len(matched) > 0 {
		fieldCursor = clamp(fieldCursor, 0, len(matched)-1)
		actualCol = matched[fieldCursor]
	} else {
		fieldCursor = 0
	}

	// Header lines: row N of M, an optional filter line, then Key|Value.
	head := fmt.Sprintf("row %d of %d", row+1, f.NumRows())
	out := []string{th.Subtle.Render(padLine(head, inner))}

	showFilterLine := filtering || strings.TrimSpace(filter) != ""
	if showFilterLine {
		out = append(out, sheetFilterLine(filter, filtering, inner, th))
	}

	sep := th.Border.Render(sheetSepChar)
	colHeader := th.Header.Render(padLine("Key", keyW)) + sep + th.Header.Render(padLine("Value", valW))
	if w := ansi.StringWidth(colHeader); w < inner {
		colHeader += th.Header.Render(strings.Repeat(" ", inner-w))
	}
	out = append(out, colHeader)

	visible := height - len(out) // remaining body lines
	if visible < 1 {
		visible = 1
	}

	if len(matched) == 0 {
		out = append(out, th.Placeholder.Render(padLine("no matching fields", inner)))
		for i := 1; i < visible; i++ {
			out = append(out, th.Text.Render(strings.Repeat(" ", inner)))
		}
		return strings.Join(out, "\n")
	}

	// --- left list: keys (matched only) --------------------------------------
	keyLines := sheetKeyListCols(f, matched, keyW, fieldCursor, th)
	maxOff := max(0, len(keyLines)-visible)
	off = clamp(off, 0, maxOff)

	// --- right pane: the cursored field value, with an inline type tag -------
	var valueText string
	if editing {
		valueText = sheetEditText(editRunes)
	} else {
		valueText = sheetSanitize(f.CellString(row, actualCol))
		if valueText == "" {
			valueText = " "
		}
	}
	valWrapped := ansi.Wrap(valueText, valW, "")
	valLines := strings.Split(valWrapped, "\n")
	if fieldType != "" && !editing && len(valLines) > 0 {
		valLines[0] = valLines[0] + " (" + fieldType + ")"
	}
	maxValOff := max(0, len(valLines)-visible)
	valOff = clamp(valOff, 0, maxValOff)

	for i := 0; i < visible; i++ {
		var left string
		if idx := off + i; idx < len(keyLines) {
			left = keyLines[idx]
		} else {
			left = th.Text.Render(strings.Repeat(" ", keyW))
		}

		var right string
		if idx := valOff + i; idx < len(valLines) {
			right = sheetRenderValueLine(valLines[idx], valW, editing, editRunes, editCur, valOff, idx, th)
		} else {
			right = th.Text.Render(strings.Repeat(" ", valW))
		}

		line := left + sep + right
		if w := ansi.StringWidth(line); w < inner {
			line += th.Text.Render(strings.Repeat(" ", inner-w))
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// sheetFilterLine renders the field-filter input line. While filtering, a
// block cursor sits at the end of the typed text; when the pattern is empty
// the placeholder hints at the action. When not filtering but a pattern is
// still applied, the text is shown plainly.
func sheetFilterLine(filter string, filtering bool, inner int, th *theme.Theme) string {
	prefix := th.Subtle.Render(" filter ")
	var body string
	switch {
	case filtering && filter == "":
		body = th.Placeholder.Render("type to filter fields")
	case filtering:
		body = th.Input.Render(filter) + th.ListSelected.Render(" ")
	default:
		body = th.Input.Render(filter)
	}
	line := prefix + body
	if w := ansi.StringWidth(line); w < inner {
		line += th.Text.Render(strings.Repeat(" ", inner-w))
	}
	return line
}

// sheetKeyList renders every field's key name as one or more wrapped lines,
// highlighting the cursor entry. Each rendered line is padded to keyW cells.
// The cursor marker prefixes the first line of the cursor entry; continuation
// lines of that entry (and all lines of other entries) use a two-space indent.
func sheetKeyList(f *data.Frame, keyW, fieldCursor int, th *theme.Theme) []string {
	const markW = 2
	avail := keyW - markW
	if avail < 1 {
		avail = 1
	}
	var lines []string
	for c := range f.Columns {
		name := sheetSanitize(f.Columns[c].Name)
		if name == "" {
			name = " "
		}
		wrapped := strings.Split(ansi.Wrap(name, avail, ""), "\n")
		cursor := c == fieldCursor
		keyStyle := th.Header
		if cursor {
			keyStyle = th.RowSelected
		}
		for i, wl := range wrapped {
			mark := "  "
			if cursor && i == 0 {
				mark = sheetCursorMark + " "
			}
			cell := keyStyle.Render(padLine(mark+wl, keyW))
			lines = append(lines, cell)
		}
	}
	return lines
}

// sheetRenderValueLine renders one wrapped value line at valW width. When
// editing and this line contains the edit cursor cell, the cursor cell is
// rendered with the selection style as a block cursor.
func sheetRenderValueLine(line string, valW int, editing bool, editRunes []rune, editCur, valOff, idx int, th *theme.Theme) string {
	if !editing {
		return th.Text.Render(padLine(line, valW))
	}
	curLine, _ := sheetEditLineOf(editRunes, editCur, valW)
	absLine := valOff + idx
	if absLine != curLine {
		return th.Input.Render(padLine(line, valW))
	}
	_, lineStart := sheetEditLineOf(editRunes, editCur, valW)
	inLine := editCur - lineStart

	r := []rune(line)
	var b strings.Builder
	if inLine < len(r) {
		b.WriteString(th.Input.Render(string(r[:inLine])))
		b.WriteString(th.ListSelected.Render(string(r[inLine])))
		b.WriteString(th.Input.Render(string(r[inLine+1:])))
	} else {
		// Cursor past the end of the line's runes: render a trailing block.
		b.WriteString(th.Input.Render(string(r)))
		b.WriteString(th.ListSelected.Render(" "))
	}
	rendered := b.String()
	if ansi.StringWidth(rendered) > valW {
		rendered = ansi.Truncate(rendered, valW, "")
	}
	if pad := valW - ansi.StringWidth(rendered); pad > 0 {
		rendered += th.Input.Render(strings.Repeat(" ", pad))
	}
	return rendered
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
