// Package popup contains the modal overlays reachable from the command
// palette. Each file registers its factory in ui.Factories from init().
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
	ui.Factories["histogram"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		f := ctx.CurrentFrame()
		if f == nil || f.NumCols() == 0 {
			return nil, errors.New("histogram: no data loaded")
		}
		b := &histBuilder{frame: f, input: histNewInput("10")}
		if arg = strings.TrimSpace(arg); arg != "" {
			idx := f.ColumnIndex(arg)
			if idx < 0 {
				return nil, fmt.Errorf("histogram: no column named %q", arg)
			}
			vals := histColumnFloats(f, idx)
			if len(vals) == 0 {
				return nil, fmt.Errorf("histogram: column %q has no numeric values", arg)
			}
			b.sel, b.colName, b.values, b.step = idx, arg, vals, histStepBuckets
		}
		return b, nil
	}
}

const (
	histStepColumn = iota
	histStepBuckets
)

// histBuilder is the two-step histogram wizard: pick a numeric-castable
// column (type-to-filter, arrows navigate), then a bucket count. Enter on
// the last step replaces the builder with the full-screen histPlot overlay.
type histBuilder struct {
	frame *data.Frame
	step  int

	filter   colFilter // column filter
	sel, off int       // column list state (indices into filter.idx)
	colName  string
	values   []float64

	input   histInput
	errText string
}

func (b *histBuilder) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return b, nil
	}
	switch b.step {
	case histStepColumn:
		names := b.frame.ColumnNames()
		b.filter.ensure(names)
		switch key.String() {
		case "esc":
			if b.filter.clear(names) {
				b.sel, b.off, b.errText = 0, 0, ""
				return b, nil
			}
			return b, ui.CloseOverlay
		case "ctrl+c":
			return b, ui.CloseOverlay
		case "ctrl+u":
			if b.filter.clear(names) {
				b.sel, b.off, b.errText = 0, 0, ""
			}
		case "up":
			b.sel = histClampInt(b.sel-1, 0, max(0, len(b.filter.idx)-1))
		case "down":
			b.sel = histClampInt(b.sel+1, 0, max(0, len(b.filter.idx)-1))
		case "home":
			b.sel = 0
		case "end":
			b.sel = max(0, len(b.filter.idx)-1)
		case "enter":
			if len(b.filter.idx) == 0 {
				return b, nil
			}
			ci := b.filter.idx[histClampInt(b.sel, 0, len(b.filter.idx)-1)]
			vals := histColumnFloats(b.frame, ci)
			if len(vals) == 0 {
				b.errText = fmt.Sprintf("column %q has no numeric values", b.frame.Columns[ci].Name)
				return b, nil
			}
			b.errText = ""
			b.colName = b.frame.Columns[ci].Name
			b.values = vals
			b.step = histStepBuckets
		default:
			if b.filter.key(key.String(), names) {
				b.sel, b.off, b.errText = 0, 0, ""
			}
		}
	case histStepBuckets:
		switch key.String() {
		case "esc":
			b.step = histStepColumn
			b.errText = ""
		case "ctrl+c":
			return b, ui.CloseOverlay
		case "enter":
			buckets := 10
			if s := strings.TrimSpace(b.input.String()); s != "" {
				v, err := strconv.Atoi(s)
				if err != nil || v < 1 || v > 50 {
					b.errText = "bucket count must be a number between 1 and 50"
					return b, nil
				}
				buckets = v
			}
			return &histPlot{
				title:   fmt.Sprintf("histogram · %s", b.colName),
				values:  b.values,
				buckets: buckets,
			}, nil
		default:
			b.input.Handle(key)
		}
	}
	return b, nil
}

func (b *histBuilder) View(width, height int, th *theme.Theme) string {
	bw := histClampInt(52, 24, max(24, width-4))
	inner := bw - 2
	var lines []string
	switch b.step {
	case histStepColumn:
		b.filter.ensure(b.frame.ColumnNames())
		idx := b.filter.idx
		lines = append(lines, th.Subtle.Render(" pick a numeric column"))
		lines = append(lines, b.filter.line(th))
		visible := histClampInt(height-10, 3, 12)
		b.sel = histClampInt(b.sel, 0, max(0, len(idx)-1))
		b.off = histScrollTo(b.sel, b.off, visible, len(idx))
		if len(idx) == 0 {
			lines = append(lines, th.Placeholder.Render("  no matching columns"))
		}
		for i := b.off; i < min(b.off+visible, len(idx)); i++ {
			col := b.frame.Columns[idx[i]]
			label := fmt.Sprintf(" %-*s %s ", inner-8, col.Name, col.Type)
			if i == b.sel {
				lines = append(lines, th.ListSelected.Render(label))
			} else {
				lines = append(lines, th.ListItem.Render(label))
			}
		}
		lines = append(lines, histFooter(b.errText, " enter select · esc close", th))
	case histStepBuckets:
		lines = append(lines,
			th.Subtle.Render(fmt.Sprintf(" column %s · %d numeric values", b.colName, len(b.values))),
			"",
			th.Text.Render(" buckets: ")+b.input.Render(th),
			histFooter(b.errText, " enter plot · esc back", th),
		)
	}
	return ui.Box("histogram", strings.Join(lines, "\n"), bw, th)
}

// histFooter renders the inline error (if any) above the key hint.
func histFooter(errText, hint string, th *theme.Theme) string {
	out := ""
	if errText != "" {
		out = th.Error.Render(" "+errText) + "\n"
	}
	return out + th.Subtle.Render(hint)
}

// histPlot is the full-screen result overlay. It scrolls vertically when the
// rendered histogram is taller than the window; q/esc closes back to the
// table (not the builder).
type histPlot struct {
	title   string
	values  []float64
	buckets int

	off  int
	page int // last visible body height, for paging
}

func (p *histPlot) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return p, ui.CloseOverlay
	case "up", "k":
		p.off--
	case "down", "j":
		p.off++
	case "pgup", "ctrl+b", "ctrl+u":
		p.off -= max(p.page, 1)
	case "pgdown", "ctrl+f", "ctrl+d":
		p.off += max(p.page, 1)
	case "g", "home":
		p.off = 0
	case "G", "end":
		p.off = 1 << 30 // clamped against content in View
	}
	if p.off < 0 {
		p.off = 0
	}
	return p, nil
}

func (p *histPlot) View(width, height int, th *theme.Theme) string {
	bw := max(min(width-2, width), 24)
	inner := bw - 2
	body := max(height-5, 3) // borders + hint + breathing room
	p.page = body

	lines := strings.Split(plot.RenderHistogram(p.values, p.buckets, inner-2, body, th), "\n")
	p.off = histClampInt(p.off, 0, max(0, len(lines)-body))

	out := make([]string, 0, body+1)
	for i := p.off; i < min(p.off+body, len(lines)); i++ {
		out = append(out, " "+lines[i])
	}
	for len(out) < body {
		out = append(out, "")
	}
	hint := " q close"
	if len(lines) > body {
		hint = " j/k scroll · q close"
	}
	out = append(out, th.Subtle.Render(hint))
	return ui.Box(p.title, strings.Join(out, "\n"), bw, th)
}

// histColumnFloats extracts the float-castable cells of column col: int64
// and float64 directly, bool/nil skipped, everything else via its display
// string and strconv. Unparsable and null cells are skipped.
func histColumnFloats(f *data.Frame, col int) []float64 {
	if f == nil || col < 0 || col >= f.NumCols() {
		return nil
	}
	out := make([]float64, 0, f.NumRows())
	for _, cell := range f.Columns[col].Cells {
		if v, ok := histFloat(cell); ok {
			out = append(out, v)
		}
	}
	return out
}

// histFloat casts one cell value to float64 when possible.
func histFloat(cell any) (float64, bool) {
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

// histScrollTo adjusts a list offset so sel stays inside the visible window.
func histScrollTo(sel, off, visible, total int) int {
	if sel < off {
		off = sel
	}
	if sel >= off+visible {
		off = sel - visible + 1
	}
	return histClampInt(off, 0, max(0, total-visible))
}

func histClampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// histInput is a minimal single-line text input (value + cursor).
type histInput struct {
	val []rune
	cur int
}

func histNewInput(initial string) histInput {
	r := []rune(initial)
	return histInput{val: r, cur: len(r)}
}

func (in *histInput) String() string { return string(in.val) }

// Handle applies one key press to the input state.
func (in *histInput) Handle(key tea.KeyPressMsg) {
	switch key.String() {
	case "backspace":
		if in.cur > 0 {
			in.val = append(in.val[:in.cur-1], in.val[in.cur:]...)
			in.cur--
		}
	case "delete":
		if in.cur < len(in.val) {
			in.val = append(in.val[:in.cur], in.val[in.cur+1:]...)
		}
	case "left":
		if in.cur > 0 {
			in.cur--
		}
	case "right":
		if in.cur < len(in.val) {
			in.cur++
		}
	case "home", "ctrl+a":
		in.cur = 0
	case "end", "ctrl+e":
		in.cur = len(in.val)
	case "ctrl+u":
		in.val = in.val[:0]
		in.cur = 0
	default:
		if key.Text != "" {
			ins := []rune(key.Text)
			in.val = append(in.val[:in.cur], append(ins, in.val[in.cur:]...)...)
			in.cur += len(ins)
		}
	}
}

// Render draws the value with a block cursor.
func (in *histInput) Render(th *theme.Theme) string {
	before := string(in.val[:in.cur])
	under := " "
	after := ""
	if in.cur < len(in.val) {
		under = string(in.val[in.cur])
		after = string(in.val[in.cur+1:])
	}
	return th.Input.Render(before) + th.ListSelected.Render(under) + th.Input.Render(after)
}
