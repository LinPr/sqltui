package popup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["register"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		if ctx.Engine() == nil {
			return nil, fmt.Errorf("register: no embedded SQL engine (only available in file mode)")
		}
		if ctx.CurrentFrame() == nil {
			return nil, fmt.Errorf("register: no table open")
		}
		p := &registerPrompt{input: []rune(strings.TrimSpace(arg))}
		p.cursor = len(p.input)
		return p, nil
	}
}

// registerPrompt asks for a table name to register the current frame under
// in the embedded SQL engine.
type registerPrompt struct {
	input  []rune
	cursor int // insertion point in runes
	errMsg string
}

// registerValidate checks a candidate table name; empty result means valid.
func registerValidate(name string) string {
	switch {
	case name == "":
		return "name must not be empty"
	case name == "_":
		return `"_" is reserved for the current frame`
	default:
		return ""
	}
}

// registerKey applies one key press. It returns a command to emit alongside
// closing, and whether to close.
func (p *registerPrompt) registerKey(key, text string) (cmd tea.Cmd, close bool) {
	switch key {
	case "esc", "ctrl+c":
		return nil, true
	case "enter":
		name := strings.TrimSpace(string(p.input))
		if msg := registerValidate(name); msg != "" {
			p.errMsg = msg
			return nil, false
		}
		return func() tea.Msg { return ui.RegisterTableMsg{Name: name} }, true
	case "backspace":
		if p.cursor > 0 {
			p.input = append(p.input[:p.cursor-1], p.input[p.cursor:]...)
			p.cursor--
		}
	case "delete":
		if p.cursor < len(p.input) {
			p.input = append(p.input[:p.cursor], p.input[p.cursor+1:]...)
		}
	case "left":
		p.cursor = max(0, p.cursor-1)
	case "right":
		p.cursor = min(len(p.input), p.cursor+1)
	case "home", "ctrl+a":
		p.cursor = 0
	case "end", "ctrl+e":
		p.cursor = len(p.input)
	case "ctrl+u":
		p.input = append([]rune{}, p.input[p.cursor:]...)
		p.cursor = 0
	default:
		if text != "" && !strings.ContainsAny(text, "\n\r") {
			r := []rune(text)
			p.input = append(p.input[:p.cursor], append(r, p.input[p.cursor:]...)...)
			p.cursor += len(r)
			p.errMsg = ""
		}
	}
	return nil, false
}

func (p *registerPrompt) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if pm, ok := msg.(tea.PasteMsg); ok {
		// Insert the paste at the cursor as one edit, flattened to one line.
		if text := []rune(pasteSanitize(pm.Content)); len(text) > 0 {
			p.input = append(p.input[:p.cursor], append(text, p.input[p.cursor:]...)...)
			p.cursor += len(text)
			p.errMsg = ""
		}
		return p, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	cmd, close := p.registerKey(key.String(), key.Text)
	if close {
		if cmd != nil {
			name := strings.TrimSpace(string(p.input))
			toast := func() tea.Msg {
				return ui.ToastMsg{Text: fmt.Sprintf("registered table %q", name)}
			}
			return p, tea.Batch(cmd, toast, ui.CloseOverlay)
		}
		return p, ui.CloseOverlay
	}
	return p, cmd
}

func (p *registerPrompt) View(width, height int, th *theme.Theme) string {
	boxW := 44
	if boxW > width-4 && width-4 >= 20 {
		boxW = width - 4
	}
	inner := boxW - 2

	before := string(p.input[:p.cursor])
	var at, after string
	if p.cursor < len(p.input) {
		at = string(p.input[p.cursor])
		after = string(p.input[p.cursor+1:])
	} else {
		at = " "
	}
	line := " " + th.Subtle.Render("table name:") + " " +
		th.Input.Render(before) +
		th.ListSelected.Render(at) +
		th.Input.Render(after)

	body := line
	if p.errMsg != "" {
		body += "\n" + th.Error.Render(ansi.Truncate(" "+p.errMsg, inner, "…"))
	} else {
		body += "\n" + th.Subtle.Render(" enter register  •  esc cancel")
	}
	return ui.Box("register", body, boxW, th)
}
