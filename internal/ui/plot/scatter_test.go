package plot

import (
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
)

func TestScatProject(t *testing.T) {
	if got := scatProject(0, 0, 10, 100); got != 0 {
		t.Fatalf("min → %d, want 0", got)
	}
	if got := scatProject(10, 0, 10, 100); got != 99 {
		t.Fatalf("max → %d, want 99", got)
	}
	if got := scatProject(5, 0, 10, 101); got != 50 {
		t.Fatalf("mid → %d, want 50", got)
	}
	// constant axis maps to the center
	if got := scatProject(7, 7, 7, 100); got != 50 {
		t.Fatalf("constant axis → %d, want 50", got)
	}
	// out-of-range clamps
	if got := scatProject(-5, 0, 10, 100); got != 0 {
		t.Fatalf("below min → %d, want 0", got)
	}
	if got := scatProject(15, 0, 10, 100); got != 99 {
		t.Fatalf("above max → %d, want 99", got)
	}
	if got := scatProject(3, 0, 10, 1); got != 0 {
		t.Fatalf("n=1 → %d, want 0", got)
	}
}

func TestScatDotBits(t *testing.T) {
	// All 8 bits are distinct and cover exactly 0xff.
	seen := rune(0)
	for cy := 0; cy < 4; cy++ {
		for cx := 0; cx < 2; cx++ {
			b := scatDotBits[cy][cx]
			if seen&b != 0 {
				t.Fatalf("duplicate bit at (%d,%d)", cx, cy)
			}
			seen |= b
		}
	}
	if seen != 0xff {
		t.Fatalf("bits cover %#x, want 0xff", seen)
	}
	// Spot-check the standard braille layout.
	if scatDotBits[0][0] != 0x01 || scatDotBits[3][1] != 0x80 {
		t.Fatalf("unexpected braille bit layout")
	}
}

func TestRenderScatterEmpty(t *testing.T) {
	th := theme.Default()
	if s := RenderScatter(nil, nil, nil, 60, 20, th); !strings.Contains(s, "no numeric data") {
		t.Fatalf("empty input: %q", s)
	}
	nan := []float64{math.NaN()}
	if s := RenderScatter(nan, nan, nil, 60, 20, th); !strings.Contains(s, "no numeric data") {
		t.Fatalf("all-NaN input: %q", s)
	}
}

func TestRenderScatterBasic(t *testing.T) {
	th := theme.Default()
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{0, 10, 20, 30, 40}
	s := RenderScatter(xs, ys, nil, 60, 15, th)
	plain := ansi.Strip(s)
	if !strings.Contains(plain, "40") || !strings.Contains(plain, "0") {
		t.Fatalf("axis labels missing:\n%s", plain)
	}
	// at least one braille dot must be present
	hasDot := false
	for _, r := range plain {
		if r >= 0x2801 && r <= 0x28ff {
			hasDot = true
			break
		}
	}
	if !hasDot {
		t.Fatalf("no braille dots rendered:\n%s", plain)
	}
	// no groups → no legend line beyond plot + axis + labels
	lines := strings.Split(plain, "\n")
	if got, want := len(lines), 15-0; got > want {
		t.Fatalf("got %d lines, want <= %d", got, want)
	}
}

func TestRenderScatterGroupsLegend(t *testing.T) {
	th := theme.Default()
	xs := []float64{1, 2, 3, 4}
	ys := []float64{1, 2, 3, 4}
	groups := []string{"a", "b", "a", "b"}
	plain := ansi.Strip(RenderScatter(xs, ys, groups, 60, 15, th))
	last := plain[strings.LastIndex(plain, "\n")+1:]
	if !strings.Contains(last, "a") || !strings.Contains(last, "b") || !strings.Contains(last, "●") {
		t.Fatalf("legend line missing groups: %q", last)
	}
}

func TestRenderScatterConstantAxis(t *testing.T) {
	th := theme.Default()
	xs := []float64{5, 5, 5}
	ys := []float64{1, 2, 3}
	s := ansi.Strip(RenderScatter(xs, ys, nil, 60, 12, th))
	if strings.Contains(s, "no numeric data") {
		t.Fatalf("constant x-axis treated as empty:\n%s", s)
	}
	// both constant
	both := ansi.Strip(RenderScatter([]float64{5}, []float64{5}, nil, 60, 12, th))
	hasDot := false
	for _, r := range both {
		if r >= 0x2801 && r <= 0x28ff {
			hasDot = true
			break
		}
	}
	if !hasDot {
		t.Fatalf("single constant point not drawn:\n%s", both)
	}
}

func TestScatAxisLine(t *testing.T) {
	line := scatAxisLine("0", "100", 4, 20)
	if !strings.HasPrefix(line, strings.Repeat(" ", 5)+"0") {
		t.Fatalf("min label misplaced: %q", line)
	}
	if !strings.HasSuffix(line, "100") {
		t.Fatalf("max label misplaced: %q", line)
	}
	if got := len(line); got != 4+1+20 {
		t.Fatalf("axis line width = %d, want %d", got, 4+1+20)
	}
	// too narrow for both labels: keep min only
	narrow := scatAxisLine("12345", "67890", 2, 8)
	if strings.Contains(narrow, "67890") {
		t.Fatalf("max label should be dropped when it cannot fit: %q", narrow)
	}
}
