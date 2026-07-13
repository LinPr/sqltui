package ui

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
)

const (
	compactColCap  = 40 // natural column width cap in compact mode
	expandedColCap = 80 // natural column width cap in expanded mode
	sampleRows     = 200
)

// TableView holds the scroll/selection state of one table widget. Rendering
// is stateless apart from remembering the last page size for paging keys.
type TableView struct {
	sel       int  // selected row
	rowOff    int  // first visible row
	colOff    int  // first visible column (expanded mode)
	colCursor int  // cursored column (expanded mode)
	expanded  bool // column mode
	modeSet   bool // column mode already picked (auto at first render, or manual 'e')

	snapCursor bool // scroll colCursor into view on next render
	lastRows   int  // page size observed at last render

	selected map[int]bool // multi-select membership; lazy-init; nil = none
}

// selectMarker is drawn in the gutter (or the leading tag column when there
// is no gutter) of rows belonging to the multi-select set.
const selectMarker = "█"

// RenderOpts carries everything Render needs besides the frame.
type RenderOpts struct {
	Width, Height  int
	Theme          *theme.Theme
	ShowBorders    bool
	ShowRowNumbers bool
	MatchRows      map[int]bool // rows emphasized with the match style
}

// --- state accessors -------------------------------------------------------

func (tv *TableView) Sel() int       { return tv.sel }
func (tv *TableView) Expanded() bool { return tv.expanded }
func (tv *TableView) ColCursor() int { return tv.colCursor }

// PageSize is the number of body rows shown at the last render (fallback 10).
func (tv *TableView) PageSize() int {
	if tv.lastRows <= 0 {
		return 10
	}
	return tv.lastRows
}

// Reset returns the view to the top-left with row 0 selected and re-arms the
// automatic column-mode pick for the next frame shown.
func (tv *TableView) Reset() {
	tv.sel, tv.rowOff, tv.colOff, tv.colCursor = 0, 0, 0, 0
	tv.snapCursor = false
	tv.modeSet = false
	tv.ClearSelect()
}

// ClampTo clamps selection and cursor to the bounds of f (used when the
// underlying frame changes but position should be roughly kept). A frame
// change invalidates the multi-select set, so it is cleared here too.
func (tv *TableView) ClampTo(f *data.Frame) {
	if f == nil {
		tv.Reset()
		return
	}
	tv.sel = clamp(tv.sel, 0, f.NumRows()-1)
	tv.rowOff = clamp(tv.rowOff, 0, max(0, f.NumRows()-1))
	tv.colCursor = clamp(tv.colCursor, 0, f.NumCols()-1)
	tv.colOff = clamp(tv.colOff, 0, max(0, f.NumCols()-1))
	tv.ClearSelect()
}

// --- navigation ------------------------------------------------------------

func (tv *TableView) Move(delta, nrows int) {
	tv.sel = clamp(tv.sel+delta, 0, nrows-1)
}

func (tv *TableView) Top() { tv.sel = 0 }

func (tv *TableView) Bottom(nrows int) {
	tv.sel = max(0, nrows-1)
}

func (tv *TableView) HalfPageUp(nrows int)   { tv.Move(-max(1, tv.PageSize()/2), nrows) }
func (tv *TableView) HalfPageDown(nrows int) { tv.Move(max(1, tv.PageSize()/2), nrows) }
func (tv *TableView) PageUp(nrows int)       { tv.Move(-max(1, tv.PageSize()), nrows) }
func (tv *TableView) PageDown(nrows int)     { tv.Move(max(1, tv.PageSize()), nrows) }

func (tv *TableView) Random(nrows int) {
	if nrows > 0 {
		tv.sel = rand.Intn(nrows)
	}
}

// JumpTo selects a specific row (clamped).
func (tv *TableView) JumpTo(row, nrows int) {
	tv.sel = clamp(row, 0, nrows-1)
}

// ToggleExpanded flips the column mode manually; the choice sticks for the
// frame currently shown (no later auto-pick overrides it).
func (tv *TableView) ToggleExpanded() {
	tv.expanded = !tv.expanded
	tv.modeSet = true
}

// LastCol moves the column cursor to the last column and requests that it be
// scrolled into view (wide mode).
func (tv *TableView) LastCol(ncols int) {
	tv.colCursor = max(0, ncols-1)
	tv.snapCursor = true
}

// NextCol / PrevCol / FirstCol move the column cursor (both column modes)
// and request that it be scrolled into view (wide mode).
func (tv *TableView) NextCol(ncols int) {
	tv.colCursor = clamp(tv.colCursor+1, 0, ncols-1)
	tv.snapCursor = true
}

func (tv *TableView) PrevCol(ncols int) {
	tv.colCursor = clamp(tv.colCursor-1, 0, ncols-1)
	tv.snapCursor = true
}

func (tv *TableView) FirstCol() {
	tv.colCursor = 0
	tv.snapCursor = true
}

// --- multi-select ----------------------------------------------------------

// ToggleSelect adds a row to (or removes it from) the multi-select set. The
// set is lazy-initialized on first use so an untouched view stays allocation
// free.
func (tv *TableView) ToggleSelect(row int) {
	if tv.selected == nil {
		tv.selected = map[int]bool{}
	}
	tv.selected[row] = !tv.selected[row]
	if !tv.selected[row] {
		delete(tv.selected, row)
	}
}

// IsSelected reports whether a row is in the multi-select set.
func (tv *TableView) IsSelected(row int) bool {
	return tv.selected != nil && tv.selected[row]
}

// SelectedRows returns the multi-select members in ascending order. The
// returned slice is freshly allocated; callers may mutate it freely.
func (tv *TableView) SelectedRows() []int {
	if tv.selected == nil {
		return nil
	}
	out := make([]int, 0, len(tv.selected))
	for r := range tv.selected {
		out = append(out, r)
	}
	sort.Ints(out)
	return out
}

// ClearSelect empties the multi-select set.
func (tv *TableView) ClearSelect() {
	tv.selected = nil
}

// --- rendering -------------------------------------------------------------

// Render draws the table into a Width x Height cell area.
func (tv *TableView) Render(f *data.Frame, o RenderOpts) string {
	th := o.Theme
	if o.Width <= 0 || o.Height <= 0 {
		return ""
	}
	if f == nil || f.NumCols() == 0 {
		return blankLines(o.Width, o.Height, th)
	}

	nrows, ncols := f.NumRows(), f.NumCols()

	// Vertical budget.
	body := o.Height - 1 // header
	if o.ShowBorders {
		body = o.Height - 4 // top, header, separator, bottom
	}
	if body < 1 {
		body = 1
	}
	tv.lastRows = body

	// Keep selection visible.
	tv.sel = clamp(tv.sel, 0, max(0, nrows-1))
	if tv.sel < tv.rowOff {
		tv.rowOff = tv.sel
	}
	if tv.sel >= tv.rowOff+body {
		tv.rowOff = tv.sel - body + 1
	}
	tv.rowOff = clamp(tv.rowOff, 0, max(0, nrows-body))

	// Horizontal budget.
	inner := o.Width
	if o.ShowBorders {
		inner = o.Width - 2
		if inner < 1 {
			inner = 1
		}
	}
	gutterW := 0
	if o.ShowRowNumbers {
		gutterW = len(strconv.Itoa(max(1, nrows))) + 1
	}
	avail := inner - gutterW
	if avail < 1 {
		avail = 1
	}

	// Column layout.
	lo := tv.rowOff
	hi := min(nrows, lo+sampleRows)

	// First render of a frame: pick the column mode automatically. When
	// compact fitting would degrade columns badly the view starts expanded.
	if !tv.modeSet {
		tv.modeSet = true
		nat := naturalWidths(f, lo, hi, compactColCap)
		tv.expanded = autoExpand(nat, fitWidths(nat, avail-(ncols-1)))
	}

	var cols []int   // visible column indices
	var widths []int // display width per visible column
	if tv.expanded {
		nat := naturalWidths(f, lo, hi, expandedColCap)
		tv.colCursor = clamp(tv.colCursor, 0, ncols-1)
		tv.colOff = clamp(tv.colOff, 0, ncols-1)
		if tv.snapCursor {
			tv.colOff = scrollColIntoView(nat, tv.colOff, tv.colCursor, avail)
			tv.snapCursor = false
		}
		cols, widths = visibleColumns(nat, tv.colOff, avail)
		// After a resize (or any window change) keep the cursor visible.
		if len(cols) > 0 {
			tv.colCursor = clamp(tv.colCursor, cols[0], cols[len(cols)-1])
		}
	} else {
		nat := naturalWidths(f, lo, hi, compactColCap)
		fitted := fitWidths(nat, avail-(ncols-1))
		tv.colCursor = clamp(tv.colCursor, 0, ncols-1)
		cols = make([]int, ncols)
		for i := range cols {
			cols[i] = i
		}
		widths = fitted
	}

	var b strings.Builder
	if o.ShowBorders {
		b.WriteString(th.Border.Render("╭" + strings.Repeat("─", inner) + "╮"))
		b.WriteString("\n")
	}

	// Header, with the cursored column emphasized (both column modes: y-copy
	// and the status bar column tag follow the cursor everywhere).
	var hb strings.Builder
	hb.WriteString(th.Header.Render(strings.Repeat(" ", gutterW)))
	for i, c := range cols {
		if i > 0 {
			hb.WriteString(th.Header.Render(" "))
		}
		cell := padCell(f.Columns[c].Name, widths[i])
		if c == tv.colCursor {
			// The cursored column's header uses the row-selection style so
			// column navigation is visible.
			hb.WriteString(th.RowSelected.Bold(true).Render(cell))
		} else {
			hb.WriteString(th.Header.Render(cell))
		}
	}
	used := gutterW
	for i, w := range widths {
		if i > 0 {
			used++
		}
		used += w
	}
	if pad := inner - used; pad > 0 {
		hb.WriteString(th.Header.Render(strings.Repeat(" ", pad)))
	}
	headLine := hb.String()
	if ansi.StringWidth(headLine) > inner {
		// Fit-mode separators can overflow a pathologically narrow viewport.
		headLine = ansi.Truncate(headLine, inner, "…")
	}
	b.WriteString(wrapBorder(headLine, o.ShowBorders, th))
	b.WriteString("\n")

	if o.ShowBorders {
		b.WriteString(th.Border.Render("├" + strings.Repeat("─", inner) + "┤"))
		b.WriteString("\n")
	}

	// Body rows.
	cells := make([]string, len(cols))
	for i := 0; i < body; i++ {
		r := tv.rowOff + i
		var line string
		if r >= nrows {
			line = th.Text.Render(strings.Repeat(" ", inner))
		} else {
			for j, c := range cols {
				cells[j] = padCell(f.CellString(r, c), widths[j])
			}
			rowText := strings.Join(cells, " ")
			sel := tv.IsSelected(r)
			style := th.RowEven
			gstyle := th.Gutter
			switch {
			case r == tv.sel:
				style = th.RowSelected
				gstyle = th.RowSelected
			case sel:
				// Multi-select members use the match style so they stay
				// distinct from the active cursor (th.RowSelected).
				style = th.Match
				gstyle = th.Match
			case o.MatchRows != nil && o.MatchRows[r]:
				style = th.Match
			case r%2 == 1:
				style = th.RowOdd
			}
			gut := ""
			if gutterW > 0 {
				num := strconv.Itoa(r + 1)
				if sel {
					// Replace the gutter's trailing space with the marker so
					// total width is unchanged.
					gut = gstyle.Render(padLeft(num, gutterW-1) + selectMarker)
				} else {
					gut = gstyle.Render(padLeft(num, gutterW-1) + " ")
				}
			} else if sel {
				// No gutter: lead with a 2-cell tag column and shave it off
				// the cell budget so the row keeps the same width.
				tag := th.Match.Render(selectMarker + " ")
				line = tag + style.Render(padLine(rowText, inner-2))
				b.WriteString(wrapBorder(line, o.ShowBorders, th))
				if i < body-1 || o.ShowBorders {
					b.WriteString("\n")
				}
				continue
			}
			line = gut + style.Render(padLine(rowText, inner-gutterW))
		}
		b.WriteString(wrapBorder(line, o.ShowBorders, th))
		if i < body-1 || o.ShowBorders {
			b.WriteString("\n")
		}
	}

	if o.ShowBorders {
		b.WriteString(th.Border.Render("╰" + strings.Repeat("─", inner) + "╯"))
	}
	return b.String()
}

func wrapBorder(line string, borders bool, th *theme.Theme) string {
	if !borders {
		return line
	}
	side := th.Border.Render("│")
	return side + line + side
}

func blankLines(w, h int, th *theme.Theme) string {
	line := th.Text.Render(strings.Repeat(" ", w))
	lines := make([]string, h)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// --- width math (pure, unit-tested) ----------------------------------------

// naturalWidths computes, per column, max(header width, widest cell among
// rows [lo, hi)), capped at capW and floored at 1.
func naturalWidths(f *data.Frame, lo, hi, capW int) []int {
	lo = clamp(lo, 0, f.NumRows())
	hi = clamp(hi, lo, f.NumRows())
	out := make([]int, f.NumCols())
	for c := range f.Columns {
		w := ansi.StringWidth(f.Columns[c].Name)
		for r := lo; r < hi; r++ {
			if cw := ansi.StringWidth(f.CellString(r, c)); cw > w {
				w = cw
			}
			if w >= capW {
				w = capW
				break
			}
		}
		out[c] = clamp(w, 1, capW)
	}
	return out
}

// autoExpand reports whether the view should start in expanded mode: compact
// fitting (fitted) would shrink some column below ~60% of its natural width
// (nat), or below 5 cells when its natural width exceeds 8.
func autoExpand(nat, fitted []int) bool {
	for i := range nat {
		if fitted[i]*10 < nat[i]*6 {
			return true
		}
		if nat[i] > 8 && fitted[i] < 5 {
			return true
		}
	}
	return false
}

// fitWidths shrinks the widths in nat so their sum fits in avail cells,
// reducing the widest columns first, never below 1. When everything already
// fits, nat is returned unchanged (copied).
func fitWidths(nat []int, avail int) []int {
	out := make([]int, len(nat))
	copy(out, nat)
	total := 0
	maxW := 0
	for _, w := range out {
		total += w
		if w > maxW {
			maxW = w
		}
	}
	if total <= avail || len(out) == 0 {
		return out
	}
	if avail < len(out) {
		for i := range out {
			out[i] = 1
		}
		return out
	}
	// Find the largest level L such that sum(min(w, L)) <= avail; capping at
	// L trims the widest columns first.
	sumCap := func(l int) int {
		s := 0
		for _, w := range out {
			s += min(w, l)
		}
		return s
	}
	lo, hi := 1, maxW
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if sumCap(mid) <= avail {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	level := lo
	extra := avail - sumCap(level)
	for i := range out {
		if out[i] > level {
			out[i] = level
			if extra > 0 {
				out[i]++
				extra--
			}
		}
	}
	return out
}

// visibleColumns returns the column indices and widths that fit in avail
// cells starting at colOff, with a single-space separator between columns.
// At least one column is always returned.
func visibleColumns(nat []int, colOff, avail int) (cols, widths []int) {
	colOff = clamp(colOff, 0, max(0, len(nat)-1))
	used := 0
	for c := colOff; c < len(nat); c++ {
		need := nat[c]
		if len(cols) > 0 {
			need++ // separator
		}
		if used+need > avail && len(cols) > 0 {
			break
		}
		w := nat[c]
		if used+need > avail { // first column wider than the viewport
			w = avail
		}
		cols = append(cols, c)
		widths = append(widths, w)
		used += need
	}
	return cols, widths
}

// scrollColIntoView returns the new colOff such that column cursor is fully
// visible in a window of avail cells.
func scrollColIntoView(nat []int, colOff, cursor, avail int) int {
	if cursor < colOff {
		return cursor
	}
	// Advance colOff until [colOff..cursor] fits.
	for off := colOff; off <= cursor; off++ {
		w := 0
		for c := off; c <= cursor; c++ {
			if c > off {
				w++
			}
			w += nat[c]
		}
		if w <= avail || off == cursor {
			return off
		}
	}
	return cursor
}

// --- small helpers ----------------------------------------------------------

func padCell(s string, w int) string {
	// Normalize CR first: a raw \r reaching the terminal returns the cursor
	// to column 0 mid-row and corrupts the whole screen line.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", "␤")
	s = strings.ReplaceAll(s, "\r", "␤")
	s = strings.ReplaceAll(s, "\t", " ")
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func padLine(s string, w int) string {
	if ansi.StringWidth(s) > w {
		return ansi.Truncate(s, w, "…")
	}
	if pad := w - ansi.StringWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
