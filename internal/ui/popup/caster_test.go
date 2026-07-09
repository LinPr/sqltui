package popup

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/ui"
)

func TestCasterQuoteIdent(t *testing.T) {
	if got := casterQuoteIdent("age"); got != `"age"` {
		t.Errorf("got %s", got)
	}
	if got := casterQuoteIdent(`we"ird`); got != `"we""ird"` {
		t.Errorf("got %s", got)
	}
}

func TestCasterBuildSQL(t *testing.T) {
	got := casterBuildSQL([]string{"a", "b", "c"}, "b", "INTEGER")
	want := `SELECT "a", CAST("b" AS INTEGER) AS "b", "c" FROM "_"`
	if got != want {
		t.Errorf("sql = %s\nwant  %s", got, want)
	}
}

func TestCasterTargetsMapping(t *testing.T) {
	want := map[string]string{
		"str": "TEXT", "i64": "INTEGER", "f64": "REAL",
		"bool": "INTEGER", "date": "TEXT", "datetime": "TEXT",
	}
	if len(casterTargets) != len(want) {
		t.Fatalf("got %d targets, want %d", len(casterTargets), len(want))
	}
	for _, ct := range casterTargets {
		if want[ct.Name] != ct.SQLType {
			t.Errorf("%s -> %s, want %s", ct.Name, ct.SQLType, want[ct.Name])
		}
	}
}

func TestCasterTwoStepFlow(t *testing.T) {
	o := &caster{cols: []string{"a", "b"}, viewRows: 10}

	// esc on step 0 closes.
	if _, close := o.casterKey("esc"); !close {
		t.Fatal("esc on column step must close")
	}

	// enter on step 0 advances; esc on step 1 goes back.
	o = &caster{cols: []string{"a", "b"}, viewRows: 10}
	o.casterKey("down")
	if got, ok := o.pickedCol(); !ok || got != "b" {
		t.Fatalf("picked column = %q ok=%v, want b", got, ok)
	}
	if cmd, close := o.casterKey("enter"); close || cmd != nil {
		t.Fatal("enter on column step must advance, not close")
	}
	if o.step != 1 {
		t.Fatalf("step = %d, want 1", o.step)
	}
	if _, close := o.casterKey("esc"); close {
		t.Fatal("esc on type step must go back, not close")
	}
	if o.step != 0 {
		t.Fatalf("step after esc = %d, want 0", o.step)
	}
}

func TestCasterColumnFilter(t *testing.T) {
	o := &caster{cols: []string{"id", "name", "node"}, viewRows: 10}

	// Letters type into the filter instead of acting as shortcuts.
	if cmd, close := o.casterKey("q"); cmd != nil || close {
		t.Fatal("q on the column step must type, not close")
	}
	if string(o.filter.query) != "q" {
		t.Fatalf("query = %q, want q", string(o.filter.query))
	}
	o.casterKey("backspace")

	o.casterKey("n")
	o.casterKey("o")
	if got, ok := o.pickedCol(); !ok || got != "node" {
		t.Fatalf("picked = %q ok=%v, want node", got, ok)
	}
	if len(o.filter.idx) != 1 {
		t.Fatalf("filtered = %v, want a single match", o.filter.idx)
	}

	// esc clears the filter before closing.
	if cmd, close := o.casterKey("esc"); cmd != nil || close {
		t.Fatal("esc with a filter set must only clear it")
	}
	if len(o.filter.query) != 0 || len(o.filter.idx) != 3 {
		t.Fatalf("after esc: query=%q idx=%v", string(o.filter.query), o.filter.idx)
	}
	if _, close := o.casterKey("esc"); !close {
		t.Fatal("esc with an empty filter must close")
	}
}

func TestCasterEnterWithNoMatchIsNoop(t *testing.T) {
	o := &caster{cols: []string{"id"}, viewRows: 10}
	o.casterKey("z")
	if len(o.filter.idx) != 0 {
		t.Fatalf("filtered = %v, want none", o.filter.idx)
	}
	if cmd, close := o.casterKey("enter"); cmd != nil || close || o.step != 0 {
		t.Fatal("enter with no match must stay on the column step")
	}
}

// casterCollect runs cmd (unwrapping batches) and returns the messages.
func casterCollect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch m := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, casterCollect(c)...)
		}
	default:
		out = append(out, m)
	}
	return out
}

func casterTestEngine(t *testing.T) *query.Engine {
	t.Helper()
	e, err := query.NewEngine()
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	t.Cleanup(func() { e.Close() })

	f := &data.Frame{Columns: []data.Column{
		{Name: "id", Type: data.TypeString, Cells: []any{"1", "2", "3"}},
		{Name: "name", Type: data.TypeString, Cells: []any{"x", "y", "z"}},
	}}
	if err := e.Register("_", f); err != nil {
		t.Fatalf("register: %v", err)
	}
	return e
}

func TestCasterExecute(t *testing.T) {
	e := casterTestEngine(t)
	o := &caster{engine: e, cols: []string{"id", "name"}, viewRows: 10}

	o.casterKey("enter") // pick column "id"
	o.casterKey("j")     // move to i64
	cmd, close := o.casterKey("enter")
	if !close {
		t.Fatal("enter on type step must close")
	}
	msgs := casterCollect(cmd)
	var applied *ui.ApplyFrameMsg
	for _, m := range msgs {
		if a, ok := m.(ui.ApplyFrameMsg); ok {
			applied = &a
		}
	}
	if applied == nil {
		t.Fatalf("no ApplyFrameMsg in %v", msgs)
	}
	if applied.Crumb != "cast" {
		t.Errorf("crumb = %q, want cast", applied.Crumb)
	}
	f := applied.Frame
	if f.NumCols() != 2 || f.NumRows() != 3 {
		t.Fatalf("shape = %d x %d, want 3 x 2", f.NumRows(), f.NumCols())
	}
	if v, ok := f.Cell(0, 0).(int64); !ok || v != 1 {
		t.Errorf("cast cell = %#v, want int64(1)", f.Cell(0, 0))
	}
	if v, ok := f.Cell(0, 1).(string); !ok || v != "x" {
		t.Errorf("untouched cell = %#v, want \"x\"", f.Cell(0, 1))
	}
}

func TestCasterExecuteError(t *testing.T) {
	e := casterTestEngine(t)
	// Drop the current-frame table so the query fails.
	if err := e.Unregister("_"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	cmd := casterExec(e, []string{"id"}, "id", "INTEGER", 0)
	msg := cmd()
	if _, ok := msg.(ui.ErrorMsg); !ok {
		t.Fatalf("got %T, want ui.ErrorMsg", msg)
	}
}
