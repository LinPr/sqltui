package popup

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// --- shared test fakes (qx-prefixed; other feature tests live in this package) ---

type qxCtx struct {
	engine  *query.Engine
	backend db.Backend
	cols    []string
	tables  []string
	base    string
}

func (c *qxCtx) CurrentFrame() *data.Frame { return nil }
func (c *qxCtx) CurrentRow() int           { return 0 }
func (c *qxCtx) SheetFieldCursor() int     { return 0 }
func (c *qxCtx) CurrentTableNamespace() string { return "" }
func (c *qxCtx) BaseCrumb() string         { return c.base }
func (c *qxCtx) Crumbs() []string          { return nil }
func (c *qxCtx) ColumnNames() []string     { return c.cols }
func (c *qxCtx) Engine() *query.Engine     { return c.engine }
func (c *qxCtx) TableNames() []string      { return c.tables }
func (c *qxCtx) Backend() db.Backend       { return c.backend }
func (c *qxCtx) KV() db.KVBackend          { return nil }
func (c *qxCtx) Theme() *theme.Theme       { return nil }
func (c *qxCtx) ThemeName() string         { return "" }
func (c *qxCtx) ShowBorders() bool         { return false }
func (c *qxCtx) ShowRowNumbers() bool      { return false }
func (c *qxCtx) Tabs() []ui.TabInfo        { return nil }
func (c *qxCtx) ActiveTab() int            { return 0 }
func (c *qxCtx) ActivePaneID() int         { return 0 }
func (c *qxCtx) PendingEdit() *ui.PendingEdit   { return nil }
func (c *qxCtx) PendingDelete() *ui.PendingDelete { return nil }

type qxFakeBackend struct {
	last string
	res  db.Result
	err  error
}

func (b *qxFakeBackend) Kind() string  { return "test" }
func (b *qxFakeBackend) Title() string { return "test" }
func (b *qxFakeBackend) Run(stmt string) (db.Result, error) {
	b.last = stmt
	return b.res, b.err
}
func (b *qxFakeBackend) Namespaces() ([]string, error)   { return nil, nil }
func (b *qxFakeBackend) Tables(string) ([]string, error) { return nil, nil }
func (b *qxFakeBackend) FetchTable(string, string, int) (*data.Frame, error) {
	return nil, nil
}
func (b *qxFakeBackend) PrimaryKeys(string, string) ([]string, error)                { return nil, nil }
func (b *qxFakeBackend) ColumnsMeta(string, string) ([]db.ColumnMeta, error)         { return nil, nil }
func (b *qxFakeBackend) ColumnIndexTypes(string, string) (map[string]string, error)  { return nil, nil }
func (b *qxFakeBackend) Close() error                                                { return nil }

// qxTestFrame builds a two-column frame: name (str) and age (i64).
func qxTestFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "name", Type: data.TypeString, Cells: []any{"ann", "bob", "cyd"}},
		{Name: "age", Type: data.TypeInt, Cells: []any{int64(1), int64(2), int64(3)}},
	}}
}

// qxFileCtx returns a file-mode context: engine with the test frame
// registered as "_", no backend.
func qxFileCtx(t *testing.T) *qxCtx {
	t.Helper()
	eng, err := query.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	if err := eng.Register("_", qxTestFrame()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return &qxCtx{
		engine: eng,
		cols:   []string{"name", "age"},
		tables: []string{"_"},
		base:   "people",
	}
}

// qxKey builds a KeyPressMsg from a key name or a single printable string.
func qxKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "delete":
		return tea.KeyPressMsg{Code: tea.KeyDelete}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+w":
		return tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

// qxSend feeds keys (by qxKey name) to an overlay and returns the final
// overlay and the last command.
func qxSend(ov ui.Overlay, keys ...string) (ui.Overlay, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		ov, cmd = ov.Update(qxKey(k))
	}
	return ov, cmd
}

// qxTypeText feeds a string one printable key at a time.
func qxTypeText(ov ui.Overlay, s string) (ui.Overlay, tea.Cmd) {
	var cmd tea.Cmd
	for _, r := range s {
		ov, cmd = ov.Update(qxKey(string(r)))
	}
	return ov, cmd
}

// qxDrain runs a command tree (expanding batches) and collects all messages.
func qxDrain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, qxDrain(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func qxFindMsg[T tea.Msg](msgs []tea.Msg) (T, bool) {
	for _, m := range msgs {
		if v, ok := m.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func qxTestTheme() *theme.Theme {
	return theme.New(theme.Palette{
		Name: "test", Bg: "#000000", Fg: "#ffffff", BgSoft: "#111111",
		FgDim: "#888888", Header: "#ffffff", Accent: "#5555ff",
		AccentFg: "#000000", Highlight: "#ff00ff", Error: "#ff0000",
		Warning: "#ffff00", Success: "#00ff00",
		Series: []string{"#111111", "#222222", "#333333", "#444444"},
	})
}

// --- tests --------------------------------------------------------------------

func TestQxQuoteIdent(t *testing.T) {
	if got := qxQuoteIdent("_"); got != `"_"` {
		t.Errorf(`qxQuoteIdent("_") = %s`, got)
	}
	if got := qxQuoteIdent(`we"ird`); got != `"we""ird"` {
		t.Errorf("qxQuoteIdent embedded quote = %s", got)
	}
}

func TestQxTargetAndBuilders(t *testing.T) {
	file := qxFileCtx(t)
	if got := qxTarget(file); got != `"_"` {
		t.Errorf("file-mode target = %s, want quoted _", got)
	}
	if got := qxSelectSQL(file, "name, age"); got != `SELECT name, age FROM "_"` {
		t.Errorf("select sql = %s", got)
	}
	if got := qxFilterSQL(file, "age > 1"); got != `SELECT * FROM "_" WHERE age > 1` {
		t.Errorf("filter sql = %s", got)
	}
	if got := qxOrderSQL(file, "age DESC"); got != `SELECT * FROM "_" ORDER BY age DESC` {
		t.Errorf("order sql = %s", got)
	}

	live := &qxCtx{backend: &qxFakeBackend{}, base: "users"}
	if got := qxFilterSQL(live, "id = 1"); got != `SELECT * FROM "users" WHERE id = 1` {
		t.Errorf("live-mode filter sql = %s", got)
	}
}

func TestQxShortTitle(t *testing.T) {
	if got := qxShortTitle("select   *\nfrom _"); got != "select * from _" {
		t.Errorf("short title = %q", got)
	}
	long := "SELECT aaaaaaaaaa, bbbbbbbbbb FROM t"
	if got := qxShortTitle(long); got != "SELECT aaaaaaaaaa, b" || len([]rune(got)) != 20 {
		t.Errorf("truncated title = %q (len %d)", got, len([]rune(got)))
	}
	if got := qxShortTitle("   "); got != "query" {
		t.Errorf("empty title = %q", got)
	}
}

func TestQxRunRouting(t *testing.T) {
	// Backend frame result takes precedence over the engine.
	be := &qxFakeBackend{res: db.Result{Frame: qxTestFrame()}}
	ctx := &qxCtx{backend: be, base: "users"}
	f, info, err := qxRun(ctx, "SELECT 1")
	if err != nil || f == nil || info != nil {
		t.Fatalf("backend frame: f=%v info=%v err=%v", f, info, err)
	}
	if be.last != "SELECT 1" {
		t.Errorf("backend saw %q", be.last)
	}

	// Backend exec result.
	be.res = db.Result{Exec: &db.ExecResult{RowsAffected: 7}}
	f, info, err = qxRun(ctx, "DELETE FROM t")
	if err != nil || f != nil || info == nil || info.rowsAffected != 7 {
		t.Fatalf("backend exec: f=%v info=%+v err=%v", f, info, err)
	}

	// Backend error.
	be.res, be.err = db.Result{}, errors.New("boom")
	if _, _, err := qxRun(ctx, "SELECT 1"); err == nil {
		t.Error("backend error not propagated")
	}

	// File mode routes to the engine.
	file := qxFileCtx(t)
	f, info, err = qxRun(file, `SELECT * FROM "_" WHERE age > 1`)
	if err != nil || info != nil {
		t.Fatalf("engine run: info=%v err=%v", info, err)
	}
	if f.NumRows() != 2 {
		t.Errorf("engine rows = %d, want 2", f.NumRows())
	}

	// Neither facility available.
	if _, _, err := qxRun(&qxCtx{}, "SELECT 1"); err == nil {
		t.Error("expected error with no engine and no backend")
	}
}

func TestQxRunCmdMessages(t *testing.T) {
	file := qxFileCtx(t)
	onFrame := func(f *data.Frame) tea.Msg {
		return ui.ApplyFrameMsg{Frame: f, Crumb: "x"}
	}

	msgs := qxDrain(qxRunCmd(file, `SELECT * FROM "_"`, onFrame))
	if _, ok := qxFindMsg[ui.ApplyFrameMsg](msgs); !ok {
		t.Errorf("frame result: got %v, want ApplyFrameMsg", msgs)
	}

	msgs = qxDrain(qxRunCmd(file, "SELECT nope FROM nowhere", onFrame))
	if _, ok := qxFindMsg[ui.ErrorMsg](msgs); !ok {
		t.Errorf("bad sql: got %v, want ErrorMsg", msgs)
	}

	be := &qxFakeBackend{res: db.Result{Exec: &db.ExecResult{RowsAffected: 3}}}
	msgs = qxDrain(qxRunCmd(&qxCtx{backend: be}, "UPDATE t SET x=1", onFrame))
	toast, ok := qxFindMsg[ui.ToastMsg](msgs)
	if !ok || toast.Text != "3 rows affected" {
		t.Errorf("exec result: got %v, want rows-affected toast", msgs)
	}
}

func TestQxInputEditing(t *testing.T) {
	in := qxNewInput("")
	feed := func(keys ...string) {
		for _, k := range keys {
			in.handle(qxKey(k))
		}
	}

	feed("a", "b", "c")
	if in.String() != "abc" || in.cursor != 3 {
		t.Fatalf("after typing: %q cursor %d", in.String(), in.cursor)
	}
	feed("left", "backspace")
	if in.String() != "ac" || in.cursor != 1 {
		t.Fatalf("after left+backspace: %q cursor %d", in.String(), in.cursor)
	}
	feed("home", "delete")
	if in.String() != "c" || in.cursor != 0 {
		t.Fatalf("after home+delete: %q cursor %d", in.String(), in.cursor)
	}
	feed("end", "x")
	if in.String() != "cx" || in.cursor != 2 {
		t.Fatalf("after end+insert: %q cursor %d", in.String(), in.cursor)
	}

	in = qxNewInput("one two")
	in.handle(qxKey("ctrl+w"))
	if in.String() != "one " {
		t.Errorf("ctrl+w: %q", in.String())
	}
	in.handle(qxKey("ctrl+u"))
	if in.String() != "" || in.cursor != 0 {
		t.Errorf("ctrl+u: %q cursor %d", in.String(), in.cursor)
	}

	// Prefill puts the cursor at the end.
	in = qxNewInput("sql")
	if in.String() != "sql" || in.cursor != 3 {
		t.Errorf("prefill: %q cursor %d", in.String(), in.cursor)
	}
}

func TestQxInputApplySuggestion(t *testing.T) {
	in := qxNewInput("select na")
	in.applySuggestion("name")
	if in.String() != "select name" || in.cursor != len("select name") {
		t.Errorf("apply at end: %q cursor %d", in.String(), in.cursor)
	}

	// Mid-string replacement keeps the tail.
	in = qxNewInput("SELECT na FROM t")
	in.cursor = len("SELECT na")
	in.applySuggestion("name")
	if in.String() != "SELECT name FROM t" {
		t.Errorf("apply mid-string: %q", in.String())
	}
	if in.cursor != len("SELECT name") {
		t.Errorf("cursor after mid-string apply = %d", in.cursor)
	}
}

func TestQxSuggest(t *testing.T) {
	var s qxSuggest
	sc := query.Schema{
		Current: []string{"name", "age"},
		Tables:  map[string][]string{"nums": nil},
	}
	// Column context after SELECT: column, then function, then keywords —
	// no table names.
	s.recompute("SELECT n", len("SELECT n"), sc)
	got := make([]string, len(s.items))
	for i, it := range s.items {
		got[i] = it.Text
	}
	want := []string{"name", "nullif(", "not", "null"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	if !s.open() || s.selected().Text != "name" || s.selected().Kind != query.KindColumn {
		t.Fatalf("open=%v selected=%+v", s.open(), s.selected())
	}
	s.cycle(1)
	if s.selected().Text != "nullif(" {
		t.Errorf("after cycle: %+v", s.selected())
	}
	s.cycle(-2)
	if s.selected().Text != "null" {
		t.Errorf("wrap backwards: %+v", s.selected())
	}
	s.clear()
	if s.open() || s.selected().Text != "" {
		t.Error("clear did not close the list")
	}

	// Table context completes table names (empty token included).
	s.recompute("SELECT name FROM ", len("SELECT name FROM "), sc)
	if len(s.items) != 1 || s.items[0].Text != "nums" || s.items[0].Kind != query.KindTable {
		t.Fatalf("table context items = %+v", s.items)
	}
}

func TestQxSuggestViewScrollsPastCap(t *testing.T) {
	var s qxSuggest
	sc := query.Schema{Current: []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}}
	// 7 columns + abs( + avg( + all/and/as/asc = 13 candidates.
	s.recompute("a", 1, sc)
	if len(s.items) <= qxMaxSuggestions {
		t.Fatalf("need more than %d items, got %d", qxMaxSuggestions, len(s.items))
	}

	th := qxTestTheme()
	lines := s.view(th)
	if len(lines) != qxMaxSuggestions+1 { // window + position line
		t.Fatalf("%d lines, want %d visible + position", len(lines), qxMaxSuggestions)
	}
	if !strings.Contains(lines[0], "a1") || !strings.Contains(lines[len(lines)-1], fmt.Sprintf("1/%d", len(s.items))) {
		t.Errorf("initial window: first=%q last=%q", lines[0], lines[len(lines)-1])
	}

	// Cycling past the window scrolls it; the selected item stays visible.
	for i := 0; i < qxMaxSuggestions; i++ {
		s.cycle(1)
	}
	lines = s.view(th)
	found := false
	for _, l := range lines {
		if strings.Contains(l, s.selected().Text) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("selected %q not visible after scrolling: %v", s.selected().Text, lines)
	}
}

func TestQxSuggestViewShowsKindAndDetail(t *testing.T) {
	var s qxSuggest
	sc := query.Schema{Tables: map[string][]string{"users": {"name"}}}
	s.recompute("select users.na from users", len("select users.na"), sc)
	if len(s.items) != 1 {
		t.Fatalf("items = %+v", s.items)
	}
	out := strings.Join(s.view(qxTestTheme()), "\n")
	for _, want := range []string{"name", "col", "users"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q: %q", want, out)
		}
	}
}

func TestQxSchemaOfFallback(t *testing.T) {
	// Contexts without CompletionSchema get current columns plus bare table
	// names with unknown column sets.
	ctx := &qxCtx{cols: []string{"a", "b"}, tables: []string{"t1", "t2"}}
	sc := qxSchemaOf(ctx)
	if !reflect.DeepEqual(sc.Current, []string{"a", "b"}) {
		t.Errorf("Current = %v", sc.Current)
	}
	if len(sc.Tables) != 2 || sc.Tables["t1"] != nil || sc.Tables["t2"] != nil {
		t.Errorf("Tables = %v", sc.Tables)
	}
}

func TestPasteSanitize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"select *\r\nfrom t", "select * from t"},
		{"a\tb", "a b"},
		{"plain", "plain"},
		{"\r\x00\x1b\x7f", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := pasteSanitize(c.in); got != c.want {
			t.Errorf("pasteSanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestQxInputPaste(t *testing.T) {
	// A multi-character paste is one edit at the cursor.
	in := qxNewInput("SELECT  FROM t")
	in.cursor = len("SELECT ")
	if !in.paste("name,\nage") {
		t.Fatal("paste should report a change")
	}
	if in.String() != "SELECT name, age FROM t" {
		t.Fatalf("text = %q", in.String())
	}
	if in.cursor != len("SELECT name, age") {
		t.Fatalf("cursor = %d, want %d", in.cursor, len("SELECT name, age"))
	}

	// Empty and control-only pastes change nothing.
	before := in.String()
	if in.paste("") || in.paste("\r") {
		t.Fatal("empty paste must not report a change")
	}
	if in.String() != before {
		t.Fatalf("text changed by empty paste: %q", in.String())
	}
}
