package popup

import (
	"errors"
	"os"
	"testing"

	"github.com/LinPr/sqltui/internal/ui"
)

func TestEditFactoryNoFrame(t *testing.T) {
	if _, err := ui.Factories["edit"](&exportStubCtx{}, ""); err == nil {
		t.Fatal("expected error with nil frame")
	}
}

func TestEditEditorCmd(t *testing.T) {
	t.Setenv("EDITOR", "myeditor --wait")
	cmd := editEditorCmd("/tmp/f.csv")
	want := []string{"myeditor", "--wait", "/tmp/f.csv"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args = %v, want %v", cmd.Args, want)
		}
	}

	t.Setenv("EDITOR", "")
	cmd = editEditorCmd("/tmp/f.csv")
	if cmd.Args[0] != "vi" {
		t.Fatalf("fallback editor = %q, want vi", cmd.Args[0])
	}
}

func TestEditRunnerFlow(t *testing.T) {
	t.Setenv("EDITOR", "true")
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}
	ov, err := ui.Factories["edit"](ctx, "")
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	runner, ok := ov.(*editRunner)
	if !ok {
		t.Fatalf("factory returned %T, want *editRunner", ov)
	}
	if runner.View(80, 24, exportTestTheme()) != "" {
		t.Fatal("runner should be invisible")
	}

	// temp file holds the frame as CSV
	b, err := os.ReadFile(runner.tmp)
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if got := string(b); got != "a,b\n1,x\n2,y\n" {
		t.Fatalf("temp csv = %q", got)
	}

	// first Update emits exec + close
	_, cmd := runner.Update(struct{}{})
	msgs := exportDrain(cmd)
	var exe *ui.ExecProcessMsg
	var closed bool
	for _, m := range msgs {
		switch m := m.(type) {
		case ui.ExecProcessMsg:
			exe = &m
		case ui.CloseOverlayMsg:
			closed = true
		}
	}
	if exe == nil || !closed {
		t.Fatalf("want ExecProcessMsg+CloseOverlayMsg, got %#v", msgs)
	}
	if got := exe.Cmd.Args[len(exe.Cmd.Args)-1]; got != runner.tmp {
		t.Fatalf("editor arg = %q, want %q", got, runner.tmp)
	}
	if exe.Cmd.Args[0] != "true" {
		t.Fatalf("editor = %q, want true", exe.Cmd.Args[0])
	}

	// second Update is a no-op
	if _, cmd := runner.Update(struct{}{}); cmd != nil {
		t.Fatal("second Update should be inert")
	}

	// simulate the user editing the file, then the process finishing
	if err := os.WriteFile(runner.tmp, []byte("a,b\n7,z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg := exe.OnDone(nil)
	apply, ok := msg.(ui.ApplyFrameMsg)
	if !ok {
		t.Fatalf("OnDone = %#v, want ApplyFrameMsg", msg)
	}
	if apply.Crumb != "edit" {
		t.Fatalf("crumb = %q, want edit", apply.Crumb)
	}
	if apply.Frame.NumRows() != 1 || apply.Frame.NumCols() != 2 {
		t.Fatalf("frame shape = %d x %d", apply.Frame.NumRows(), apply.Frame.NumCols())
	}
	if v := apply.Frame.Cell(0, 0); v != int64(7) {
		t.Fatalf("cell(0,0) = %#v, want int64(7) (type inference)", v)
	}
	if v := apply.Frame.Cell(0, 1); v != "z" {
		t.Fatalf("cell(0,1) = %#v, want z", v)
	}
	if _, err := os.Stat(runner.tmp); !os.IsNotExist(err) {
		t.Fatal("temp file should be removed after reload")
	}
}

func TestEditOnDoneErrors(t *testing.T) {
	t.Setenv("EDITOR", "true")
	ctx := &exportStubCtx{frame: exportTestFrame(), crumb: "t"}

	// editor process failure surfaces as ErrorMsg
	ov, err := ui.Factories["edit"](ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	runner := ov.(*editRunner)
	if m := editReload(runner.tmp, 0, errors.New("boom")); func() bool {
		_, ok := m.(ui.ErrorMsg)
		return !ok
	}() {
		t.Fatalf("got %#v, want ErrorMsg", m)
	}

	// unreadable temp file surfaces as ErrorMsg
	if m := editReload("/nonexistent/edit.csv", 0, nil); func() bool {
		_, ok := m.(ui.ErrorMsg)
		return !ok
	}() {
		t.Fatalf("got %#v, want ErrorMsg", m)
	}
}
