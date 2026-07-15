package popup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["info"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		f := ctx.CurrentFrame()
		if f == nil {
			return nil, fmt.Errorf("info: no table open")
		}
		return newInfoOverlay(ctx, f), nil
	}
}

// infoColRow is one precomputed line of the per-column mini table.
type infoColRow struct {
	Name    string
	Type    string
	NotNull string
	Default string
	Comment string
}

// columnsMetaMsg carries the async column-metadata result back to the overlay
// that issued the fetch. The owner pointer lets stale messages from a prior
// instance be ignored.
type columnsMetaMsg struct {
	owner *infoOverlay
	meta  []db.ColumnMeta
	err   error
}

// infoOverlay shows metadata about the current frame: title, breadcrumb,
// shape, estimated size and a scrollable per-column listing. In db mode the
// listing is refreshed asynchronously from live column metadata; in file mode
// it falls back to the frame's inferred DType.
type infoOverlay struct {
	ctx      ui.AppContext
	frame    *data.Frame
	loading  bool
	meta     []db.ColumnMeta
	loadErr  string

	title  string
	crumbs []string
	rows   int
	cols   int
	size   int64
	list   []infoColRow

	offset   int // first visible list row
	viewRows int // list rows shown by the last View (page size)
}

func newInfoOverlay(ctx ui.AppContext, f *data.Frame) *infoOverlay {
	o := &infoOverlay{
		ctx:      ctx,
		frame:    f,
		title:    ctx.BaseCrumb(),
		crumbs:   ctx.Crumbs(),
		rows:     f.NumRows(),
		cols:     f.NumCols(),
		size:     data.EstimatedSize(f),
		viewRows: 10,
		loading:  ctx.Backend() != nil,
	}
	// Build the initial listing from the frame so the overlay shows something
	// immediately; in db mode this is a placeholder until the meta arrives.
	o.list = infoRowsFromFrame(f)
	return o
}

// infoRowsFromFrame builds the file-mode fallback listing from a frame's
// inferred column types.
func infoRowsFromFrame(f *data.Frame) []infoColRow {
	rows := make([]infoColRow, 0, len(f.Columns))
	for i := range f.Columns {
		c := &f.Columns[i]
		rows = append(rows, infoColRow{
			Name: c.Name,
			Type: c.Type.String(),
		})
	}
	return rows
}

// infoRowsFromMeta rebuilds the listing from live column metadata.
func infoRowsFromMeta(meta []db.ColumnMeta) []infoColRow {
	rows := make([]infoColRow, 0, len(meta))
	for i := range meta {
		m := &meta[i]
		rows = append(rows, infoColRow{
			Name:    m.Name,
			Type:    m.DataType,
			NotNull: m.IsNullable,
			Default: m.Default,
			Comment: m.Comment,
		})
	}
	return rows
}

// Init kicks off the async column-metadata fetch in db mode
// (ui.OverlayIniter). In file mode there is nothing to fetch.
func (o *infoOverlay) Init() tea.Cmd {
	be := o.ctx.Backend()
	if be == nil {
		return nil
	}
	ns := o.ctx.CurrentTableNamespace()
	table := o.ctx.BaseCrumb()
	owner := o
	return func() tea.Msg {
		m, err := be.ColumnsMeta(ns, table)
		return columnsMetaMsg{owner: owner, meta: m, err: err}
	}
}

func (o *infoOverlay) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if m, ok := msg.(columnsMetaMsg); ok && m.owner == o {
		o.loading = false
		if m.err != nil {
			o.loadErr = m.err.Error()
		} else {
			o.meta = m.meta
			o.list = infoRowsFromMeta(m.meta)
		}
		o.offset = 0
		return o, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "q", "esc":
		return o, ui.CloseOverlay
	case "up", "k":
		o.offset--
	case "down", "j":
		o.offset++
	case "pgup", "ctrl+u", "ctrl+b":
		o.offset -= max(1, o.viewRows)
	case "pgdown", "ctrl+f":
		o.offset += max(1, o.viewRows)
	case "g", "home":
		o.offset = 0
	case "G", "end":
		o.offset = len(o.list) // clamped below
	}
	o.offset = infoClamp(o.offset, 0, max(0, len(o.list)-max(1, o.viewRows)))
	return o, nil
}

func (o *infoOverlay) View(width, height int, th *theme.Theme) string {
	boxW := infoClamp(70, 30, max(30, width-4))
	inner := boxW - 2

	var lines []string
	label := func(k, v string) string {
		return " " + th.Subtle.Render(k) + " " + th.Text.Render(v)
	}
	lines = append(lines, label("table:", o.title))
	if len(o.crumbs) > 1 {
		lines = append(lines, label("stack:", strings.Join(o.crumbs, " > ")))
	}
	lines = append(lines, label("shape:", fmt.Sprintf("%d x %d", o.rows, o.cols)))
	lines = append(lines, label("size: ", infoHumanSize(o.size)))
	lines = append(lines, "")

	// Mini table geometry: name | type | not null | default | comment.
	const nameCap = 24
	typeW, nullW, defaultW := 12, 8, 14
	nameW := 8
	for _, r := range o.list {
		if w := len(r.Name); w > nameW {
			nameW = w
		}
	}
	if nameW > nameCap {
		nameW = nameCap
	}
	commentW := inner - nameW - typeW - nullW - defaultW - 6
	if commentW < 6 {
		commentW = 6
	}
	header := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s ",
		nameW, "column", typeW, "type", nullW, "not null", defaultW, "default", commentW, "comment")
	lines = append(lines, th.Header.Render(ansi.Truncate(header, inner, "…")))

	// How many list rows fit: full height minus borders (2) and header lines.
	avail := height - 2 - len(lines) - 1 // -1 for the footer hint
	if avail < 3 {
		avail = 3
	}
	if avail > len(o.list) {
		avail = len(o.list)
	}
	o.viewRows = max(1, avail)
	o.offset = infoClamp(o.offset, 0, max(0, len(o.list)-o.viewRows))

	switch {
	case o.loading:
		lines = append(lines, th.Subtle.Render(" loading..."))
	case o.loadErr != "":
		lines = append(lines, th.Error.Render(" "+ansi.Truncate(o.loadErr, inner, "…")))
	default:
		for i := o.offset; i < o.offset+o.viewRows && i < len(o.list); i++ {
			r := o.list[i]
			name := ansi.Truncate(r.Name, nameW, "…")
			row := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s ",
				nameW, name, typeW, ansi.Truncate(r.Type, typeW, "…"),
				nullW, ansi.Truncate(r.NotNull, nullW, "…"),
				defaultW, ansi.Truncate(r.Default, defaultW, "…"),
				commentW, ansi.Truncate(r.Comment, commentW, "…"))
			lines = append(lines, th.Text.Render(ansi.Truncate(row, inner, "…")))
		}
	}

	hint := " q/esc close"
	if !o.loading && o.loadErr == "" && len(o.list) > o.viewRows {
		hint = fmt.Sprintf(" %d-%d of %d  •  j/k scroll  •  q/esc close",
			o.offset+1, o.offset+o.viewRows, len(o.list))
	}
	lines = append(lines, th.Subtle.Render(hint))

	return ui.Box("info", strings.Join(lines, "\n"), boxW, th)
}

// infoHumanSize renders a byte count as B / KB / MB / GB with one decimal.
func infoHumanSize(n int64) string {
	const k = 1024.0
	f := float64(n)
	switch {
	case f >= k*k*k:
		return fmt.Sprintf("%.1f GB", f/(k*k*k))
	case f >= k*k:
		return fmt.Sprintf("%.1f MB", f/(k*k))
	case f >= k:
		return fmt.Sprintf("%.1f KB", f/k)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func infoClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var (
	_ ui.Overlay       = (*infoOverlay)(nil)
	_ ui.OverlayIniter = (*infoOverlay)(nil)
)
