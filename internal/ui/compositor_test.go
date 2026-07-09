package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func plainBase(ch string, w, h int) string {
	line := strings.Repeat(ch, w)
	lines := make([]string, h)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func TestCompositePlain(t *testing.T) {
	base := plainBase("a", 6, 5)
	box := "XX\nXX"
	got := Composite(base, box, 6, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	// box is 2x2, centered: x=(6-2)/2=2, y=(5-2)/2=1
	want := []string{"aaaaaa", "aaXXaa", "aaXXaa", "aaaaaa", "aaaaaa"}
	for i, l := range lines {
		if l != want[i] {
			t.Fatalf("line %d = %q, want %q\nfull:\n%s", i, l, want[i], got)
		}
	}
}

func TestCompositeStyled(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Background(lipgloss.Color("#000000"))
	base := ""
	for i := 0; i < 5; i++ {
		if i > 0 {
			base += "\n"
		}
		base += styled.Render(strings.Repeat("b", 8))
	}
	box := styled.Render("OO") + "\n" + styled.Render("OO")
	got := Composite(base, box, 8, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 8 {
			t.Fatalf("line %d width = %d, want 8 (%q)", i, w, l)
		}
	}
	// stripped content check: box centered at x=3, y=1
	mid := ansi.Strip(lines[2])
	if mid != "bbbOObbb" {
		t.Fatalf("stripped middle line = %q, want bbbOObbb", mid)
	}
	top := ansi.Strip(lines[0])
	if top != "bbbbbbbb" {
		t.Fatalf("stripped top line = %q", top)
	}
}

func TestCompositeBoxLargerThanBase(t *testing.T) {
	base := plainBase("a", 4, 2)
	box := plainBase("X", 10, 6)
	got := Composite(base, box, 4, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected clipping to 2 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if ansi.StringWidth(l) != 4 {
			t.Fatalf("line %d not clipped to width 4: %q", i, l)
		}
	}
}

func TestCompositeWideRunesAtRightEdge(t *testing.T) {
	// A double-width rune straddling the overlay's right cut column must not
	// leave the composited line one cell too wide (it would hard-wrap).
	for w := 19; w <= 24; w++ {
		line := strings.Repeat("汉", w/2)
		if w%2 == 1 {
			line += "x"
		}
		base := strings.Join([]string{line, line, line, line, line}, "\n")
		for bw := 1; bw <= 6; bw++ {
			box := strings.Repeat("A", bw)
			out := Composite(base, box, w, 5)
			for i, line := range strings.Split(out, "\n") {
				if got := ansi.StringWidth(line); got != w {
					t.Fatalf("w=%d box=%d line %d width = %d, want %d (%q)", w, bw, i, got, w, line)
				}
			}
		}
	}
}
