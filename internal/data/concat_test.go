package data

import (
	"reflect"
	"testing"
)

func TestConcat(t *testing.T) {
	tests := []struct {
		name      string
		frames    []*Frame
		wantNames []string
		wantTypes []DType
		wantCols  [][]any
		wantErr   bool
	}{
		{
			name:    "no frames",
			frames:  nil,
			wantErr: true,
		},
		{
			name:    "nil frame",
			frames:  []*Frame{nil},
			wantErr: true,
		},
		{
			name: "single frame copies through",
			frames: []*Frame{
				frameOf(Column{Name: "a", Type: TypeInt, Cells: []any{int64(1)}}),
			},
			wantNames: []string{"a"},
			wantTypes: []DType{TypeInt},
			wantCols:  [][]any{{int64(1)}},
		},
		{
			name: "same schema stacks rows",
			frames: []*Frame{
				frameOf(Column{Name: "a", Type: TypeInt, Cells: []any{int64(1), int64(2)}}),
				frameOf(Column{Name: "a", Type: TypeInt, Cells: []any{int64(3)}}),
			},
			wantNames: []string{"a"},
			wantTypes: []DType{TypeInt},
			wantCols:  [][]any{{int64(1), int64(2), int64(3)}},
		},
		{
			name: "union fills missing with nulls",
			frames: []*Frame{
				frameOf(Column{Name: "a", Type: TypeInt, Cells: []any{int64(1)}}),
				frameOf(Column{Name: "b", Type: TypeString, Cells: []any{"x"}}),
			},
			wantNames: []string{"a", "b"},
			wantTypes: []DType{TypeInt, TypeString},
			wantCols: [][]any{
				{int64(1), nil},
				{nil, "x"},
			},
		},
		{
			name: "type conflict unifies to string",
			frames: []*Frame{
				frameOf(Column{Name: "a", Type: TypeInt, Cells: []any{int64(1), nil}}),
				frameOf(Column{Name: "a", Type: TypeFloat, Cells: []any{2.5}}),
			},
			wantNames: []string{"a"},
			wantTypes: []DType{TypeString},
			wantCols:  [][]any{{"1", nil, "2.5"}},
		},
		{
			name: "column order is first-seen across frames",
			frames: []*Frame{
				frameOf(
					Column{Name: "b", Type: TypeString, Cells: []any{"b1"}},
					Column{Name: "a", Type: TypeString, Cells: []any{"a1"}},
				),
				frameOf(
					Column{Name: "c", Type: TypeString, Cells: []any{"c2"}},
					Column{Name: "a", Type: TypeString, Cells: []any{"a2"}},
				),
			},
			wantNames: []string{"b", "a", "c"},
			wantTypes: []DType{TypeString, TypeString, TypeString},
			wantCols: [][]any{
				{"b1", nil},
				{"a1", "a2"},
				{nil, "c2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Concat(tt.frames...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if names := got.ColumnNames(); !reflect.DeepEqual(names, tt.wantNames) {
				t.Fatalf("names = %v, want %v", names, tt.wantNames)
			}
			for i := range got.Columns {
				if got.Columns[i].Type != tt.wantTypes[i] {
					t.Errorf("col %q type = %v, want %v", got.Columns[i].Name, got.Columns[i].Type, tt.wantTypes[i])
				}
				if !reflect.DeepEqual(got.Columns[i].Cells, tt.wantCols[i]) {
					t.Errorf("col %q cells = %#v, want %#v", got.Columns[i].Name, got.Columns[i].Cells, tt.wantCols[i])
				}
			}
		})
	}
}
