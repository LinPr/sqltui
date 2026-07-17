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
	keyW = 4
	if f != nil {
		for _, c := range f.Columns {
			if w := ansi.StringWidth(c.Name); w > keyW {
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
// left list. The key name is wrapped to keyW; it is always at least 1.
func sheetKeyEntryHeight(f *data.Frame, field, keyW int) int {
	if f == nil || field < 0 || field >= f.NumCols() {
		return 1
	}
	wrapped := ansi.Wrap(sheetSanitize(f.Columns[field].Name), keyW, "")
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
// rendered line is padded to keyW cells; the cursor row uses th.RowSelected.
func sheetKeyListCols(f *data.Frame, cols []int, keyW, fieldCursor int, th *theme.Theme) []string {
	var lines []string
	for pos, c := range cols {
		name := sheetSanitize(f.Columns[c].Name)
		if name == "" {
			name = " "
		}
		wrapped := strings.Split(ansi.Wrap(name, keyW, ""), "\n")
		cursor := pos == fieldCursor
		keyStyle := th.Header
		if cursor {
			keyStyle = th.RowSelected
		}
		for _, wl := range wrapped {
			cell := keyStyle.Render(padLine(wl, keyW))
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

// renderSheet draws the master-detail sheet into a width x height area.
//
// Default (non-cursor) rows show a compact "key: value" line across the
// full width. The cursor row expands into a left key pane (keyW) + separator +
// right value pane (valW) with full-value scrolling support. When editing is
// true the right pane shows the edit runes with a block cursor.
//
// off is the left-list scroll offset (Pane.SheetOff); valOff is the right
// value scroll offset (Pane.SheetValOff). Both are clamped defensively.
//
// filter narrows the left list to fuzzy-matched columns; fieldCursor is an
// index INTO the matched set.
func renderSheet(f *data.Frame, row, off, width, height, fieldCursor int, editing bool, editRunes []rune, editCur, valOff int, filter string, filtering bool, th *theme.Theme) string {
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
	actualCol := -1
	if len(matched) > 0 {
		fieldCursor = clamp(fieldCursor, 0, len(matched)-1)
		actualCol = matched[fieldCursor]
	} else {
		fieldCursor = 0
	}

	// Header: row N of M
	head := fmt.Sprintf("row %d of %d", row+1, f.NumRows())
	out := []string{th.Subtle.Render(padLine(head, inner))}

	showFilterLine := filtering || strings.TrimSpace(filter) != ""
	if showFilterLine {
		out = append(out, sheetFilterLine(filter, filtering, inner, th))
	}

	// Column header line
	sep := th.Border.Render(sheetSepChar)
	colHeader := th.Header.Render(padLine("Key", keyW)) + sep + th.Header.Render(padLine("Value", valW))
	if w := ansi.StringWidth(colHeader); w < inner {
		colHeader += th.Header.Render(strings.Repeat(" ", inner-w))
	}
	out = append(out, colHeader)

	visible := height - len(out) - 1
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

	// Build all body lines: compact rows + expanded cursor row.
	// Each matched entry contributes N lines (key wrap height for compact rows;
	// max(keyHeight, valHeight) for the cursor row).
	type bodyLine struct {
		text string
	}
	var body []bodyLine

	for pos, c := range matched {
		cursor := pos == fieldCursor
		colName := sheetSanitize(f.Columns[c].Name)
		if colName == "" {
			colName = " "
		}

		if cursor {
			// Expanded cursor row: keyW + sep + full-width value (scrollable).
			// We emit one logical "slot" per entry in the key-list, mirroring
			// sheetKeyListCols so that SheetOff tracks correctly.
			keyNameLines := strings.Split(ansi.Wrap(colName, keyW, ""), "\n")

			var valueText string
			if editing {
				valueText = sheetEditText(editRunes)
			} else {
				valueText = sheetSanitize(f.CellString(row, actualCol))
				if valueText == "" {
					valueText = " "
				}
			}
			valWrapped := strings.Split(ansi.Wrap(valueText, valW, ""), "\n")
			maxValOff := max(0, len(valWrapped)-visible)
			valOff = clamp(valOff, 0, maxValOff)

			entryHeight := len(keyNameLines)
			for li := 0; li < entryHeight; li++ {
				var kl string
				if li < len(keyNameLines) {
					kl = keyNameLines[li]
				}
				left := th.RowSelected.Render(padLine(kl, keyW))

				var right string
				if idx := valOff + li; idx < len(valWrapped) {
					right = sheetRenderValueLine(valWrapped[idx], valW, editing, editRunes, editCur, valOff, idx, th)
				} else {
					if editing {
						right = th.Input.Render(strings.Repeat(" ", valW))
					} else {
						right = th.Text.Render(strings.Repeat(" ", valW))
					}
				}
				line := left + sep + right
				if w := ansi.StringWidth(line); w < inner {
					line += th.Text.Render(strings.Repeat(" ", inner-w))
				}
				body = append(body, bodyLine{line})
			}
		} else {
			// Compact rows: each entry spans the same number of lines as its key
			// would wrap to, matching edgeFollowSheet's geometry. The first line
			// shows "key: truncated_value"; continuation lines show the
			// wrapped key fragment followed by blanks.
			keyNameLines := strings.Split(ansi.Wrap(colName, keyW, ""), "\n")
			val := sheetSanitize(f.CellString(row, c))
			valAvail := inner - keyW - 2
			if valAvail < 1 {
				valAvail = 1
			}
			dispVal := val
			if ansi.StringWidth(dispVal) > valAvail {
				dispVal = ansi.Truncate(dispVal, valAvail, "")
			}
			for li, kl := range keyNameLines {
				keyPart := th.Header.Render(padLine(kl, keyW))
				var valPart string
				if li == 0 {
					valPart = th.Text.Render(": " + dispVal)
				} else {
					valPart = th.Text.Render(strings.Repeat(" ", inner-keyW))
				}
				line := keyPart + valPart
				if w := ansi.StringWidth(line); w < inner {
					line += th.Text.Render(strings.Repeat(" ", inner-w))
				}
				body = append(body, bodyLine{line})
			}
		}
	}

	// Scroll offset
	maxOff := max(0, len(body)-visible)
	off = clamp(off, 0, maxOff)

	for i := 0; i < visible; i++ {
		if idx := off + i; idx < len(body) {
			out = append(out, body[idx].text)
		} else {
			out = append(out, th.Text.Render(strings.Repeat(" ", inner)))
		}
	}
	hint := padLine("j/k move  e edit  / filter  q back", inner)
	out = append(out, th.Subtle.Render(hint))
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
// highlighting the cursor entry with th.RowSelected. Each rendered line is
// padded to keyW cells.
func sheetKeyList(f *data.Frame, keyW, fieldCursor int, th *theme.Theme) []string {
	var lines []string
	for c := range f.Columns {
		name := sheetSanitize(f.Columns[c].Name)
		if name == "" {
			name = " "
		}
		wrapped := strings.Split(ansi.Wrap(name, keyW, ""), "\n")
		cursor := c == fieldCursor
		keyStyle := th.Header
		if cursor {
			keyStyle = th.RowSelected
		}
		for _, wl := range wrapped {
			cell := keyStyle.Render(padLine(wl, keyW))
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

// drawerLeftWidth returns the width of the left (key list) pane when the
// sheet is in drawer-edit mode. The left pane takes 2/5 of the total width,
// with a minimum of 10 columns.
func drawerLeftWidth(totalW int) int {
	w := totalW * 2 / 5
	if w < 10 {
		return 10
	}
	return w
}

// renderSheetDrawer renders the right-side editing drawer for the active sheet field.
// It shows the field name and type as a header, then the full value content area (scrollable, editable).
func renderSheetDrawer(f *data.Frame, row, col int, editRunes []rune, editCur, valOff, width, height int, fieldType string, th *theme.Theme) string {
	if width <= 0 || height <= 0 {
		return blankLines(max(1, width), max(1, height), th)
	}
	if f == nil || row < 0 || row >= f.NumRows() || col < 0 || col >= f.NumCols() {
		return blankLines(width, height, th)
	}

	colName := sheetSanitize(f.Columns[col].Name)
	header := colName
	if fieldType != "" {
		header = colName + "(" + fieldType + ")"
	}

	var lines []string
	lines = append(lines, th.Header.Render(padLine(header, width)))
	lines = append(lines, th.Border.Render(strings.Repeat("─", width)))

	contentH := height - len(lines) - 1
	if contentH < 1 {
		contentH = 1
	}

	var valueText string
	if len(editRunes) > 0 {
		valueText = sheetEditText(editRunes)
	} else {
		valueText = sheetSanitize(f.CellString(row, col))
		if valueText == "" {
			valueText = " "
		}
	}

	valWrapped := strings.Split(ansi.Wrap(valueText, width, ""), "\n")
	maxValOff := max(0, len(valWrapped)-contentH)
	valOff = clamp(valOff, 0, maxValOff)

	for i := 0; i < contentH; i++ {
		idx := valOff + i
		if idx < len(valWrapped) {
			line := sheetRenderValueLine(valWrapped[idx], width, true, editRunes, editCur, valOff, idx, th)
			lines = append(lines, line)
		} else {
			lines = append(lines, th.Input.Render(strings.Repeat(" ", width)))
		}
	}

	hint := padLine("ctrl+s save  esc cancel", width)
	lines = append(lines, th.Subtle.Render(hint))
	return strings.Join(lines, "\n")
}

// joinDrawerSplit joins a left sheet content and right drawer content with a vertical separator.
// leftContent and rightContent are already rendered to leftW and rightW cells wide respectively.
func joinDrawerSplit(leftContent, rightContent string, leftW, rightW, h int, th *theme.Theme) string {
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")

	sep := th.Border.Render("│")

	blank := func(w int) string { return strings.Repeat(" ", w) }

	var result []string
	for i := 0; i < h; i++ {
		ll := blank(leftW)
		if i < len(leftLines) {
			ll = leftLines[i]
		}
		rl := blank(rightW)
		if i < len(rightLines) {
			rl = rightLines[i]
		}
		result = append(result, ll+sep+rl)
	}
	return strings.Join(result, "\n")
}

// renderSheetKeyPanel renders the left key-only panel at exactly keyW width when
// the sheet is in drawer-edit mode. Unlike renderSheet, it does NOT call
// sheetTableGeometry internally — the caller pre-computes keyW so the panel
// matches the pre-edit key column exactly.
func renderSheetKeyPanel(f *data.Frame, row, off, keyW, height, fieldCursor int, filter string, filtering bool, th *theme.Theme) string {
	if f == nil || f.NumRows() == 0 || keyW <= 0 || height <= 0 {
		return blankLines(max(1, keyW), max(1, height), th)
	}
	row = clamp(row, 0, f.NumRows()-1)

	matched := sheetMatchedCols(f, filter)
	if len(matched) > 0 {
		fieldCursor = clamp(fieldCursor, 0, len(matched)-1)
	}

	head := fmt.Sprintf("row %d of %d", row+1, f.NumRows())
	out := []string{th.Subtle.Render(padLine(head, keyW))}

	if filtering || strings.TrimSpace(filter) != "" {
		out = append(out, sheetFilterLine(filter, filtering, keyW, th))
	}

	out = append(out, th.Header.Render(padLine("Key", keyW)))

	visible := height - len(out) - 1
	if visible < 1 {
		visible = 1
	}

	var body []string
	if len(matched) == 0 {
		body = append(body, th.Placeholder.Render(padLine("no match", keyW)))
	} else {
		body = sheetKeyListCols(f, matched, keyW, fieldCursor, th)
	}

	maxOff := max(0, len(body)-visible)
	off = clamp(off, 0, maxOff)

	for i := 0; i < visible; i++ {
		if idx := off + i; idx < len(body) {
			out = append(out, body[idx])
		} else {
			out = append(out, th.Text.Render(strings.Repeat(" ", keyW)))
		}
	}

	hint := padLine("e edit  / filter  j/k nav", keyW)
	out = append(out, th.Subtle.Render(hint))
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
