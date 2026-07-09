package popup

import (
	"testing"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/ui"
)

func scatTestFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "x", Type: data.TypeInt, Cells: []any{int64(1), int64(2), int64(3), nil}},
		{Name: "y", Type: data.TypeFloat, Cells: []any{1.5, nil, 3.5, 4.0}},
		{Name: "grp", Type: data.TypeString, Cells: []any{"a", "b", "a", nil}},
	}}
}

func TestScatPoints(t *testing.T) {
	f := scatTestFrame()
	xs, ys, groups := scatPoints(f, 0, 1, 2)
	// rows 1 (y null) and 3 (x null) are skipped
	if len(xs) != 2 || len(ys) != 2 || len(groups) != 2 {
		t.Fatalf("got %d/%d/%d points, want 2/2/2", len(xs), len(ys), len(groups))
	}
	if xs[0] != 1 || ys[0] != 1.5 || groups[0] != "a" {
		t.Fatalf("first point = (%v, %v, %q)", xs[0], ys[0], groups[0])
	}
	if xs[1] != 3 || ys[1] != 3.5 || groups[1] != "a" {
		t.Fatalf("second point = (%v, %v, %q)", xs[1], ys[1], groups[1])
	}
	// no group column → nil groups
	_, _, groups = scatPoints(f, 0, 1, -1)
	if groups != nil {
		t.Fatalf("groupCol=-1 → groups %v, want nil", groups)
	}
}

func TestScatPointsNullGroupLabel(t *testing.T) {
	f := &data.Frame{Columns: []data.Column{
		{Name: "x", Type: data.TypeInt, Cells: []any{int64(1)}},
		{Name: "y", Type: data.TypeInt, Cells: []any{int64(2)}},
		{Name: "g", Type: data.TypeString, Cells: []any{nil}},
	}}
	_, _, groups := scatPoints(f, 0, 1, 2)
	if len(groups) != 1 || groups[0] != "(null)" {
		t.Fatalf("null group cell → %v, want [(null)]", groups)
	}
}

func TestScatBuilderFlow(t *testing.T) {
	b := &scatBuilder{frame: scatTestFrame()}

	// step X: select column x
	ov, _ := b.Update(histKey("enter"))
	if ov != ui.Overlay(b) || b.step != scatStepY || b.xCol != 0 {
		t.Fatalf("after X: step=%d xCol=%d", b.step, b.xCol)
	}
	// step Y: non-numeric column rejected inline
	b.sel = 2 // grp
	b.Update(histKey("enter"))
	if b.step != scatStepY || b.errText == "" {
		t.Fatalf("non-numeric Y must stay with error: step=%d err=%q", b.step, b.errText)
	}
	// pick y
	b.sel = 1
	b.Update(histKey("enter"))
	if b.step != scatStepGroup || b.yCol != 1 {
		t.Fatalf("after Y: step=%d yCol=%d", b.step, b.yCol)
	}
	// group list starts with "(none)"; pick grp (index 3 = column 2)
	b.sel = 3
	ov, _ = b.Update(histKey("enter"))
	p, ok := ov.(*scatPlot)
	if !ok {
		t.Fatalf("enter on group did not open the plot: %T", ov)
	}
	if len(p.xs) != 2 || len(p.groups) != 2 {
		t.Fatalf("plot state: xs=%v groups=%v", p.xs, p.groups)
	}

	// esc on the plot closes back to the table
	_, cmd := p.Update(histKey("esc"))
	if cmd == nil {
		t.Fatal("esc on plot must produce a command")
	}
	if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
		t.Fatalf("esc on plot must close the overlay, got %T", cmd())
	}
}

func TestScatBuilderNoneGroup(t *testing.T) {
	b := &scatBuilder{frame: scatTestFrame()}
	b.Update(histKey("enter")) // x
	b.sel = 1
	b.Update(histKey("enter")) // y
	// keep "(none)" selected
	ov, _ := b.Update(histKey("enter"))
	p, ok := ov.(*scatPlot)
	if !ok {
		t.Fatalf("expected plot overlay, got %T", ov)
	}
	if p.groups != nil {
		t.Fatalf("(none) grouping → groups %v, want nil", p.groups)
	}
}

func TestScatBuilderEscStepsBack(t *testing.T) {
	b := &scatBuilder{frame: scatTestFrame()}
	b.Update(histKey("enter")) // to step Y
	b.Update(histKey("esc"))
	if b.step != scatStepX {
		t.Fatalf("esc should step back, step=%d", b.step)
	}
	_, cmd := b.Update(histKey("esc"))
	if cmd == nil {
		t.Fatal("esc on first step must close")
	}
	if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
		t.Fatalf("esc on first step must close the overlay, got %T", cmd())
	}
}

func TestScatBuilderFilter(t *testing.T) {
	b := &scatBuilder{frame: scatTestFrame()}

	// X step: type to narrow, enter picks the match (column y, index 1).
	b.Update(histKey("y"))
	if len(b.filter.idx) != 1 || b.filter.idx[0] != 1 {
		t.Fatalf("filtered = %v, want [1]", b.filter.idx)
	}
	b.Update(histKey("enter"))
	if b.step != scatStepY || b.xCol != 1 {
		t.Fatalf("after filtered X: step=%d xCol=%d", b.step, b.xCol)
	}
	// The filter resets between steps.
	if len(b.filter.query) != 0 || len(b.filter.idx) != 3 {
		t.Fatalf("filter not reset: query=%q idx=%v", string(b.filter.query), b.filter.idx)
	}

	// Y step: no match → enter is a no-op; esc clears the filter instead of
	// stepping back.
	b.Update(histKey("z"))
	if ov, _ := b.Update(histKey("enter")); ov != ui.Overlay(b) || b.step != scatStepY {
		t.Fatal("enter with no match must stay on the Y step")
	}
	_, cmd := b.Update(histKey("esc"))
	if cmd != nil || b.step != scatStepY || len(b.filter.query) != 0 {
		t.Fatalf("esc must clear the filter, not step back: step=%d query=%q",
			b.step, string(b.filter.query))
	}
	b.sel = 0 // column x
	b.Update(histKey("enter"))
	if b.step != scatStepGroup {
		t.Fatalf("step = %d, want group", b.step)
	}

	// Group step: filter includes "(none)" and the columns; pick grp.
	for _, k := range []string{"g", "r", "p"} {
		b.Update(histKey(k))
	}
	if len(b.filter.idx) != 1 || b.filter.idx[0] != 3 {
		t.Fatalf("group filtered = %v, want [3]", b.filter.idx)
	}
	ov, _ := b.Update(histKey("enter"))
	p, ok := ov.(*scatPlot)
	if !ok {
		t.Fatalf("expected plot overlay, got %T", ov)
	}
	if len(p.groups) != 2 {
		t.Fatalf("groups = %v, want 2 entries", p.groups)
	}
}

func TestScatterplotFactoryRegistered(t *testing.T) {
	if _, ok := ui.Factories["scatterplot"]; !ok {
		t.Fatal("factory \"scatterplot\" not registered")
	}
}
