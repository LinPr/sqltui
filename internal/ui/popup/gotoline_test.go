package popup

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/ui"
)

// gotoCollect runs cmd (unwrapping batches) and returns the produced messages.
func gotoCollect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch m := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, gotoCollect(c)...)
		}
	default:
		out = append(out, m)
	}
	return out
}

func gotoJumpRow(t *testing.T, cmd tea.Cmd) int {
	t.Helper()
	msgs := gotoCollect(cmd)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	jump, ok := msgs[0].(ui.JumpToRowMsg)
	if !ok {
		t.Fatalf("got %T, want ui.JumpToRowMsg", msgs[0])
	}
	return jump.Row
}

func TestGotoPrefill(t *testing.T) {
	p := newGotoPrompt("5")
	if p.input != "5" || p.cursor != 1 {
		t.Fatalf("prefill: input=%q cursor=%d, want %q 1", p.input, p.cursor, "5")
	}
	// Non-digits in arg are dropped.
	p = newGotoPrompt("x7y")
	if p.input != "7" {
		t.Fatalf("prefill filter: input=%q, want %q", p.input, "7")
	}
}

func TestGotoTypingAndEnter(t *testing.T) {
	p := newGotoPrompt("1")
	p.gotoKey("2", "2")
	p.gotoKey("a", "a") // non-digit ignored
	if p.input != "12" {
		t.Fatalf("input = %q, want %q", p.input, "12")
	}
	cmd, close := p.gotoKey("enter", "")
	if !close {
		t.Fatal("enter must close")
	}
	if row := gotoJumpRow(t, cmd); row != 11 {
		t.Errorf("row = %d, want 11 (1-based 12)", row)
	}
}

func TestGotoRowClampedAtZero(t *testing.T) {
	p := newGotoPrompt("0")
	cmd, close := p.gotoKey("enter", "")
	if !close {
		t.Fatal("enter must close")
	}
	if row := gotoJumpRow(t, cmd); row != 0 {
		t.Errorf("row = %d, want 0 (clamped)", row)
	}
}

func TestGotoEditing(t *testing.T) {
	p := newGotoPrompt("123")
	p.gotoKey("backspace", "")
	if p.input != "12" || p.cursor != 2 {
		t.Fatalf("after backspace: input=%q cursor=%d", p.input, p.cursor)
	}
	p.gotoKey("home", "")
	p.gotoKey("9", "9")
	if p.input != "912" || p.cursor != 1 {
		t.Fatalf("after home+9: input=%q cursor=%d", p.input, p.cursor)
	}
	p.gotoKey("right", "")
	p.gotoKey("backspace", "")
	if p.input != "92" {
		t.Fatalf("after right+backspace: input=%q, want 92", p.input)
	}
	p.gotoKey("end", "")
	if p.cursor != 2 {
		t.Fatalf("after end: cursor=%d, want 2", p.cursor)
	}
	p.gotoKey("delete", "") // at end: no-op
	if p.input != "92" {
		t.Fatalf("after delete at end: input=%q, want 92", p.input)
	}
}

func TestGotoEmptyEnterAndEscJustClose(t *testing.T) {
	p := newGotoPrompt("")
	cmd, close := p.gotoKey("enter", "")
	if !close || cmd != nil {
		t.Fatalf("empty enter: close=%v cmd=%v, want plain close", close, cmd)
	}
	p = newGotoPrompt("4")
	cmd, close = p.gotoKey("esc", "")
	if !close || cmd != nil {
		t.Fatalf("esc: close=%v cmd=%v, want plain close", close, cmd)
	}
}

func TestGotoPaste(t *testing.T) {
	p := newGotoPrompt("")

	// Only digits survive a paste; the chunk lands as one edit.
	ov, cmd := p.Update(tea.PasteMsg{Content: "1x2\r\n3"})
	if ov != ui.Overlay(p) || cmd != nil {
		t.Fatal("paste replaced the overlay or produced a command")
	}
	if p.input != "123" || p.cursor != 3 {
		t.Fatalf("after paste: input=%q cursor=%d", p.input, p.cursor)
	}

	// Paste inserts at the cursor.
	p.gotoKey("left", "")
	p.Update(tea.PasteMsg{Content: "9"})
	if p.input != "1293" || p.cursor != 3 {
		t.Fatalf("cursor paste: input=%q cursor=%d", p.input, p.cursor)
	}

	// A digit-free paste is a no-op.
	p.Update(tea.PasteMsg{Content: "abc\n"})
	if p.input != "1293" {
		t.Fatalf("digit-free paste changed input: %q", p.input)
	}

	// Enter jumps to the pasted row (1-based -> 0-based).
	cmd, close := p.gotoKey("enter", "")
	if !close {
		t.Fatal("enter should close")
	}
	if row := gotoJumpRow(t, cmd); row != 1292 {
		t.Fatalf("row = %d, want 1292", row)
	}
}
