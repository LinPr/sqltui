package popup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["schema"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return newSchemaOverlay(ctx), nil
	}
}

type schemaKind uint8

const (
	schemaHeader schemaKind = iota // section title, not selectable
	schemaNote                     // informational line, not selectable
	schemaTab                      // open tab entry
	schemaLive                     // live backend table entry
	schemaGroup                    // namespace group header (enter expands/collapses)
)

// schemaRow is one display line of the schema browser. Rows are rebuilt from
// the overlay state (groups, filter) on every update, so the cursor always
// indexes the freshly built list.
type schemaRow struct {
	kind  schemaKind
	label string
	tab   int    // tab index for schemaTab rows
	ns    string // namespace for schemaLive rows
	table string // table name for schemaLive rows
	group int    // index into groups for schemaGroup rows
}

func (r schemaRow) selectable() bool {
	return r.kind == schemaTab || r.kind == schemaLive || r.kind == schemaGroup
}

// schemaNsGroup is the live-table listing of one namespace. Only the
// connected namespace loads eagerly; the others stay collapsed until the
// user expands them (lazy, asynchronous).
type schemaNsGroup struct {
	ns        string
	connected bool
	open      bool
	loading   bool
	loaded    bool
	errText   string
	tables    []string
}

// schemaOverlay browses all open tabs plus (in db mode) the live tables of
// the connected backend, grouped by namespace. A fuzzy filter line at the
// top narrows both sections; enter jumps to a tab, opens a live table, or
// expands a namespace group. All backend listings run asynchronously
// (tea.Cmd goroutines) so a slow or dead connection can never freeze the
// update loop.
type schemaOverlay struct {
	tabs       []ui.TabInfo
	backend    db.Backend
	beTitle    string
	fullscreen bool   // db mode: render as a full page, not a floating box
	nsLoaded   bool   // namespace listing arrived
	nsErr      string // namespace listing error
	flat       bool   // single empty namespace: no group headers
	groups     []schemaNsGroup
	connected  string // connected namespace ("" when unknown)

	input   histInput // fuzzy filter text
	cursor  int       // index into the last built rows
	offset  int       // first visible row
	viewLen int       // rows shown by the last View
}

// schemaNamespacesMsg delivers the async namespace listing (plus the eagerly
// loaded tables of the connected namespace) to its overlay.
type schemaNamespacesMsg struct {
	owner     *schemaOverlay
	nss       []string
	connected string
	err       error

	eager     string // namespace whose tables were loaded eagerly
	hasEager  bool
	tables    []string
	tablesErr error
}

// schemaTablesMsg delivers one namespace's lazy table listing.
type schemaTablesMsg struct {
	owner  *schemaOverlay
	ns     string
	tables []string
	err    error
}

func newSchemaOverlay(ctx ui.AppContext) *schemaOverlay {
	o := &schemaOverlay{viewLen: 10, tabs: ctx.Tabs()}
	if be := ctx.Backend(); be != nil {
		o.backend = be
		o.beTitle = be.Title()
		o.fullscreen = true
	}

	// Start on the active tab when possible, else the first selectable row.
	rows := o.rows()
	o.cursor = -1
	active := ctx.ActiveTab()
	for i, r := range rows {
		if r.kind == schemaTab && r.tab == active {
			o.cursor = i
			break
		}
	}
	if o.cursor < 0 {
		o.cursor = schemaNextSelectable(rows, -1, +1)
	}
	return o
}

// Init kicks off the async namespace listing (ui.OverlayIniter). The
// connected namespace — reported by backends implementing the optional
// CurrentNamespace interface — is resolved and eagerly listed in the same
// goroutine.
func (o *schemaOverlay) Init() tea.Cmd {
	return o.loadCmd()
}

// loadCmd runs the async namespace listing (plus the eager tables of the
// connected namespace) and stamps the result with this overlay so stale
// messages from a previous instance are ignored. It is used by Init and by
// the 'r' refresh handler.
func (o *schemaOverlay) loadCmd() tea.Cmd {
	be := o.backend
	if be == nil {
		return nil
	}
	return func() tea.Msg {
		m := schemaNamespacesMsg{owner: o}
		m.nss, m.err = be.Namespaces()
		if cn, ok := be.(interface{ CurrentNamespace() string }); ok {
			m.connected = cn.CurrentNamespace()
		}
		if eager, ok := schemaEagerNs(m.nss, m.connected); ok {
			m.eager, m.hasEager = eager, true
			m.tables, m.tablesErr = be.Tables(eager)
		}
		return m
	}
}

// schemaEagerNs picks the namespace to list eagerly: the connected one when
// it is present in the listing, else a sole namespace (sqlite-style).
func schemaEagerNs(nss []string, connected string) (string, bool) {
	if connected != "" {
		for _, ns := range nss {
			if ns == connected {
				return ns, true
			}
		}
	}
	if len(nss) == 1 {
		return nss[0], true
	}
	return "", false
}

// applyNamespaces rebuilds the group list from the namespace listing,
// putting the connected namespace first.
func (o *schemaOverlay) applyNamespaces(m schemaNamespacesMsg) {
	o.nsLoaded = true
	if m.err != nil {
		o.nsErr = m.err.Error()
	}
	o.connected = m.connected
	o.flat = len(m.nss) == 1 && m.nss[0] == ""

	ordered := make([]string, 0, len(m.nss))
	if m.hasEager {
		ordered = append(ordered, m.eager)
	}
	for _, ns := range m.nss {
		if m.hasEager && ns == m.eager {
			continue
		}
		ordered = append(ordered, ns)
	}
	o.groups = make([]schemaNsGroup, 0, len(ordered))
	for _, ns := range ordered {
		g := schemaNsGroup{ns: ns, connected: ns == m.connected}
		if m.hasEager && ns == m.eager {
			g.open = true
			g.loaded = true
			g.tables = m.tables
			if m.tablesErr != nil {
				g.errText = m.tablesErr.Error()
			}
		}
		o.groups = append(o.groups, g)
	}
}

// pattern returns the current (trimmed) filter text.
func (o *schemaOverlay) pattern() string {
	return strings.TrimSpace(o.input.String())
}

// fuzzyIndices returns the indices of items fuzzy-matching pattern, best
// match first; with an empty pattern every index is returned in order.
func fuzzyIndices(pattern string, items []string) []int {
	if pattern == "" {
		idx := make([]int, len(items))
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	matches := fuzzy.Find(pattern, items)
	idx := make([]int, len(matches))
	for i, m := range matches {
		idx[i] = m.Index
	}
	return idx
}

// rows flattens the overlay state into display lines, honouring the filter:
// tabs and every already-loaded table are searched; unloaded namespaces stay
// collapsed and are flagged instead of scanned.
func (o *schemaOverlay) rows() []schemaRow {
	pattern := o.pattern()
	var out []schemaRow

	// --- open tabs (file mode only) -------------------------------------
	// In db mode the browser is a page in the connect -> browser -> table
	// stack; open tabs are not relevant, so the tabs section is skipped.
	if o.backend == nil {
		titles := make([]string, len(o.tabs))
		for i, t := range o.tabs {
			titles[i] = t.Title
		}
		tabIdx := fuzzyIndices(pattern, titles)
		if pattern == "" || len(tabIdx) > 0 {
			out = append(out, schemaRow{kind: schemaHeader, label: "tabs"})
			if len(o.tabs) == 0 {
				out = append(out, schemaRow{kind: schemaNote, label: "(none)"})
			}
			for _, i := range tabIdx {
				out = append(out, schemaRow{
					kind:  schemaTab,
					label: fmt.Sprintf("%s  %s", o.tabs[i].Title, o.tabs[i].Shape),
					tab:   i,
				})
			}
		}
		return out
	}

	// --- live tables -----------------------------------------------------
	out = append(out, schemaRow{kind: schemaHeader, label: "tables " + o.beTitle})
	switch {
	case !o.nsLoaded:
		out = append(out, schemaRow{kind: schemaNote, label: "(loading …)"})
	case o.nsErr != "":
		out = append(out, schemaRow{kind: schemaNote, label: "error: " + o.nsErr})
	case o.flat:
		out = append(out, o.flatRows(pattern)...)
	default:
		out = append(out, o.groupRows(pattern)...)
	}
	return out
}

// flatRows lists the sole (empty) namespace without group headers.
func (o *schemaOverlay) flatRows(pattern string) []schemaRow {
	if len(o.groups) == 0 {
		return nil
	}
	g := &o.groups[0]
	switch {
	case g.errText != "":
		return []schemaRow{{kind: schemaNote, label: fmt.Sprintf("error: %s", g.errText)}}
	case g.loading:
		return []schemaRow{{kind: schemaNote, label: "(loading …)"}}
	case len(g.tables) == 0 && pattern == "":
		return []schemaRow{{kind: schemaNote, label: "(no tables)"}}
	}
	var out []schemaRow
	for _, i := range fuzzyIndices(pattern, g.tables) {
		out = append(out, schemaRow{kind: schemaLive, label: g.tables[i], table: g.tables[i]})
	}
	return out
}

// groupRows lists the namespaces as sections. Without a filter every group
// renders with its open/collapsed state; with a filter the loaded tables are
// searched across all groups (collapsed ones included, they just do not get
// loaded), and unloaded groups only surface when their name matches.
func (o *schemaOverlay) groupRows(pattern string) []schemaRow {
	var out []schemaRow
	unshownUnloaded := 0
	for gi := range o.groups {
		g := &o.groups[gi]
		if pattern == "" {
			out = append(out, schemaRow{kind: schemaGroup, label: schemaGroupLabel(g), group: gi})
			if !g.open {
				continue
			}
			switch {
			case g.loading:
				out = append(out, schemaRow{kind: schemaNote, label: "(loading …)"})
			case g.errText != "":
				out = append(out, schemaRow{kind: schemaNote, label: "error: " + g.errText})
			case len(g.tables) == 0:
				out = append(out, schemaRow{kind: schemaNote, label: "(no tables)"})
			default:
				for _, t := range g.tables {
					out = append(out, schemaRow{kind: schemaLive, label: t, ns: g.ns, table: t})
				}
			}
			continue
		}

		// Filtering: search loaded groups; flag unloaded ones.
		if g.loaded {
			idx := fuzzyIndices(pattern, g.tables)
			if len(idx) == 0 {
				continue
			}
			out = append(out, schemaRow{kind: schemaHeader, label: schemaGroupLabel(g), group: gi})
			for _, i := range idx {
				out = append(out, schemaRow{kind: schemaLive, label: g.tables[i], ns: g.ns, table: g.tables[i]})
			}
			continue
		}
		if len(fuzzyIndices(pattern, []string{g.ns})) > 0 {
			label := "▸ " + g.ns + "  (not loaded — enter to load)"
			if g.loading {
				label = "▸ " + g.ns + "  (loading …)"
			}
			out = append(out, schemaRow{kind: schemaGroup, label: label, group: gi})
		} else {
			unshownUnloaded++
		}
	}
	if pattern != "" && unshownUnloaded > 0 {
		out = append(out, schemaRow{kind: schemaNote,
			label: fmt.Sprintf("(%d namespaces not loaded — clear filter to expand)", unshownUnloaded)})
	}
	return out
}

// schemaGroupLabel renders a namespace group header for the unfiltered view.
func schemaGroupLabel(g *schemaNsGroup) string {
	marker := "▸"
	if g.open {
		marker = "▾"
	}
	label := marker + " " + g.ns
	if g.connected {
		label += "  (connected)"
	}
	if !g.open {
		label += "  (enter to expand)"
	}
	return label
}

// schemaNextSelectable returns the index of the next selectable row from i
// in direction dir (+1/-1). When no selectable row exists in that direction
// it returns -1 for a fresh scan (i < 0) or i itself otherwise.
func schemaNextSelectable(rows []schemaRow, i, dir int) int {
	for j := i + dir; j >= 0 && j < len(rows); j += dir {
		if rows[j].selectable() {
			return j
		}
	}
	if i < 0 {
		return -1
	}
	return i
}

// resetCursor puts the cursor on the first selectable row (after the filter
// changed and the row list was rebuilt from scratch).
func (o *schemaOverlay) resetCursor(rows []schemaRow) {
	o.cursor = schemaNextSelectable(rows, -1, +1)
	o.offset = 0
}

// clampCursor keeps the cursor on a selectable row after the rows changed
// shape (async load arrived, group toggled).
func (o *schemaOverlay) clampCursor(rows []schemaRow) {
	if len(rows) == 0 {
		o.cursor = -1
		return
	}
	if o.cursor >= len(rows) {
		o.cursor = len(rows) - 1
	}
	if o.cursor >= 0 && rows[o.cursor].selectable() {
		return
	}
	if j := schemaNextSelectable(rows, o.cursor, +1); j >= 0 && j < len(rows) && rows[j].selectable() {
		o.cursor = j
		return
	}
	if j := schemaNextSelectable(rows, o.cursor, -1); j >= 0 && j < len(rows) && rows[j].selectable() {
		o.cursor = j
		return
	}
	o.cursor = schemaNextSelectable(rows, -1, +1)
}

func (o *schemaOverlay) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case schemaNamespacesMsg:
		if m.owner != o {
			return o, nil // stale result from a closed instance
		}
		o.applyNamespaces(m)
		rows := o.rows()
		o.clampCursor(rows)
		o.scrollToCursor(rows)
		return o, nil

	case schemaTablesMsg:
		if m.owner != o {
			return o, nil
		}
		for i := range o.groups {
			g := &o.groups[i]
			if g.ns != m.ns {
				continue
			}
			g.loading = false
			g.loaded = true
			if m.err != nil {
				g.errText = m.err.Error()
			} else {
				g.tables = m.tables
			}
		}
		rows := o.rows()
		o.clampCursor(rows)
		o.scrollToCursor(rows)
		return o, nil
	}

	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	rows := o.rows()
	switch key.String() {
	case "esc":
		if o.input.String() != "" {
			o.input = histInput{}
			o.resetCursor(o.rows())
			return o, nil
		}
		return o, o.closeCmd()
	case "ctrl+c":
		return o, ui.CloseOverlay
	case "up", "ctrl+p":
		o.cursor = schemaNextSelectable(rows, o.cursor, -1)
	case "down", "ctrl+n":
		o.cursor = schemaNextSelectable(rows, o.cursor, +1)
	case "pgup":
		o.movePage(rows, -1)
	case "pgdown":
		o.movePage(rows, +1)
	case "r":
		// Re-trigger the async namespace listing, dropping any cached state
		// so the refresh starts from a clean slate. Only acts as refresh
		// when the filter is empty; otherwise 'r' is a filter character.
		if o.input.String() == "" {
			o.nsLoaded = false
			o.nsErr = ""
			o.groups = nil
			return o, o.loadCmd()
		}
		before := o.input.String()
		o.input.Handle(key)
		if o.input.String() != before {
			o.resetCursor(o.rows())
			return o, nil
		}
	case "enter":
		return o, o.activate(rows)
	default:
		before := o.input.String()
		o.input.Handle(key)
		if o.input.String() != before {
			o.resetCursor(o.rows())
			return o, nil
		}
	}
	o.scrollToCursor(rows)
	return o, nil
}

// closeCmd is what esc does at the top level of the browser. In database
// mode the browser is a page in the login -> browser -> table stack, so esc
// always steps back to the connection form — whether or not tabs are open
// (the "connect" factory is registered by the db-mode integration). In file
// mode there is no form and esc just closes the box.
func (o *schemaOverlay) closeCmd() tea.Cmd {
	if o.backend != nil {
		return tea.Sequence(
			ui.CloseOverlay,
			func() tea.Msg { return ui.RunCommandMsg{Name: "connect"} },
		)
	}
	return ui.CloseOverlay
}

// Fullscreen reports whether the browser renders as a full page
// (ui.FullscreenOverlay): only in database mode, where a live backend
// section exists. In file mode it stays a floating box over the table.
func (o *schemaOverlay) Fullscreen() bool { return o.fullscreen }

// movePage jumps the cursor by one window height, snapping to a selectable
// row in the same direction (falling back to the opposite one at the ends).
func (o *schemaOverlay) movePage(rows []schemaRow, dir int) {
	target := o.cursor + dir*max(1, o.viewLen)
	target = schemaClamp(target, 0, len(rows)-1)
	if target >= 0 && target < len(rows) && rows[target].selectable() {
		o.cursor = target
		return
	}
	if next := schemaNextSelectable(rows, target, dir); next >= 0 && next < len(rows) && rows[next].selectable() {
		o.cursor = next
		return
	}
	o.cursor = schemaNextSelectable(rows, target, -dir)
}

// activate returns the command for the row under the cursor.
func (o *schemaOverlay) activate(rows []schemaRow) tea.Cmd {
	if o.cursor < 0 || o.cursor >= len(rows) {
		return nil
	}
	r := rows[o.cursor]
	switch r.kind {
	case schemaTab:
		idx := r.tab
		return tea.Sequence(
			ui.CloseOverlay,
			func() tea.Msg { return ui.JumpToTabMsg{Index: idx} },
		)
	case schemaLive:
		// Delegates to the "opentable" factory registered by the db-mode
		// integration. Close first so the pop cannot race with the pushed
		// table loader overlay.
		arg := r.ns + "\t" + r.table
		return tea.Sequence(
			ui.CloseOverlay,
			func() tea.Msg { return ui.RunCommandMsg{Name: "opentable", Arg: arg} },
		)
	case schemaGroup:
		return o.toggleGroup(r.group)
	}
	return nil
}

// toggleGroup expands or collapses a namespace group, starting the lazy
// asynchronous table listing on first expansion.
func (o *schemaOverlay) toggleGroup(gi int) tea.Cmd {
	if gi < 0 || gi >= len(o.groups) {
		return nil
	}
	g := &o.groups[gi]
	if g.open && o.pattern() == "" {
		g.open = false
		return nil
	}
	g.open = true
	if g.loaded || g.loading {
		return nil
	}
	g.loading = true
	be, ns := o.backend, g.ns
	return func() tea.Msg {
		tables, err := be.Tables(ns)
		return schemaTablesMsg{owner: o, ns: ns, tables: tables, err: err}
	}
}

// scrollToCursor keeps the cursor inside the visible window.
func (o *schemaOverlay) scrollToCursor(rows []schemaRow) {
	if o.cursor < 0 {
		return
	}
	vl := max(1, o.viewLen)
	if o.cursor < o.offset {
		o.offset = o.cursor
		// Pull a section header directly above the cursor into view.
		if o.offset > 0 && !rows[o.offset-1].selectable() {
			o.offset--
		}
	}
	if o.cursor >= o.offset+vl {
		o.offset = o.cursor - vl + 1
	}
	o.offset = schemaClamp(o.offset, 0, max(0, len(rows)-vl))
}

// inputLine renders the filter line at the top of the box.
func (o *schemaOverlay) inputLine(th *theme.Theme) string {
	line := th.Subtle.Render(" filter ") + o.input.Render(th)
	if len(o.input.String()) == 0 {
		line += th.Placeholder.Render(" type to filter")
	}
	return line
}

func (o *schemaOverlay) View(width, height int, th *theme.Theme) string {
	boxW := schemaClamp(70, 30, max(30, width-4))
	if o.fullscreen {
		// Full page: the frame fills most of the body (the app blanks the
		// rest), so the table behind can never show through.
		boxW = max(30, width-8)
	}
	inner := boxW - 2

	rows := o.rows()
	avail := height - 2 - 1 - 1 // borders + filter line + footer hint
	if o.fullscreen {
		avail = height - 2 - 2 - 1 - 1 // frame is body height-2, centered
	}
	if avail < 3 {
		avail = 3
	}
	if !o.fullscreen && avail > len(rows) {
		avail = len(rows)
	}
	o.viewLen = max(1, avail)
	o.offset = schemaClamp(o.offset, 0, max(0, len(rows)-o.viewLen))

	lines := []string{o.inputLine(th)}
	if len(rows) == 0 {
		lines = append(lines, th.Placeholder.Render("  no matches"))
	}
	for i := o.offset; i < o.offset+o.viewLen && i < len(rows); i++ {
		r := rows[i]
		switch {
		case r.kind == schemaHeader:
			lines = append(lines, th.PopupTitle.Render(ansi.Truncate(" "+r.label, inner, "…")))
		case r.kind == schemaNote:
			lines = append(lines, th.Subtle.Render(ansi.Truncate("   "+r.label, inner, "…")))
		case r.kind == schemaGroup && i == o.cursor:
			lines = append(lines, th.ListSelected.Render(schemaPad(" "+r.label, inner)))
		case r.kind == schemaGroup:
			lines = append(lines, th.PopupTitle.Render(ansi.Truncate(" "+r.label, inner, "…")))
		case i == o.cursor:
			lines = append(lines, th.ListSelected.Render(schemaPad(" > "+r.label, inner)))
		default:
			lines = append(lines, th.ListItem.Render(schemaPad("   "+r.label, inner)))
		}
	}

	// Full page: pad the listing with blank rows so the frame keeps its
	// fixed height even when there are few entries.
	if o.fullscreen {
		for len(lines) < 1+o.viewLen {
			lines = append(lines, "")
		}
	}

	hint := " enter open/expand  •  esc close"
	if o.fullscreen {
		hint = " enter open/expand  •  esc back to connect"
	}
	if o.input.String() != "" {
		hint = " enter open  •  esc clear filter"
	}
	if len(rows) > o.viewLen {
		hint = fmt.Sprintf(" %d-%d of %d  • %s",
			o.offset+1, min(o.offset+o.viewLen, len(rows)), len(rows), strings.TrimPrefix(hint, " "))
	}
	lines = append(lines, th.Subtle.Render(hint))

	return ui.Box("schema", strings.Join(lines, "\n"), boxW, th)
}

// schemaPad truncates or pads s to exactly w display cells.
func schemaPad(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func schemaClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// compile-time interface checks
var (
	_ ui.Overlay           = (*schemaOverlay)(nil)
	_ ui.OverlayIniter     = (*schemaOverlay)(nil)
	_ ui.FullscreenOverlay = (*schemaOverlay)(nil)
)
