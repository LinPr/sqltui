// Package popup contains the modal overlays that plug into the app via the
// ui.Factories registry.
package popup

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// Command describes one palette entry. The table is exported so the help
// overlay can render the full command reference.
type Command struct {
	Name        string   // canonical name dispatched in ui.RunCommandMsg
	Aliases     []string // alternative spellings accepted in the palette
	Description string
}

// Commands is the palette command table, in display order.
var Commands = []Command{
	{Name: "query", Aliases: []string{"q"}, Description: "full SQL editor with completion"},
	{Name: "select", Aliases: []string{"s"}, Description: "inline SELECT column list"},
	{Name: "filter", Aliases: []string{"f"}, Description: "inline WHERE clause"},
	{Name: "order", Aliases: []string{"o", "sort"}, Description: "inline ORDER BY clause"},
	{Name: "search", Aliases: nil, Description: "exact text search"},
	{Name: "fuzzysearch", Aliases: nil, Description: "fuzzy text search"},
	{Name: "schema", Aliases: nil, Description: "browse loaded tables / schemas"},
	{Name: "info", Aliases: nil, Description: "table info and column stats"},
	{Name: "export", Aliases: nil, Description: "export current frame to a file"},
	{Name: "import", Aliases: nil, Description: "import a data file as a new tab"},
	{Name: "cast", Aliases: nil, Description: "cast a column to another type"},
	{Name: "register", Aliases: nil, Description: "register current frame as a SQL table"},
	{Name: "histogram", Aliases: nil, Description: "build a histogram of a column"},
	{Name: "scatterplot", Aliases: nil, Description: "build a scatter plot of two columns"},
	{Name: "edit", Aliases: nil, Description: "edit frame in $EDITOR, reimport on save"},
	{Name: "theme", Aliases: []string{"themeselector"}, Description: "theme selector with live preview"},
	{Name: "toggleborders", Aliases: nil, Description: "toggle table borders"},
	{Name: "togglerownumbers", Aliases: nil, Description: "toggle row-number gutter"},
	{Name: "reset", Aliases: nil, Description: "reset current tab to its base frame"},
	{Name: "reloadconfig", Aliases: nil, Description: "reload configuration from disk"},
	{Name: "help", Aliases: nil, Description: "show help overlay"},
	{Name: "tabs", Aliases: nil, Description: "open the tab switcher"},
	{Name: "goto", Aliases: nil, Description: "jump to a row number"},
	{Name: "quit", Aliases: nil, Description: "quit the application"},
}

func init() {
	ui.Factories["palette"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return newPalette(arg), nil
	}
}

// paletteEntry is one fuzzy-matchable string mapped back to its command.
type paletteEntry struct {
	text string
	cmd  int // index into Commands
}

// paletteCorpus lists every name and alias once (built lazily, read-only).
var paletteCorpus = func() []paletteEntry {
	var es []paletteEntry
	for i, c := range Commands {
		es = append(es, paletteEntry{text: c.Name, cmd: i})
		for _, a := range c.Aliases {
			es = append(es, paletteEntry{text: a, cmd: i})
		}
	}
	return es
}()

// paletteSource adapts the corpus to the fuzzy matcher.
type paletteSource struct{ entries []paletteEntry }

func (s paletteSource) String(i int) string { return s.entries[i].text }
func (s paletteSource) Len() int            { return len(s.entries) }

// paletteResolve maps an exactly-typed name or alias (case-insensitive) to
// its canonical command name.
func paletteResolve(token string) (string, bool) {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return "", false
	}
	for _, e := range paletteCorpus {
		if e.text == token {
			return Commands[e.cmd].Name, true
		}
	}
	return "", false
}

// paletteFilter returns the indices of Commands matching pattern, best match
// first. An empty pattern lists every command in table order.
func paletteFilter(pattern string) []int {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		all := make([]int, len(Commands))
		for i := range all {
			all[i] = i
		}
		return all
	}
	matches := fuzzy.FindFrom(pattern, paletteSource{paletteCorpus})
	seen := make(map[int]bool, len(matches))
	var out []int
	for _, m := range matches {
		ci := paletteCorpus[m.Index].cmd
		if !seen[ci] {
			seen[ci] = true
			out = append(out, ci)
		}
	}
	return out
}

// palette is the command palette overlay: an input line plus a fuzzy-filtered
// command list.
type palette struct {
	input    []rune
	cursor   int
	filtered []int // indices into Commands
	sel      int   // selection within filtered
	offset   int   // list scroll offset
}

func newPalette(prefill string) *palette {
	p := &palette{input: []rune(prefill)}
	p.cursor = len(p.input)
	p.refilter()
	return p
}

func (p *palette) refilter() {
	// Filter on the command token only, so "filter age > 30" keeps the
	// filter command highlighted while the argument is typed.
	token := string(p.input)
	if i := strings.IndexByte(token, ' '); i >= 0 {
		token = token[:i]
	}
	p.filtered = paletteFilter(token)
	p.sel = 0
	p.offset = 0
}

// dispatchTarget computes what pressing enter would run: the resolved first
// token (with the remainder as the argument) or the current list selection.
func (p *palette) dispatchTarget() (name, arg string, ok bool) {
	text := string(p.input)
	token, rest := text, ""
	if i := strings.IndexByte(text, ' '); i >= 0 {
		token, rest = text[:i], strings.TrimSpace(text[i+1:])
	}
	if n, found := paletteResolve(token); found {
		return n, rest, true
	}
	if len(p.filtered) > 0 {
		return Commands[p.filtered[p.sel]].Name, rest, true
	}
	if strings.TrimSpace(token) != "" {
		// Let the app report the unknown command.
		return strings.TrimSpace(token), rest, true
	}
	return "", "", false
}

// paletteDispatch closes the palette, then delivers the command. Sequence
// (not Batch) so the palette is popped before any follow-up overlay opens.
func paletteDispatch(name, arg string) tea.Cmd {
	return tea.Sequence(ui.CloseOverlay, func() tea.Msg {
		return ui.RunCommandMsg{Name: name, Arg: arg}
	})
}

// insertText inserts printable input at the cursor. Typing a space right
// after a single-letter alias ("q", "s", "f", "o") hands off to the inline
// prompt immediately for a fluid feel; the returned command is non-nil in
// that case.
func (p *palette) insertText(text string) tea.Cmd {
	if text == " " && p.cursor == len(p.input) {
		if tok := string(p.input); len([]rune(tok)) == 1 {
			if name, ok := paletteResolve(tok); ok {
				return paletteDispatch(name, "")
			}
		}
	}
	rs := []rune(text)
	p.input = append(p.input[:p.cursor], append(rs, p.input[p.cursor:]...)...)
	p.cursor += len(rs)
	p.refilter()
	return nil
}

// paste inserts pasted text at the cursor, flattened to a single line. It
// deliberately bypasses insertText's alias-space dispatch: a paste is one
// edit, never an implicit enter.
func (p *palette) paste(content string) {
	rs := []rune(pasteSanitize(content))
	if len(rs) == 0 {
		return
	}
	p.input = append(p.input[:p.cursor], append(rs, p.input[p.cursor:]...)...)
	p.cursor += len(rs)
	p.refilter()
}

func (p *palette) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if pm, ok := msg.(tea.PasteMsg); ok {
		p.paste(pm.Content)
		return p, nil
	}
	key, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return p, nil
	}
	switch key.String() {
	case "esc", "ctrl+c":
		return p, ui.CloseOverlay
	case "enter":
		if name, arg, ok := p.dispatchTarget(); ok {
			return p, paletteDispatch(name, arg)
		}
		return p, ui.CloseOverlay
	case "up", "ctrl+p":
		if p.sel > 0 {
			p.sel--
		}
		return p, nil
	case "down", "ctrl+n":
		if p.sel < len(p.filtered)-1 {
			p.sel++
		}
		return p, nil
	case "backspace":
		if p.cursor > 0 {
			p.input = append(p.input[:p.cursor-1], p.input[p.cursor:]...)
			p.cursor--
			p.refilter()
		}
		return p, nil
	case "delete":
		if p.cursor < len(p.input) {
			p.input = append(p.input[:p.cursor], p.input[p.cursor+1:]...)
			p.refilter()
		}
		return p, nil
	case "left":
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case "right":
		if p.cursor < len(p.input) {
			p.cursor++
		}
		return p, nil
	case "home", "ctrl+a":
		p.cursor = 0
		return p, nil
	case "end", "ctrl+e":
		p.cursor = len(p.input)
		return p, nil
	case "ctrl+u":
		p.input = append([]rune(nil), p.input[p.cursor:]...)
		p.cursor = 0
		p.refilter()
		return p, nil
	}
	if key.Text != "" {
		return p, p.insertText(key.Text)
	}
	return p, nil
}

func (p *palette) View(width, height int, th *theme.Theme) string {
	w := paletteClamp(width-4, 24, 70)
	inner := w - 2

	// Input line with a block cursor.
	var in strings.Builder
	in.WriteString(th.Subtle.Render(" > "))
	before := string(p.input[:p.cursor])
	under := " "
	after := ""
	if p.cursor < len(p.input) {
		under = string(p.input[p.cursor])
		after = string(p.input[p.cursor+1:])
	}
	if len(p.input) == 0 {
		in.WriteString(th.ListSelected.Render(" "))
		in.WriteString(th.Placeholder.Render("type a command"))
	} else {
		in.WriteString(th.Input.Render(before))
		in.WriteString(th.ListSelected.Render(under))
		in.WriteString(th.Input.Render(after))
	}

	// Visible list window.
	listH := paletteClamp(height-8, 3, 12)
	if listH > len(p.filtered) {
		listH = len(p.filtered)
	}
	if p.sel < p.offset {
		p.offset = p.sel
	}
	if listH > 0 && p.sel >= p.offset+listH {
		p.offset = p.sel - listH + 1
	}
	if p.offset > len(p.filtered)-listH {
		p.offset = paletteMax(0, len(p.filtered)-listH)
	}

	lines := []string{in.String()}
	if len(p.filtered) == 0 {
		lines = append(lines, th.Placeholder.Render(" no matching command"))
	}
	nameW := 20
	for i := p.offset; i < p.offset+listH; i++ {
		c := Commands[p.filtered[i]]
		label := c.Name
		if len(c.Aliases) > 0 {
			label += " (" + strings.Join(c.Aliases, ", ") + ")"
		}
		label = ansi.Truncate(label, nameW, "…")
		pad := strings.Repeat(" ", paletteMax(0, nameW-ansi.StringWidth(label)))
		if i == p.sel {
			row := " " + label + pad + " " + c.Description
			if extra := inner - ansi.StringWidth(row); extra > 0 {
				row += strings.Repeat(" ", extra)
			}
			lines = append(lines, th.ListSelected.Render(ansi.Truncate(row, inner, "…")))
		} else {
			lines = append(lines, th.ListItem.Render(" "+label+pad+" ")+th.Subtle.Render(c.Description))
		}
	}
	lines = append(lines, th.Placeholder.Render(" enter run · esc close"))

	return ui.Box("commands", strings.Join(lines, "\n"), w, th)
}

func paletteClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func paletteMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
