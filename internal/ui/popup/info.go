package popup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
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
	Name  string
	Dtype string
	Nulls int
}

// infoOverlay shows metadata about the current frame: title, breadcrumb,
// shape, estimated size and a scrollable per-column listing.
type infoOverlay struct {
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
		title:    ctx.BaseCrumb(),
		crumbs:   ctx.Crumbs(),
		rows:     f.NumRows(),
		cols:     f.NumCols(),
		size:     data.EstimatedSize(f),
		viewRows: 10,
	}
	for i := range f.Columns {
		c := &f.Columns[i]
		o.list = append(o.list, infoColRow{
			Name:  c.Name,
			Dtype: c.Type.String(),
			Nulls: data.NullCount(c),
		})
	}
	return o
}

func (o *infoOverlay) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
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
	case "pgdown", "ctrl+d", "ctrl+f":
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

	// Mini table geometry: name | dtype | nulls.
	dtypeW, nullsW := 8, 8
	nameW := inner - 2 - dtypeW - 1 - nullsW - 1
	if nameW < 8 {
		nameW = 8
	}
	header := fmt.Sprintf(" %-*s %-*s %*s ", nameW, "column", dtypeW, "type", nullsW, "nulls")
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

	for i := o.offset; i < o.offset+o.viewRows && i < len(o.list); i++ {
		r := o.list[i]
		name := ansi.Truncate(r.Name, nameW, "…")
		row := fmt.Sprintf(" %-*s %-*s %*d ", nameW, name, dtypeW, r.Dtype, nullsW, r.Nulls)
		lines = append(lines, th.Text.Render(ansi.Truncate(row, inner, "…")))
	}

	hint := " q/esc close"
	if len(o.list) > o.viewRows {
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
