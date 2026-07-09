package plot

import (
	"math"
	"strings"
	"testing"

	"github.com/LinPr/sqltui/internal/theme"
)

func TestHistBuckets(t *testing.T) {
	lo, hi, counts := histBuckets([]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 10}, 5)
	if lo != 0 || hi != 10 {
		t.Fatalf("range = [%v, %v], want [0, 10]", lo, hi)
	}
	if len(counts) != 5 {
		t.Fatalf("len(counts) = %d, want 5", len(counts))
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 10 {
		t.Fatalf("total count = %d, want 10", total)
	}
	// 0,1 → b0; 2,3 → b1; 4,5 → b2; 6,7 → b3; 8,10 → b4 (max in last bucket)
	want := []int{2, 2, 2, 2, 2}
	for i := range want {
		if counts[i] != want[i] {
			t.Fatalf("counts = %v, want %v", counts, want)
		}
	}
}

func TestHistBucketsSingleValue(t *testing.T) {
	lo, hi, counts := histBuckets([]float64{3.5, 3.5, 3.5}, 10)
	if lo != 3.5 || hi != 3.5 {
		t.Fatalf("range = [%v, %v], want [3.5, 3.5]", lo, hi)
	}
	if len(counts) != 1 || counts[0] != 3 {
		t.Fatalf("counts = %v, want [3]", counts)
	}
}

func TestHistBucketsMaxLandsInLastBucket(t *testing.T) {
	_, _, counts := histBuckets([]float64{0, 10}, 4)
	if counts[len(counts)-1] != 1 {
		t.Fatalf("max value not in last bucket: %v", counts)
	}
}

func TestHistFinite(t *testing.T) {
	got := histFinite([]float64{1, math.NaN(), 2, math.Inf(1), math.Inf(-1), 3})
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("histFinite = %v, want [1 2 3]", got)
	}
}

func TestHistBar(t *testing.T) {
	if got := histBar(1, 4); got != "████" {
		t.Fatalf("full bar = %q", got)
	}
	if got := histBar(0, 4); got != "" {
		t.Fatalf("empty bar = %q", got)
	}
	if got := histBar(0.5, 4); histRuneLen(got) != 2 {
		t.Fatalf("half bar = %q (len %d), want 2 cells", got, histRuneLen(got))
	}
	// out-of-range ratios clamp instead of panicking
	if got := histBar(2, 3); got != "███" {
		t.Fatalf("clamped bar = %q", got)
	}
	if got := histBar(-1, 3); got != "" {
		t.Fatalf("negative bar = %q", got)
	}
	if got := histBar(0.5, 0); got != "" {
		t.Fatalf("zero-width bar = %q", got)
	}
}

func TestRenderHistogramEmptyAndNaN(t *testing.T) {
	th := theme.Default()
	if s := RenderHistogram(nil, 10, 60, 20, th); !strings.Contains(s, "no numeric data") {
		t.Fatalf("empty input: %q", s)
	}
	nan := []float64{math.NaN(), math.NaN()}
	if s := RenderHistogram(nan, 10, 60, 20, th); !strings.Contains(s, "no numeric data") {
		t.Fatalf("all-NaN input: %q", s)
	}
}

func TestRenderHistogramShape(t *testing.T) {
	th := theme.Default()
	vals := []float64{1, 1, 2, 2, 2, 3}
	s := RenderHistogram(vals, 3, 60, 20, th)
	lines := strings.Split(s, "\n")
	if len(lines) != 4 { // header + 3 buckets
		t.Fatalf("got %d lines, want 4:\n%s", len(lines), s)
	}
	// buckets clamp: 0 → 1 bucket, 999 → 50 buckets
	if got := strings.Split(RenderHistogram(vals, 0, 60, 20, th), "\n"); len(got) != 2 {
		t.Fatalf("buckets=0 → %d lines, want 2 (header + 1 bucket)", len(got))
	}
	if got := strings.Split(RenderHistogram(vals, 999, 200, 60, th), "\n"); len(got) != 51 {
		t.Fatalf("buckets=999 → %d lines, want 51 (header + 50 buckets)", len(got))
	}
}

func TestRenderHistogramSingleDistinctValue(t *testing.T) {
	th := theme.Default()
	s := RenderHistogram([]float64{7, 7, 7, 7}, 10, 60, 20, th)
	lines := strings.Split(s, "\n")
	if len(lines) != 2 {
		t.Fatalf("single value → %d lines, want 2:\n%s", len(lines), s)
	}
	if !strings.Contains(s, "= 7") {
		t.Fatalf("single-value label missing: %q", s)
	}
	if !strings.Contains(s, "4") {
		t.Fatalf("count missing: %q", s)
	}
}
