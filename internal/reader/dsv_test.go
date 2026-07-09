package reader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LinPr/sqltui/internal/data"
)

// fixture builds a Source over a testdata file.
func fixture(t *testing.T, name string) *Source {
	t.Helper()
	src, err := FromFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	return src
}

// noInferOpts returns defaults with type inference disabled, so parsed cells
// stay strings and tests are independent of the inference implementation.
func noInferOpts(f Format) Options {
	opt := DefaultOptions()
	opt.Format = f
	opt.InferSchema = InferNo
	return opt
}

// readOne runs a registered reader and asserts a single frame comes back.
func readOne(t *testing.T, name string, opt Options) NamedFrame {
	t.Helper()
	r, err := For(opt.Format)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := r.Read(fixture(t, name), opt)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if len(frames) != 1 {
		t.Fatalf("read %s: got %d frames, want 1", name, len(frames))
	}
	return frames[0]
}

func checkGrid(t *testing.T, f *data.Frame, header []string, rows [][]any) {
	t.Helper()
	if got := f.ColumnNames(); !reflect.DeepEqual(got, header) {
		t.Fatalf("columns = %v, want %v", got, header)
	}
	if f.NumRows() != len(rows) {
		t.Fatalf("rows = %d, want %d", f.NumRows(), len(rows))
	}
	for i, want := range rows {
		if got := f.Row(i); !reflect.DeepEqual(got, want) {
			t.Errorf("row %d = %#v, want %#v", i, got, want)
		}
	}
}

func TestDSVBasic(t *testing.T) {
	nf := readOne(t, "basic.csv", noInferOpts(FormatCSV))
	if nf.Name != "basic" {
		t.Errorf("name = %q, want %q", nf.Name, "basic")
	}
	checkGrid(t, nf.Frame,
		[]string{"id", "name", "score"},
		[][]any{
			{"1", "alice", "3.5"},
			{"2", "bob", "4.25"},
			{"3", "carol", "2.75"},
		})
}

func TestDSVTypeInference(t *testing.T) {
	opt := DefaultOptions()
	opt.Format = FormatCSV
	nf := readOne(t, "basic.csv", opt)
	f := nf.Frame
	if f.Columns[0].Type != data.TypeInt {
		t.Errorf("id column type = %v, want TypeInt", f.Columns[0].Type)
	}
	if got := f.Cell(0, 0); got != int64(1) {
		t.Errorf("id cell = %#v, want int64(1)", got)
	}
	if f.Columns[2].Type != data.TypeFloat {
		t.Errorf("score column type = %v, want TypeFloat", f.Columns[2].Type)
	}
	if f.Columns[1].Type != data.TypeString {
		t.Errorf("name column type = %v, want TypeString", f.Columns[1].Type)
	}
}

func TestDSVNoHeader(t *testing.T) {
	opt := noInferOpts(FormatCSV)
	opt.NoHeader = true
	nf := readOne(t, "noheader.csv", opt)
	checkGrid(t, nf.Frame,
		[]string{"column_1", "column_2"},
		[][]any{{"1", "alice"}, {"2", "bob"}})
}

func TestDSVRagged(t *testing.T) {
	tests := []struct {
		name     string
		truncate bool
		ignore   bool
		wantErr  bool
		wantRows [][]any
	}{
		{
			name:    "long row errors by default",
			wantErr: true,
		},
		{
			name:     "truncate keeps trimmed long rows",
			truncate: true,
			wantRows: [][]any{
				{"1", "2", nil},
				{"1", "2", "3"},
				{"5", "6", "7"},
			},
		},
		{
			name:   "ignore drops long rows",
			ignore: true,
			wantRows: [][]any{
				{"1", "2", nil},
				{"5", "6", "7"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := noInferOpts(FormatCSV)
			opt.TruncateRagged = tt.truncate
			opt.IgnoreErrors = tt.ignore
			r, _ := For(FormatCSV)
			frames, err := r.Read(fixture(t, "ragged.csv"), opt)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error for long row, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			checkGrid(t, frames[0].Frame, []string{"a", "b", "c"}, tt.wantRows)
		})
	}
}

func TestTSV(t *testing.T) {
	nf := readOne(t, "basic.tsv", noInferOpts(FormatTSV))
	checkGrid(t, nf.Frame,
		[]string{"id", "name"},
		[][]any{{"1", "alice"}, {"2", "bo,b"}})
}

func TestCSVQuoted(t *testing.T) {
	nf := readOne(t, "quoted.csv", noInferOpts(FormatCSV))
	checkGrid(t, nf.Frame,
		[]string{"a", "b"},
		[][]any{
			{"x,y", "2"},
			{`he said "hi"`, "3"},
		})
}

func TestDSVCustomSeparatorAndQuote(t *testing.T) {
	opt := noInferOpts(FormatDSV)
	opt.Separator = ';'
	opt.Quote = '\''
	nf := readOne(t, "custquote.dsv", opt)
	checkGrid(t, nf.Frame,
		[]string{"a", "b"},
		[][]any{
			{"x;y", "2"},
			{`it's "fine"`, "3"},
		})
}

func TestDSVStripsUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.csv")
	if err := os.WriteFile(p, []byte("\xef\xbb\xbfid,name\n1,alice\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := (dsvReader{}).Read(src, noInferOpts(FormatCSV))
	if err != nil {
		t.Fatal(err)
	}
	f := frames[0].Frame
	if got := f.ColumnNames(); !reflect.DeepEqual(got, []string{"id", "name"}) {
		t.Fatalf("columns = %q, want [id name]", got)
	}
	if f.ColumnIndex("id") != 0 {
		t.Errorf("ColumnIndex(id) = %d, want 0", f.ColumnIndex("id"))
	}
}

func TestJSONStripsUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bom.json")
	if err := os.WriteFile(p, []byte("\xef\xbb\xbf[{\"a\": 1}]"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := (jsonReader{}).Read(src, noInferOpts(FormatJSON))
	if err != nil {
		t.Fatalf("BOM-prefixed json failed to parse: %v", err)
	}
	checkGrid(t, frames[0].Frame, []string{"a"}, [][]any{{int64(1)}})
}
