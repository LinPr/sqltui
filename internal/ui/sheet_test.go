package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/theme"
)

// sheetTestFrame builds a frame with several short fields and one long value
// that wraps in a narrow value column.
func sheetTestFrame() *data.Frame {
	f := data.New("id", "name", "comment", "notes")
	f.AppendRow([]any{
		"1",
		"alice",
		"a longer comment cell that should wrap across several value lines when the value column is narrow",
		"",
	})
	return f
}

// longKeyFrame has a key name that exceeds the keyW cap of 20 and so wraps in
// the left list.
func longKeyFrame() *data.Frame {
	f := data.New("id", "this_is_a_very_long_column_name_that_wraps", "v")
	f.AppendRow([]any{"1", "alpha", "beta"})
	return f
}

func TestSheetTableGeometry(t *testing.T) {
	f := sheetTestFrame()
	// Names are id/name/comment/notes; the key cell reserves a two-cell marker
	// prefix, so the longest key cell is "comment" (7) + 2 = 9. At inner=40
	// the cap of 20 keeps the natural 9.
	keyW, valW := sheetTableGeometry(f, 40)
	if keyW != 9 {
		t.Fatalf("keyW = %d, want 9", keyW)
	}
	if valW != 40-9-1 {
		t.Fatalf("valW = %d, want %d", valW, 40-9-1)
	}
	// A very long key name does NOT widen keyW past 20.
	lf := longKeyFrame()
	keyW, _ = sheetTableGeometry(lf, 80)
	if keyW != 20 {
		t.Fatalf("keyW = %d, want 20 (cap)", keyW)
	}
	// Tiny inner: valW stays >= 1 by shrinking keyW.
	keyW, valW = sheetTableGeometry(f, 4)
	if valW < 1 {
		t.Fatalf("valW = %d, want >= 1", valW)
	}
	if keyW+1+valW > 4 && keyW > 1 {
		t.Fatalf("geometry %d+%d+1 overflows inner 4", keyW, valW)
	}
}

func TestSheetKeyEntryHeightAndOffsets(t *testing.T) {
	f := sheetTestFrame()
	// At keyW=20 the longest name "comment" (7) fits on one line, so every
	// entry has height 1.
	const keyW = 20
	for c := 0; c < f.NumCols(); c++ {
		if h := sheetKeyEntryHeight(f, c, keyW); h != 1 {
			t.Fatalf("entry height field %d = %d, want 1", c, h)
		}
	}
	// Offsets are field indices (each entry is 1 line) and the total is the
	// column count.
	for c := 0; c <= f.NumCols(); c++ {
		if got := sheetKeyLineOffset(f, c, keyW); got != c {
			t.Fatalf("offset field %d = %d, want %d", c, got, c)
		}
	}
	if got := sheetKeyLineCount(f, keyW); got != f.NumCols() {
		t.Fatalf("key line count = %d, want %d", got, f.NumCols())
	}
}

func TestSheetKeyEntryHeightWrapsLongName(t *testing.T) {
	f := longKeyFrame()
	const keyW = 20
	// Field 1's name is 44 chars; avail = keyW-2 = 18, so it wraps to 3 lines
	// (44 / 18 = 2.44 -> ceil 3).
	h := sheetKeyEntryHeight(f, 1, keyW)
	wrapped := ansi.Wrap(f.Columns[1].Name, keyW-2, "")
	want := strings.Count(wrapped, "\n") + 1
	if h != want {
		t.Fatalf("wrapped entry height = %d, want %d", h, want)
	}
	if h < 2 {
		t.Fatalf("long key name should wrap to >= 2 lines, got %d", h)
	}
	// The offset of field 2 = height of field 0 + height of field 1.
	off2 := sheetKeyLineOffset(f, 2, keyW)
	wantOff2 := sheetKeyEntryHeight(f, 0, keyW) + sheetKeyEntryHeight(f, 1, keyW)
	if off2 != wantOff2 {
		t.Fatalf("offset field 2 = %d, want %d", off2, wantOff2)
	}
	// Total equals the sum of all entry heights.
	total := sheetKeyLineCount(f, keyW)
	sum := sheetKeyEntryHeight(f, 0, keyW) + sheetKeyEntryHeight(f, 1, keyW) + sheetKeyEntryHeight(f, 2, keyW)
	if total != sum {
		t.Fatalf("key line count = %d, want %d", total, sum)
	}
	// Offset past the last field equals the total.
	if sheetKeyLineOffset(f, f.NumCols(), keyW) != total {
		t.Fatalf("offset past last field != total")
	}
}

func TestSheetValueLineCount(t *testing.T) {
	f := sheetTestFrame()
	const valW = 10
	// id "1" -> 1 line.
	if got := sheetValueLineCount(f, 0, 0, valW); got != 1 {
		t.Fatalf("value line count id = %d, want 1", got)
	}
	// comment wraps; line count = wrapped line count.
	commentVal := "a longer comment cell that should wrap across several value lines when the value column is narrow"
	want := strings.Count(ansi.Wrap(commentVal, valW, ""), "\n") + 1
	if got := sheetValueLineCount(f, 0, 2, valW); got != want {
		t.Fatalf("value line count comment = %d, want %d", got, want)
	}
	// empty notes -> a single space wraps to 1 line.
	if got := sheetValueLineCount(f, 0, 3, valW); got != 1 {
		t.Fatalf("value line count notes = %d, want 1", got)
	}
}

func TestRenderSheetKeyListAndSeparator(t *testing.T) {
	f := sheetTestFrame()
	th := theme.Default()
	out := renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "│") {
		t.Fatalf("separator missing:\n%s", plain)
	}
	if !strings.Contains(plain, "Key") || !strings.Contains(plain, "Value") {
		t.Fatalf("column header missing:\n%s", plain)
	}
	if !strings.Contains(plain, sheetCursorMark) {
		t.Fatalf("cursor marker missing on field 0:\n%s", plain)
	}
	// The cursor field key (id) should appear in the rendered output.
	if !strings.Contains(plain, "id") {
		t.Fatalf("cursor field key 'id' missing:\n%s", plain)
	}
}

func TestRenderSheetShowsOnlyCursorFieldValue(t *testing.T) {
	f := sheetTestFrame()
	th := theme.Default()
	// Cursor on field 2 (comment). The right pane must show the comment
	// value but NOT the name value.
	out := renderSheet(f, 0, 0, 40, 8, 2, false, nil, 0, 0, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "a longer comment cell") {
		t.Fatalf("cursor field value missing from right pane:\n%s", plain)
	}
	if strings.Contains(plain, "alice") {
		t.Fatalf("non-cursor field value 'alice' must not appear in master-detail right pane:\n%s", plain)
	}
}

func TestRenderSheetLongKeyWraps(t *testing.T) {
	f := longKeyFrame()
	th := theme.Default()
	out := renderSheet(f, 0, 0, 50, 10, 1, false, nil, 0, 0, th)
	plain := ansi.Strip(out)
	// The wrapped key name should appear (at least the first fragment). The
	// keyW cap is 20, so the name is truncated to fit on the first line.
	if !strings.Contains(plain, "this_is_a_very_lon") {
		t.Fatalf("wrapped key name fragment missing:\n%s", plain)
	}
	// The cursor marker should be on field 1's first wrapped line.
	if !strings.Contains(plain, sheetCursorMark) {
		t.Fatalf("cursor marker missing:\n%s", plain)
	}
}

func TestRenderSheetEdgeFollow(t *testing.T) {
	f := sheetTestFrame()
	th := theme.Default()
	// A small height so only a few body lines are visible; cursor the last
	// field (notes). Simulate the edge-follow offset that app.go would have
	// computed (renderSheet itself only renders from off), then confirm the
	// cursor block lands inside the visible window.
	const inner, height = 40, 5
	lastField := f.NumCols() - 1
	keyW, _ := sheetTableGeometry(f, inner)
	visible := height - 2
	curStart := sheetKeyLineOffset(f, lastField, keyW)
	curEnd := curStart + sheetKeyEntryHeight(f, lastField, keyW) - 1
	off := curEnd - visible + 1
	if off > curStart {
		off = curStart
	}
	maxOff := max(0, sheetKeyLineCount(f, keyW)-visible)
	off = clamp(off, 0, maxOff)

	out := renderSheet(f, 0, off, inner, height, lastField, false, nil, 0, 0, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "notes") {
		t.Fatalf("cursor field 'notes' should be visible under edge-follow:\n%s", plain)
	}
	if !strings.Contains(plain, sheetCursorMark) {
		t.Fatalf("cursor marker missing:\n%s", plain)
	}
}

func TestRenderSheetEditMode(t *testing.T) {
	f := sheetTestFrame()
	th := theme.Default()
	// Edit the id field; the edit runes should appear in the rendered output.
	editRunes := []rune("99")
	out := renderSheet(f, 0, 0, 40, 6, 0, true, editRunes, 2, 0, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "99") {
		t.Fatalf("edit runes missing from right pane:\n%s", plain)
	}
	// While editing the right pane shows the edit runes; the static value
	// "1" must not appear as the value of the cursored field. The row header
	// line ("row 1 of 1") does contain "1" so only inspect the value column
	// line (the cursor field's value row).
	lines := strings.Split(plain, "\n")
	found99 := false
	for _, l := range lines {
		// The value row is the one starting with the cursor marker.
		if strings.Contains(l, sheetCursorMark) && strings.Contains(l, "99") {
			found99 = true
			if strings.Contains(l, " 1 ") {
				t.Fatalf("static value '1' should be replaced by edit runes on cursor row:\n%s", plain)
			}
		}
	}
	if !found99 {
		t.Fatalf("edit runes not found on the cursor row:\n%s", plain)
	}
}

func TestActSheetDownScrollsOnlyAtEdge(t *testing.T) {
	// Build an app with many short fields so several fit in a small viewport
	// before the cursor reaches the bottom edge. Long key names wrap so the
	// left list has more lines than fields.
	f := data.New("a", "b", "c", "d", "e", "g", "h", "i", "j", "k", "l", "m")
	row := make([]any, f.NumCols())
	for i := range row {
		row[i] = "x"
	}
	f.AppendRow(row)

	a := New(Options{
		Frames:    []reader.NamedFrame{{Name: "t", Frame: f}},
		ThemeName: "catppuccin-mocha",
	})
	a.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	a.Update(key("enter"))
	p := a.pane()
	if p.Mode != ModeSheet {
		t.Fatal("enter should open the sheet")
	}

	// Move down a few fields; SheetOff should stay 0 while the cursor entry
	// still fits within the visible window.
	for i := 0; i < 3; i++ {
		a.Update(key("j"))
	}
	keyW, _ := sheetTableGeometry(f, 40)
	visible := 8 - 2
	if p.SheetOff == 0 {
		curStart := sheetKeyLineOffset(f, p.SheetField, keyW)
		curEnd := curStart + sheetKeyEntryHeight(f, p.SheetField, keyW) - 1
		if curEnd >= visible {
			t.Fatalf("SheetOff=0 but cursor block [%d,%d] is past visible %d", curStart, curEnd, visible)
		}
	}

	// Now move down many times to force the cursor far past the bottom edge.
	for i := 0; i < f.NumCols(); i++ {
		a.Update(key("j"))
	}
	// The offset must have advanced and the cursor field must remain visible.
	curStart := sheetKeyLineOffset(f, p.SheetField, keyW)
	curEnd := curStart + sheetKeyEntryHeight(f, p.SheetField, keyW) - 1
	if curStart < p.SheetOff || curEnd >= p.SheetOff+visible {
		t.Fatalf("cursor block [%d,%d] not within window [%d,%d)", curStart, curEnd, p.SheetOff, p.SheetOff+visible)
	}
}

// stubConfirmSaveOverlay is a trivial overlay registered as the "confirmsave"
// factory so the ActEdit -> ctrl+s path can be exercised without depending on
// the real confirm popup (built by a concurrent agent).
type stubConfirmSaveOverlay struct{}

func (stubConfirmSaveOverlay) Update(tea.Msg) (Overlay, tea.Cmd) { return stubConfirmSaveOverlay{}, nil }
func (stubConfirmSaveOverlay) View(int, int, *theme.Theme) string { return "" }

func TestActEditTogglesAndCtrlSCommits(t *testing.T) {
	Factories["confirmsave"] = func(ctx AppContext, arg string) (Overlay, error) {
		return stubConfirmSaveOverlay{}, nil
	}
	defer delete(Factories, "confirmsave")

	a := newTestApp(t)
	a.Update(key("enter"))
	p := a.pane()
	if p.Mode != ModeSheet {
		t.Fatal("enter should open the sheet")
	}

	// e enters inline edit; SheetEdit is prefilled with the current value.
	a.Update(key("e"))
	if !p.SheetEditing {
		t.Fatal("e should toggle SheetEditing on")
	}
	if len(p.SheetEdit) == 0 {
		t.Fatal("SheetEdit should be prefilled with the current field value")
	}
	if p.SheetEditCur != len(p.SheetEdit) {
		t.Fatalf("SheetEditCur = %d, want %d", p.SheetEditCur, len(p.SheetEdit))
	}

	// While editing, j must NOT move the field.
	prevField := p.SheetField
	a.Update(key("j"))
	if p.SheetField != prevField {
		t.Fatalf("j while editing moved the field: %d -> %d", prevField, p.SheetField)
	}

	// Typing appends to the edit buffer.
	a.Update(key("Z"))
	if string(p.SheetEdit) == "" || p.SheetEdit[len(p.SheetEdit)-1] != 'Z' {
		t.Fatalf("typed rune not appended: %q", string(p.SheetEdit))
	}

	// ctrl+s sets pendingEdit and dispatches confirmsave (overlay pushed by
	// the stub factory).
	a.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if a.pendingEdit == nil {
		t.Fatal("ctrl+s should set pendingEdit")
	}
	if a.pendingEdit.NewValue != string(p.SheetEdit) {
		t.Fatalf("pendingEdit.NewValue = %q, want %q", a.pendingEdit.NewValue, string(p.SheetEdit))
	}
	if len(a.overlays) == 0 {
		t.Fatal("confirmsave overlay should be pushed")
	}
}

func TestActEditEscCancels(t *testing.T) {
	a := newTestApp(t)
	a.Update(key("enter"))
	p := a.pane()
	a.Update(key("e"))
	if !p.SheetEditing {
		t.Fatal("e should toggle SheetEditing on")
	}
	a.Update(key("esc"))
	if p.SheetEditing {
		t.Fatal("esc should cancel inline edit")
	}
	if p.SheetEdit != nil {
		t.Fatal("esc should clear the edit buffer")
	}
}
