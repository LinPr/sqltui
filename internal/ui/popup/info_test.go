package popup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// infoTestCtx is a minimal AppContext stub for info overlay tests.
type infoTestCtx struct {
	frame  *data.Frame
	crumbs []string
}

func (c infoTestCtx) CurrentFrame() *data.Frame { return c.frame }
func (c infoTestCtx) CurrentRow() int           { return 0 }
func (c infoTestCtx) SheetFieldCursor() int     { return 0 }
func (c infoTestCtx) CurrentTableNamespace() string { return "" }
func (c infoTestCtx) BaseCrumb() string {
	if len(c.crumbs) > 0 {
		return c.crumbs[0]
	}
	return ""
}
func (c infoTestCtx) Crumbs() []string      { return c.crumbs }
func (c infoTestCtx) ColumnNames() []string { return nil }
func (c infoTestCtx) Engine() *query.Engine { return nil }
func (c infoTestCtx) TableNames() []string  { return nil }
func (c infoTestCtx) Backend() db.Backend   { return nil }
func (c infoTestCtx) KV() db.KVBackend      { return nil }
func (c infoTestCtx) Theme() *theme.Theme   { return nil }
func (c infoTestCtx) ThemeName() string     { return "" }
func (c infoTestCtx) ShowBorders() bool     { return false }
func (c infoTestCtx) ShowRowNumbers() bool  { return false }
func (c infoTestCtx) Tabs() []ui.TabInfo    { return nil }
func (c infoTestCtx) ActiveTab() int        { return 0 }
func (c infoTestCtx) ActivePaneID() int     { return 0 }
func (c infoTestCtx) PendingEdit() *ui.PendingEdit   { return nil }
func (c infoTestCtx) PendingDelete() *ui.PendingDelete { return nil }

func infoTestFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "id", Type: data.TypeInt, Cells: []any{int64(1), int64(2), nil}},
		{Name: "name", Type: data.TypeString, Cells: []any{"a", nil, nil}},
		{Name: "score", Type: data.TypeFloat, Cells: []any{1.5, 2.5, 3.5}},
	}}
}

func infoTestTheme(t *testing.T) *theme.Theme {
	t.Helper()
	if th, ok := theme.Builtin("catppuccin-mocha"); ok {
		return th
	}
	names := theme.Names()
	if len(names) == 0 {
		t.Fatal("no builtin themes")
	}
	th, _ := theme.Builtin(names[0])
	return th
}

func TestInfoFactoryRegistered(t *testing.T) {
	f, ok := ui.Factories["info"]
	if !ok {
		t.Fatal("info factory not registered")
	}
	if _, err := f(infoTestCtx{}, ""); err == nil {
		t.Error("expected error when no frame is open")
	}
	ov, err := f(infoTestCtx{frame: infoTestFrame(), crumbs: []string{"tbl"}}, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if ov == nil {
		t.Fatal("factory returned nil overlay")
	}
}

func TestInfoBuild(t *testing.T) {
	ctx := infoTestCtx{frame: infoTestFrame(), crumbs: []string{"tbl", "filter"}}
	o := newInfoOverlay(ctx, ctx.frame)
	if o.rows != 3 || o.cols != 3 {
		t.Errorf("shape = %d x %d, want 3 x 3", o.rows, o.cols)
	}
	if len(o.list) != 3 {
		t.Fatalf("list len = %d, want 3", len(o.list))
	}
	want := []infoColRow{
		{Name: "id", Dtype: "i64", Nulls: 1},
		{Name: "name", Dtype: "str", Nulls: 2},
		{Name: "score", Dtype: "f64", Nulls: 0},
	}
	for i, w := range want {
		if o.list[i] != w {
			t.Errorf("list[%d] = %+v, want %+v", i, o.list[i], w)
		}
	}
	if o.size != data.EstimatedSize(ctx.frame) {
		t.Errorf("size = %d, want %d", o.size, data.EstimatedSize(ctx.frame))
	}
}

func TestInfoCloseKeys(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: tea.KeyEscape},
	} {
		o := newInfoOverlay(infoTestCtx{frame: infoTestFrame()}, infoTestFrame())
		_, cmd := o.Update(k)
		if cmd == nil {
			t.Fatalf("key %q: no cmd", k.String())
		}
		if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
			t.Errorf("key %q: cmd msg = %T, want CloseOverlayMsg", k.String(), cmd())
		}
	}
}

func TestInfoScrollClamp(t *testing.T) {
	// Frame with many columns so the list must scroll.
	f := &data.Frame{}
	for i := 0; i < 40; i++ {
		f.Columns = append(f.Columns, data.Column{
			Name: "c" + strings.Repeat("x", i%5), Type: data.TypeString, Cells: []any{nil},
		})
	}
	o := newInfoOverlay(infoTestCtx{frame: f, crumbs: []string{"t"}}, f)
	th := infoTestTheme(t)
	o.View(80, 20, th) // establishes viewRows

	if o.viewRows >= 40 {
		t.Fatalf("viewRows = %d, expected a scrolling window", o.viewRows)
	}

	// Scrolling up from the top stays at 0.
	o.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if o.offset != 0 {
		t.Errorf("offset after up at top = %d, want 0", o.offset)
	}
	// End jumps to the max offset.
	o.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	wantMax := 40 - o.viewRows
	if o.offset != wantMax {
		t.Errorf("offset after end = %d, want %d", o.offset, wantMax)
	}
	// Scrolling further down stays clamped.
	o.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if o.offset != wantMax {
		t.Errorf("offset after down at bottom = %d, want %d", o.offset, wantMax)
	}
	// Home returns to the top.
	o.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	if o.offset != 0 {
		t.Errorf("offset after home = %d, want 0", o.offset)
	}
}

func TestInfoViewContent(t *testing.T) {
	ctx := infoTestCtx{frame: infoTestFrame(), crumbs: []string{"people", "filter"}}
	o := newInfoOverlay(ctx, ctx.frame)
	th := infoTestTheme(t)
	v := o.View(80, 24, th)
	for _, want := range []string{"people", "3 x 3", "id", "name", "score", "i64", "f64"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestInfoHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, c := range cases {
		if got := infoHumanSize(c.n); got != c.want {
			t.Errorf("infoHumanSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
