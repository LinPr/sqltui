package popup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// exportStubCtx is a minimal AppContext for state-level tests.
type exportStubCtx struct {
	frame *data.Frame
	crumb string
}

func (c *exportStubCtx) CurrentFrame() *data.Frame { return c.frame }
func (c *exportStubCtx) CurrentRow() int           { return 0 }
func (c *exportStubCtx) BaseCrumb() string         { return c.crumb }
func (c *exportStubCtx) Crumbs() []string          { return nil }
func (c *exportStubCtx) ColumnNames() []string {
	if c.frame == nil {
		return nil
	}
	return c.frame.ColumnNames()
}
func (c *exportStubCtx) Engine() *query.Engine { return nil }
func (c *exportStubCtx) TableNames() []string  { return nil }
func (c *exportStubCtx) Backend() db.Backend   { return nil }
func (c *exportStubCtx) KV() db.KVBackend      { return nil }
func (c *exportStubCtx) Theme() *theme.Theme   { return nil }
func (c *exportStubCtx) ThemeName() string     { return "" }
func (c *exportStubCtx) ShowBorders() bool     { return false }
func (c *exportStubCtx) ShowRowNumbers() bool  { return false }
func (c *exportStubCtx) Tabs() []ui.TabInfo    { return nil }
func (c *exportStubCtx) ActiveTab() int        { return 0 }
func (c *exportStubCtx) ActivePaneID() int     { return 0 }

func exportTestFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(1), int64(2)}},
		{Name: "b", Type: data.TypeString, Cells: []any{"x", "y"}},
	}}
}

// exportDrain executes a command tree and flattens the produced messages.
func exportDrain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, exportDrain(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func exportTestTheme() *theme.Theme {
	return theme.New(theme.Palette{
		Bg: "#000000", Fg: "#ffffff", BgSoft: "#111111", FgDim: "#888888",
		Header: "#ffffff", Accent: "#5555ff", AccentFg: "#000000",
		Highlight: "#ffff00", Error: "#ff0000", Warning: "#ffaa00", Success: "#00ff00",
	})
}

func exportNew(t *testing.T, ctx ui.AppContext) *exporter {
	t.Helper()
	factory, ok := ui.Factories["export"]
	if !ok {
		t.Fatal("export factory not registered")
	}
	ov, err := factory(ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	e, ok := ov.(*exporter)
	if !ok {
		t.Fatalf("factory returned %T, want *exporter", ov)
	}
	return e
}

func TestExportFactoryNoFrame(t *testing.T) {
	if _, err := ui.Factories["export"](&exportStubCtx{}, ""); err == nil {
		t.Fatal("expected error with nil frame")
	}
}

func TestExportWizardCSV(t *testing.T) {
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "mytab"}
	e := exportNew(t, ctx)

	if e.step != exportStepFormat {
		t.Fatalf("initial step = %d, want format", e.step)
	}
	// csv sorts first in the registry.
	if e.format() != "csv" {
		t.Fatalf("first format = %q, want csv", e.format())
	}
	e.handleKey("enter", "")
	if e.step != exportStepOptions {
		t.Fatalf("step = %d, want options", e.step)
	}

	// separator row focused: type ';'
	e.handleKey(";", ";")
	if e.sep != ';' {
		t.Fatalf("sep = %q, want ;", e.sep)
	}
	// move to header row and toggle it off
	e.handleKey("down", "")
	e.handleKey("space", " ")
	if e.header {
		t.Fatal("header should be off after toggle")
	}

	e.handleKey("enter", "")
	if e.step != exportStepPath {
		t.Fatalf("step = %d, want path", e.step)
	}
	if got := string(e.path); got != "mytab.csv" {
		t.Fatalf("prefill = %q, want mytab.csv", got)
	}

	dst := filepath.Join(t.TempDir(), "out.csv")
	e.handleKey("ctrl+u", "")
	e.handleKey(dst, dst) // paste-style insert
	msgs := exportDrain(e.handleKey("enter", ""))

	var toast, closed bool
	for _, m := range msgs {
		switch m := m.(type) {
		case ui.ToastMsg:
			toast = true
			if !strings.Contains(m.Text, dst) {
				t.Errorf("toast %q does not mention %q", m.Text, dst)
			}
		case ui.CloseOverlayMsg:
			closed = true
		case ui.ErrorMsg:
			t.Fatalf("unexpected error: %v", m.Err)
		}
	}
	if !toast || !closed {
		t.Fatalf("want toast+close, got %#v", msgs)
	}

	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "1;x\n2;y\n" {
		t.Fatalf("file content = %q", got)
	}
}

func TestExportOverwriteConfirm(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "exists.csv")
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}
	e := exportNew(t, ctx)
	e.step = exportStepPath
	e.path = []rune(dst)
	e.cursor = len(e.path)

	if cmd := e.handleKey("enter", ""); cmd != nil {
		t.Fatalf("expected confirm step, got messages %#v", exportDrain(cmd))
	}
	if e.step != exportStepConfirm {
		t.Fatalf("step = %d, want confirm", e.step)
	}

	// decline: back to path, file untouched
	e.handleKey("n", "n")
	if e.step != exportStepPath {
		t.Fatalf("step = %d, want path after decline", e.step)
	}
	if b, _ := os.ReadFile(dst); string(b) != "old" {
		t.Fatal("file overwritten despite decline")
	}

	// accept
	e.handleKey("enter", "")
	msgs := exportDrain(e.handleKey("y", "y"))
	var toast bool
	for _, m := range msgs {
		if _, ok := m.(ui.ToastMsg); ok {
			toast = true
		}
	}
	if !toast {
		t.Fatalf("want toast after overwrite, got %#v", msgs)
	}
	if b, _ := os.ReadFile(dst); string(b) == "old" {
		t.Fatal("file not overwritten after confirm")
	}
}

func TestExportPathErrorStaysOpen(t *testing.T) {
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}
	e := exportNew(t, ctx)
	e.step = exportStepPath
	e.path = []rune(filepath.Join(t.TempDir(), "no", "such", "dir", "x.csv"))
	e.cursor = len(e.path)

	msgs := exportDrain(e.handleKey("enter", ""))
	var errSeen bool
	for _, m := range msgs {
		switch m.(type) {
		case ui.ErrorMsg:
			errSeen = true
		case ui.CloseOverlayMsg:
			t.Fatal("overlay closed on path error")
		}
	}
	if !errSeen {
		t.Fatalf("want ErrorMsg, got %#v", msgs)
	}
	if e.step != exportStepPath {
		t.Fatalf("step = %d, want path (stay open)", e.step)
	}
}

func TestExportBackNavigation(t *testing.T) {
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}
	e := exportNew(t, ctx)

	// esc on step 1 closes
	msgs := exportDrain(e.handleKey("esc", ""))
	if len(msgs) != 1 {
		t.Fatalf("got %#v", msgs)
	}
	if _, ok := msgs[0].(ui.CloseOverlayMsg); !ok {
		t.Fatalf("esc on format step: got %#v, want CloseOverlayMsg", msgs[0])
	}

	// json: format -> options -> path, then esc walks back
	for e.format() != "json" {
		e.handleKey("down", "")
	}
	e.handleKey("enter", "")
	if e.step != exportStepOptions {
		t.Fatalf("step = %d, want options", e.step)
	}
	e.handleKey("space", " ")
	if !e.pretty {
		t.Fatal("pretty should toggle on")
	}
	e.handleKey("enter", "")
	if e.step != exportStepPath {
		t.Fatalf("step = %d, want path", e.step)
	}
	if got := string(e.path); got != "t.json" {
		t.Fatalf("prefill = %q, want t.json", got)
	}
	e.handleKey("esc", "")
	if e.step != exportStepOptions {
		t.Fatalf("esc from path: step = %d, want options", e.step)
	}
	e.handleKey("left", "")
	if e.step != exportStepFormat {
		t.Fatalf("left from options: step = %d, want format", e.step)
	}
}

func TestExportPathEditing(t *testing.T) {
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "ab"}
	e := exportNew(t, ctx)
	e.handleKey("enter", "") // csv -> options
	e.handleKey("enter", "") // -> path, prefill "ab.csv"

	e.handleKey("home", "")
	e.handleKey("right", "")
	e.handleKey("x", "x") // a x b.csv
	if got := string(e.path); got != "axb.csv" {
		t.Fatalf("insert: %q", got)
	}
	e.handleKey("backspace", "")
	if got := string(e.path); got != "ab.csv" {
		t.Fatalf("backspace: %q", got)
	}
	e.handleKey("delete", "")
	if got := string(e.path); got != "a.csv" {
		t.Fatalf("delete: %q", got)
	}
	e.handleKey("end", "")
	if e.cursor != len(e.path) {
		t.Fatalf("cursor = %d, want %d", e.cursor, len(e.path))
	}

	// empty path refused with inline hint
	e.handleKey("ctrl+u", "")
	if cmd := e.handleKey("enter", ""); cmd != nil {
		t.Fatalf("empty path should not submit, got %#v", exportDrain(cmd))
	}
	if e.errText == "" {
		t.Fatal("expected inline error for empty path")
	}
}

func TestExportExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := exportExpandPath("~/x.csv"); got != filepath.Join(home, "x.csv") {
		t.Fatalf("expand = %q", got)
	}
	if got := exportExpandPath("~"); got != home {
		t.Fatalf("expand ~ = %q", got)
	}
	if got := exportExpandPath("plain.csv"); got != "plain.csv" {
		t.Fatalf("plain path changed: %q", got)
	}
	if got := exportExpandPath("~user/x"); got != "~user/x" {
		t.Fatalf("~user should be untouched: %q", got)
	}
}

func TestExportParquetCompressionPick(t *testing.T) {
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}
	e := exportNew(t, ctx)
	for e.format() != "parquet" {
		e.handleKey("down", "")
	}
	e.handleKey("enter", "")
	if e.step != exportStepOptions {
		t.Fatalf("step = %d, want options", e.step)
	}
	if exportCompressions[e.compSel] != "snappy" {
		t.Fatalf("default compression = %q, want snappy", exportCompressions[e.compSel])
	}
	e.handleKey("down", "")
	if exportCompressions[e.compSel] != "gzip" {
		t.Fatalf("compression = %q, want gzip", exportCompressions[e.compSel])
	}
	e.handleKey("enter", "")
	if got := string(e.path); got != "t.parquet" {
		t.Fatalf("prefill = %q, want t.parquet", got)
	}
}

func TestExportViewSmoke(t *testing.T) {
	th := exportTestTheme()
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}
	e := exportNew(t, ctx)
	for _, step := range []exportStep{exportStepFormat, exportStepOptions, exportStepPath, exportStepConfirm} {
		e.step = step
		if v := e.View(80, 24, th); v == "" {
			t.Fatalf("empty view for step %d", step)
		}
	}
}

func TestExportPaste(t *testing.T) {
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "people"}
	e := exportNew(t, ctx)

	// The format page ignores pastes.
	e.Update(tea.PasteMsg{Content: "zzz"})
	if e.step != exportStepFormat {
		t.Fatalf("format page consumed paste: step %d", e.step)
	}

	// Path page: paste inserts at the cursor as one edit, flattened.
	e.step = exportStepPath
	e.path = []rune(".csv")
	e.cursor = 0
	e.errText = "old"
	ov, cmd := e.Update(tea.PasteMsg{Content: "out\r\nfile"})
	if ov != ui.Overlay(e) || cmd != nil {
		t.Fatal("paste replaced the overlay or produced a command")
	}
	if got := string(e.path); got != "out file.csv" {
		t.Fatalf("path after paste = %q", got)
	}
	if e.cursor != len([]rune("out file")) || e.errText != "" {
		t.Fatalf("cursor=%d errText=%q", e.cursor, e.errText)
	}

	// Options page: pasting one character (with clipboard newline) sets the
	// csv separator; multi-character pastes are rejected.
	e2 := exportNew(t, ctx)
	for i, f := range e2.formats {
		if f == "csv" {
			e2.fmtSel = i
		}
	}
	e2.step = exportStepOptions
	e2.optSel = 0
	e2.Update(tea.PasteMsg{Content: "|\n"})
	if e2.sep != '|' {
		t.Fatalf("sep after paste = %q, want |", e2.sep)
	}
	e2.Update(tea.PasteMsg{Content: "ab"})
	if e2.sep != '|' {
		t.Fatalf("multi-char paste changed sep: %q", e2.sep)
	}

	// Header row focused: paste does not toggle or set anything.
	e2.optSel = 1
	header := e2.header
	e2.Update(tea.PasteMsg{Content: ";"})
	if e2.sep != '|' || e2.header != header {
		t.Fatal("paste on header row changed options")
	}
}
