package data

import (
	"testing"
	"time"
)

func TestNullCount(t *testing.T) {
	tests := []struct {
		name  string
		cells []any
		want  int
	}{
		{"empty column", nil, 0},
		{"no nulls", []any{"a", "b"}, 0},
		{"some nulls", []any{nil, "a", nil, int64(1)}, 2},
		{"empty string is not null", []any{""}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Column{Name: "c", Cells: tt.cells}
			if got := NullCount(&c); got != tt.want {
				t.Errorf("NullCount = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEstimatedSize(t *testing.T) {
	tests := []struct {
		name string
		f    *Frame
		want int64
	}{
		{"empty frame", New("a"), 0},
		{
			"strings are len+16",
			frameOf(Column{Name: "s", Type: TypeString, Cells: []any{"abc", ""}}),
			(3 + 16) + (0 + 16),
		},
		{
			"scalars are 16, times 24",
			frameOf(Column{Name: "m", Cells: []any{int64(1), 2.5, true, nil, time.Now()}}),
			16 + 16 + 16 + 16 + 24,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimatedSize(tt.f); got != tt.want {
				t.Errorf("EstimatedSize = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestShape(t *testing.T) {
	f := frameOf(
		Column{Name: "a", Cells: []any{int64(1), int64(2), int64(3)}},
		Column{Name: "b", Cells: []any{"x", "y", "z"}},
	)
	rows, cols := Shape(f)
	if rows != 3 || cols != 2 {
		t.Errorf("Shape = (%d, %d), want (3, 2)", rows, cols)
	}
}
