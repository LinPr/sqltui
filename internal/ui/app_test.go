package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/reader"
)

func key(s string) tea.KeyPressMsg {
	r := []rune(s)
	if len(r) == 1 {
		return tea.KeyPressMsg{Code: r[0], Text: s}
	}
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{}
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	a := New(Options{
		Frames: []reader.NamedFrame{
			{Name: "one", Frame: testFrame(20)},
			{Name: "two", Frame: testFrame(5)},
		},
		ThemeName:      "catppuccin-mocha",
		ShowBorders:    true,
		ShowRowNumbers: true,
	})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return a
}

func TestAppNavigationAndView(t *testing.T) {
	a := newTestApp(t)

	a.Update(key("j"))
	a.Update(key("j"))
	if a.CurrentRow() != 2 {
		t.Fatalf("row = %d, want 2", a.CurrentRow())
	}
	a.Update(key("k"))
	if a.CurrentRow() != 1 {
		t.Fatalf("row = %d, want 1", a.CurrentRow())
	}

	v := a.View()
	if !v.AltScreen {
		t.Fatal("view must use the alt screen")
	}
	if !strings.Contains(v.Content, "one") {
		t.Fatal("view should contain the tab title in the status bar")
	}
}

func TestAppTabSwitchAndApplyFrame(t *testing.T) {
	a := newTestApp(t)

	a.Update(key("L"))
	if a.ActiveTab() != 1 || a.BaseCrumb() != "two" {
		t.Fatalf("active tab = %d (%s), want 1 (two)", a.ActiveTab(), a.BaseCrumb())
	}
	a.Update(key("H"))
	if a.ActiveTab() != 0 {
		t.Fatalf("active tab = %d, want 0", a.ActiveTab())
	}

	a.Update(ApplyFrameMsg{Frame: testFrame(3), Crumb: "filter"})
	if got := a.Crumbs(); len(got) != 2 || got[1] != "filter" {
		t.Fatalf("crumbs = %v", got)
	}

	a.Update(ApplyFrameMsg{Frame: testFrame(2), Crumb: "query", NewTab: true, TabTitle: "result"})
	if len(a.Tabs()) != 3 || a.BaseCrumb() != "result" {
		t.Fatalf("tabs = %v, active title = %s", a.Tabs(), a.BaseCrumb())
	}
}

func TestAppPopSemantics(t *testing.T) {
	a := newTestApp(t)
	a.Update(ApplyFrameMsg{Frame: testFrame(3), Crumb: "filter"})

	// q pops the derived frame
	a.Update(key("q"))
	if got := a.Crumbs(); len(got) != 1 {
		t.Fatalf("crumbs after pop = %v", got)
	}
	// q on base closes the tab
	a.Update(key("q"))
	if len(a.Tabs()) != 1 {
		t.Fatalf("tabs = %d, want 1", len(a.Tabs()))
	}
	// q on last base quits
	_, cmd := a.Update(key("q"))
	if cmd == nil {
		t.Fatal("expected quit command on closing the last tab")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected a quit message")
	}
}

func TestAppSearchFlow(t *testing.T) {
	a := newTestApp(t)

	a.Update(key("/"))
	if !a.search.Active() || !a.search.Fuzzy() {
		t.Fatal("/ should start fuzzy search")
	}
	a.Update(key("a"))
	a.Update(key("esc"))
	if a.search.Active() {
		t.Fatal("esc should cancel search")
	}
	if got := a.Crumbs(); len(got) != 1 {
		t.Fatalf("cancel must not touch the stack: %v", got)
	}

	a.Update(key("s"))
	if !a.search.Active() || a.search.Fuzzy() {
		t.Fatal("s should start exact search")
	}
	a.Update(key("a"))
	a.Update(key("enter"))
	if got := a.Crumbs(); len(got) != 2 || got[1] != "search" {
		t.Fatalf("commit should push with crumb search: %v", got)
	}
}

func TestAppSheetFlow(t *testing.T) {
	a := newTestApp(t)

	a.Update(key("enter"))
	if p := a.pane(); p.Mode != ModeSheet {
		t.Fatal("enter should open the sheet")
	}
	v := a.View()
	plain := ansi.Strip(v.Content)
	if !strings.Contains(plain, "row 1 of 20") {
		t.Fatal("sheet header missing")
	}
	if !strings.Contains(plain, "│") {
		t.Fatalf("sheet missing the key|value separator:\n%s", plain)
	}
	if !strings.Contains(plain, "Key") || !strings.Contains(plain, "Value") {
		t.Fatalf("sheet missing the key/value column header:\n%s", plain)
	}
	a.Update(key("q"))
	if p := a.pane(); p.Mode != ModeTable {
		t.Fatal("q should return to the table")
	}
}

func TestAppUnknownCommandShowsError(t *testing.T) {
	a := newTestApp(t)
	a.Update(RunCommandMsg{Name: "nonsense"})
	if len(a.overlays) != 1 {
		t.Fatalf("expected error overlay, got %d overlays", len(a.overlays))
	}
	v := a.View()
	if !strings.Contains(v.Content, "unknown command") {
		t.Fatal("error text missing from view")
	}
	// any key closes
	_, cmd := a.Update(key("x"))
	if cmd == nil {
		t.Fatal("error box should emit a close command")
	}
	a.Update(cmd())
	if len(a.overlays) != 0 {
		t.Fatal("overlay should be closed")
	}
}

func TestAppFactoryDispatch(t *testing.T) {
	called := ""
	Factories["testcmd"] = func(ctx AppContext, arg string) (Overlay, error) {
		called = arg
		return nil, nil // side-effect command
	}
	defer delete(Factories, "testcmd")

	a := newTestApp(t)
	a.Update(RunCommandMsg{Name: "testcmd", Arg: "hello"})
	if called != "hello" {
		t.Fatalf("factory not invoked with arg: %q", called)
	}
	if len(a.overlays) != 0 {
		t.Fatal("nil overlay must not be pushed")
	}

	Factories["testerr"] = func(ctx AppContext, arg string) (Overlay, error) {
		return nil, errors.New("boom")
	}
	defer delete(Factories, "testerr")
	a.Update(RunCommandMsg{Name: "testerr"})
	if len(a.overlays) != 1 {
		t.Fatal("factory error should surface as an error overlay")
	}
}

func TestAppToast(t *testing.T) {
	a := newTestApp(t)
	_, cmd := a.Update(ToastMsg{Text: "saved"})
	if cmd == nil {
		t.Fatal("toast should schedule an expiry tick")
	}
	v := a.View()
	if !strings.Contains(v.Content, "saved") {
		t.Fatal("toast text missing from view")
	}
	a.Update(toastExpiredMsg{id: a.toastID})
	if a.toastText != "" {
		t.Fatal("toast should clear on expiry")
	}
}

func TestAppJumpMessages(t *testing.T) {
	a := newTestApp(t)
	a.Update(JumpToRowMsg{Row: 15})
	if a.CurrentRow() != 15 {
		t.Fatalf("row = %d, want 15", a.CurrentRow())
	}
	a.Update(JumpToRowMsg{Row: 999})
	if a.CurrentRow() != 19 {
		t.Fatalf("row = %d, want clamp to 19", a.CurrentRow())
	}
	a.Update(JumpToTabMsg{Index: 1})
	if a.ActiveTab() != 1 {
		t.Fatalf("tab = %d, want 1", a.ActiveTab())
	}
	a.Update(CloseTabMsg{Index: 1})
	if len(a.Tabs()) != 1 || a.ActiveTab() != 0 {
		t.Fatalf("tabs = %d active = %d", len(a.Tabs()), a.ActiveTab())
	}
}

func TestCloseTabBeforeActiveFollowsPane(t *testing.T) {
	a := newTestApp(t)
	a.Update(ApplyFrameMsg{Frame: testFrame(2), Crumb: "query", NewTab: true, TabTitle: "three"})
	a.Update(ApplyFrameMsg{Frame: testFrame(2), Crumb: "query", NewTab: true, TabTitle: "four"})
	a.Update(JumpToTabMsg{Index: 2}) // "three" active

	a.Update(CloseTabMsg{Index: 0}) // close a tab before the active one
	if a.BaseCrumb() != "three" {
		t.Fatalf("active tab after closing an earlier tab = %q, want %q", a.BaseCrumb(), "three")
	}

	// Closing a tab after the active one leaves it alone too.
	a.Update(CloseTabMsg{Index: 2}) // "four"
	if a.BaseCrumb() != "three" {
		t.Fatalf("active tab after closing a later tab = %q, want %q", a.BaseCrumb(), "three")
	}
}

func TestApplyFrameRoutesToOriginatingPane(t *testing.T) {
	a := newTestApp(t)
	originID := a.ActivePaneID() // tab "one"
	a.Update(key("L"))           // switch to tab "two" while the "query" runs

	a.Update(ApplyFrameMsg{Frame: testFrame(3), Crumb: "filter", PaneID: originID})
	if got := a.tabs[0].Crumbs(); len(got) != 2 || got[1] != "filter" {
		t.Fatalf("origin pane crumbs = %v, want [one filter]", got)
	}
	if got := a.tabs[1].Crumbs(); len(got) != 1 {
		t.Fatalf("active pane received the frame: crumbs = %v", got)
	}

	// A vanished pane falls back to a new tab instead of hitting the active one.
	a.Update(CloseTabMsg{Index: 0})
	a.Update(ApplyFrameMsg{Frame: testFrame(3), Crumb: "filter", PaneID: originID})
	if len(a.Tabs()) != 2 {
		t.Fatalf("tabs = %d, want 2 (fallback new tab)", len(a.Tabs()))
	}
	if got := a.tabs[0].Crumbs(); len(got) != 1 {
		t.Fatalf("surviving pane corrupted: crumbs = %v", got)
	}
}

func TestApplyFrameRegisterAs(t *testing.T) {
	eng, err := query.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	a := New(Options{
		Frames: []reader.NamedFrame{{Name: "one", Frame: testFrame(3)}},
		Engine: eng,
	})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	a.Update(ApplyFrameMsg{Frame: testFrame(2), Crumb: "table", NewTab: true, TabTitle: "b", RegisterAs: "b"})
	f, err := eng.Query(`SELECT * FROM "b"`)
	if err != nil {
		t.Fatalf("imported tab not queryable by name: %v", err)
	}
	if f.NumRows() != 2 {
		t.Fatalf("rows = %d, want 2", f.NumRows())
	}
}

func TestCopyKeyDispatch(t *testing.T) {
	if got := actionFor("y", TableBindings); got != ActCopyCell {
		t.Fatalf("y resolves to %q, want %q", got, ActCopyCell)
	}
	if got := actionFor("Y", TableBindings); got != ActCopyRow {
		t.Fatalf("Y resolves to %q, want %q", got, ActCopyRow)
	}

	a := newTestApp(t)

	// y copies the cursored column's cell (column 0 initially).
	_, cmd := a.Update(key("y"))
	if cmd == nil {
		t.Fatal("y should emit a clipboard command")
	}
	if !strings.Contains(a.toastText, "copied cell id") {
		t.Fatalf("toast = %q, want cell copy confirmation for column id", a.toastText)
	}

	// The cursored column is used in compact mode too.
	a.Update(key("l")) // next column
	a.Update(key("y"))
	if !strings.Contains(a.toastText, "copied cell name") {
		t.Fatalf("toast = %q, want cell copy confirmation for column name", a.toastText)
	}

	// Y copies the whole row.
	a.Update(key("j"))
	_, cmd = a.Update(key("Y"))
	if cmd == nil {
		t.Fatal("Y should emit a clipboard command")
	}
	if !strings.Contains(a.toastText, "copied row 2") {
		t.Fatalf("toast = %q, want row copy confirmation", a.toastText)
	}
}

func TestAppStatusBarColumnAndModeTags(t *testing.T) {
	a := newTestApp(t)
	plain := ansi.Strip(a.View().Content)
	if !strings.Contains(plain, "[fit]") {
		t.Fatalf("compact mode tag missing:\n%s", plain)
	}
	if !strings.Contains(plain, "col: id") {
		t.Fatalf("column tag missing:\n%s", plain)
	}

	a.Update(key("w")) // manual expand
	a.Update(key("l")) // cursor to column 1
	plain = ansi.Strip(a.View().Content)
	if !strings.Contains(plain, "[wide]") {
		t.Fatalf("expanded mode tag missing:\n%s", plain)
	}
	if !strings.Contains(plain, "col: name") {
		t.Fatalf("column tag should follow the column cursor in expanded mode:\n%s", plain)
	}
}

func TestAppAutoExpandsWideFrame(t *testing.T) {
	a := New(Options{
		Frames: []reader.NamedFrame{{Name: "w", Frame: wideFrame(5)}},
	})
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	a.View() // first render triggers the auto pick
	if !a.pane().Table.Expanded() {
		t.Fatal("wide frame should start in expanded mode")
	}

	// A frame that fits stays compact.
	b := New(Options{
		Frames: []reader.NamedFrame{{Name: "n", Frame: testFrame(5)}},
	})
	b.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	b.View()
	if b.pane().Table.Expanded() {
		t.Fatal("narrow frame should start in compact mode")
	}
}

func TestKeymapRemapped(t *testing.T) {
	cases := []struct{ key, want string }{
		{"?", ActHelp}, // help, no longer exact search
		{"f1", ActHelp},
		{"s", ActExact},
		{"w", ActExpand}, // fit/wide toggle, no longer next column
		{"e", ""},        // unbound
		{"b", ""},        // unbound
		{"esc", ActEscBack},
		{"left", ActLeft},
		{"h", ActLeft},
		{"right", ActRight},
		{"l", ActRight},
		{"_", ActFirstCol},
		{"$", ActLastCol},
	}
	for _, c := range cases {
		if got := actionFor(c.key, TableBindings, GlobalBindings); got != c.want {
			t.Errorf("actionFor(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestHelpKeyDispatchesHelpCommand(t *testing.T) {
	called := false
	Factories["help"] = func(ctx AppContext, arg string) (Overlay, error) {
		called = true
		return nil, nil
	}
	defer delete(Factories, "help")

	a := newTestApp(t)
	a.Update(key("?"))
	if !called {
		t.Fatal("? should dispatch the help command")
	}
}

func TestModeToggleKey(t *testing.T) {
	a := newTestApp(t)
	a.View() // auto-pick fit for the narrow frame
	if a.pane().Table.Expanded() {
		t.Fatal("test frame should start in fit mode")
	}
	a.Update(key("w"))
	if !a.pane().Table.Expanded() {
		t.Fatal("w should switch to wide mode")
	}
	a.Update(key("w"))
	if a.pane().Table.Expanded() {
		t.Fatal("w should switch back to fit mode")
	}
}

func TestColumnCursorKeysBothModes(t *testing.T) {
	// Fit mode: h/l move the column cursor.
	a := newTestApp(t)
	a.View()
	a.Update(key("l"))
	if got := a.pane().Table.ColCursor(); got != 1 {
		t.Fatalf("colCursor after l = %d, want 1", got)
	}
	a.Update(key("h"))
	if got := a.pane().Table.ColCursor(); got != 0 {
		t.Fatalf("colCursor after h = %d, want 0", got)
	}

	// Wide mode: the viewport follows the cursor.
	b := New(Options{Frames: []reader.NamedFrame{{Name: "w", Frame: wideFrame(5)}}})
	b.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	b.View() // auto-picks wide
	tv := &b.pane().Table
	if !tv.Expanded() {
		t.Fatal("wide frame should render in wide mode")
	}
	for i := 0; i < 3; i++ {
		b.Update(key("l"))
	}
	if tv.ColCursor() != 3 {
		t.Fatalf("colCursor = %d, want 3", tv.ColCursor())
	}
	b.View() // render applies the snap
	if tv.colOff == 0 {
		t.Fatal("viewport did not follow the cursor to the right")
	}
	visible := false
	nat := naturalWidths(wideFrame(5), 0, 5, expandedColCap)
	cols, _ := visibleColumns(nat, tv.colOff, 50)
	for _, c := range cols {
		if c == tv.ColCursor() {
			visible = true
		}
	}
	if !visible {
		t.Fatalf("cursored column %d not in visible window %v", tv.ColCursor(), cols)
	}
	for i := 0; i < 3; i++ {
		b.Update(key("h"))
	}
	b.View()
	if tv.ColCursor() != 0 || tv.colOff != 0 {
		t.Fatalf("cursor/viewport after h = %d/%d, want 0/0", tv.ColCursor(), tv.colOff)
	}
}

func TestEscPopsFrameNeverClosesTab(t *testing.T) {
	a := newTestApp(t)
	a.Update(ApplyFrameMsg{Frame: testFrame(3), Crumb: "filter"})

	// esc pops the derived frame like q.
	a.Update(key("esc"))
	if got := a.Crumbs(); len(got) != 1 {
		t.Fatalf("crumbs after esc = %v, want base only", got)
	}
	// On the base frame in file mode esc does nothing: no tab close, no quit.
	_, cmd := a.Update(key("esc"))
	if cmd != nil {
		t.Fatal("esc on a file-mode base frame must be a no-op")
	}
	if len(a.Tabs()) != 2 {
		t.Fatalf("tabs = %d, esc must never close a tab", len(a.Tabs()))
	}
	if len(a.overlays) != 0 {
		t.Fatal("esc in file mode must not open overlays")
	}
}

func TestEscSheetReturnsToTable(t *testing.T) {
	a := newTestApp(t)
	a.Update(key("enter"))
	if a.pane().Mode != ModeSheet {
		t.Fatal("enter should open the sheet")
	}
	a.Update(key("esc"))
	if a.pane().Mode != ModeTable {
		t.Fatal("esc should return from the sheet to the table")
	}
	if len(a.Tabs()) != 2 {
		t.Fatal("esc from the sheet must not close tabs")
	}
}

// fakeKV is a minimal db.KVBackend for esc-hierarchy tests.
type fakeKV struct{}

func (fakeKV) Title() string                     { return "redis://h/0" }
func (fakeKV) Do([]string) (string, error)       { return "", nil }
func (fakeKV) ScanKeys(string) ([]string, error) { return nil, nil }
func (fakeKV) Value(string) (string, error)      { return "", nil }
func (fakeKV) Close() error                      { return nil }

func TestEscBaseFrameDbModeOpensBrowser(t *testing.T) {
	for name, opts := range map[string]Options{
		"sql": {Frames: []reader.NamedFrame{{Name: "t", Frame: testFrame(3)}}, Backend: fakeBackend{}},
		"kv":  {Frames: []reader.NamedFrame{{Name: "t", Frame: testFrame(3)}}, KV: fakeKV{}},
	} {
		called := false
		Factories["schema"] = func(ctx AppContext, arg string) (Overlay, error) {
			called = true
			return errBox{err: errors.New("stub")}, nil
		}
		a := New(opts)
		a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		a.Update(key("esc"))
		delete(Factories, "schema")
		if !called {
			t.Errorf("%s: esc on the base frame must dispatch the schema/browser command", name)
		}
		if len(a.overlays) != 1 {
			t.Errorf("%s: browser overlay not pushed (%d overlays)", name, len(a.overlays))
		}
		if len(a.Tabs()) != 1 {
			t.Errorf("%s: esc must not close the tab", name)
		}
	}
}

func TestEscEmptyWorkspaceDbModeOpensConnect(t *testing.T) {
	called := false
	Factories["connect"] = func(ctx AppContext, arg string) (Overlay, error) {
		called = true
		return errBox{err: errors.New("stub")}, nil
	}
	defer delete(Factories, "connect")

	a := New(Options{Backend: fakeBackend{}}) // no tabs
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := a.Update(key("esc"))
	if !called {
		t.Fatal("esc on an empty db workspace must dispatch connect")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, quit := msg.(tea.QuitMsg); quit {
				t.Fatal("esc must never quit")
			}
		}
	}
	if len(a.overlays) != 1 {
		t.Fatal("connection form overlay not pushed")
	}
}

func TestEscEmptyWorkspaceFileModeNoop(t *testing.T) {
	a := New(Options{}) // no tabs, no connection
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := a.Update(key("esc"))
	if cmd != nil {
		t.Fatal("esc on an empty file-mode workspace must be a no-op")
	}
	if len(a.overlays) != 0 {
		t.Fatal("no overlay should open in file mode")
	}
}

// fakeBackend is a minimal db.Backend for status bar tests.
type fakeBackend struct{}

func (fakeBackend) Kind() string                                        { return "mysql" }
func (fakeBackend) Title() string                                       { return "mysql://h/db" }
func (fakeBackend) Run(string) (db.Result, error)                       { return db.Result{}, nil }
func (fakeBackend) Namespaces() ([]string, error)                       { return nil, nil }
func (fakeBackend) Tables(string) ([]string, error)                     { return nil, nil }
func (fakeBackend) FetchTable(string, string, int) (*data.Frame, error) { return nil, nil }
func (fakeBackend) PrimaryKeys(string, string) ([]string, error)            { return nil, nil }
func (fakeBackend) ColumnsMeta(string, string) ([]db.ColumnMeta, error)    { return nil, nil }
func (fakeBackend) Close() error                                           { return nil }

func TestAppStatusBarConnectionTitle(t *testing.T) {
	a := New(Options{
		Frames:  []reader.NamedFrame{{Name: "one", Frame: testFrame(5)}},
		Backend: fakeBackend{},
	})
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	plain := ansi.Strip(a.View().Content)
	if !strings.Contains(plain, "mysql://h/db") {
		t.Fatalf("connection title missing from status bar:\n%s", plain)
	}
}

func TestEngineAccessorSyncsCurrentFrame(t *testing.T) {
	eng, err := query.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	a := New(Options{
		Frames: []reader.NamedFrame{{Name: "one", Frame: testFrame(3)}},
		Engine: eng,
	})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Tab switches / pushes no longer register eagerly; the "_" alias is
	// synced lazily when SQL is about to run (Engine()).
	a.Update(ApplyFrameMsg{Frame: testFrame(7), Crumb: "filter"})
	f, err := a.Engine().Query(`SELECT * FROM "_"`)
	if err != nil {
		t.Fatal(err)
	}
	if f.NumRows() != 7 {
		t.Fatalf(`"_" rows = %d, want 7 (current frame)`, f.NumRows())
	}
}
