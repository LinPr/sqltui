package popup

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/ui"
)

// registerCollect runs cmd (unwrapping batches) and returns the messages.
func registerCollect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch m := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, registerCollect(c)...)
		}
	default:
		out = append(out, m)
	}
	return out
}

func TestRegisterValidate(t *testing.T) {
	if registerValidate("people") != "" {
		t.Error("plain name must validate")
	}
	if registerValidate("") == "" {
		t.Error("empty name must be rejected")
	}
	if registerValidate("_") == "" {
		t.Error(`"_" must be rejected`)
	}
	if registerValidate("_x") != "" {
		t.Error(`names merely starting with "_" are allowed`)
	}
}

func TestRegisterTyping(t *testing.T) {
	p := &registerPrompt{}
	for _, s := range []string{"m", "y", "t", "a", "b"} {
		p.registerKey(s, s)
	}
	if string(p.input) != "mytab" {
		t.Fatalf("input = %q, want mytab", string(p.input))
	}
	p.registerKey("backspace", "")
	p.registerKey("home", "")
	p.registerKey("X", "X")
	if string(p.input) != "Xmyta" {
		t.Fatalf("input = %q, want Xmyta", string(p.input))
	}
	p.registerKey("ctrl+u", "")
	if string(p.input) != "myta" || p.cursor != 0 {
		t.Fatalf("after ctrl+u: input=%q cursor=%d", string(p.input), p.cursor)
	}
}

func TestRegisterEnterInvalidStaysOpen(t *testing.T) {
	p := &registerPrompt{}
	cmd, close := p.registerKey("enter", "")
	if close || cmd != nil {
		t.Fatal("enter on empty input must stay open with an inline error")
	}
	if p.errMsg == "" {
		t.Fatal("expected inline error message")
	}

	p = &registerPrompt{input: []rune("_"), cursor: 1}
	if _, close := p.registerKey("enter", ""); close {
		t.Fatal(`enter on "_" must stay open`)
	}
}

func TestRegisterEnterValid(t *testing.T) {
	p := &registerPrompt{input: []rune("  people  "), cursor: 10}
	cmd, close := p.registerKey("enter", "")
	if !close {
		t.Fatal("valid enter must close")
	}
	msgs := registerCollect(cmd)
	if len(msgs) != 1 {
		t.Fatalf("registerKey produced %d messages, want 1", len(msgs))
	}
	reg, ok := msgs[0].(ui.RegisterTableMsg)
	if !ok {
		t.Fatalf("got %T, want ui.RegisterTableMsg", msgs[0])
	}
	if reg.Name != "people" {
		t.Errorf("name = %q, want trimmed %q", reg.Name, "people")
	}
}

func TestRegisterUpdateEmitsToastAndClose(t *testing.T) {
	p := &registerPrompt{input: []rune("people"), cursor: 6}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msgs := registerCollect(cmd)

	var gotRegister, gotToast, gotClose bool
	for _, m := range msgs {
		switch m.(type) {
		case ui.RegisterTableMsg:
			gotRegister = true
		case ui.ToastMsg:
			gotToast = true
		case ui.CloseOverlayMsg:
			gotClose = true
		}
	}
	if !gotRegister || !gotToast || !gotClose {
		t.Fatalf("register=%v toast=%v close=%v, want all true", gotRegister, gotToast, gotClose)
	}
}

func TestRegisterPaste(t *testing.T) {
	p := &registerPrompt{errMsg: "old"}

	// The paste lands as one edit, flattened to a single line, and clears
	// the inline error.
	ov, cmd := p.Update(tea.PasteMsg{Content: "my\r\ntable"})
	if ov != ui.Overlay(p) || cmd != nil {
		t.Fatal("paste replaced the overlay or produced a command")
	}
	if got := string(p.input); got != "my table" {
		t.Fatalf("input = %q, want %q", got, "my table")
	}
	if p.cursor != len([]rune("my table")) || p.errMsg != "" {
		t.Fatalf("cursor=%d errMsg=%q", p.cursor, p.errMsg)
	}

	// Paste inserts at the cursor.
	p.registerKey("home", "")
	p.Update(tea.PasteMsg{Content: "x_"})
	if got := string(p.input); got != "x_my table" {
		t.Fatalf("cursor paste: %q", got)
	}

	// Control-only paste is a no-op and keeps the error message.
	p.errMsg = "kept"
	p.Update(tea.PasteMsg{Content: "\r"})
	if string(p.input) != "x_my table" || p.errMsg != "kept" {
		t.Fatalf("empty paste changed state: %q err=%q", string(p.input), p.errMsg)
	}
}
