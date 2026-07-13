package popup

import (
	"strings"
	"testing"

	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/ui"
)

func TestConfirmDeleteFactoryRegistered(t *testing.T) {
	f, ok := ui.Factories["confirmdelete"]
	if !ok {
		t.Fatal("confirmdelete factory not registered")
	}
	if _, err := f(&confirmTestCtx{}, ""); err == nil {
		t.Error("expected error when no pending delete is set")
	}
	pd := &ui.PendingDelete{Rows: []int{1, 2, 3}, Table: "users"}
	ov, err := f(&confirmTestCtx{pd: pd}, "")
	if err != nil || ov == nil {
		t.Fatalf("factory: overlay=%v err=%v", ov, err)
	}
}

func TestConfirmDeletePaletteListedOnce(t *testing.T) {
	count := 0
	for _, c := range Commands {
		if c.Name == "confirmdelete" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("confirmdelete listed %d times, want 1", count)
	}
}

func TestConfirmDeleteConfirmSendsDeleterows(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{0, 4}, Table: "users"}
	ctx := &confirmTestCtx{pd: pd}
	ov, err := ui.Factories["confirmdelete"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	for _, key := range []string{"y", "Y", "enter"} {
		_, cmd := ov.Update(qxKey(key))
		msgs := confirmDrain(cmd)
		if !confirmHasClose(msgs) {
			t.Errorf("key %q: no CloseOverlayMsg in %v", key, msgs)
		}
		if !confirmHasRunCmd(msgs, "deleterows") {
			t.Errorf("key %q: no RunCommandMsg{deleterows} in %v", key, msgs)
		}
	}
}

func TestConfirmDeleteCancelSendsCanceldelete(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{0}, Table: "users"}
	ctx := &confirmTestCtx{pd: pd}
	ov, err := ui.Factories["confirmdelete"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	for _, key := range []string{"n", "N", "esc", "ctrl+c"} {
		_, cmd := ov.Update(qxKey(key))
		msgs := confirmDrain(cmd)
		if !confirmHasClose(msgs) {
			t.Errorf("key %q: no CloseOverlayMsg in %v", key, msgs)
		}
		if !confirmHasRunCmd(msgs, "canceldelete") {
			t.Errorf("key %q: no RunCommandMsg{canceldelete} in %v", key, msgs)
		}
	}
}

func TestConfirmDeleteIgnoresOtherKeys(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{0}, Table: "users"}
	ctx := &confirmTestCtx{pd: pd}
	ov, _ := ui.Factories["confirmdelete"](ctx, "")
	for _, key := range []string{"x", "1", "down", "up"} {
		_, cmd := ov.Update(qxKey(key))
		if cmd != nil {
			t.Errorf("key %q produced an unexpected cmd: %v", key, cmd)
		}
	}
}

func TestConfirmDeleteViewContent(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{1, 2, 3}, Table: "users"}
	ctx := &confirmTestCtx{pd: pd, be: &errConfirmBackend{}}
	ov, _ := ui.Factories["confirmdelete"](ctx, "")
	out := ov.View(80, 24, qxTestTheme())
	for _, want := range []string{"Delete 3 row(s)?", "from", "users", "y confirm"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestConfirmDeleteViewHidesTableWhenFileMode(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{1}}
	ctx := &confirmTestCtx{pd: pd} // no backend: file mode
	ov, _ := ui.Factories["confirmdelete"](ctx, "")
	out := ov.View(80, 24, qxTestTheme())
	if strings.Contains(out, "from users") {
		t.Errorf("file-mode view must not show 'from <table>': %q", out)
	}
	if !strings.Contains(out, "Delete 1 row(s)?") {
		t.Errorf("view missing question: %q", out)
	}
}

func TestConfirmDeleteViewHidesTableWhenTableEmpty(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{1}, Table: ""}
	ctx := &confirmTestCtx{pd: pd, be: &errConfirmBackend{}}
	ov, _ := ui.Factories["confirmdelete"](ctx, "")
	out := ov.View(80, 24, qxTestTheme())
	if strings.Contains(out, "from ") {
		t.Errorf("db-mode view must not show 'from' when table is empty: %q", out)
	}
}

func TestConfirmDeleteViewGuardsSmallSize(t *testing.T) {
	pd := &ui.PendingDelete{Rows: []int{1}, Table: "users"}
	ctx := &confirmTestCtx{pd: pd, be: &errConfirmBackend{}}
	ov, _ := ui.Factories["confirmdelete"](ctx, "")
	_ = ov.View(10, 4, qxTestTheme())
}

func TestConfirmDeleteOverlayInterface(t *testing.T) {
	var _ ui.Overlay = (*confirmDeleteOverlay)(nil)
}

// Compile-time check that confirmTestCtx satisfies the full AppContext
// interface (a stale stub would surface as a compile error here).
var _ ui.AppContext = (*confirmTestCtx)(nil)

// Silence unused-import for db when only the interface assertion above runs.
var _ db.Backend = (*errConfirmBackend)(nil)
