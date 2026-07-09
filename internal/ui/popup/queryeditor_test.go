package popup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/ui"
)

// qxOpenEditor builds an editor and delivers its Init catalog snapshot, as
// the app does right after pushing the overlay.
func qxOpenEditor(t *testing.T, ctx ui.AppContext, arg string) *qxEditor {
	t.Helper()
	ed := qxNewEditor(ctx, arg)
	if cmd := ed.Init(); cmd != nil {
		ov, _ := ed.Update(cmd())
		ed = ov.(*qxEditor)
	}
	return ed
}

func TestQueryFactoryRegistered(t *testing.T) {
	f, ok := ui.Factories["query"]
	if !ok {
		t.Fatal(`Factories["query"] not registered`)
	}
	ov, err := f(qxFileCtx(t), "")
	if err != nil || ov == nil {
		t.Fatalf("factory: ov=%v err=%v", ov, err)
	}
}

func TestQueryEditorPrefillDoesNotExecute(t *testing.T) {
	f := ui.Factories["query"]
	ov, err := f(qxFileCtx(t), "SELECT * FROM _")
	if err != nil {
		t.Fatal(err)
	}
	ed, ok := ov.(*qxEditor)
	if !ok {
		t.Fatalf("factory returned %T", ov)
	}
	if ed.in.String() != "SELECT * FROM _" {
		t.Errorf("prefill = %q", ed.in.String())
	}
	if ed.sug.open() {
		t.Error("suggestions open right after prefill")
	}
}

func TestQueryEditorSuggestions(t *testing.T) {
	ed := qxOpenEditor(t, qxFileCtx(t), "")
	ov, _ := qxTypeText(ed, "SEL")
	ed = ov.(*qxEditor)
	if !ed.sug.open() || ed.sug.selected().Text != "SELECT" {
		t.Fatalf("after typing SEL: items=%v", ed.sug.items)
	}

	// Enter applies the highlighted suggestion and closes the list.
	ov, cmd := qxSend(ed, "enter")
	ed = ov.(*qxEditor)
	if cmd != nil {
		t.Fatal("apply-suggestion enter must not execute")
	}
	if ed.in.String() != "SELECT" || ed.sug.open() {
		t.Fatalf("after apply: %q open=%v", ed.in.String(), ed.sug.open())
	}

	// tab cycles when the list is open.
	ov, _ = qxTypeText(ed, " n")
	ed = ov.(*qxEditor)
	if !ed.sug.open() || ed.sug.selected().Text != "name" {
		t.Fatalf("column suggestions: %v", ed.sug.items)
	}
	ov, _ = qxSend(ed, "tab")
	ed = ov.(*qxEditor)
	if ed.sug.selected().Text == "name" {
		t.Error("tab did not cycle the highlight")
	}

	// esc first closes the list, then the overlay.
	ov, cmd = qxSend(ed, "esc")
	ed = ov.(*qxEditor)
	if cmd != nil || ed.sug.open() {
		t.Fatal("first esc should only close the list")
	}
	_, cmd = qxSend(ed, "esc")
	msgs := qxDrain(cmd)
	if _, ok := qxFindMsg[ui.CloseOverlayMsg](msgs); !ok {
		t.Errorf("second esc: got %v, want CloseOverlayMsg", msgs)
	}
}

func TestQueryEditorExecute(t *testing.T) {
	ed := qxOpenEditor(t, qxFileCtx(t), "")
	ov, _ := qxTypeText(ed, "SELECT * FROM _")
	// Typing "_" matched the registered table; close the list so enter runs.
	ov, _ = qxSend(ov, "esc")
	_, cmd := qxSend(ov, "enter")
	msgs := qxDrain(cmd)

	if _, ok := qxFindMsg[ui.CloseOverlayMsg](msgs); !ok {
		t.Errorf("execute did not close the overlay: %v", msgs)
	}
	apply, ok := qxFindMsg[ui.ApplyFrameMsg](msgs)
	if !ok {
		t.Fatalf("no ApplyFrameMsg in %v", msgs)
	}
	if !apply.NewTab || apply.Crumb != "query" {
		t.Errorf("NewTab=%v Crumb=%q", apply.NewTab, apply.Crumb)
	}
	if apply.TabTitle != "SELECT * FROM _" {
		t.Errorf("TabTitle = %q", apply.TabTitle)
	}
	if apply.Frame == nil || apply.Frame.NumRows() != 3 {
		t.Errorf("result frame = %+v", apply.Frame)
	}
}

func TestQueryEditorExecuteError(t *testing.T) {
	ed := qxOpenEditor(t, qxFileCtx(t), "")
	ov, _ := qxTypeText(ed, "SELECT nope FROM nowhere")
	ov, _ = qxSend(ov, "esc") // close any suggestion list
	_, cmd := qxSend(ov, "enter")
	msgs := qxDrain(cmd)
	if _, ok := qxFindMsg[ui.ErrorMsg](msgs); !ok {
		t.Errorf("bad statement: got %v, want ErrorMsg", msgs)
	}
}

func TestQueryEditorEmptyEnterCloses(t *testing.T) {
	ed := qxOpenEditor(t, qxFileCtx(t), "")
	_, cmd := qxSend(ed, "enter")
	msgs := qxDrain(cmd)
	if _, ok := qxFindMsg[ui.CloseOverlayMsg](msgs); !ok {
		t.Errorf("empty enter: got %v, want CloseOverlayMsg", msgs)
	}
	if _, ok := qxFindMsg[ui.ApplyFrameMsg](msgs); ok {
		t.Error("empty enter must not execute")
	}
}

// qxSchemaCtx implements the optional schema provider and warmer
// extensions, simulating an app in live database mode: columns listed in
// pending only become visible after a warm-up "fetched" them.
type qxSchemaCtx struct {
	qxCtx
	sc        query.Schema
	pending   map[string][]string
	warmCalls [][]string
}

func (c *qxSchemaCtx) CompletionSchema() query.Schema {
	tables := make(map[string][]string, len(c.sc.Tables))
	for k, v := range c.sc.Tables {
		tables[k] = v
	}
	return query.Schema{Current: c.sc.Current, Tables: tables}
}

func (c *qxSchemaCtx) WarmCompletionSchema(tables ...string) {
	c.warmCalls = append(c.warmCalls, tables)
	for _, t := range tables {
		if cols, ok := c.pending[t]; ok {
			c.sc.Tables[t] = cols
		}
	}
}

func TestQueryEditorUsesSchemaProvider(t *testing.T) {
	ctx := &qxSchemaCtx{sc: query.Schema{
		Current: []string{"c1"},
		Tables:  map[string][]string{"users": {"id"}, "orders": nil},
	}}
	ed := qxOpenEditorCtx(t, ctx)

	ov, _ := qxTypeText(ed, "select * from ")
	ed = ov.(*qxEditor)
	if len(ed.sug.items) != 2 {
		t.Fatalf("table suggestions = %+v", ed.sug.items)
	}
	for i, want := range []string{"orders", "users"} {
		if it := ed.sug.items[i]; it.Text != want || it.Kind != query.KindTable {
			t.Errorf("item %d = %+v, want table %q", i, it, want)
		}
	}
}

// qxOpenEditorCtx opens an editor on ctx and delivers its Init message.
func qxOpenEditorCtx(t *testing.T, ctx ui.AppContext) *qxEditor {
	t.Helper()
	ed := qxNewEditor(ctx, "")
	if cmd := ed.Init(); cmd != nil {
		ov, _ := ed.Update(cmd())
		ed = ov.(*qxEditor)
	}
	return ed
}

func TestQueryEditorDotWarmUp(t *testing.T) {
	ctx := &qxSchemaCtx{
		sc:      query.Schema{Tables: map[string][]string{"users": nil}},
		pending: map[string][]string{"users": {"id", "name"}},
	}
	ed := qxOpenEditorCtx(t, ctx)
	// Init warmed the table listing (no specific tables).
	if len(ctx.warmCalls) != 1 || len(ctx.warmCalls[0]) != 0 {
		t.Fatalf("init warm calls = %v", ctx.warmCalls)
	}

	// Typing "users." kicks one async column fetch for that table; nothing
	// is suggested until it lands.
	ov, cmd := qxTypeText(ed, "select * from users where users.")
	ed = ov.(*qxEditor)
	if cmd == nil {
		t.Fatal("no warm command for dot completion")
	}
	if ed.sug.open() {
		t.Fatalf("columns suggested before the fetch: %+v", ed.sug.items)
	}

	// Running the command fetches the columns and reopens the list.
	ov, _ = ed.Update(cmd())
	ed = ov.(*qxEditor)
	if len(ctx.warmCalls) != 2 || len(ctx.warmCalls[1]) != 1 || ctx.warmCalls[1][0] != "users" {
		t.Fatalf("warm calls = %v", ctx.warmCalls)
	}
	if !ed.sug.open() || len(ed.sug.items) != 2 {
		t.Fatalf("suggestions after fetch = %+v", ed.sug.items)
	}
	for i, want := range []string{"id", "name"} {
		if it := ed.sug.items[i]; it.Text != want || it.Kind != query.KindColumn || it.Detail != "users" {
			t.Errorf("item %d = %+v, want column %q of users", i, it, want)
		}
	}

	// Continuing to type in the same dot context must not re-fetch.
	ov, cmd = qxTypeText(ed, "na")
	ed = ov.(*qxEditor)
	if cmd != nil && cmd() != nil {
		t.Error("column fetch repeated for a warmed table")
	}
	if !ed.sug.open() || ed.sug.selected().Text != "name" {
		t.Fatalf("dot completion after warm = %+v", ed.sug.items)
	}
	if len(ctx.warmCalls) != 2 {
		t.Errorf("warm calls = %v, want no repeats", ctx.warmCalls)
	}
}

func TestQueryEditorView(t *testing.T) {
	ed := qxOpenEditor(t, qxFileCtx(t), "")
	ov, _ := qxTypeText(ed, "SEL")
	out := ov.View(80, 24, qxTestTheme())
	if !strings.Contains(out, "query") {
		t.Error("view missing title")
	}
	if !strings.Contains(out, "SELECT") {
		t.Error("view missing suggestion")
	}
	// Must not panic at tiny sizes.
	_ = ov.View(5, 2, qxTestTheme())
}

func TestQueryEditorPaste(t *testing.T) {
	e := qxOpenEditor(t, qxFileCtx(t), "")

	// Multi-line SQL arrives as one PasteMsg and is flattened to one line.
	ov, _ := e.Update(tea.PasteMsg{Content: "select *\r\nfrom x"})
	e = ov.(*qxEditor)
	if got := e.in.String(); got != "select * from x" {
		t.Fatalf("editor text after paste = %q", got)
	}

	// Paste inserts at the cursor, not at the end.
	ov, _ = e.Update(qxKey("home"))
	e = ov.(*qxEditor)
	ov, _ = e.Update(tea.PasteMsg{Content: "-- c\n"})
	e = ov.(*qxEditor)
	if got := e.in.String(); got != "-- c select * from x" {
		t.Fatalf("editor text after cursor paste = %q", got)
	}

	// Empty paste leaves the editor untouched.
	ov, _ = e.Update(tea.PasteMsg{Content: ""})
	e = ov.(*qxEditor)
	if got := e.in.String(); got != "-- c select * from x" {
		t.Fatalf("editor text after empty paste = %q", got)
	}
}
