package writer_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/writer"
)

var update = flag.Bool("update", false, "rewrite golden files")

// sampleFrame covers every cell type plus nulls and separator/pipe/quote
// characters that need escaping.
func sampleFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "s", Type: data.TypeString, Cells: []any{
			"hello", `pipe|and,comma "q"`, nil,
		}},
		{Name: "i", Type: data.TypeInt, Cells: []any{
			int64(1), int64(-42), nil,
		}},
		{Name: "f", Type: data.TypeFloat, Cells: []any{
			float64(1.5), float64(-0.25), nil,
		}},
		{Name: "b", Type: data.TypeBool, Cells: []any{
			true, false, nil,
		}},
		{Name: "d", Type: data.TypeDate, Cells: []any{
			time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			time.Date(1969, 12, 31, 0, 0, 0, 0, time.UTC),
			nil,
		}},
		{Name: "ts", Type: data.TypeDatetime, Cells: []any{
			time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			time.Date(2023, 6, 30, 23, 59, 59, 0, time.UTC),
			nil,
		}},
	}}
}

func TestTextWriters(t *testing.T) {
	noHeader := writer.DefaultOptions()
	noHeader.Header = false
	semicolon := writer.DefaultOptions()
	semicolon.Separator = ';'
	pretty := writer.DefaultOptions()
	pretty.Pretty = true

	tests := []struct {
		name   string
		format writer.Format
		opt    writer.Options
		golden string
	}{
		{"csv", writer.FormatCSV, writer.DefaultOptions(), "sample.csv.golden"},
		{"csv-noheader", writer.FormatCSV, noHeader, "sample_noheader.csv.golden"},
		{"csv-semicolon", writer.FormatCSV, semicolon, "sample_semicolon.csv.golden"},
		{"tsv", writer.FormatTSV, writer.DefaultOptions(), "sample.tsv.golden"},
		{"json", writer.FormatJSON, writer.DefaultOptions(), "sample.json.golden"},
		{"json-pretty", writer.FormatJSON, pretty, "sample_pretty.json.golden"},
		{"jsonl", writer.FormatJSONL, writer.DefaultOptions(), "sample.jsonl.golden"},
		{"markdown", writer.FormatMarkdown, writer.DefaultOptions(), "sample.md.golden"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, err := writer.For(tc.format)
			if err != nil {
				t.Fatal(err)
			}
			buf := &bytes.Buffer{}
			if err := w.Write(buf, sampleFrame(), tc.opt); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", tc.golden)
			if *update {
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != string(want) {
				t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestJSONEmptyFrame(t *testing.T) {
	w, err := writer.For(writer.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	if err := w.Write(buf, data.New("a", "b"), writer.DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "[]\n" {
		t.Errorf("empty frame: got %q, want %q", got, "[]\n")
	}
}

func TestUnknownFormat(t *testing.T) {
	if _, err := writer.For(writer.Format("nope")); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestJSONDuplicateColumnNames(t *testing.T) {
	f := &data.Frame{Columns: []data.Column{
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(1)}},
		{Name: "a", Type: data.TypeInt, Cells: []any{int64(2)}},
	}}
	for _, format := range []writer.Format{writer.FormatJSON, writer.FormatJSONL} {
		w, err := writer.For(format)
		if err != nil {
			t.Fatal(err)
		}
		buf := &bytes.Buffer{}
		if err := w.Write(buf, f, writer.DefaultOptions()); err != nil {
			t.Fatal(err)
		}
		got := buf.String()
		if !strings.Contains(got, `"a":1`) || !strings.Contains(got, `"a_2":2`) {
			t.Errorf("%s: duplicate columns not deduped: %q", format, got)
		}
	}
}

func TestMarkdownBackslashPipeRoundtrip(t *testing.T) {
	f := &data.Frame{Columns: []data.Column{
		{Name: "s", Type: data.TypeString, Cells: []any{`a\|b`, `a\b`}},
		{Name: "z", Type: data.TypeString, Cells: []any{"z1", "z2"}},
	}}
	w, err := writer.For(writer.FormatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	if err := w.Write(buf, f, writer.DefaultOptions()); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "t.md")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := reader.FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	rd, err := reader.For(reader.FormatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	opt := reader.DefaultOptions()
	opt.Format = reader.FormatMarkdown
	opt.InferSchema = reader.InferNo
	frames, err := rd.Read(src, opt)
	if err != nil {
		t.Fatal(err)
	}
	got := frames[0].Frame
	wantRows := [][]any{{`a\|b`, "z1"}, {`a\b`, "z2"}}
	if got.NumCols() != 2 || got.NumRows() != 2 {
		t.Fatalf("shape = %dx%d, want 2x2", got.NumRows(), got.NumCols())
	}
	for i, want := range wantRows {
		for c := range want {
			if got.Columns[c].Cells[i] != want[c] {
				t.Errorf("row %d col %d = %#v, want %#v", i, c, got.Columns[c].Cells[i], want[c])
			}
		}
	}
}
