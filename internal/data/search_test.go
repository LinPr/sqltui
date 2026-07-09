package data

import (
	"reflect"
	"testing"
)

func searchFixture() *Frame {
	return frameOf(
		Column{Name: "name", Type: TypeString, Cells: []any{"apple pie", "Banana", "cherry", nil}},
		Column{Name: "qty", Type: TypeInt, Cells: []any{int64(10), int64(200), int64(3), int64(42)}},
	)
}

func TestSearchExact(t *testing.T) {
	f := searchFixture()
	tests := []struct {
		name   string
		needle string
		want   []int
	}{
		{"substring in one column", "err", []int{2}},
		{"case sensitive", "banana", []int{}},
		{"matches display string of int", "200", []int{1}},
		{"empty needle matches all", "", []int{0, 1, 2, 3}},
		{"no match", "zzz", []int{}},
		{"row matched once despite two hits", "0", []int{0, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchExact(f, tt.needle)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchExact(%q) = %v, want %v", tt.needle, got, tt.want)
			}
		})
	}
}

func TestSearchFuzzy(t *testing.T) {
	f := searchFixture()
	tests := []struct {
		name   string
		needle string
		want   []int
	}{
		{"empty needle matches all", "", []int{0, 1, 2, 3}},
		{"subsequence match", "apie", []int{0}},
		{"no match", "xyzq", []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchFuzzy(f, tt.needle)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchFuzzy(%q) = %v, want %v", tt.needle, got, tt.want)
			}
		})
	}
}

func TestSearchFuzzyPreservesFrameOrder(t *testing.T) {
	f := frameOf(Column{Name: "w", Type: TypeString, Cells: []any{
		"xxcatxx", "cat", "c-a-t", "dog",
	}})
	got := SearchFuzzy(f, "cat")
	want := []int{0, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SearchFuzzy order = %v, want %v", got, want)
	}
}
