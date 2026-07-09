package popup

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/ui"
)

func histTestFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "name", Type: data.TypeString, Cells: []any{"a", "b", "c"}},
		{Name: "n", Type: data.TypeInt, Cells: []any{int64(1), int64(2), nil}},
		{Name: "mixed", Type: data.TypeString, Cells: []any{"1.5", "x", " 2 "}},
		{Name: "flag", Type: data.TypeBool, Cells: []any{true, false, true}},
	}}
}

func histKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
}

func TestHistColumnFloats(t *testing.T) {
	f := histTestFrame()
	if got := histColumnFloats(f, 0); len(got) != 0 {
		t.Fatalf("string column → %v, want none", got)
	}
	if got := histColumnFloats(f, 1); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("int column with null → %v, want [1 2]", got)
	}
	if got := histColumnFloats(f, 2); len(got) != 2 || got[0] != 1.5 || got[1] != 2 {
		t.Fatalf("parsable strings → %v, want [1.5 2]", got)
	}
	if got := histColumnFloats(f, 3); len(got) != 0 {
		t.Fatalf("bool column → %v, want none", got)
	}
	if got := histColumnFloats(nil, 0); got != nil {
		t.Fatalf("nil frame → %v, want nil", got)
	}
}

func TestHistBuilderFlow(t *testing.T) {
	b := &histBuilder{frame: histTestFrame(), input: histNewInput("10")}

	// enter on non-numeric column shows an inline error, stays on step 1
	ov, _ := b.Update(histKey("enter"))
	if ov != ui.Overlay(b) || b.errText == "" || b.step != histStepColumn {
		t.Fatalf("non-numeric selection: step=%d err=%q", b.step, b.errText)
	}

	// move to the numeric column and select it
	b.Update(histKey("down"))
	b.Update(histKey("enter"))
	if b.step != histStepBuckets || b.colName != "n" || len(b.values) != 2 {
		t.Fatalf("after selecting n: step=%d col=%q values=%v", b.step, b.colName, b.values)
	}

	// bad bucket count → inline error, still the builder
	b.Update(histKey("backspace"))
	b.Update(histKey("backspace"))
	b.Update(histKey("0"))
	ov, _ = b.Update(histKey("enter"))
	if _, ok := ov.(*histBuilder); !ok || b.errText == "" {
		t.Fatalf("buckets=0 must be rejected inline (err=%q)", b.errText)
	}

	// fix it → replaced by the plot overlay
	b.Update(histKey("backspace"))
	b.Update(histKey("5"))
	ov, _ = b.Update(histKey("enter"))
	p, ok := ov.(*histPlot)
	if !ok {
		t.Fatalf("enter did not open the plot overlay: %T", ov)
	}
	if p.buckets != 5 || len(p.values) != 2 {
		t.Fatalf("plot state: buckets=%d values=%v", p.buckets, p.values)
	}

	// q on the plot closes the whole thing (back to the table)
	_, cmd := p.Update(histKey("q"))
	if cmd == nil {
		t.Fatal("q on plot must produce a command")
	}
	if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
		t.Fatalf("q on plot must close the overlay, got %T", cmd())
	}
}

func TestHistBuilderEmptyBucketsDefaultsToTen(t *testing.T) {
	b := &histBuilder{frame: histTestFrame(), input: histNewInput("10")}
	b.Update(histKey("down"))
	b.Update(histKey("enter"))
	b.Update(histKey("backspace"))
	b.Update(histKey("backspace"))
	ov, _ := b.Update(histKey("enter"))
	p, ok := ov.(*histPlot)
	if !ok || p.buckets != 10 {
		t.Fatalf("empty input should default to 10 buckets, got %T", ov)
	}
}

func TestHistBuilderColumnFilter(t *testing.T) {
	b := &histBuilder{frame: histTestFrame(), input: histNewInput("10")}

	// Typing narrows the column list; letters no longer navigate.
	for _, k := range []string{"m", "i", "x"} {
		b.Update(histKey(k))
	}
	if len(b.filter.idx) != 1 || b.filter.idx[0] != 2 {
		t.Fatalf("filtered = %v, want [2] (mixed)", b.filter.idx)
	}
	b.Update(histKey("enter"))
	if b.step != histStepBuckets || b.colName != "mixed" {
		t.Fatalf("after filtered enter: step=%d col=%q, want buckets/mixed", b.step, b.colName)
	}
}

func TestHistBuilderEscClearsFilterFirst(t *testing.T) {
	b := &histBuilder{frame: histTestFrame(), input: histNewInput("10")}
	b.Update(histKey("z"))
	if len(b.filter.idx) != 0 {
		t.Fatalf("filtered = %v, want none", b.filter.idx)
	}
	// enter with no match is a no-op
	if ov, _ := b.Update(histKey("enter")); ov != ui.Overlay(b) || b.step != histStepColumn {
		t.Fatal("enter with no match must stay on the column step")
	}
	// first esc clears the filter, second closes
	_, cmd := b.Update(histKey("esc"))
	if cmd != nil || len(b.filter.query) != 0 || len(b.filter.idx) != 4 {
		t.Fatalf("esc must clear the filter: query=%q idx=%v", string(b.filter.query), b.filter.idx)
	}
	_, cmd = b.Update(histKey("esc"))
	if cmd == nil {
		t.Fatal("esc with an empty filter must close")
	}
	if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
		t.Fatalf("esc cmd msg = %T, want CloseOverlayMsg", cmd())
	}
}

func TestHistInputEditing(t *testing.T) {
	in := histNewInput("10")
	in.Handle(histKey("left"))
	in.Handle(histKey("2"))
	if in.String() != "120" {
		t.Fatalf("insert at cursor → %q, want 120", in.String())
	}
	in.Handle(histKey("backspace"))
	if in.String() != "10" {
		t.Fatalf("backspace → %q, want 10", in.String())
	}
}

func TestHistogramFactoryRegistered(t *testing.T) {
	if _, ok := ui.Factories["histogram"]; !ok {
		t.Fatal("factory \"histogram\" not registered")
	}
}
