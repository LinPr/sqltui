package popup

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/ui"
)

// refreshFakeBackend embeds qxFakeBackend and overrides FetchTable to record
// the call and return a small frame, so the refresh overlay test can assert
// what was fetched and observe the resulting ReplaceBaseMsg.
type refreshFakeBackend struct {
	*qxFakeBackend
	frame    *data.Frame
	fetchNS  string
	fetchTbl string
	fetchLim int
	called   bool
}

func (b *refreshFakeBackend) FetchTable(ns, table string, limit int) (*data.Frame, error) {
	b.called = true
	b.fetchNS, b.fetchTbl, b.fetchLim = ns, table, limit
	return b.frame, nil
}

// refreshFakeBackendErr embeds qxFakeBackend and overrides FetchTable to
// always fail, for the error path.
type refreshFakeBackendErr struct {
	*qxFakeBackend
	err error
}

func (b *refreshFakeBackendErr) FetchTable(string, string, int) (*data.Frame, error) {
	return nil, b.err
}

// refreshDrain runs a command tree (expanding tea.Batch and tea.Sequence
// messages, the latter being unexported) and collects all leaf messages. It
// mirrors the reflective unwrap in schema_test.go's schemaTestMsgs.
func refreshDrain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, refreshDrain(c)...)
		}
		return out
	}
	if rv := reflect.ValueOf(msg); rv.IsValid() && rv.Kind() == reflect.Slice &&
		strings.Contains(rv.Type().String(), "sequenceMsg") {
		var out []tea.Msg
		for i := 0; i < rv.Len(); i++ {
			if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
				out = append(out, refreshDrain(c)...)
			}
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func TestRefreshFactoryRequiresConnection(t *testing.T) {
	if _, err := ui.Factories["refreshtable"](&qxCtx{}, ""); err == nil {
		t.Fatal("expected error without a live connection")
	}
}

func TestRefreshFactoryAndDispatch(t *testing.T) {
	frame := qxTestFrame()
	be := &refreshFakeBackend{qxFakeBackend: &qxFakeBackend{}, frame: frame}
	ctx := &qxCtx{backend: be, base: "users"}

	// Palette lists the command exactly once.
	count := 0
	for _, c := range Commands {
		if c.Name == "refreshtable" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("refreshtable listed %d times, want 1", count)
	}

	ov, err := ui.Factories["refreshtable"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	ro := ov.(*refreshTableOverlay)
	if ro.ns != "" || ro.table != "users" {
		t.Fatalf("overlay state = ns=%q table=%q, want ns=\"\" table=\"users\"", ro.ns, ro.table)
	}

	// Init kicks off the async fetch with the pane's namespace/table and the
	// refresh limit (200).
	loaded := runRefreshCmd(ro.Init()).(refreshTableMsg)
	if loaded.err != nil || loaded.frame != frame {
		t.Fatalf("unexpected load result: %+v", loaded)
	}
	if !be.called || be.fetchNS != "" || be.fetchTbl != "users" || be.fetchLim != refreshLimit {
		t.Fatalf("FetchTable(%q, %q, %d) called=%v, want (\"\", \"users\", %d)",
			be.fetchNS, be.fetchTbl, be.fetchLim, be.called, refreshLimit)
	}

	// The success result closes the overlay, emits a ReplaceBaseMsg carrying
	// the frame bound for the active pane, and toasts "refreshed".
	_, cmd := ro.Update(loaded)
	msgs := refreshDrain(cmd)
	var sawClose, sawReplace, sawToast bool
	for _, m := range msgs {
		switch v := m.(type) {
		case ui.CloseOverlayMsg:
			sawClose = true
		case ui.ReplaceBaseMsg:
			sawReplace = true
			if v.Frame == nil {
				t.Errorf("ReplaceBaseMsg carried a nil frame")
			}
			if v.Frame != frame {
				t.Errorf("ReplaceBaseMsg frame mismatch: got %p, want %p", v.Frame, frame)
			}
		case ui.ToastMsg:
			sawToast = true
			if v.Text != "refreshed" {
				t.Errorf("toast = %q, want \"refreshed\"", v.Text)
			}
		}
	}
	if !sawClose || !sawReplace || !sawToast {
		t.Fatalf("dispatch produced msgs=%v, want CloseOverlay+ReplaceBaseMsg+ToastMsg", msgs)
	}
}

func TestRefreshFetchErrorSurfaces(t *testing.T) {
	boom := &errString{s: "fetch failed"}
	be := &refreshFakeBackendErr{qxFakeBackend: &qxFakeBackend{}, err: boom}
	ctx := &qxCtx{backend: be, base: "users"}

	ov, err := ui.Factories["refreshtable"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	ro := ov.(*refreshTableOverlay)
	loaded := runRefreshCmd(ro.Init()).(refreshTableMsg)
	if loaded.err == nil {
		t.Fatal("expected fetch error")
	}
	_, cmd := ro.Update(loaded)
	msgs := refreshDrain(cmd)
	var sawErr bool
	for _, m := range msgs {
		if em, ok := m.(ui.ErrorMsg); ok && em.Err != nil {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatalf("error dispatch produced msgs=%v, want ErrorMsg", msgs)
	}
}

func TestRefreshEscCloses(t *testing.T) {
	be := &refreshFakeBackend{qxFakeBackend: &qxFakeBackend{}, frame: qxTestFrame()}
	ctx := &qxCtx{backend: be, base: "users"}
	ov, _ := ui.Factories["refreshtable"](ctx, "")
	_, cmd := ov.Update(qxKey("esc"))
	if _, ok := runRefreshCmd(cmd).(ui.CloseOverlayMsg); !ok {
		t.Fatal("esc must close the refresh overlay")
	}
}

type errString struct{ s string }

func (e *errString) Error() string { return e.s }

// runRefreshCmd is a nil-safe helper to run a tea.Cmd and get its leaf message
// (the refresh overlay's Init returns a single fetch command, never a batch or
// sequence).
func runRefreshCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}
