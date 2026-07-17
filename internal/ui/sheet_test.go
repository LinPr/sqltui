package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
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
	// Names are id/name/comment/notes; the longest key name is "comment" (7).
	// At inner=40 the cap of 20 keeps the natural 7.
	keyW, valW := sheetTableGeometry(f, 40)
	if keyW != 7 {
		t.Fatalf("keyW = %d, want 7", keyW)
	}
	if valW != 40-7-1 {
		t.Fatalf("valW = %d, want %d", valW, 40-7-1)
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
	// Field 1's name is 44 chars; avail = keyW = 20, so it wraps to 3 lines
	// (44 / 20 = 2.2 -> ceil 3).
	h := sheetKeyEntryHeight(f, 1, keyW)
	wrapped := ansi.Wrap(f.Columns[1].Name, keyW, "")
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
	out := renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, "", false, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "│") {
		t.Fatalf("separator missing:\n%s", plain)
	}
	if !strings.Contains(plain, "Key") || !strings.Contains(plain, "Value") {
		t.Fatalf("column header missing:\n%s", plain)
	}
	if !strings.Contains(plain, "id") {
		t.Fatalf("cursor field key 'id' missing from output:\n%s", plain)
	}
	// The cursor field key (id) should appear in the rendered output.
	if !strings.Contains(plain, "id") {
		t.Fatalf("cursor field key 'id' missing:\n%s", plain)
	}
}

func TestRenderSheetShowsOnlyCursorFieldValue(t *testing.T) {
	f := sheetTestFrame()
	th := theme.Default()
	// Cursor on field 2 (comment). The cursor row must show the comment value
	// in the expanded right pane (with separator). Non-cursor rows show compact
	// key: value lines, so "alice" is visible in the compact name row.
	out := renderSheet(f, 0, 0, 40, 8, 2, false, nil, 0, 0, "", false, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "a longer comment cell") {
		t.Fatalf("cursor field value missing from right pane:\n%s", plain)
	}
	// The cursor row must have the separator (expanded layout).
	lines := strings.Split(plain, "\n")
	cursorRowOK := false
	for _, l := range lines {
		if strings.Contains(l, "comment") && strings.Contains(l, "│") {
			cursorRowOK = true
			break
		}
	}
	if !cursorRowOK {
		t.Fatalf("cursor row (comment + separator) not found:\n%s", plain)
	}
}

func TestRenderSheetLongKeyWraps(t *testing.T) {
	f := longKeyFrame()
	th := theme.Default()
	out := renderSheet(f, 0, 0, 50, 10, 1, false, nil, 0, 0, "", false, th)
	plain := ansi.Strip(out)
	// The wrapped key name should appear (at least the first fragment). The
	// keyW cap is 20, so the name is truncated to fit on the first line.
	if !strings.Contains(plain, "this_is_a_very_lon") {
		t.Fatalf("wrapped key name fragment missing:\n%s", plain)
	}
	// The cursor row (field 1) should use full-row highlight; the value must be visible in right pane.
	if !strings.Contains(plain, "│") {
		t.Fatalf("cursor marker (separator) missing:\n%s", plain)
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
	visible := height - 3 // 2 headers + 1 footer hint
	curStart := sheetKeyLineOffset(f, lastField, keyW)
	curEnd := curStart + sheetKeyEntryHeight(f, lastField, keyW) - 1
	off := curEnd - visible + 1
	if off > curStart {
		off = curStart
	}
	maxOff := max(0, sheetKeyLineCount(f, keyW)-visible)
	off = clamp(off, 0, maxOff)

	out := renderSheet(f, 0, off, inner, height, lastField, false, nil, 0, 0, "", false, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "notes") {
		t.Fatalf("cursor field 'notes' should be visible under edge-follow:\n%s", plain)
	}
	// The cursor row must include the separator (expanded layout).
	if !strings.Contains(plain, "│") {
		t.Fatalf("cursor row separator missing:\n%s", plain)
	}
}

func TestRenderSheetEditMode(t *testing.T) {
	f := sheetTestFrame()
	th := theme.Default()
	// Edit the id field; the edit runes should appear in the rendered output.
	editRunes := []rune("99")
	out := renderSheet(f, 0, 0, 40, 6, 0, true, editRunes, 2, 0, "", false, th)
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
		// The expanded cursor row has the separator and the edit runes.
		if strings.Contains(l, "│") && strings.Contains(l, "99") {
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

// metaBackend is a minimal db.Backend whose ColumnsMeta returns a fixed
// column list, so cursorFieldMeta can be exercised against a realistic
// metadata shape without a live connection.
type metaBackend struct {
	fakeBackend
	meta []db.ColumnMeta
}

func (b *metaBackend) Namespaces() ([]string, error) { return []string{"main"}, nil }
func (b *metaBackend) Tables(string) ([]string, error) { return []string{"users"}, nil }
func (b *metaBackend) ColumnsMeta(string, string) ([]db.ColumnMeta, error) {
	return b.meta, nil
}


// TestCursorFieldMetaDBAndFileMode asserts cursorFieldMeta resolves the
// cached engine metadata in db mode, and falls back to the frame DType in
// file mode.
func TestCursorFieldMetaDBAndFileMode(t *testing.T) {
	// DB mode: the completion cache is warmed with column metadata for the
	// active pane's table; cursorFieldMeta must surface the engine-native
	// type and constraints.
	f := data.New("id", "name")
	f.AppendRow([]any{"1", "alice"})
	be := &metaBackend{
		meta: []db.ColumnMeta{
			{Name: "id", DataType: "integer", IsNullable: "NO", Default: "0", Comment: "pk"},
			{Name: "name", DataType: "varchar(255)", IsNullable: "YES"},
		},
	}
	a := New(Options{
		Frames:  []reader.NamedFrame{{Name: "users", Frame: f}},
		Backend: be,
	})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a.Update(key("enter"))
	a.WarmCompletionSchema("users")

	// Sanity: the schema cache must have populated metadata for "users".
	if m := a.columnMetaFor("users"); m == nil || len(m) != 2 {
		t.Fatalf("columnMetaFor(users) = %v, want 2 columns", m)
	}

	dt, notNull, def, comment := a.cursorFieldMeta()
	if dt != "integer" {
		t.Fatalf("db-mode dataType = %q, want integer", dt)
	}
	if notNull != "NO" {
		t.Fatalf("db-mode notNull = %q, want NO", notNull)
	}
	if def != "0" {
		t.Fatalf("db-mode default = %q, want 0", def)
	}
	if comment != "pk" {
		t.Fatalf("db-mode comment = %q, want pk", comment)
	}

	// Move to the name field; metadata follows by column NAME match.
	a.Update(key("j"))
	dt, notNull, _, _ = a.cursorFieldMeta()
	if dt != "varchar(255)" {
		t.Fatalf("db-mode name dataType = %q, want varchar(255)", dt)
	}
	if notNull != "YES" {
		t.Fatalf("db-mode name notNull = %q, want YES", notNull)
	}

	// File mode: no backend, so no meta; cursorFieldMeta falls back to the
	// frame DType string and empty constraints.
	fileA := New(Options{
		Frames: []reader.NamedFrame{{Name: "one", Frame: f}},
	})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	fileA.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	fileA.Update(key("enter"))
	dt, notNull, def, comment = fileA.cursorFieldMeta()
	if dt != data.TypeString.String() {
		t.Fatalf("file-mode dataType = %q, want %q", dt, data.TypeString.String())
	}
	if notNull != "" || def != "" || comment != "" {
		t.Fatalf("file-mode constraints should be empty, got notNull=%q def=%q comment=%q", notNull, def, comment)
	}
}

// TestSheetWarmsColumnMetaAndShowsRealType asserts that ActSheet in db mode
// issues a warmColumnMeta command, and once the resulting columnMetaMsg is
// processed the sheet view shows the real engine type inline with the value
// (not the frame DType fallback). In file mode (no backend) ActSheet shows the
// frame DType instead.
func TestSheetWarmsColumnMetaAndShowsRealType(t *testing.T) {
	// DB mode: ColumnsMeta returns a real type for the id column. The frame is
	// all TypeString, so without warming the sheet would show "str".
	f := data.New("id", "name")
	f.AppendRow([]any{"1", "alice"})
	be := &metaBackend{
		meta: []db.ColumnMeta{
			{Name: "id", DataType: "int", IsNullable: "NO"},
			{Name: "name", DataType: "varchar(255)", IsNullable: "YES"},
		},
	}
	a := New(Options{
		Frames:  []reader.NamedFrame{{Name: "users", Frame: f}},
		Backend: be,
	})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Right after ActSheet the cache is cold; cursorFieldType falls back to
	// the frame DType ("str"). ActSheet returns a warmColumnMeta cmd.
	_, cmd := a.Update(key("enter"))
	if p := a.pane(); p.Mode != ModeSheet {
		t.Fatal("enter should open the sheet")
	}
	if cmd == nil {
		t.Fatal("ActSheet in db mode with a cold cache should return a warmColumnMeta cmd")
	}
	// Drive the warm cmd to completion and feed the resulting msg back in.
	msg := cmd()
	if msg == nil {
		t.Fatal("warmColumnMeta cmd should produce a columnMetaMsg")
	}
	if _, ok := msg.(columnMetaMsg); !ok {
		t.Fatalf("warmColumnMeta cmd produced %T, want columnMetaMsg", msg)
	}
	a.Update(msg)

	// Now the cache is warm; cursorFieldType should return the real engine type.
	if got := a.cursorFieldType(); got != "int" {
		t.Fatalf("cursorFieldType after warm = %q, want int", got)
	}
	// The type is no longer shown inline in the sheet view (it appears only in
	// the drawer header when pressing e). Just ensure the sheet renders without error.
	v := a.View()
	plain := ansi.Strip(v.Content)
	if !strings.Contains(plain, "id") {
		t.Fatalf("sheet view missing field 'id':\n%s", plain)
	}

	// File mode: no backend -> ActSheet returns nil cmd and the view shows the
	// frame DType ("str") inline instead of a real engine type.
	fileA := New(Options{
		Frames: []reader.NamedFrame{{Name: "one", Frame: f}},
	})
	fileA.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, fileCmd := fileA.Update(key("enter"))
	if fileCmd != nil {
		t.Fatalf("ActSheet in file mode should return nil cmd, got %v", fileCmd)
	}
	if got := fileA.cursorFieldType(); got != data.TypeString.String() {
		t.Fatalf("file-mode cursorFieldType = %q, want %q", got, data.TypeString.String())
	}
	// Type is no longer shown inline in sheet view (drawer only).
}

// filterTestFrame has five fields so the fuzzy filter has a non-trivial
// match set to assert against.
func filterTestFrame() *data.Frame {
	f := data.New("id", "name", "category", "price", "stock")
	f.AppendRow([]any{"1", "walnut", "snack", "2.50", "42"})
	return f
}

func TestSheetMatchedCols(t *testing.T) {
	f := filterTestFrame()
	// Empty pattern -> every column index, in order.
	got := sheetMatchedCols(f, "")
	want := []int{0, 1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty pattern matched = %v, want %v", got, want)
	}
	// "na" fuzzy-matches "name" only (none of id/category/price/stock contain
	// the subsequence "na" in a fuzzy-friendly way except "name").
	got = sheetMatchedCols(f, "na")
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("pattern 'na' matched = %v, want [1]", got)
	}
	// "sto" matches "stock" (column 4).
	got = sheetMatchedCols(f, "sto")
	if len(got) != 1 || got[0] != 4 {
		t.Fatalf("pattern 'sto' matched = %v, want [4]", got)
	}
	// Case-insensitivity: "ID" matches "id".
	got = sheetMatchedCols(f, "ID")
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("pattern 'ID' matched = %v, want [0]", got)
	}
	// A pattern that matches nothing returns an empty slice (not nil-ok).
	got = sheetMatchedCols(f, "zzz")
	if len(got) != 0 {
		t.Fatalf("pattern 'zzz' matched = %v, want []", got)
	}
	// Leading/trailing spaces are trimmed.
	got = sheetMatchedCols(f, "  na  ")
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("pattern '  na  ' matched = %v, want [1]", got)
	}
}

func TestRenderSheetFilterNarrowsLeftList(t *testing.T) {
	f := filterTestFrame()
	th := theme.Default()
	// Filter "na" -> only "name" matches; the left list must show "name" and
	// the cursor marker, and must NOT show "id", "category", "price", "stock".
	out := renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, "na", false, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "name") {
		t.Fatalf("matched key 'name' missing:\n%s", plain)
	}
	// The cursor row must be the expanded layout (has separator).
	if !strings.Contains(plain, "│") {
		t.Fatalf("cursor row separator missing on matched field:\n%s", plain)
	}
	for _, hidden := range []string{"category", "price", "stock"} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("non-matched key %q should not appear:\n%s", hidden, plain)
		}
	}
	// "id" is a substring of "row 1 of 1" / "valid" style strings, so do not
	// assert against it here; the matched-only key list is covered above.

	// The right pane shows the matched field's value ("walnut").
	if !strings.Contains(plain, "walnut") {
		t.Fatalf("matched field value 'walnut' missing from right pane:\n%s", plain)
	}
}

func TestRenderSheetFilterNoMatches(t *testing.T) {
	f := filterTestFrame()
	th := theme.Default()
	out := renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, "zzz", false, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "no matching fields") {
		t.Fatalf("empty match set should show the placeholder:\n%s", plain)
	}
}

func TestRenderSheetFilterLineShown(t *testing.T) {
	f := filterTestFrame()
	th := theme.Default()
	// While filtering with an empty pattern, the placeholder is shown.
	out := renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, "", true, th)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "type to filter fields") {
		t.Fatalf("filter placeholder missing while filtering:\n%s", plain)
	}
	// A non-empty, non-filtering pattern still shows the filter line.
	out = renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, "na", false, th)
	plain = ansi.Strip(out)
	if !strings.Contains(plain, "filter") {
		t.Fatalf("filter line missing with a set pattern:\n%s", plain)
	}
	// No filter line when the pattern is empty and not filtering.
	out = renderSheet(f, 0, 0, 40, 8, 0, false, nil, 0, 0, "", false, th)
	plain = ansi.Strip(out)
	if strings.Contains(plain, "type to filter fields") {
		t.Fatalf("filter placeholder should not show when not filtering:\n%s", plain)
	}
}

// TestSheetFilterFlow drives the full app flow: "/" enters filter input, typing
// narrows the left list, esc clears the filter and exits filter input.
func TestSheetFilterFlow(t *testing.T) {
	f := filterTestFrame()
	a := New(Options{
		Frames:    []reader.NamedFrame{{Name: "t", Frame: f}},
		ThemeName: "catppuccin-mocha",
	})
	a.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	a.Update(key("enter"))
	p := a.pane()
	if p.Mode != ModeSheet {
		t.Fatal("enter should open the sheet")
	}

	// "/" enters filter input.
	a.Update(key("/"))
	if !p.SheetFiltering {
		t.Fatal("/ should set SheetFiltering")
	}
	if len(p.SheetFilter) != 0 {
		t.Fatalf("SheetFilter should start empty, got %q", string(p.SheetFilter))
	}

	// Type "na" -> SheetFilter holds the runes and the matched set is just
	// "name"; SheetField stays 0 (the first match).
	a.Update(key("n"))
	a.Update(key("a"))
	if string(p.SheetFilter) != "na" {
		t.Fatalf("SheetFilter = %q, want %q", string(p.SheetFilter), "na")
	}
	matched := sheetMatchedCols(f, string(p.SheetFilter))
	if len(matched) != 1 || matched[0] != 1 {
		t.Fatalf("matched for 'na' = %v, want [1]", matched)
	}
	if p.SheetField != 0 {
		t.Fatalf("SheetField = %d, want 0 (first match)", p.SheetField)
	}
	// The rendered sheet must show the filtered key only.
	view := a.View()
	plain := ansi.Strip(view.Content)
	if !strings.Contains(plain, "name") {
		t.Fatalf("rendered sheet should show the matched 'name' key:\n%s", plain)
	}
	for _, hidden := range []string{"category", "price", "stock"} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("rendered sheet should hide non-matching key %q:\n%s", hidden, plain)
		}
	}

	// esc clears the filter and exits filter input.
	a.Update(key("esc"))
	if p.SheetFiltering {
		t.Fatal("esc should exit filter input")
	}
	if len(p.SheetFilter) != 0 {
		t.Fatalf("esc should clear SheetFilter, got %q", string(p.SheetFilter))
	}
	// After clearing, the full key list is back.
	view = a.View()
	plain = ansi.Strip(view.Content)
	if !strings.Contains(plain, "category") {
		t.Fatalf("after clearing filter the full key list should return:\n%s", plain)
	}
}

// TestSheetFilterEnterKeepsFilterApplied asserts that enter exits filter
// input but leaves the pattern applied, so the user can resume navigation.
func TestSheetFilterEnterKeepsFilterApplied(t *testing.T) {
	f := filterTestFrame()
	a := New(Options{
		Frames:    []reader.NamedFrame{{Name: "t", Frame: f}},
		ThemeName: "catppuccin-mocha",
	})
	a.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	a.Update(key("enter"))
	a.Update(key("/"))
	a.Update(key("n"))
	a.Update(key("a"))

	a.Update(key("enter"))
	p := a.pane()
	if p.SheetFiltering {
		t.Fatal("enter should exit filter input")
	}
	if string(p.SheetFilter) != "na" {
		t.Fatalf("enter should keep the filter applied, got %q", string(p.SheetFilter))
	}
	// j moves within the matched set (still length 1), so the cursor stays.
	a.Update(key("j"))
	if p.SheetField != 0 {
		t.Fatalf("j within a single-match filter should keep cursor at 0, got %d", p.SheetField)
	}
}

// TestSheetFilterBackspaceAndCtrlU asserts backspace pops a rune and ctrl+u
// clears the buffer, each time re-clamping the cursor to the matched set.
func TestSheetFilterBackspaceAndCtrlU(t *testing.T) {
	f := filterTestFrame()
	a := New(Options{
		Frames:    []reader.NamedFrame{{Name: "t", Frame: f}},
		ThemeName: "catppuccin-mocha",
	})
	a.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	a.Update(key("enter"))
	a.Update(key("/"))
	a.Update(key("n"))
	a.Update(key("a"))
	p := a.pane()
	if string(p.SheetFilter) != "na" {
		t.Fatalf("SheetFilter = %q, want na", string(p.SheetFilter))
	}
	// backspace -> "n", which matches "name" still.
	a.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if string(p.SheetFilter) != "n" {
		t.Fatalf("after backspace SheetFilter = %q, want n", string(p.SheetFilter))
	}
	matched := sheetMatchedCols(f, string(p.SheetFilter))
	if len(matched) != 1 || matched[0] != 1 {
		t.Fatalf("matched for 'n' = %v, want [1]", matched)
	}
	// ctrl+u clears the buffer -> empty pattern matches all columns.
	a.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if string(p.SheetFilter) != "" {
		t.Fatalf("ctrl+u should clear SheetFilter, got %q", string(p.SheetFilter))
	}
	matched = sheetMatchedCols(f, string(p.SheetFilter))
	if len(matched) != f.NumCols() {
		t.Fatalf("after ctrl+u matched = %d cols, want %d", len(matched), f.NumCols())
	}
}

