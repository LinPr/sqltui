package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
)

// appName/appVersion label the right edge of the status bar.
const (
	appName    = "sqltui"
	appVersion = "0.1.0"
)

// statusInfo carries the values the status bar displays.
type statusInfo struct {
	Tab, NTabs int      // 1-based tab index, tab count
	Row, NRows int      // 1-based selected row, row count
	NCols      int      // column count
	Crumbs     []string // breadcrumb chain
	Col        string   // current column name ("" hides the tag)
	Mode       string   // column mode: "wide" or "fit" ("" hides the tag)
	Conn       string   // connection title in db mode ("" hides it)
}

// sbSeg is one status bar segment. drop orders graceful degradation on
// narrow terminals: segments with the highest drop value vanish first,
// drop 0 never drops (the final width truncation still applies).
type sbSeg struct {
	text string
	drop int
}

// dropOne removes the segment with the highest positive drop value from the
// two lists. It reports false when nothing is droppable.
func dropOne(left, right []sbSeg) (l, r []sbSeg, ok bool) {
	li, ri, best := -1, -1, 0
	for i, s := range left {
		if s.drop > best {
			li, ri, best = i, -1, s.drop
		}
	}
	for i, s := range right {
		if s.drop > best {
			li, ri, best = -1, i, s.drop
		}
	}
	switch {
	case li >= 0:
		return append(left[:li], left[li+1:]...), right, true
	case ri >= 0:
		return left, append(right[:ri], right[ri+1:]...), true
	default:
		return left, right, false
	}
}

// segsWidth is the display width of segments joined with sep-cell gaps.
func segsWidth(segs []sbSeg, sep int) int {
	w := 0
	for i, s := range segs {
		if i > 0 {
			w += sep
		}
		w += ansi.StringWidth(s.text)
	}
	return w
}

// renderStatusBar draws the bottom bar: colored segment tags on the left
// (tab, row, shape, current column, column mode), the breadcrumb in the
// middle (truncated from the left) and the connection title plus app name on
// the right. When the bar does not fit, the least important segments are
// dropped first so the line always stays within width.
func renderStatusBar(info statusInfo, width int, th *theme.Theme) string {
	if width <= 0 {
		return ""
	}
	bg := lipgloss.Color(th.Palette.Bg)

	tag := func(i int, text string) string {
		return lipgloss.NewStyle().
			Background(th.SeriesColor(i)).
			Foreground(bg).
			Bold(true).
			Render(" " + text + " ")
	}

	left := []sbSeg{
		{text: tag(0, fmt.Sprintf("Tab %d/%d", info.Tab, info.NTabs))},
		{text: tag(1, fmt.Sprintf("Row %d", info.Row))},
		{text: tag(2, fmt.Sprintf("%d x %d", info.NRows, info.NCols)), drop: 1},
	}
	if info.Col != "" {
		left = append(left, sbSeg{text: tag(3, "col: "+info.Col), drop: 4})
	}
	if info.Mode != "" {
		left = append(left, sbSeg{text: tag(4, "["+info.Mode+"]"), drop: 5})
	}

	var right []sbSeg
	if info.Conn != "" {
		right = append(right, sbSeg{text: th.StatusBar.Render(" " + info.Conn + " "), drop: 3})
	}
	right = append(right, sbSeg{text: th.StatusBar.Render(" " + appName + " " + appVersion + " "), drop: 2})

	// Degrade: drop segments until tags + right side leave at least one cell
	// of slack for the breadcrumb area.
	for segsWidth(left, 1)+segsWidth(right, 0)+1 > width {
		var ok bool
		if left, right, ok = dropOne(left, right); !ok {
			break
		}
	}

	sp := th.StatusBar.Render(" ")
	leftParts := make([]string, len(left))
	for i, s := range left {
		leftParts[i] = s.text
	}
	leftStr := strings.Join(leftParts, sp)
	var rightStr string
	for _, s := range right {
		rightStr += s.text
	}

	crumb := strings.Join(info.Crumbs, " > ")
	room := width - ansi.StringWidth(leftStr) - ansi.StringWidth(rightStr) - 2
	if room < 0 {
		room = 0
	}
	if ansi.StringWidth(crumb) > room {
		w := ansi.StringWidth(crumb)
		crumb = ansi.TruncateLeft(crumb, w-room+1, "…")
	}
	mid := th.StatusBar.Render(" " + crumb)

	pad := width - ansi.StringWidth(leftStr) - ansi.StringWidth(mid) - ansi.StringWidth(rightStr)
	if pad < 0 {
		pad = 0
	}
	line := leftStr + mid + th.StatusBar.Render(strings.Repeat(" ", pad)) + rightStr
	if ansi.StringWidth(line) > width {
		line = ansi.Truncate(line, width, "")
	}
	return line
}
