package plot

import (
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/LinPr/sqltui/internal/theme"
)

// scatDotBits maps a dot position inside one character cell (col 0..1,
// row 0..3) to its bit in the braille pattern block (U+2800 + bits).
var scatDotBits = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// maximum number of entries spelled out on the legend line.
const scatMaxLegend = 12

// RenderScatter draws an x/y scatter plot on a braille-dot canvas (2x4 dots
// per character cell) sized to fit within w x h, including y-axis labels on
// the left, an x-axis label line at the bottom and, when groups is non-nil,
// a legend line mapping each distinct group to its series color.
//
// Points with NaN/Inf on either axis are skipped; groups (when non-nil) is
// indexed alongside xs/ys. Empty input and constant axes are handled.
func RenderScatter(xs, ys []float64, groups []string, w, h int, th *theme.Theme) string {
	type point struct {
		x, y  float64
		group int
	}
	var names []string
	index := map[string]int{}
	var pts []point
	n := min(len(xs), len(ys))
	for i := 0; i < n; i++ {
		x, y := xs[i], ys[i]
		if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
			continue
		}
		gi := 0
		if groups != nil {
			name := ""
			if i < len(groups) {
				name = groups[i]
			}
			var ok bool
			if gi, ok = index[name]; !ok {
				gi = len(names)
				index[name] = gi
				names = append(names, name)
			}
		}
		pts = append(pts, point{x, y, gi})
	}
	if len(pts) == 0 {
		return th.Subtle.Render("no numeric data to plot")
	}

	xmin, xmax := pts[0].x, pts[0].x
	ymin, ymax := pts[0].y, pts[0].y
	for _, p := range pts {
		xmin, xmax = math.Min(xmin, p.x), math.Max(xmax, p.x)
		ymin, ymax = math.Min(ymin, p.y), math.Max(ymax, p.y)
	}

	yminS, ymaxS := scatFmt(ymin), scatFmt(ymax)
	gutter := max(len(yminS), len(ymaxS))
	legendLines := 0
	if groups != nil {
		legendLines = 1
	}
	plotW := max(w-gutter-1, 2)
	plotH := max(h-2-legendLines, 2)

	// Rasterize points onto the dot grid; the first point to hit a cell
	// decides that cell's color.
	cells := make([][]rune, plotH)
	owner := make([][]int, plotH)
	for r := range cells {
		cells[r] = make([]rune, plotW)
		owner[r] = make([]int, plotW)
		for c := range owner[r] {
			owner[r][c] = -1
		}
	}
	for _, p := range pts {
		dx := scatProject(p.x, xmin, xmax, plotW*2)
		dy := plotH*4 - 1 - scatProject(p.y, ymin, ymax, plotH*4)
		cr, cc := dy/4, dx/2
		cells[cr][cc] |= scatDotBits[dy%4][dx%2]
		if owner[cr][cc] < 0 {
			owner[cr][cc] = p.group
		}
	}

	styles := make([]lipgloss.Style, max(len(names), 1))
	for i := range styles {
		styles[i] = lipgloss.NewStyle().Foreground(th.SeriesColor(i))
	}

	var b strings.Builder
	for r := 0; r < plotH; r++ {
		label := ""
		switch r {
		case 0:
			label = ymaxS
		case plotH - 1:
			label = yminS
		}
		b.WriteString(th.Subtle.Render(strings.Repeat(" ", gutter-len(label)) + label + "│"))
		// Batch runs of same-colored cells to keep the string small.
		run := strings.Builder{}
		runOwner := -2
		flush := func() {
			if run.Len() == 0 {
				return
			}
			if runOwner < 0 {
				b.WriteString(run.String())
			} else {
				b.WriteString(styles[runOwner].Render(run.String()))
			}
			run.Reset()
		}
		for c := 0; c < plotW; c++ {
			ch, own := ' ', -1
			if cells[r][c] != 0 {
				ch, own = 0x2800+cells[r][c], owner[r][c]
			}
			if own != runOwner {
				flush()
				runOwner = own
			}
			run.WriteRune(ch)
		}
		flush()
		b.WriteString("\n")
	}

	b.WriteString(th.Subtle.Render(strings.Repeat(" ", gutter) + "└" + strings.Repeat("─", plotW)))
	b.WriteString("\n")
	b.WriteString(th.Subtle.Render(scatAxisLine(scatFmt(xmin), scatFmt(xmax), gutter, plotW)))

	if groups != nil {
		b.WriteString("\n")
		shown := min(len(names), scatMaxLegend)
		parts := make([]string, 0, shown+1)
		for i := 0; i < shown; i++ {
			name := names[i]
			if name == "" {
				name = "(null)"
			}
			parts = append(parts, styles[i].Render("●")+" "+th.Text.Render(name))
		}
		if len(names) > shown {
			parts = append(parts, th.Subtle.Render("+"+strconv.Itoa(len(names)-shown)+" more"))
		}
		b.WriteString(strings.Join(parts, "  "))
	}
	return b.String()
}

// scatAxisLine lays out the x-axis min/max labels under the plot area.
func scatAxisLine(minS, maxS string, gutter, plotW int) string {
	line := strings.Repeat(" ", gutter+1) + minS
	gap := plotW - len(minS) - len(maxS)
	if gap >= 1 {
		line += strings.Repeat(" ", gap) + maxS
	}
	return line
}

// scatProject maps v in [lo, hi] onto a dot index in [0, n). A degenerate
// range (hi <= lo, i.e. a constant axis) maps everything to the center.
func scatProject(v, lo, hi float64, n int) int {
	if n <= 1 {
		return 0
	}
	if hi <= lo {
		return n / 2
	}
	i := int((v-lo)/(hi-lo)*float64(n-1) + 0.5)
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	return i
}

// scatFmt formats an axis label compactly.
func scatFmt(v float64) string {
	return strconv.FormatFloat(v, 'g', 4, 64)
}
