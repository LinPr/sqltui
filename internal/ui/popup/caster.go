package popup

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["cast"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		if ctx.Engine() == nil {
			return nil, fmt.Errorf("cast: no embedded SQL engine (only available in file mode)")
		}
		cols := ctx.ColumnNames()
		if len(cols) == 0 {
			return nil, fmt.Errorf("cast: no table open")
		}
		return &caster{engine: ctx.Engine(), cols: cols, paneID: ctx.ActivePaneID()}, nil
	}
}

// colFilter is the minimal type-to-filter state shared by the column pickers
// (cast, histogram, scatterplot): a query plus the fuzzy-matched positions
// into the underlying item list. Printable keys type, so pickers using it
// must keep their shortcuts on arrows/ctrl chords.
type colFilter struct {
	query []rune
	idx   []int // matched positions, best first (identity when query empty)
}

// ensure lazily (re-)computes the matches when they were never built.
func (f *colFilter) ensure(items []string) {
	if f.idx == nil {
		f.refresh(items)
	}
}

// refresh recomputes the matched positions for items.
func (f *colFilter) refresh(items []string) {
	f.idx = fuzzyIndices(strings.TrimSpace(string(f.query)), items)
}

// clear resets the query; it reports whether there was anything to clear.
func (f *colFilter) clear(items []string) bool {
	if len(f.query) == 0 {
		return false
	}
	f.query = nil
	f.refresh(items)
	return true
}

// key applies one key press. It reports whether the key was consumed
// (printable text or backspace editing the query).
func (f *colFilter) key(key string, items []string) bool {
	if key == "backspace" {
		if len(f.query) > 0 {
			f.query = f.query[:len(f.query)-1]
			f.refresh(items)
		}
		return true
	}
	if r := []rune(key); len(r) == 1 && unicode.IsPrint(r[0]) {
		f.query = append(f.query, r[0])
		f.refresh(items)
		return true
	}
	return false
}

// line renders the filter input line for the top of a picker box.
func (f *colFilter) line(th *theme.Theme) string {
	line := th.Subtle.Render(" filter ") + th.Input.Render(string(f.query)) +
		th.ListSelected.Render(" ")
	if len(f.query) == 0 {
		line += th.Placeholder.Render(" type to filter")
	}
	return line
}

// casterTarget is one selectable cast target: the display dtype name and
// the SQL type used in the CAST expression. Note SQLite affinity: bool has
// no dedicated storage class (INTEGER 0/1) and date/datetime stay TEXT.
type casterTarget struct {
	Name    string
	SQLType string
}

var casterTargets = []casterTarget{
	{"str", "TEXT"},
	{"i64", "INTEGER"},
	{"f64", "REAL"},
	{"bool", "INTEGER"},
	{"date", "TEXT"},
	{"datetime", "TEXT"},
}

// caster is a two-step picker: choose a column (type-to-filter, arrows
// navigate), then a target type; the cast runs through the embedded engine
// against the current frame ("_").
type caster struct {
	engine *query.Engine
	cols   []string
	paneID int // pane the cast was started from (result routing)

	step       int       // 0 = pick column, 1 = pick type
	filter     colFilter // column filter (step 0)
	colCursor  int       // index into filter.idx
	colOffset  int
	typeCursor int
	viewRows   int // list rows shown by the last View (page size)
}

// casterQuoteIdent wraps an identifier in double quotes, doubling embedded
// quotes, so arbitrary column names are safe in the generated SQL.
func casterQuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// casterBuildSQL selects every column of "_", replacing target with a CAST
// to sqlType (aliased back to its own name so the shape is unchanged).
func casterBuildSQL(cols []string, target, sqlType string) string {
	var b strings.Builder
	b.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		q := casterQuoteIdent(c)
		if c == target {
			b.WriteString("CAST(" + q + " AS " + sqlType + ") AS " + q)
		} else {
			b.WriteString(q)
		}
	}
	b.WriteString(` FROM "_"`)
	return b.String()
}

// casterExec returns the command that runs the cast query and yields either
// an ApplyFrameMsg or an ErrorMsg.
func casterExec(engine *query.Engine, cols []string, target, sqlType string, paneID int) tea.Cmd {
	q := casterBuildSQL(cols, target, sqlType)
	return func() tea.Msg {
		f, err := engine.Query(q)
		if err != nil {
			return ui.ErrorMsg{Err: fmt.Errorf("cast %s: %w", target, err)}
		}
		return ui.ApplyFrameMsg{Frame: f, Crumb: "cast", PaneID: paneID}
	}
}

// pickedCol returns the column highlighted on the (filtered) column step.
func (o *caster) pickedCol() (string, bool) {
	o.filter.ensure(o.cols)
	if len(o.filter.idx) == 0 {
		return "", false
	}
	return o.cols[o.filter.idx[casterClamp(o.colCursor, 0, len(o.filter.idx)-1)]], true
}

// casterKey applies one key press. It returns a command to emit alongside
// closing, and whether to close.
func (o *caster) casterKey(key string) (cmd tea.Cmd, close bool) {
	page := max(1, o.viewRows)

	if o.step == 1 {
		switch key {
		case "esc", "ctrl+c":
			o.step = 0
			return nil, false
		case "q":
			return nil, true
		case "up", "k":
			o.typeCursor--
		case "down", "j":
			o.typeCursor++
		case "pgup", "ctrl+u", "ctrl+b":
			o.typeCursor -= page
		case "pgdown", "ctrl+d", "ctrl+f":
			o.typeCursor += page
		case "g", "home":
			o.typeCursor = 0
		case "G", "end":
			o.typeCursor = len(casterTargets) // clamped below
		case "enter":
			col, ok := o.pickedCol()
			if !ok {
				return nil, false
			}
			t := casterTargets[casterClamp(o.typeCursor, 0, len(casterTargets)-1)]
			return casterExec(o.engine, o.cols, col, t.SQLType, o.paneID), true
		}
		o.typeCursor = casterClamp(o.typeCursor, 0, len(casterTargets)-1)
		return nil, false
	}

	// Column step: printable keys type into the filter, arrows navigate.
	o.filter.ensure(o.cols)
	switch key {
	case "esc":
		if o.filter.clear(o.cols) {
			o.colCursor, o.colOffset = 0, 0
			return nil, false
		}
		return nil, true
	case "ctrl+c":
		return nil, true
	case "up":
		o.colCursor--
	case "down":
		o.colCursor++
	case "pgup", "ctrl+b":
		o.colCursor -= page
	case "pgdown", "ctrl+d", "ctrl+f":
		o.colCursor += page
	case "ctrl+u":
		if o.filter.clear(o.cols) {
			o.colCursor, o.colOffset = 0, 0
			return nil, false
		}
		o.colCursor -= page
	case "home":
		o.colCursor = 0
	case "end":
		o.colCursor = len(o.filter.idx) // clamped below
	case "enter":
		if len(o.filter.idx) == 0 {
			return nil, false
		}
		o.step = 1
		return nil, false
	default:
		if o.filter.key(key, o.cols) {
			o.colCursor, o.colOffset = 0, 0
			return nil, false
		}
	}
	o.colCursor = casterClamp(o.colCursor, 0, max(0, len(o.filter.idx)-1))
	return nil, false
}

func (o *caster) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	cmd, close := o.casterKey(key.String())
	if close {
		if cmd != nil {
			return o, tea.Batch(cmd, ui.CloseOverlay)
		}
		return o, ui.CloseOverlay
	}
	return o, cmd
}

func (o *caster) View(width, height int, th *theme.Theme) string {
	boxW := casterClamp(44, 24, max(24, width-4))
	inner := boxW - 2

	item := func(text string, selected bool) string {
		text = ansi.Truncate(text, inner, "…")
		if pad := inner - ansi.StringWidth(text); pad > 0 {
			text += strings.Repeat(" ", pad)
		}
		if selected {
			return th.ListSelected.Render(text)
		}
		return th.ListItem.Render(text)
	}

	var lines []string
	if o.step == 0 {
		o.filter.ensure(o.cols)
		idx := o.filter.idx
		lines = append(lines, th.Subtle.Render(" cast which column?"))
		lines = append(lines, o.filter.line(th))

		avail := height - 2 - 3 // borders + prompt + filter + footer
		if avail < 3 {
			avail = 3
		}
		if avail > len(idx) {
			avail = len(idx)
		}
		o.viewRows = max(1, avail)
		o.colCursor = casterClamp(o.colCursor, 0, max(0, len(idx)-1))
		if o.colCursor < o.colOffset {
			o.colOffset = o.colCursor
		}
		if o.colCursor >= o.colOffset+o.viewRows {
			o.colOffset = o.colCursor - o.viewRows + 1
		}
		o.colOffset = casterClamp(o.colOffset, 0, max(0, len(idx)-o.viewRows))

		if len(idx) == 0 {
			lines = append(lines, th.Placeholder.Render("  no matching columns"))
		}
		for i := o.colOffset; i < o.colOffset+o.viewRows && i < len(idx); i++ {
			lines = append(lines, item(" "+o.cols[idx[i]], i == o.colCursor))
		}
		lines = append(lines, th.Subtle.Render(" enter pick  •  esc cancel"))
	} else {
		o.viewRows = len(casterTargets)
		col, _ := o.pickedCol()
		lines = append(lines, " "+th.Subtle.Render("cast")+" "+
			th.Text.Render(ansi.Truncate(col, max(1, inner-14), "…"))+" "+
			th.Subtle.Render("to:"))
		for i, t := range casterTargets {
			lines = append(lines, item(" "+t.Name, i == o.typeCursor))
		}
		lines = append(lines, th.Subtle.Render(" enter cast  •  esc back"))
	}

	return ui.Box("cast", strings.Join(lines, "\n"), boxW, th)
}

func casterClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
