package reader

import (
	"reflect"
	"testing"

	"github.com/LinPr/sqltui/internal/data"
)

func TestFWFAutoDetect(t *testing.T) {
	nf := readOne(t, "data.fwf", noInferOpts(FormatFWF))
	checkGrid(t, nf.Frame,
		[]string{"id", "name", "city"},
		[][]any{
			{"1", "alice", "paris"},
			{"2", "bob", "oslo"},
			{"30", "carola", "lisbon"},
		})
}

func TestFWFAutoDetectNoHeader(t *testing.T) {
	opt := noInferOpts(FormatFWF)
	opt.NoHeader = true
	nf := readOne(t, "data.fwf", opt)
	f := nf.Frame
	want := []string{"column_1", "column_2", "column_3"}
	if got := f.ColumnNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	if f.NumRows() != 4 {
		t.Fatalf("rows = %d, want 4", f.NumRows())
	}
	if got := f.Row(0); !reflect.DeepEqual(got, []any{"id", "name", "city"}) {
		t.Errorf("row 0 = %#v", got)
	}
}

func TestFWFExplicitWidths(t *testing.T) {
	opt := noInferOpts(FormatFWF)
	opt.Widths = []int{2, 2, 2}
	opt.SeparatorLength = 1
	nf := readOne(t, "widths.fwf", opt)
	checkGrid(t, nf.Frame,
		[]string{"aa", "bb", "cc"},
		[][]any{
			{"11", "x2", "33"},
			{"44", "y5", "66"},
		})
}

func TestFWFTypeInference(t *testing.T) {
	opt := DefaultOptions()
	opt.Format = FormatFWF
	nf := readOne(t, "data.fwf", opt)
	if got := nf.Frame.Columns[0].Type; got != data.TypeInt {
		t.Errorf("id column type = %v, want TypeInt", got)
	}
	if got := nf.Frame.Cell(2, 0); got != int64(30) {
		t.Errorf("cell(2,0) = %#v, want int64(30)", got)
	}
}

func TestDetectFwfSpans(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  [][2]int
	}{
		{
			name:  "two columns",
			lines: []string{"ab  cd", "xy  zw"},
			want:  [][2]int{{0, 4}, {4, 6}},
		},
		{
			name:  "empty input",
			lines: nil,
			want:  nil,
		},
		{
			name:  "single column",
			lines: []string{"abc", "de"},
			want:  [][2]int{{0, 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFwfSpans(tt.lines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("detectFwfSpans(%q) = %v, want %v", tt.lines, got, tt.want)
			}
		})
	}
}
