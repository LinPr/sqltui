package popup

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["goto"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return newGotoPrompt(arg), nil
	}
}

// gotoPrompt is a one-line numeric prompt; enter jumps to the entered
// 1-based row number.
type gotoPrompt struct {
	input  string // digits only
	cursor int    // insertion point in bytes (digits are 1 byte each)
}

func newGotoPrompt(arg string) *gotoPrompt {
	p := &gotoPrompt{input: gotoDigits(arg)}
	p.cursor = len(p.input)
	return p
}

// gotoDigits keeps only ASCII digits from s.
func gotoDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// gotoKey applies one key press (string form plus printable text). It
// returns a command to emit alongside closing, and whether to close.
func (p *gotoPrompt) gotoKey(key, text string) (cmd tea.Cmd, close bool) {
	switch key {
	case "esc", "ctrl+c":
		return nil, true
	case "enter":
		if p.input == "" {
			return nil, true
		}
		n, err := strconv.Atoi(p.input)
		if err != nil {
			// Absurdly long digit strings overflow int; jump to the end.
			n = int(^uint(0) >> 1)
		}
		row := max(0, n-1)
		return func() tea.Msg { return ui.JumpToRowMsg{Row: row} }, true
	case "backspace":
		if p.cursor > 0 {
			p.input = p.input[:p.cursor-1] + p.input[p.cursor:]
			p.cursor--
		}
		return nil, false
	case "delete":
		if p.cursor < len(p.input) {
			p.input = p.input[:p.cursor] + p.input[p.cursor+1:]
		}
		return nil, false
	case "left":
		p.cursor = max(0, p.cursor-1)
		return nil, false
	case "right":
		p.cursor = min(len(p.input), p.cursor+1)
		return nil, false
	case "home", "ctrl+a":
		p.cursor = 0
		return nil, false
	case "end", "ctrl+e":
		p.cursor = len(p.input)
		return nil, false
	}
	if d := gotoDigits(text); d != "" {
		p.input = p.input[:p.cursor] + d + p.input[p.cursor:]
		p.cursor += len(d)
	}
	return nil, false
}

func (p *gotoPrompt) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if pm, ok := msg.(tea.PasteMsg); ok {
		// Pasted text is filtered to digits, then inserted at the cursor as
		// one edit (bracketed paste arrives as a single message).
		if d := gotoDigits(pm.Content); d != "" {
			p.input = p.input[:p.cursor] + d + p.input[p.cursor:]
			p.cursor += len(d)
		}
		return p, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	cmd, close := p.gotoKey(key.String(), key.Text)
	if close {
		if cmd != nil {
			return p, tea.Batch(cmd, ui.CloseOverlay)
		}
		return p, ui.CloseOverlay
	}
	return p, cmd
}

func (p *gotoPrompt) View(width, height int, th *theme.Theme) string {
	boxW := 30
	if boxW > width-4 && width-4 >= 10 {
		boxW = width - 4
	}

	// Render the input with a block cursor at the insertion point.
	before := p.input[:p.cursor]
	var at, after string
	if p.cursor < len(p.input) {
		at = p.input[p.cursor : p.cursor+1]
		after = p.input[p.cursor+1:]
	} else {
		at = " "
	}
	line := " " + th.Subtle.Render("go to row:") + " " +
		th.Input.Render(before) +
		th.ListSelected.Render(at) +
		th.Input.Render(after)

	return ui.Box("goto", line, boxW, th)
}
