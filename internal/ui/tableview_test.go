package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
)

func testFrame(rows int) *data.Frame {
	f := data.New("id", "name", "comment")
	for i := 0; i < rows; i++ {
		f.AppendRow([]any{"1", "alice", "a longer comment cell"})
	}
	return f
}

// wideFrame builds a frame whose columns are far too wide to fit compactly
// in a narrow viewport.
func wideFrame(rows int) *data.Frame {
	f := data.New("alpha_column", "beta_column", "gamma_column", "delta_column")
	for i := 0; i < rows; i++ {
		f.AppendRow([]any{
			strings.Repeat("a", 30), strings.Repeat("b", 30),
			strings.Repeat("c", 30), strings.Repeat("d", 30),
		})
	}
	return f
}

func sum(ws []int) int {
	s := 0
	for _, w := range ws {
		s += w
	}
	return s
}

func TestFitWidthsAlreadyFits(t *testing.T) {
	nat := []int{5, 10, 3}
	got := fitWidths(nat, 18)
	if !reflect.DeepEqual(got, nat) {
		t.Fatalf("expected unchanged widths, got %v", got)
	}
}

func TestFitWidthsExactFit(t *testing.T) {
	nat := []int{5, 10, 3}
	got := fitWidths(nat, 18)
	if sum(got) != 18 {
		t.Fatalf("expected exact fit 18, got %v (sum %d)", got, sum(got))
	}
}

func TestFitWidthsShrinksWidestFirst(t *testing.T) {
	nat := []int{4, 20, 6}
	got := fitWidths(nat, 20)
	if sum(got) > 20 {
		t.Fatalf("sum %d exceeds available 20: %v", sum(got), got)
	}
	// the narrow columns must be untouched; only the widest shrinks
	if got[0] != 4 || got[2] != 6 {
		t.Fatalf("narrow columns were shrunk: %v", got)
	}
	if got[1] >= 20 {
		t.Fatalf("widest column not shrunk: %v", got)
	}
}

func TestFitWidthsEqualizesWideColumns(t *testing.T) {
	nat := []int{30, 30, 2}
	got := fitWidths(nat, 22)
	if sum(got) > 22 {
		t.Fatalf("sum %d exceeds 22: %v", sum(got), got)
	}
	if got[2] != 2 {
		t.Fatalf("narrow column shrunk: %v", got)
	}
	if diff := got[0] - got[1]; diff < -1 || diff > 1 {
		t.Fatalf("wide columns should shrink to about the same level: %v", got)
	}
}

func TestFitWidthsMinWidthOne(t *testing.T) {
	nat := []int{10, 10, 10}
	got := fitWidths(nat, 2) // less than one cell per column
	for i, w := range got {
		if w != 1 {
			t.Fatalf("column %d: expected min width 1, got %d (%v)", i, w, got)
		}
	}
}

func TestNaturalWidthsCapAndHeader(t *testing.T) {
	f := data.New("a_very_long_header", "b")
	f.AppendRow([]any{"x", strings.Repeat("y", 100)})
	got := naturalWidths(f, 0, 1, 40)
	if got[0] != len("a_very_long_header") {
		t.Fatalf("header width not respected: %v", got)
	}
	if got[1] != 40 {
		t.Fatalf("cap 40 not applied: %v", got)
	}
}

func TestVisibleColumnsWindow(t *testing.T) {
	nat := []int{10, 10, 10, 10}
	cols, widths := visibleColumns(nat, 1, 21) // fits col1 (10) + sep + col2 (10)
	if !reflect.DeepEqual(cols, []int{1, 2}) {
		t.Fatalf("expected cols [1 2], got %v", cols)
	}
	if !reflect.DeepEqual(widths, []int{10, 10}) {
		t.Fatalf("expected widths [10 10], got %v", widths)
	}
}

func TestVisibleColumnsAlwaysAtLeastOne(t *testing.T) {
	nat := []int{50}
	cols, widths := visibleColumns(nat, 0, 10)
	if len(cols) != 1 || widths[0] != 10 {
		t.Fatalf("oversized single column should be clipped to viewport: %v %v", cols, widths)
	}
}

func TestScrollColIntoView(t *testing.T) {
	nat := []int{10, 10, 10, 10, 10}
	// cursor left of window snaps window to cursor
	if off := scrollColIntoView(nat, 3, 1, 25); off != 1 {
		t.Fatalf("expected off 1, got %d", off)
	}
	// cursor right of window advances until it fits: cols 3,4 = 10+1+10=21 <= 25
	if off := scrollColIntoView(nat, 0, 4, 25); off != 3 {
		t.Fatalf("expected off 3, got %d", off)
	}
	// cursor wider than viewport still lands on the cursor column
	if off := scrollColIntoView(nat, 0, 2, 5); off != 2 {
		t.Fatalf("expected off 2, got %d", off)
	}
}

func TestScrollIntoViewOnRender(t *testing.T) {
	f := testFrame(100)
	tv := &TableView{}
	th := theme.Default()
	o := RenderOpts{Width: 60, Height: 11, Theme: th} // 10 body rows

	tv.Render(f, o)
	if tv.rowOff != 0 {
		t.Fatalf("initial rowOff = %d, want 0", tv.rowOff)
	}

	tv.JumpTo(50, f.NumRows())
	tv.Render(f, o)
	if tv.sel != 50 {
		t.Fatalf("sel = %d, want 50", tv.sel)
	}
	if tv.sel < tv.rowOff || tv.sel >= tv.rowOff+10 {
		t.Fatalf("selection %d not within window [%d, %d)", tv.sel, tv.rowOff, tv.rowOff+10)
	}

	tv.JumpTo(0, f.NumRows())
	tv.Render(f, o)
	if tv.rowOff != 0 {
		t.Fatalf("rowOff = %d after jump to top, want 0", tv.rowOff)
	}
}

func TestPageSizeReflectsBorders(t *testing.T) {
	f := testFrame(100)
	th := theme.Default()

	tv := &TableView{}
	tv.Render(f, RenderOpts{Width: 60, Height: 20, Theme: th})
	if tv.PageSize() != 19 {
		t.Fatalf("borderless page size = %d, want 19", tv.PageSize())
	}
	tv.Render(f, RenderOpts{Width: 60, Height: 20, Theme: th, ShowBorders: true})
	if tv.PageSize() != 16 {
		t.Fatalf("bordered page size = %d, want 16", tv.PageSize())
	}
}

func TestClampToOnFrameSwitch(t *testing.T) {
	big := testFrame(100)
	small := testFrame(3)
	tv := &TableView{}
	tv.JumpTo(80, big.NumRows())
	tv.ClampTo(small)
	if tv.sel != 2 {
		t.Fatalf("sel = %d after clamping to 3-row frame, want 2", tv.sel)
	}
	tv.ClampTo(nil)
	if tv.sel != 0 || tv.rowOff != 0 {
		t.Fatalf("clamping to nil should reset, got sel=%d off=%d", tv.sel, tv.rowOff)
	}
}

func TestRenderLineCountAndWidth(t *testing.T) {
	f := testFrame(5)
	th := theme.Default()
	out := (&TableView{}).Render(f, RenderOpts{Width: 40, Height: 12, Theme: th, ShowRowNumbers: true})
	lines := strings.Split(out, "\n")
	if len(lines) != 12 {
		t.Fatalf("expected 12 lines, got %d", len(lines))
	}

	out = (&TableView{}).Render(f, RenderOpts{Width: 40, Height: 12, Theme: th, ShowBorders: true})
	lines = strings.Split(out, "\n")
	if len(lines) != 12 {
		t.Fatalf("expected 12 lines with borders, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "╭") || !strings.Contains(lines[11], "╯") {
		t.Fatalf("border glyphs missing:\n%s", out)
	}
}

func TestAutoExpandThresholds(t *testing.T) {
	cases := []struct {
		name        string
		nat, fitted []int
		want        bool
	}{
		{"everything fits", []int{10, 10}, []int{10, 10}, false},
		{"exactly 60 percent stays compact", []int{20}, []int{12}, false},
		{"below 60 percent expands", []int{20}, []int{11}, true},
		{"wide column squeezed under 5 cells", []int{9, 3}, []int{4, 3}, true},
		{"narrow column may go under 5 cells", []int{8, 3}, []int{5, 3}, false},
		{"one bad column among good ones", []int{10, 40, 10}, []int{10, 20, 10}, true},
		{"empty", nil, nil, false},
	}
	for _, c := range cases {
		if got := autoExpand(c.nat, c.fitted); got != c.want {
			t.Errorf("%s: autoExpand(%v, %v) = %v, want %v", c.name, c.nat, c.fitted, got, c.want)
		}
	}
}

func TestRenderAutoPicksExpanded(t *testing.T) {
	tv := &TableView{}
	tv.Render(wideFrame(5), RenderOpts{Width: 60, Height: 10, Theme: theme.Default()})
	if !tv.Expanded() {
		t.Fatal("wide frame in a narrow viewport should auto-pick expanded mode")
	}
}

func TestRenderAutoPicksCompact(t *testing.T) {
	tv := &TableView{}
	tv.Render(testFrame(5), RenderOpts{Width: 60, Height: 10, Theme: theme.Default()})
	if tv.Expanded() {
		t.Fatal("frame that fits comfortably should auto-pick compact mode")
	}
}

func TestManualColumnModeSticks(t *testing.T) {
	th := theme.Default()
	o := RenderOpts{Width: 60, Height: 10, Theme: th}

	// Manual toggle before the first render wins over the auto pick.
	tv := &TableView{}
	tv.ToggleExpanded()
	tv.Render(testFrame(5), o) // auto would pick compact
	if !tv.Expanded() {
		t.Fatal("manual expanded choice must not be overridden by the auto pick")
	}

	// Manual toggle after the auto pick sticks across renders.
	tv2 := &TableView{}
	tv2.Render(wideFrame(5), o)
	tv2.ToggleExpanded() // back to compact by hand
	tv2.Render(wideFrame(5), o)
	if tv2.Expanded() {
		t.Fatal("manual compact choice must stick for the current frame")
	}

	// Reset re-arms the auto pick (a new frame was pushed).
	tv2.Reset()
	tv2.Render(wideFrame(5), o)
	if !tv2.Expanded() {
		t.Fatal("auto pick should run again after Reset")
	}
}

func TestHeaderEmphasizesCursoredColumnBothModes(t *testing.T) {
	f := testFrame(5)
	th := theme.Default()

	for _, expanded := range []bool{false, true} {
		tv := &TableView{}
		if expanded {
			tv.ToggleExpanded()
		} else {
			tv.modeSet = true // pin fit mode
		}
		o := RenderOpts{Width: 60, Height: 10, Theme: th}
		before := tv.Render(f, o)
		tv.NextCol(f.NumCols())
		after := tv.Render(f, o)
		if before == after {
			t.Errorf("expanded=%v: moving the column cursor must change the header emphasis", expanded)
		}
	}
}

func TestPadCellSanitizesCarriageReturns(t *testing.T) {
	got := padCell("line1\r\nline2\rx", 20)
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("padCell leaked a raw control char: %q", got)
	}
	if !strings.Contains(got, "line1␤line2␤x") {
		t.Fatalf("padCell = %q, want CR/LF shown as ␤", got)
	}
}

func TestToggleSelectAndIsSelected(t *testing.T) {
	tv := &TableView{}
	if tv.IsSelected(0) {
		t.Fatal("fresh view reports a selected row")
	}
	tv.ToggleSelect(3)
	if !tv.IsSelected(3) {
		t.Fatal("ToggleSelect(3) did not mark row 3 selected")
	}
	// Toggling again removes the row (dedup).
	tv.ToggleSelect(3)
	if tv.IsSelected(3) {
		t.Fatal("second ToggleSelect(3) did not clear the row")
	}
	// Multiple distinct rows coexist.
	tv.ToggleSelect(1)
	tv.ToggleSelect(5)
	if !tv.IsSelected(1) || !tv.IsSelected(5) {
		t.Fatal("independent rows not tracked after re-toggle")
	}
}

func TestSelectedRowsSorted(t *testing.T) {
	tv := &TableView{}
	tv.ToggleSelect(7)
	tv.ToggleSelect(2)
	tv.ToggleSelect(5)
	want := []int{2, 5, 7}
	if got := tv.SelectedRows(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectedRows = %v, want %v", got, want)
	}
	// Returned slice is a copy: mutating it must not affect the view.
	got := tv.SelectedRows()
	got[0] = 999
	if tv.IsSelected(999) {
		t.Fatal("SelectedRows returned an alias, not a copy")
	}
}

func TestSelectedRowsEmptyWhenNone(t *testing.T) {
	tv := &TableView{}
	if got := tv.SelectedRows(); got != nil {
		t.Fatalf("SelectedRows on untouched view = %v, want nil", got)
	}
	// After clearing, SelectedRows is nil again.
	tv.ToggleSelect(0)
	tv.ClearSelect()
	if got := tv.SelectedRows(); got != nil {
		t.Fatalf("SelectedRows after ClearSelect = %v, want nil", got)
	}
}

func TestClearSelectEmptiesSet(t *testing.T) {
	tv := &TableView{}
	tv.ToggleSelect(1)
	tv.ToggleSelect(2)
	tv.ClearSelect()
	if tv.IsSelected(1) || tv.IsSelected(2) {
		t.Fatal("ClearSelect did not empty the set")
	}
	if got := tv.SelectedRows(); len(got) != 0 {
		t.Fatalf("SelectedRows after ClearSelect = %v, want empty", got)
	}
}

func TestResetClearsSelect(t *testing.T) {
	tv := &TableView{}
	tv.ToggleSelect(1)
	tv.Reset()
	if tv.IsSelected(1) {
		t.Fatal("Reset did not clear the multi-select set")
	}
}

func TestClampToClearsSelect(t *testing.T) {
	tv := &TableView{}
	tv.ToggleSelect(1)
	tv.ClampTo(testFrame(3))
	if tv.IsSelected(1) {
		t.Fatal("ClampTo did not clear the multi-select set")
	}
	// ClampTo(nil) routes through Reset, which also clears.
	tv.ToggleSelect(0)
	tv.ClampTo(nil)
	if tv.IsSelected(0) {
		t.Fatal("ClampTo(nil) did not clear the multi-select set")
	}
}

func TestRenderSelectedRowShowsMarkerInGutter(t *testing.T) {
	f := testFrame(5)
	th := theme.Default()
	o := RenderOpts{Width: 40, Height: 12, Theme: th, ShowRowNumbers: true}

	tv := &TableView{}
	out := tv.Render(f, o)
	if strings.Contains(out, selectMarker) {
		t.Fatalf("no rows selected, but marker present:\n%s", out)
	}

	tv.ToggleSelect(0)
	out = tv.Render(f, o)
	lines := strings.Split(out, "\n")
	// Row 0 is the first body row, which sits right under the header (and
	// below a border line when borders are on).
	body0 := lines[1]
	if !strings.ContainsRune(body0, []rune(selectMarker)[0]) {
		t.Fatalf("selected row 0 missing marker in:\n%s", out)
	}
	// Row 1 is not selected: its body line must not carry the marker.
	body1 := lines[2]
	if strings.ContainsRune(body1, []rune(selectMarker)[0]) {
		t.Fatalf("non-selected row 1 unexpectedly carries marker in:\n%s", out)
	}
}

func TestRenderSelectedRowShowsTagWithoutGutter(t *testing.T) {
	f := testFrame(5)
	th := theme.Default()
	o := RenderOpts{Width: 40, Height: 12, Theme: th} // no gutter

	tv := &TableView{}
	tv.ToggleSelect(0)
	out := tv.Render(f, o)
	if !strings.Contains(out, selectMarker) {
		t.Fatalf("selected row missing marker tag without gutter:\n%s", out)
	}
	// Width of each body line must still equal the inner width so column
	// alignment is preserved.
	for i, ln := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(ln); w != o.Width {
			t.Fatalf("line %d width = %d, want %d (alignment broken):\n%s", i, w, o.Width, out)
		}
	}
}

func TestRenderCursorKeepsMarkerWhenSelected(t *testing.T) {
	f := testFrame(5)
	th := theme.Default()
	o := RenderOpts{Width: 40, Height: 12, Theme: th, ShowRowNumbers: true}

	tv := &TableView{}
	tv.JumpTo(2, f.NumRows())
	tv.ToggleSelect(2)
	out := tv.Render(f, o)

	// The cursor row (row 2, i.e. "3" in the gutter) must carry both the
	// marker and remain on screen.
	body2 := strings.Split(out, "\n")[3]
	if !strings.ContainsRune(body2, []rune(selectMarker)[0]) {
		t.Fatalf("cursor+selected row 2 missing marker:\n%s", out)
	}
}
