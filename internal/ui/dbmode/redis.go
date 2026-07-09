package dbmode

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/db/redisbe"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// --- key browser -----------------------------------------------------------------

// keySection is one key-type group of the browser. Sections load lazily: the
// key list is only scanned when the section is first opened (or rescanned).
type keySection struct {
	keyType string
	open    bool
	loading bool
	loaded  bool
	errText string
	keys    []string
}

// keysLoadedMsg delivers one section's scan result.
type keysLoadedMsg struct {
	keyType string
	keys    []string
	err     error
}

// keyBrowser browses the keys of a live key-value connection, grouped by key
// type. Enter on a section header toggles it (triggering a lazy scan), enter
// on a key opens the value viewer, "r" rescans everything. "/" starts a
// substring filter that narrows the keys of the loaded sections; esc clears
// it before stepping back to the connection form.
type keyBrowser struct {
	kv       db.KVBackend
	openTabs int // open tabs when the browser was built (see closeCmd)
	sections []keySection
	cursor   int // index into rows()
	offset   int
	viewLen  int

	filter    []rune // substring the shown keys must contain
	filtering bool   // keys currently go to the filter input
}

func newKeyBrowser(kv db.KVBackend, openTabs int) *keyBrowser {
	b := &keyBrowser{kv: kv, openTabs: openTabs, viewLen: 10}
	for _, kt := range redisbe.KeyTypes {
		b.sections = append(b.sections, keySection{keyType: kt})
	}
	return b
}

// closeCmd is what q does at the top level of the browser: close it, back to
// the table if one is open. With no open tabs closing would leave a blank
// workspace, so it steps back to the connection form instead.
func (b *keyBrowser) closeCmd() tea.Cmd {
	if b.openTabs == 0 {
		return b.escCmd()
	}
	return ui.CloseOverlay
}

// escCmd is what esc does at the top level of the browser. The browser is a
// page in the login -> browser -> table stack, so esc always steps back to
// the connection form, whether or not tabs are open.
func (b *keyBrowser) escCmd() tea.Cmd {
	return tea.Sequence(
		ui.CloseOverlay,
		func() tea.Msg { return ui.RunCommandMsg{Name: "connect"} },
	)
}

// Fullscreen marks the browser as a full page (ui.FullscreenOverlay); it
// only exists in database mode.
func (b *keyBrowser) Fullscreen() bool { return true }

// Init opens the first section so the browser shows something immediately;
// the remaining sections stay lazy.
func (b *keyBrowser) Init() tea.Cmd {
	if len(b.sections) == 0 {
		return nil
	}
	b.sections[0].open = true
	return b.scanSection(0)
}

// scanSection starts the asynchronous key scan for one section.
func (b *keyBrowser) scanSection(i int) tea.Cmd {
	s := &b.sections[i]
	if s.loading {
		return nil
	}
	s.loading = true
	s.loaded = false
	s.errText = ""
	s.keys = nil
	kv, kt := b.kv, s.keyType
	return func() tea.Msg {
		keys, err := kv.ScanKeys(kt)
		return keysLoadedMsg{keyType: kt, keys: keys, err: err}
	}
}

// browserRow is one display line: a section header (key == "") or a key.
type browserRow struct {
	section int
	key     string // "" for headers and notes
	note    string // informational text for non-selectable note rows
}

func (r browserRow) selectable() bool { return r.note == "" }

// matchKeys returns the section keys that contain the filter substring
// (case-insensitive); with an empty filter every key passes.
func (b *keyBrowser) matchKeys(keys []string) []string {
	pat := strings.ToLower(strings.TrimSpace(string(b.filter)))
	if pat == "" {
		return keys
	}
	var out []string
	for _, k := range keys {
		if strings.Contains(strings.ToLower(k), pat) {
			out = append(out, k)
		}
	}
	return out
}

// rows flattens the sections into display lines, applying the key filter to
// the loaded sections.
func (b *keyBrowser) rows() []browserRow {
	filtered := strings.TrimSpace(string(b.filter)) != ""
	var out []browserRow
	for i := range b.sections {
		s := &b.sections[i]
		out = append(out, browserRow{section: i})
		if !s.open {
			continue
		}
		switch {
		case s.loading:
			out = append(out, browserRow{section: i, note: "loading..."})
		case s.errText != "":
			out = append(out, browserRow{section: i, note: "error: " + s.errText})
		case len(s.keys) == 0:
			out = append(out, browserRow{section: i, note: "(no keys)"})
		default:
			keys := b.matchKeys(s.keys)
			if len(keys) == 0 && filtered {
				out = append(out, browserRow{section: i, note: "(no matching keys)"})
				continue
			}
			for _, k := range keys {
				out = append(out, browserRow{section: i, key: k})
			}
		}
	}
	return out
}

// move advances the cursor to the next selectable row in direction dir.
func (b *keyBrowser) move(rows []browserRow, dir int) {
	for j := b.cursor + dir; j >= 0 && j < len(rows); j += dir {
		if rows[j].selectable() {
			b.cursor = j
			return
		}
	}
}

func (b *keyBrowser) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case keysLoadedMsg:
		for i := range b.sections {
			s := &b.sections[i]
			if s.keyType != m.keyType {
				continue
			}
			s.loading = false
			s.loaded = true
			if m.err != nil {
				s.errText = m.err.Error()
			} else {
				s.keys = m.keys
			}
		}
		b.clampCursor()
		return b, nil

	case tea.KeyPressMsg:
		if b.filtering {
			return b.filterKey(m)
		}
		rows := b.rows()
		switch m.String() {
		case "q":
			return b, b.closeCmd()
		case "esc":
			if len(b.filter) > 0 {
				b.filter = nil
				b.clampCursor()
				return b, nil
			}
			return b, b.escCmd()
		case "/":
			b.filtering = true
			return b, nil
		case "up", "k":
			b.move(rows, -1)
		case "down", "j":
			b.move(rows, +1)
		case "g", "home":
			b.cursor = 0
		case "G", "end":
			b.cursor = len(rows) - 1
			if b.cursor >= 0 && !rows[b.cursor].selectable() {
				b.move(rows, -1)
			}
		case "r":
			return b, b.rescan()
		case "enter":
			return b, b.activate(rows)
		}
		b.scrollToCursor(rows)
		return b, nil
	}
	return b, nil
}

// filterKey handles one key press while the filter input is focused.
func (b *keyBrowser) filterKey(m tea.KeyPressMsg) (ui.Overlay, tea.Cmd) {
	switch m.String() {
	case "esc":
		// esc clears the filter (and leaves the input); a second esc on the
		// browser then closes it.
		b.filter = nil
		b.filtering = false
		b.clampCursor()
		return b, nil
	case "enter":
		// Keep the narrowed list and give the keys back to navigation.
		b.filtering = false
		return b, nil
	case "up":
		b.move(b.rows(), -1)
		b.scrollToCursor(b.rows())
		return b, nil
	case "down":
		b.move(b.rows(), +1)
		b.scrollToCursor(b.rows())
		return b, nil
	case "backspace":
		if len(b.filter) > 0 {
			b.filter = b.filter[:len(b.filter)-1]
			b.clampCursor()
		}
		return b, nil
	case "ctrl+u":
		b.filter = nil
		b.clampCursor()
		return b, nil
	case "ctrl+c":
		return b, ui.CloseOverlay
	}
	if t := m.Text; t != "" {
		for _, r := range t {
			if r >= ' ' && r != 0x7f {
				b.filter = append(b.filter, r)
			}
		}
		b.clampCursor()
	}
	return b, nil
}

// rescan reloads every open section and marks the rest unloaded.
func (b *keyBrowser) rescan() tea.Cmd {
	var cmds []tea.Cmd
	for i := range b.sections {
		s := &b.sections[i]
		if s.open {
			if cmd := b.scanSection(i); cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			s.loaded = false
			s.errText = ""
			s.keys = nil
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// activate handles enter: toggle a section (lazily scanning it) or open the
// value viewer for a key.
func (b *keyBrowser) activate(rows []browserRow) tea.Cmd {
	if b.cursor < 0 || b.cursor >= len(rows) {
		return nil
	}
	r := rows[b.cursor]
	if !r.selectable() {
		return nil
	}
	if r.key == "" { // section header
		s := &b.sections[r.section]
		s.open = !s.open
		if s.open && !s.loaded {
			return b.scanSection(r.section)
		}
		return nil
	}
	kv, key := b.kv, r.key
	viewer := newValueViewer(key, func() (string, error) { return kv.Value(key) })
	return func() tea.Msg { return ui.PushOverlayMsg{Overlay: viewer} }
}

// clampCursor keeps the cursor on a selectable row after the rows changed.
func (b *keyBrowser) clampCursor() {
	rows := b.rows()
	if len(rows) == 0 {
		b.cursor = 0
		return
	}
	if b.cursor >= len(rows) {
		b.cursor = len(rows) - 1
	}
	if b.cursor < 0 {
		b.cursor = 0
	}
	if !rows[b.cursor].selectable() {
		b.move(rows, -1)
		if !rows[b.cursor].selectable() {
			b.move(rows, +1)
		}
	}
}

func (b *keyBrowser) scrollToCursor(rows []browserRow) {
	vl := b.viewLen
	if vl < 1 {
		vl = 1
	}
	if b.cursor < b.offset {
		b.offset = b.cursor
	}
	if b.cursor >= b.offset+vl {
		b.offset = b.cursor - vl + 1
	}
	if maxOff := len(rows) - vl; b.offset > maxOff {
		b.offset = maxOff
	}
	if b.offset < 0 {
		b.offset = 0
	}
}

func (b *keyBrowser) View(width, height int, th *theme.Theme) string {
	boxW := width - 4
	if boxW > 70 {
		boxW = 70
	}
	if boxW < 30 {
		boxW = 30
	}
	inner := boxW - 2

	rows := b.rows()
	showFilter := b.filtering || len(b.filter) > 0
	avail := height - 2 - 1 // borders + footer hint
	if showFilter {
		avail-- // filter line
	}
	if avail < 3 {
		avail = 3
	}
	if avail > len(rows) {
		avail = len(rows)
	}
	b.viewLen = avail
	if b.viewLen < 1 {
		b.viewLen = 1
	}
	b.scrollToCursor(rows)

	var lines []string
	if showFilter {
		line := th.Text.Render(" / ") + th.Input.Render(string(b.filter))
		if b.filtering {
			line += th.ListSelected.Render(" ")
		}
		if len(b.filter) == 0 && b.filtering {
			line += th.Placeholder.Render(" type to filter keys")
		}
		lines = append(lines, line)
	}
	for i := b.offset; i < b.offset+b.viewLen && i < len(rows); i++ {
		r := rows[i]
		s := b.sections[r.section]
		switch {
		case r.note != "":
			lines = append(lines, th.Subtle.Render(ansi.Truncate("   "+r.note, inner, "…")))
		case r.key == "":
			marker := "+"
			if s.open {
				marker = "-"
			}
			label := " " + marker + " " + s.keyType
			if i == b.cursor {
				lines = append(lines, th.ListSelected.Render(browserPad(label, inner)))
			} else {
				lines = append(lines, th.PopupTitle.Render(ansi.Truncate(label, inner, "…")))
			}
		case i == b.cursor:
			lines = append(lines, th.ListSelected.Render(browserPad(" > "+r.key, inner)))
		default:
			lines = append(lines, th.ListItem.Render(browserPad("   "+r.key, inner)))
		}
	}
	hint := " enter open · / filter · r rescan · q close · esc back"
	if b.filtering {
		hint = " enter done · esc clear · ↑↓ move"
	} else if len(b.filter) > 0 {
		hint = " enter open · / edit filter · esc clear filter"
	}
	lines = append(lines, th.Subtle.Render(hint))

	return ui.Box("keys "+b.kv.Title(), strings.Join(lines, "\n"), boxW, th)
}

// browserPad truncates or pads s to exactly w display cells.
func browserPad(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// --- value viewer -----------------------------------------------------------------

// valueLoadedMsg delivers the asynchronously fetched value text.
type valueLoadedMsg struct {
	text string
	err  error
}

// valueViewer shows one scrollable block of text (a rendered key value or a
// raw-command result). With a non-nil loader it fetches the text on Init.
type valueViewer struct {
	title   string
	loader  func() (string, error)
	loading bool
	text    string
	errText string
	offset  int
	pageLen int
}

// newValueViewer builds a viewer that loads its text asynchronously when
// shown.
func newValueViewer(title string, loader func() (string, error)) *valueViewer {
	return &valueViewer{title: title, loader: loader, loading: true, pageLen: 10}
}

// newValueViewerText builds a viewer around already-available text.
func newValueViewerText(title, text string) *valueViewer {
	return &valueViewer{title: title, text: text, pageLen: 10}
}

// Fullscreen marks the viewer as a full page (ui.FullscreenOverlay); it
// only exists in database mode.
func (v *valueViewer) Fullscreen() bool { return true }

func (v *valueViewer) Init() tea.Cmd {
	if v.loader == nil {
		v.loading = false
		return nil
	}
	load := v.loader
	return func() tea.Msg {
		text, err := load()
		return valueLoadedMsg{text: text, err: err}
	}
}

func (v *valueViewer) lines() []string {
	switch {
	case v.loading:
		return []string{"loading..."}
	case v.errText != "":
		return strings.Split(v.errText, "\n")
	default:
		return strings.Split(v.text, "\n")
	}
}

func (v *valueViewer) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case valueLoadedMsg:
		v.loading = false
		if m.err != nil {
			v.errText = m.err.Error()
		} else {
			v.text = m.text
		}
		v.offset = 0
		return v, nil

	case tea.KeyPressMsg:
		n := len(v.lines())
		switch m.String() {
		case "q", "esc":
			return v, ui.CloseOverlay
		case "up", "k":
			v.offset--
		case "down", "j":
			v.offset++
		case "pgup", "ctrl+b":
			v.offset -= v.pageLen
		case "pgdown", "ctrl+f":
			v.offset += v.pageLen
		case "g", "home":
			v.offset = 0
		case "G", "end":
			v.offset = n - v.pageLen
		}
		if maxOff := n - v.pageLen; v.offset > maxOff {
			v.offset = maxOff
		}
		if v.offset < 0 {
			v.offset = 0
		}
		return v, nil
	}
	return v, nil
}

func (v *valueViewer) View(width, height int, th *theme.Theme) string {
	boxW := width - 4
	if boxW > 76 {
		boxW = 76
	}
	if boxW < 30 {
		boxW = 30
	}
	inner := boxW - 2

	all := v.lines()
	avail := height - 2 - 1 // borders + footer hint
	if avail < 3 {
		avail = 3
	}
	if avail > len(all) {
		avail = len(all)
	}
	v.pageLen = avail
	if maxOff := len(all) - avail; v.offset > maxOff {
		v.offset = maxOff
	}
	if v.offset < 0 {
		v.offset = 0
	}

	style := th.Text
	if v.errText != "" {
		style = th.Error
	}
	if v.loading {
		style = th.Subtle
	}
	var lines []string
	for i := v.offset; i < v.offset+avail && i < len(all); i++ {
		lines = append(lines, style.Render(ansi.Truncate(" "+all[i], inner, "…")))
	}
	hint := " ↑↓ scroll · q/esc back"
	if len(all) > avail {
		hint = strings.TrimRight(hint, " ") + " · " +
			viewerRange(v.offset, avail, len(all))
	}
	lines = append(lines, th.Subtle.Render(hint))

	title := v.title
	if title == "" {
		title = "value"
	}
	return ui.Box(title, strings.Join(lines, "\n"), boxW, th)
}

func viewerRange(off, page, total int) string {
	end := off + page
	if end > total {
		end = total
	}
	return fmt.Sprintf("%d-%d of %d", off+1, end, total)
}

// --- raw command prompt --------------------------------------------------------------

// redisDoneMsg reports the outcome of one raw command execution.
type redisDoneMsg struct {
	command string
	out     string
	err     error
}

// redisPrompt is a single-line raw-command input with completion from the
// built-in command reference. Tab applies/cycles suggestions, enter runs the
// command asynchronously and shows the result in the value viewer.
type redisPrompt struct {
	kv      db.KVBackend
	input   []rune
	sugg    []redisbe.CommandHelp
	sel     int
	cycling bool // true while tab is cycling through sugg
	errText string
	running bool
}

func newRedisPrompt(ctx ui.AppContext, arg string) *redisPrompt {
	p := &redisPrompt{kv: ctx.KV(), input: []rune(arg)}
	p.recompute()
	return p
}

// recompute refreshes the suggestion list: command names that the current
// input is a case-insensitive prefix of.
func (p *redisPrompt) recompute() {
	p.sugg = nil
	p.sel = 0
	p.cycling = false
	text := strings.ToUpper(strings.TrimLeft(string(p.input), " "))
	if text == "" {
		return
	}
	for _, h := range redisbe.CommandHelps {
		if strings.HasPrefix(h.Command, text) {
			p.sugg = append(p.sugg, h)
			if len(p.sugg) == maxPromptSuggestions {
				return
			}
		}
	}
}

// maxPromptSuggestions caps the completion dropdown size.
const maxPromptSuggestions = 8

// applyTab applies the highlighted suggestion; repeated tabs cycle through
// the list (the suggestion list is intentionally not recomputed so cycling
// keeps working after the first application).
func (p *redisPrompt) applyTab() {
	if len(p.sugg) == 0 {
		return
	}
	if p.cycling {
		p.sel = (p.sel + 1) % len(p.sugg)
	} else {
		p.cycling = true
	}
	p.input = []rune(p.sugg[p.sel].Command + " ")
}

// startRun executes the current input asynchronously. A second run while one
// is in flight is ignored.
func (p *redisPrompt) startRun() tea.Cmd {
	fields := strings.Fields(string(p.input))
	if p.running || len(fields) == 0 {
		return nil
	}
	p.running = true
	p.errText = ""
	kv, command := p.kv, strings.Join(fields, " ")
	return func() tea.Msg {
		out, err := kv.Do(fields)
		return redisDoneMsg{command: command, out: out, err: err}
	}
}

func (p *redisPrompt) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case redisDoneMsg:
		p.running = false
		if m.err != nil {
			p.errText = m.err.Error()
			return p, nil
		}
		// Replace the prompt with the result viewer, titled with the command.
		return newValueViewerText(m.command, m.out), nil

	case tea.KeyPressMsg:
		switch m.String() {
		case "esc", "ctrl+c":
			return p, ui.CloseOverlay
		case "enter":
			return p, p.startRun()
		case "tab":
			p.applyTab()
			return p, nil
		case "up":
			if n := len(p.sugg); n > 0 {
				p.sel = (p.sel - 1 + n) % n
			}
			return p, nil
		case "down":
			if n := len(p.sugg); n > 0 {
				p.sel = (p.sel + 1) % n
			}
			return p, nil
		case "backspace":
			if n := len(p.input); n > 0 {
				p.input = p.input[:n-1]
			}
			p.recompute()
			return p, nil
		case "ctrl+u":
			p.input = nil
			p.recompute()
			return p, nil
		}
		if t := m.Text; t != "" {
			for _, r := range t {
				if r >= ' ' && r != 0x7f {
					p.input = append(p.input, r)
				}
			}
			p.recompute()
		}
		return p, nil
	}
	return p, nil
}

func (p *redisPrompt) View(width, height int, th *theme.Theme) string {
	boxW := width - 4
	if boxW > 70 {
		boxW = 70
	}
	if boxW < 30 {
		boxW = 30
	}
	inner := boxW - 2

	// Input line with the tail kept visible while typing long commands.
	val := string(p.input)
	room := inner - 4
	if room < 1 {
		room = 1
	}
	if r := []rune(val); len(r) > room {
		val = "…" + string(r[len(r)-room+1:])
	}
	cur := th.ListSelected.Render(" ")
	lines := []string{th.Text.Render(" > ") + th.Input.Render(val) + cur}

	for i, h := range p.sugg {
		detail := " " + h.Args
		if h.Desc != "" {
			detail += " — " + h.Desc
		}
		name := " " + h.Command
		if i == p.sel {
			line := name + detail
			lines = append(lines, th.ListSelected.Render(browserPad(line, inner)))
		} else {
			row := th.ListItem.Render(name) + th.Subtle.Render(detail)
			lines = append(lines, ansi.Truncate(row, inner, "…"))
		}
	}

	lines = append(lines, "")
	switch {
	case p.running:
		lines = append(lines, th.Warning.Render(" running..."))
	case p.errText != "":
		for _, l := range strings.Split(ansi.Wrap(p.errText, inner-2, ""), "\n") {
			lines = append(lines, th.Error.Render(" "+l))
		}
	default:
		lines = append(lines, th.Placeholder.Render(" enter run · tab complete · esc close"))
	}

	return ui.Box("redis command", strings.Join(lines, "\n"), boxW, th)
}

// compile-time interface checks
var (
	_ ui.Overlay           = (*keyBrowser)(nil)
	_ ui.OverlayIniter     = (*keyBrowser)(nil)
	_ ui.FullscreenOverlay = (*keyBrowser)(nil)
	_ ui.Overlay           = (*valueViewer)(nil)
	_ ui.OverlayIniter     = (*valueViewer)(nil)
	_ ui.FullscreenOverlay = (*valueViewer)(nil)
	_ ui.Overlay           = (*redisPrompt)(nil)
)
