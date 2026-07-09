package popup

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
	"github.com/LinPr/sqltui/internal/ui/plot"
)

func init() {
	ui.Factories["scatterplot"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		f := ctx.CurrentFrame()
		if f == nil || f.NumCols() == 0 {
			return nil, errors.New("scatterplot: no data loaded")
		}
		return &scatBuilder{frame: f}, nil
	}
}

const (
	scatStepX = iota
	scatStepY
	scatStepGroup
)

// scatNone is the first entry of the group-by list (skip grouping).
const scatNone = "(none)"

// scatBuilder is the three-step scatter wizard: pick the X column, the Y
// column, then an optional group-by column (each list is type-to-filter,
// arrows navigate). Enter on the last step replaces the builder with the
// full-screen scatPlot overlay.
type scatBuilder struct {
	frame *data.Frame
	step  int

	filter   colFilter // per-step item filter (reset on step change)
	sel, off int       // list state (indices into filter.idx)
	xCol     int
	yCol     int

	errText string
}

// scatItems returns the list entries for the current step.
func (b *scatBuilder) scatItems() []string {
	names := b.frame.ColumnNames()
	if b.step == scatStepGroup {
		return append([]string{scatNone}, names...)
	}
	return names
}

func (b *scatBuilder) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return b, nil
	}
	items := b.scatItems()
	b.filter.ensure(items)
	switch key.String() {
	case "ctrl+c":
		return b, ui.CloseOverlay
	case "esc":
		if b.filter.clear(items) {
			b.sel, b.off, b.errText = 0, 0, ""
			return b, nil
		}
		if b.step == scatStepX {
			return b, ui.CloseOverlay
		}
		b.step--
		b.scatResetList()
	case "up":
		b.sel = scatClampInt(b.sel-1, 0, max(0, len(b.filter.idx)-1))
	case "down":
		b.sel = scatClampInt(b.sel+1, 0, max(0, len(b.filter.idx)-1))
	case "home":
		b.sel = 0
	case "end":
		b.sel = max(0, len(b.filter.idx)-1)
	case "ctrl+u":
		if b.filter.clear(items) {
			b.sel, b.off, b.errText = 0, 0, ""
		}
	case "enter":
		return b.scatAdvance()
	default:
		if b.filter.key(key.String(), items) {
			b.sel, b.off, b.errText = 0, 0, ""
		}
	}
	return b, nil
}

// scatResetList clears the list state and the filter for a new step.
func (b *scatBuilder) scatResetList() {
	b.sel, b.off, b.errText = 0, 0, ""
	b.filter = colFilter{}
	b.filter.ensure(b.scatItems())
}

// scatAdvance handles enter on each step.
func (b *scatBuilder) scatAdvance() (ui.Overlay, tea.Cmd) {
	items := b.scatItems()
	b.filter.ensure(items)
	if len(b.filter.idx) == 0 {
		return b, nil
	}
	itemIdx := b.filter.idx[scatClampInt(b.sel, 0, len(b.filter.idx)-1)]
	switch b.step {
	case scatStepX, scatStepY:
		if len(scatColumnFloats(b.frame, itemIdx)) == 0 {
			b.errText = fmt.Sprintf("column %q has no numeric values", b.frame.Columns[itemIdx].Name)
			return b, nil
		}
		if b.step == scatStepX {
			b.xCol = itemIdx
		} else {
			b.yCol = itemIdx
		}
		b.step++
		b.scatResetList()
		return b, nil
	default: // scatStepGroup
		groupCol := itemIdx - 1 // "(none)" occupies index 0
		xs, ys, groups := scatPoints(b.frame, b.xCol, b.yCol, groupCol)
		if len(xs) == 0 {
			b.errText = "no rows with numeric values in both columns"
			return b, nil
		}
		title := fmt.Sprintf("scatter · %s vs %s",
			b.frame.Columns[b.xCol].Name, b.frame.Columns[b.yCol].Name)
		if groupCol >= 0 {
			title += " by " + b.frame.Columns[groupCol].Name
		}
		return &scatPlot{title: title, xs: xs, ys: ys, groups: groups}, nil
	}
}

func (b *scatBuilder) View(width, height int, th *theme.Theme) string {
	bw := scatClampInt(52, 24, max(24, width-4))
	inner := bw - 2
	items := b.scatItems()

	head := ""
	switch b.step {
	case scatStepX:
		head = " pick the X column"
	case scatStepY:
		head = fmt.Sprintf(" x = %s · pick the Y column", b.frame.Columns[b.xCol].Name)
	case scatStepGroup:
		head = fmt.Sprintf(" x = %s · y = %s · group by",
			b.frame.Columns[b.xCol].Name, b.frame.Columns[b.yCol].Name)
	}

	b.filter.ensure(items)
	idx := b.filter.idx
	lines := []string{th.Subtle.Render(head), b.filter.line(th)}
	visible := scatClampInt(height-10, 3, 12)
	b.sel = scatClampInt(b.sel, 0, max(0, len(idx)-1))
	b.off = scatScrollTo(b.sel, b.off, visible, len(idx))
	if len(idx) == 0 {
		lines = append(lines, th.Placeholder.Render("  no matching columns"))
	}
	for i := b.off; i < min(b.off+visible, len(idx)); i++ {
		itemIdx := idx[i]
		dtype := ""
		if ci := itemIdx - (len(items) - b.frame.NumCols()); ci >= 0 {
			dtype = b.frame.Columns[ci].Type.String()
		}
		label := fmt.Sprintf(" %-*s %s ", inner-8, items[itemIdx], dtype)
		if i == b.sel {
			lines = append(lines, th.ListSelected.Render(label))
		} else {
			lines = append(lines, th.ListItem.Render(label))
		}
	}
	if b.errText != "" {
		lines = append(lines, th.Error.Render(" "+b.errText))
	}
	hint := " enter select · esc close"
	if b.step != scatStepX {
		hint = " enter select · esc back"
	}
	lines = append(lines, th.Subtle.Render(hint))
	return ui.Box("scatterplot", strings.Join(lines, "\n"), bw, th)
}

// scatPlot is the full-screen scatter overlay; q/esc closes back to the
// table (not the builder). It re-renders to the current window size.
type scatPlot struct {
	title  string
	xs, ys []float64
	groups []string
}

func (p *scatPlot) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			return p, ui.CloseOverlay
		}
	}
	return p, nil
}

func (p *scatPlot) View(width, height int, th *theme.Theme) string {
	bw := max(min(width-2, width), 24)
	inner := bw - 2
	body := max(height-5, 4)

	lines := strings.Split(plot.RenderScatter(p.xs, p.ys, p.groups, inner-2, body, th), "\n")
	out := make([]string, 0, body+1)
	for i := 0; i < min(len(lines), body); i++ {
		out = append(out, " "+lines[i])
	}
	for len(out) < body {
		out = append(out, "")
	}
	out = append(out, th.Subtle.Render(" q close"))
	return ui.Box(p.title, strings.Join(out, "\n"), bw, th)
}

// scatPoints extracts the plottable rows: both x and y must cast to float.
// groupCol < 0 disables grouping (groups returned nil); null group cells
// become "(null)".
func scatPoints(f *data.Frame, xCol, yCol, groupCol int) (xs, ys []float64, groups []string) {
	for r := 0; r < f.NumRows(); r++ {
		x, okX := scatFloat(f.Cell(r, xCol))
		y, okY := scatFloat(f.Cell(r, yCol))
		if !okX || !okY {
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
		if groupCol >= 0 {
			s := data.FormatValue(f.Cell(r, groupCol))
			if s == "" {
				s = "(null)"
			}
			groups = append(groups, s)
		}
	}
	return xs, ys, groups
}

// scatColumnFloats extracts the float-castable cells of one column.
func scatColumnFloats(f *data.Frame, col int) []float64 {
	if f == nil || col < 0 || col >= f.NumCols() {
		return nil
	}
	out := make([]float64, 0, f.NumRows())
	for _, cell := range f.Columns[col].Cells {
		if v, ok := scatFloat(cell); ok {
			out = append(out, v)
		}
	}
	return out
}

// scatFloat casts one cell value to float64 when possible: int64/float64
// directly, nil/bool skipped, everything else parsed from its display string.
func scatFloat(cell any) (float64, bool) {
	switch v := cell.(type) {
	case nil:
		return 0, false
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case bool:
		return 0, false
	default:
		s := strings.TrimSpace(data.FormatValue(cell))
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
}

// scatScrollTo adjusts a list offset so sel stays inside the visible window.
func scatScrollTo(sel, off, visible, total int) int {
	if sel < off {
		off = sel
	}
	if sel >= off+visible {
		off = sel - visible + 1
	}
	return scatClampInt(off, 0, max(0, total-visible))
}

func scatClampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
