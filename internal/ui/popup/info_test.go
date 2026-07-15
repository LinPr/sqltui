package popup

import (
	"errors"
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
	ns     string
	backend db.Backend
}

func (c infoTestCtx) CurrentFrame() *data.Frame { return c.frame }
func (c infoTestCtx) CurrentRow() int           { return 0 }
func (c infoTestCtx) SheetFieldCursor() int     { return 0 }
func (c infoTestCtx) CurrentTableNamespace() string { return c.ns }
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
func (c infoTestCtx) Backend() db.Backend   { return c.backend }
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

// infoTestBackend is a stub db.Backend for info overlay tests; ColumnsMeta
// returns the provided slice (or err).
type infoTestBackend struct {
	meta []db.ColumnMeta
	err  error
}

func (b *infoTestBackend) Kind() string                  { return "stub" }
func (b *infoTestBackend) Title() string                 { return "stub://db" }
func (b *infoTestBackend) Run(string) (db.Result, error) { return db.Result{}, errors.New("not implemented") }
func (b *infoTestBackend) Namespaces() ([]string, error) { return []string{""}, nil }
func (b *infoTestBackend) Tables(string) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (b *infoTestBackend) FetchTable(string, string, int) (*data.Frame, error) {
	return nil, errors.New("not implemented")
}
func (b *infoTestBackend) PrimaryKeys(string, string) ([]string, error) { return nil, nil }
func (b *infoTestBackend) ColumnsMeta(string, string) ([]db.ColumnMeta, error) {
	return b.meta, b.err
}
func (b *infoTestBackend) Close() error { return nil }

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
	// File-mode fallback: Type is the frame DType, metadata fields empty.
	want := []infoColRow{
		{Name: "id", Type: "i64"},
		{Name: "name", Type: "str"},
		{Name: "score", Type: "f64"},
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

// TestInfoFileModeUsesDType asserts that without a backend the listing uses
// the frame's inferred DType and leaves the metadata fields blank.
func TestInfoFileModeUsesDType(t *testing.T) {
	ctx := infoTestCtx{frame: infoTestFrame(), crumbs: []string{"people"}}
	o := newInfoOverlay(ctx, ctx.frame)
	if o.loading {
		t.Fatal("file mode should not be loading")
	}
	if cmd := o.Init(); cmd != nil {
		t.Fatalf("Init in file mode should return nil, got %v", cmd)
	}
	if len(o.list) != 3 {
		t.Fatalf("list len = %d, want 3", len(o.list))
	}
	want := []infoColRow{
		{Name: "id", Type: "i64"},
		{Name: "name", Type: "str"},
		{Name: "score", Type: "f64"},
	}
	for i, w := range want {
		if o.list[i] != w {
			t.Errorf("list[%d] = %+v, want %+v", i, o.list[i], w)
		}
	}
	th := infoTestTheme(t)
	v := o.View(80, 24, th)
	for _, want := range []string{"i64", "str", "f64"} {
		if !strings.Contains(v, want) {
			t.Errorf("file-mode view missing %q", want)
		}
	}
}

// TestInfoDBModeLoadsMeta drives Init+Update with a fake backend whose
// ColumnsMeta returns two entries and asserts the listing shows the real
// metadata (not the frame DType).
func TestInfoDBModeLoadsMeta(t *testing.T) {
	be := &infoTestBackend{meta: []db.ColumnMeta{
		{Name: "id", DataType: "int", IsNullable: "NO", Default: ""},
		{Name: "email", DataType: "varchar(255)", IsNullable: "YES", Default: "", Comment: "primary contact"},
	}}
	ctx := infoTestCtx{
		frame:   infoTestFrame(),
		crumbs:  []string{"users"},
		ns:      "public",
		backend: be,
	}
	o := newInfoOverlay(ctx, ctx.frame)
	if !o.loading {
		t.Fatal("db mode should start loading")
	}
	// Pre-fetch listing is the frame fallback (placeholder).
	if len(o.list) != 3 || o.list[0].Type != "i64" {
		t.Fatalf("placeholder listing = %+v", o.list)
	}

	cmd := o.Init()
	if cmd == nil {
		t.Fatal("Init in db mode should return a cmd")
	}
	msg := cmd()
	cm, ok := msg.(columnsMetaMsg)
	if !ok {
		t.Fatalf("Init cmd msg = %T, want columnsMetaMsg", msg)
	}
	if cm.owner != o {
		t.Fatal("columnsMetaMsg owner mismatch")
	}
	if cm.err != nil {
		t.Fatalf("ColumnsMeta err = %v", cm.err)
	}
	if len(cm.meta) != 2 {
		t.Fatalf("meta len = %d, want 2", len(cm.meta))
	}

	ov, _ := o.Update(cm)
	o = ov.(*infoOverlay)
	if o.loading {
		t.Fatal("still loading after Update")
	}
	if len(o.list) != 2 {
		t.Fatalf("list len after meta = %d, want 2", len(o.list))
	}
	want := []infoColRow{
		{Name: "id", Type: "int", NotNull: "NO", Default: "", Comment: ""},
		{Name: "email", Type: "varchar(255)", NotNull: "YES", Default: "", Comment: "primary contact"},
	}
	for i, w := range want {
		if o.list[i] != w {
			t.Errorf("list[%d] = %+v, want %+v", i, o.list[i], w)
		}
	}
	// The frame DType (i64/str/f64) must NOT appear once metadata loaded.
	th := infoTestTheme(t)
	v := o.View(80, 24, th)
	for _, gone := range []string{"i64", "str", "f64"} {
		if strings.Contains(v, gone) {
			t.Errorf("db-mode view should not contain frame DType %q", gone)
		}
	}
	for _, want := range []string{"int", "varchar(255)", "primary contact", "NO", "YES"} {
		if !strings.Contains(v, want) {
			t.Errorf("db-mode view missing %q", want)
		}
	}
}

// TestInfoDBModeLoadError asserts a ColumnsMeta error is surfaced in the view.
func TestInfoDBModeLoadError(t *testing.T) {
	be := &infoTestBackend{err: errors.New("boom")}
	ctx := infoTestCtx{frame: infoTestFrame(), crumbs: []string{"users"}, backend: be}
	o := newInfoOverlay(ctx, ctx.frame)
	cmd := o.Init()
	msg := cmd().(columnsMetaMsg)
	ov, _ := o.Update(msg)
	o = ov.(*infoOverlay)
	if o.loading {
		t.Fatal("should stop loading on error")
	}
	if o.loadErr != "boom" {
		t.Errorf("loadErr = %q, want %q", o.loadErr, "boom")
	}
	th := infoTestTheme(t)
	if v := o.View(80, 24, th); !strings.Contains(v, "boom") {
		t.Errorf("view should contain error, got:\n%s", v)
	}
}

// TestInfoIgnoresStaleMetaMsg asserts a columnsMetaMsg from another owner is
// ignored.
func TestInfoIgnoresStaleMetaMsg(t *testing.T) {
	ctx := infoTestCtx{frame: infoTestFrame(), crumbs: []string{"users"}}
	o := newInfoOverlay(ctx, ctx.frame)
	before := len(o.list)
	other := &infoOverlay{}
	ov, _ := o.Update(columnsMetaMsg{owner: other, meta: []db.ColumnMeta{{Name: "x", DataType: "int"}}})
	o = ov.(*infoOverlay)
	if len(o.list) != before {
		t.Fatalf("stale msg mutated list: now %d, want %d", len(o.list), before)
	}
}

// TestInfoCtrlDNotPageDown asserts ctrl+d was removed from the info key list
// (it is table-delete now) and no longer pages down.
func TestInfoCtrlDNotPageDown(t *testing.T) {
	f := &data.Frame{}
	for i := 0; i < 40; i++ {
		f.Columns = append(f.Columns, data.Column{Name: "c", Type: data.TypeString, Cells: []any{nil}})
	}
	o := newInfoOverlay(infoTestCtx{frame: f, crumbs: []string{"t"}}, f)
	o.View(80, 20, infoTestTheme(t)) // establish viewRows
	maxOff := 40 - o.viewRows
	// From the top, ctrl+d should NOT advance the offset.
	o.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if o.offset != 0 {
		t.Errorf("ctrl+d advanced offset to %d, want 0 (removed from key list)", o.offset)
	}
	// pgdown still pages down.
	o.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if o.offset <= 0 || o.offset > maxOff {
		t.Errorf("pgdown offset = %d, want (0,%d]", o.offset, maxOff)
	}
	// ctrl+f still pages down (from top).
	o.offset = 0
	o.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if o.offset <= 0 || o.offset > maxOff {
		t.Errorf("ctrl+f offset = %d, want (0,%d]", o.offset, maxOff)
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
