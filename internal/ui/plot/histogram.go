// Package plot renders pure-text charts (histograms, scatter plots) for the
// UI. Functions here take an area and a theme and return a styled string;
// they never import bubbletea and hold no state.
package plot

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/LinPr/sqltui/internal/theme"
)

// histPartials are the left-aligned partial block runes, from 1/8 to 7/8.
var histPartials = []rune("▏▎▍▌▋▊▉")

// RenderHistogram draws a horizontal-bar histogram of values into a string
// no wider than w. One line per bucket: right-aligned bucket range, a bar of
// block runes proportional to the bucket count, and the count itself. The
// caller is expected to scroll vertically when the result has more lines
// than h; the renderer never truncates buckets.
//
// NaN/Inf values are skipped. With no usable values a friendly message is
// returned. buckets is clamped to 1..50; a single distinct value collapses
// to one bucket.
func RenderHistogram(values []float64, buckets int, w, h int, th *theme.Theme) string {
	_ = h // vertical scrolling is the caller's job
	vals := histFinite(values)
	if len(vals) == 0 {
		return th.Subtle.Render("no numeric data to plot")
	}
	buckets = histClamp(buckets, 1, 50)
	if w < 16 {
		w = 16
	}

	lo, hi, counts := histBuckets(vals, buckets)
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	labels := make([]string, len(counts))
	if lo == hi {
		labels[0] = "= " + histFmt(lo)
	} else {
		span := hi - lo
		n := float64(len(counts))
		for i := range counts {
			l := lo + span*float64(i)/n
			r := lo + span*float64(i+1)/n
			labels[i] = histFmt(l) + " .. " + histFmt(r)
		}
	}
	labelW := 0
	for _, l := range labels {
		if len(l) > labelW {
			labelW = len(l)
		}
	}
	countW := len(strconv.Itoa(maxCount))
	barW := w - labelW - countW - 2 // "label bar count"
	if barW < 1 {
		barW = 1
	}

	barStyle := lipgloss.NewStyle().Foreground(th.SeriesColor(0))

	var b strings.Builder
	b.WriteString(th.Subtle.Render(fmt.Sprintf("%d values in [%s, %s]", len(vals), histFmt(lo), histFmt(hi))))
	for i, c := range counts {
		b.WriteString("\n")
		bar := ""
		if maxCount > 0 {
			bar = histBar(float64(c)/float64(maxCount), barW)
		}
		if c > 0 && bar == "" {
			bar = string(histPartials[0]) // never hide a non-empty bucket
		}
		b.WriteString(th.Subtle.Render(fmt.Sprintf("%*s ", labelW, labels[i])))
		b.WriteString(barStyle.Render(bar))
		b.WriteString(strings.Repeat(" ", barW-histRuneLen(bar)+1))
		b.WriteString(th.Text.Render(strconv.Itoa(c)))
	}
	return b.String()
}

// histFinite filters out NaN and Inf values.
func histFinite(values []float64) []float64 {
	out := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

// histBuckets computes the value range and per-bucket counts for n buckets.
// vals must be non-empty and finite. When all values are identical a single
// bucket holding everything is returned regardless of n.
func histBuckets(vals []float64, n int) (lo, hi float64, counts []int) {
	lo, hi = vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if lo == hi {
		return lo, hi, []int{len(vals)}
	}
	counts = make([]int, n)
	span := hi - lo
	for _, v := range vals {
		i := int((v - lo) / span * float64(n))
		if i >= n {
			i = n - 1 // v == hi lands in the last bucket
		}
		if i < 0 {
			i = 0
		}
		counts[i]++
	}
	return lo, hi, counts
}

// histBar renders a bar of block runes filling ratio (0..1) of width cells,
// using eighth-block partials for the fractional tail.
func histBar(ratio float64, width int) string {
	if width <= 0 {
		return ""
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	eighths := int(math.Round(ratio * float64(width) * 8))
	full, rem := eighths/8, eighths%8
	if full >= width { // guard rounding at ratio==1
		return strings.Repeat("█", width)
	}
	s := strings.Repeat("█", full)
	if rem > 0 {
		s += string(histPartials[rem-1])
	}
	return s
}

// histFmt formats an axis/bucket-edge number compactly.
func histFmt(v float64) string {
	return strconv.FormatFloat(v, 'g', 4, 64)
}

func histClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// histRuneLen counts runes (all block runes are single-width).
func histRuneLen(s string) int { return len([]rune(s)) }
