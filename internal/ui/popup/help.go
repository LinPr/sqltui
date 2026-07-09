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
	ui.Factories["help"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return newHelpOverlay(), nil
	}
}

// helpLine is one prerendered row of the help listing: either a section
// header (Section set, Keys empty) or a keys/description pair.
type helpLine struct {
	Section string
	Keys    string
	Text    string
}

// helpOverlay is a scrollable cheat sheet built from the exported binding
// lists plus the palette command table.
type helpOverlay struct {
	lines []helpLine

	offset   int // first visible line
	viewRows int // lines shown by the last View (page size)
}

func newHelpOverlay() *helpOverlay {
	return &helpOverlay{lines: helpBuildLines(), viewRows: 10}
}

// helpBuildLines flattens the binding lists and the command table into the
// scrollable line model. Kept separate from rendering so it can be tested.
func helpBuildLines() []helpLine {
	var lines []helpLine
	section := func(name string, bindings []ui.Binding) {
		lines = append(lines, helpLine{Section: name})
		for _, b := range bindings {
			lines = append(lines, helpLine{
				Keys: strings.Join(b.Keys, ", "),
				Text: b.Help,
			})
		}
	}
	section("Global", ui.GlobalBindings)
	section("Table", ui.TableBindings)
	section("Sheet", ui.SheetBindings)

	if len(Commands) > 0 {
		lines = append(lines, helpLine{Section: "Commands (:)"})
		for _, c := range Commands {
			names := append([]string{c.Name}, c.Aliases...)
			lines = append(lines, helpLine{
				Keys: strings.Join(names, ", "),
				Text: c.Description,
			})
		}
	}
	return lines
}

// helpKey applies one key (tea.KeyPressMsg.String() form); it reports
// whether the overlay should close.
func (o *helpOverlay) helpKey(key string) (close bool) {
	page := max(1, o.viewRows)
	switch key {
	case "q", "esc", "f1":
		return true
	case "up", "k":
		o.offset--
	case "down", "j":
		o.offset++
	case "pgup", "ctrl+u", "ctrl+b":
		o.offset -= page
	case "pgdown", "ctrl+d", "ctrl+f":
		o.offset += page
	case "g", "home":
		o.offset = 0
	case "G", "end":
		o.offset = len(o.lines) // clamped below
	}
	o.offset = helpClamp(o.offset, 0, max(0, len(o.lines)-page))
	return false
}

func (o *helpOverlay) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	if o.helpKey(key.String()) {
		return o, ui.CloseOverlay
	}
	return o, nil
}

func (o *helpOverlay) View(width, height int, th *theme.Theme) string {
	boxW := helpClamp(70, 30, max(30, width-4))
	inner := boxW - 2

	// Left column width: widest key string, capped to a third of the box.
	keyW := 0
	for _, l := range o.lines {
		if l.Section == "" {
			if w := ansi.StringWidth(l.Keys); w > keyW {
				keyW = w
			}
		}
	}
	keyW = helpClamp(keyW, 6, max(6, inner/3))

	avail := height - 2 - 1 // borders + footer hint
	if avail < 3 {
		avail = 3
	}
	if avail > len(o.lines) {
		avail = len(o.lines)
	}
	o.viewRows = max(1, avail)
	o.offset = helpClamp(o.offset, 0, max(0, len(o.lines)-o.viewRows))

	var out []string
	for i := o.offset; i < o.offset+o.viewRows && i < len(o.lines); i++ {
		l := o.lines[i]
		if l.Section != "" {
			out = append(out, th.PopupTitle.Render(ansi.Truncate(" "+l.Section, inner, "…")))
			continue
		}
		keys := ansi.Truncate(l.Keys, keyW, "…")
		keys += strings.Repeat(" ", keyW-ansi.StringWidth(keys))
		line := " " + th.Match.Render(keys) + "  " + th.Text.Render(l.Text)
		out = append(out, line)
	}

	hint := " j/k scroll  •  q/esc/f1 close"
	if len(o.lines) > o.viewRows {
		hint = fmt.Sprintf(" %d-%d of %d  •  j/k scroll  •  q/esc/f1 close",
			o.offset+1, o.offset+o.viewRows, len(o.lines))
	}
	out = append(out, th.Subtle.Render(hint))

	return ui.Box("help", strings.Join(out, "\n"), boxW, th)
}

func helpClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
