package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
)

// Composite centers box over base within a w x h area, splicing the box into
// the base line by line. Both strings may contain ANSI escapes; splicing is
// width-aware. Base lines are padded (with spaces) to w and the base is
// padded to h lines so the overlay always lands where expected.
func Composite(base, box string, w, h int) string {
	if w <= 0 || h <= 0 {
		return base
	}
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > h {
		baseLines = baseLines[:h]
	}

	boxLines := strings.Split(box, "\n")
	bw := 0
	for _, l := range boxLines {
		if lw := ansi.StringWidth(l); lw > bw {
			bw = lw
		}
	}
	if bw > w {
		bw = w
	}
	bh := len(boxLines)
	if bh > h {
		boxLines = boxLines[:h]
		bh = h
	}

	x := (w - bw) / 2
	y := (h - bh) / 2

	out := make([]string, len(baseLines))
	for i, bl := range baseLines {
		if pad := w - ansi.StringWidth(bl); pad > 0 {
			bl = bl + strings.Repeat(" ", pad)
		}
		j := i - y
		if j < 0 || j >= bh {
			out[i] = bl
			continue
		}
		line := ansi.Truncate(boxLines[j], bw, "")
		if pad := bw - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		left := ansi.Truncate(bl, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		right := ansi.TruncateLeft(bl, x+bw, "")
		// TruncateLeft keeps a double-width rune that straddles the cut
		// column, leaving the fragment one cell too wide; the composited line
		// would then hard-wrap in the terminal. Drop the straddling rune and
		// pad with spaces so the line stays exactly w cells.
		if d := ansi.StringWidth(right) - (w - x - bw); d > 0 {
			right = strings.Repeat(" ", d) + ansi.TruncateLeft(bl, x+bw+d, "")
		}
		out[i] = left + line + right
	}
	return strings.Join(out, "\n")
}

// FillPage centers box within a blank w x h area: every line is padded to
// exactly w cells and the area to exactly h lines, so a fullscreen page
// overlay lets nothing from the view behind it show through.
func FillPage(box string, w, h int) string {
	if w <= 0 || h <= 0 {
		return box
	}
	return Composite(strings.Repeat("\n", h-1), box, w, h)
}

// Box frames content in a rounded border of total width w, with the title
// embedded in the top edge. Content lines are truncated/padded to fit.
func Box(title, content string, w int, th *theme.Theme) string {
	return boxWith(title, content, w, th.PopupBorder, th.PopupTitle)
}

// boxWith is Box with explicit border/title styles (used by the error box).
func boxWith(title, content string, w int, border, titleStyle lipgloss.Style) string {
	if w < 4 {
		w = 4
	}
	inner := w - 2

	var b strings.Builder
	b.WriteString(border.Render("╭"))
	used := 0
	if title != "" {
		if tw := ansi.StringWidth(title); tw > inner-4 {
			title = ansi.Truncate(title, max(inner-4, 1), "…")
		}
		b.WriteString(border.Render("─ "))
		b.WriteString(titleStyle.Render(title))
		b.WriteString(border.Render(" "))
		used = 3 + ansi.StringWidth(title)
	}
	if fill := inner - used; fill > 0 {
		b.WriteString(border.Render(strings.Repeat("─", fill)))
	}
	b.WriteString(border.Render("╮"))
	b.WriteString("\n")

	for _, line := range strings.Split(content, "\n") {
		line = ansi.Truncate(line, inner, "…")
		if pad := inner - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(border.Render("│"))
		b.WriteString(line)
		b.WriteString(border.Render("│"))
		b.WriteString("\n")
	}

	b.WriteString(border.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}
