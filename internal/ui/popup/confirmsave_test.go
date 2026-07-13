package popup

import (
	"errors"
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

// confirmTestCtx is a self-contained AppContext stub for the confirm overlay
// tests. It holds the PendingEdit/PendingDelete values the factories read.
type confirmTestCtx struct {
	pe *ui.PendingEdit
	pd *ui.PendingDelete
	be db.Backend
}

func (c *confirmTestCtx) CurrentFrame() *data.Frame        { return nil }
func (c *confirmTestCtx) CurrentRow() int                  { return 0 }
func (c *confirmTestCtx) SheetFieldCursor() int            { return 0 }
func (c *confirmTestCtx) CurrentTableNamespace() string    { return "" }
func (c *confirmTestCtx) BaseCrumb() string                { return "users" }
func (c *confirmTestCtx) Crumbs() []string                 { return nil }
func (c *confirmTestCtx) ColumnNames() []string            { return nil }
func (c *confirmTestCtx) Engine() *query.Engine            { return nil }
func (c *confirmTestCtx) TableNames() []string             { return nil }
func (c *confirmTestCtx) Backend() db.Backend              { return c.be }
func (c *confirmTestCtx) KV() db.KVBackend                 { return nil }
func (c *confirmTestCtx) Theme() *theme.Theme              { return nil }
func (c *confirmTestCtx) ThemeName() string                { return "" }
func (c *confirmTestCtx) ShowBorders() bool                { return false }
func (c *confirmTestCtx) ShowRowNumbers() bool             { return false }
func (c *confirmTestCtx) Tabs() []ui.TabInfo               { return nil }
func (c *confirmTestCtx) ActiveTab() int                   { return 0 }
func (c *confirmTestCtx) ActivePaneID() int                { return 0 }
func (c *confirmTestCtx) PendingEdit() *ui.PendingEdit     { return c.pe }
func (c *confirmTestCtx) PendingDelete() *ui.PendingDelete { return c.pd }

// confirmDrain runs a command tree, expanding tea.Batch and tea.Sequence, and
// collects the leaf messages. Mirrors the reflective unwrap in schema_test.go.
func confirmDrain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, confirmDrain(c)...)
		}
		return out
	}
	if rv := reflect.ValueOf(msg); rv.IsValid() && rv.Kind() == reflect.Slice &&
		strings.Contains(rv.Type().String(), "sequenceMsg") {
		var out []tea.Msg
		for i := 0; i < rv.Len(); i++ {
			if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
				out = append(out, confirmDrain(c)...)
			}
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

// confirmHas reports whether the drained messages contain one of the given
// (type, name) RunCommandMsgs.
func confirmHasRunCmd(msgs []tea.Msg, name string) bool {
	for _, m := range msgs {
		if rc, ok := m.(ui.RunCommandMsg); ok && rc.Name == name {
			return true
		}
	}
	return false
}

func confirmHasClose(msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(ui.CloseOverlayMsg); ok {
			return true
		}
	}
	return false
}

func TestConfirmSaveFactoryRegistered(t *testing.T) {
	f, ok := ui.Factories["confirmsave"]
	if !ok {
		t.Fatal("confirmsave factory not registered")
	}
	if _, err := f(&confirmTestCtx{}, ""); err == nil {
		t.Error("expected error when no pending edit is set")
	}
	pe := &ui.PendingEdit{ColName: "name", OldValue: "ann", NewValue: "amy"}
	ov, err := f(&confirmTestCtx{pe: pe}, "")
	if err != nil || ov == nil {
		t.Fatalf("factory: overlay=%v err=%v", ov, err)
	}
}

func TestConfirmSavePaletteListedOnce(t *testing.T) {
	count := 0
	for _, c := range Commands {
		if c.Name == "confirmsave" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("confirmsave listed %d times, want 1", count)
	}
}

func TestConfirmSaveConfirmSendsSaveedit(t *testing.T) {
	pe := &ui.PendingEdit{ColName: "name", OldValue: "ann", NewValue: "amy"}
	ctx := &confirmTestCtx{pe: pe}
	ov, err := ui.Factories["confirmsave"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	for _, key := range []string{"y", "Y", "enter"} {
		_, cmd := ov.Update(qxKey(key))
		msgs := confirmDrain(cmd)
		if !confirmHasClose(msgs) {
			t.Errorf("key %q: no CloseOverlayMsg in %v", key, msgs)
		}
		if !confirmHasRunCmd(msgs, "saveedit") {
			t.Errorf("key %q: no RunCommandMsg{saveedit} in %v", key, msgs)
		}
	}
}

func TestConfirmSaveCancelSendsCanceledit(t *testing.T) {
	pe := &ui.PendingEdit{ColName: "name", OldValue: "ann", NewValue: "amy"}
	ctx := &confirmTestCtx{pe: pe}
	ov, err := ui.Factories["confirmsave"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	for _, key := range []string{"n", "N", "esc", "ctrl+c"} {
		_, cmd := ov.Update(qxKey(key))
		msgs := confirmDrain(cmd)
		if !confirmHasClose(msgs) {
			t.Errorf("key %q: no CloseOverlayMsg in %v", key, msgs)
		}
		if !confirmHasRunCmd(msgs, "canceledit") {
			t.Errorf("key %q: no RunCommandMsg{canceledit} in %v", key, msgs)
		}
	}
}

func TestConfirmSaveIgnoresOtherKeys(t *testing.T) {
	pe := &ui.PendingEdit{ColName: "name", OldValue: "ann", NewValue: "amy"}
	ctx := &confirmTestCtx{pe: pe}
	ov, _ := ui.Factories["confirmsave"](ctx, "")
	for _, key := range []string{"x", "1", "down", "up"} {
		_, cmd := ov.Update(qxKey(key))
		if cmd != nil {
			t.Errorf("key %q produced an unexpected cmd: %v", key, cmd)
		}
	}
}

func TestConfirmSaveViewContent(t *testing.T) {
	pe := &ui.PendingEdit{ColName: "name", OldValue: "ann", NewValue: "amy"}
	ctx := &confirmTestCtx{pe: pe}
	ov, _ := ui.Factories["confirmsave"](ctx, "")
	out := ov.View(80, 24, qxTestTheme())
	for _, want := range []string{"Save changes to name?", "old:", "ann", "new:", "amy", "y confirm"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestConfirmSaveViewTruncatesLongValues(t *testing.T) {
	pe := &ui.PendingEdit{
		ColName:  "name",
		OldValue: strings.Repeat("a", 200),
		NewValue: strings.Repeat("b", 200),
	}
	ctx := &confirmTestCtx{pe: pe}
	ov, _ := ui.Factories["confirmsave"](ctx, "")
	out := ov.View(40, 24, qxTestTheme())
	if !strings.Contains(out, "…") {
		t.Errorf("view did not truncate long values: %q", out)
	}
}

func TestConfirmSaveViewGuardsSmallSize(t *testing.T) {
	pe := &ui.PendingEdit{ColName: "name", OldValue: "ann", NewValue: "amy"}
	ctx := &confirmTestCtx{pe: pe}
	ov, _ := ui.Factories["confirmsave"](ctx, "")
	// Must not panic for tiny sizes.
	_ = ov.View(10, 4, qxTestTheme())
}

// errConfirmBackend is a stub backend for the delete view test.
type errConfirmBackend struct{}

func (b *errConfirmBackend) Kind() string                                        { return "test" }
func (b *errConfirmBackend) Title() string                                       { return "test" }
func (b *errConfirmBackend) Run(string) (db.Result, error)                       { return db.Result{}, errors.New("nope") }
func (b *errConfirmBackend) Namespaces() ([]string, error)                       { return nil, nil }
func (b *errConfirmBackend) Tables(string) ([]string, error)                     { return nil, nil }
func (b *errConfirmBackend) FetchTable(string, string, int) (*data.Frame, error) { return nil, nil }
func (b *errConfirmBackend) PrimaryKeys(string, string) ([]string, error)        { return nil, nil }
func (b *errConfirmBackend) Close() error                                        { return nil }

func TestConfirmSaveOverlayInterface(t *testing.T) {
	var _ ui.Overlay = (*confirmSaveOverlay)(nil)
}
