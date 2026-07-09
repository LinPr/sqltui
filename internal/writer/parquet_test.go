package writer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/writer"
)

// TestParquetRoundtrip writes a frame covering every dtype (plus nulls) and
// reads it back through the parquet reader, once per compression codec.
func TestParquetRoundtrip(t *testing.T) {
	codecs := []writer.Compression{
		writer.CompressionNone,
		writer.CompressionSnappy,
		writer.CompressionGzip,
		writer.CompressionZstd,
		writer.CompressionLZ4,
		writer.CompressionBrotli,
	}
	for _, codec := range codecs {
		t.Run(string(codec), func(t *testing.T) {
			frame := sampleFrame()
			path := filepath.Join(t.TempDir(), "sample.parquet")
			fh, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			w, err := writer.For(writer.FormatParquet)
			if err != nil {
				t.Fatal(err)
			}
			opt := writer.DefaultOptions()
			opt.Compression = codec
			if err := w.Write(fh, frame, opt); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := fh.Close(); err != nil {
				t.Fatal(err)
			}

			src, err := reader.FromFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := reader.For(reader.FormatParquet)
			if err != nil {
				t.Fatal(err)
			}
			frames, err := r.Read(src, reader.DefaultOptions())
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if len(frames) != 1 {
				t.Fatalf("got %d frames, want 1", len(frames))
			}
			if frames[0].Name != "sample" {
				t.Errorf("frame name = %q, want %q", frames[0].Name, "sample")
			}
			got := frames[0].Frame

			if !reflect.DeepEqual(got.ColumnNames(), frame.ColumnNames()) {
				t.Fatalf("column names = %v, want %v", got.ColumnNames(), frame.ColumnNames())
			}
			for i, col := range frame.Columns {
				if got.Columns[i].Type != col.Type {
					t.Errorf("column %q type = %v, want %v", col.Name, got.Columns[i].Type, col.Type)
				}
				if !reflect.DeepEqual(got.Columns[i].Cells, col.Cells) {
					t.Errorf("column %q cells = %#v, want %#v", col.Name, got.Columns[i].Cells, col.Cells)
				}
			}
		})
	}
}

func TestParquetUnsupportedCompression(t *testing.T) {
	w, err := writer.For(writer.FormatParquet)
	if err != nil {
		t.Fatal(err)
	}
	opt := writer.DefaultOptions()
	opt.Compression = writer.Compression("bogus")
	fh, err := os.Create(filepath.Join(t.TempDir(), "x.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if err := w.Write(fh, sampleFrame(), opt); err == nil {
		t.Error("expected error for unsupported compression")
	}
}

// TestParquetDuplicateColumnNames ensures empty and duplicate column names
// are made unique in the output schema instead of colliding.
func TestParquetDuplicateColumnNames(t *testing.T) {
	frame := &data.Frame{Columns: []data.Column{
		{Name: "", Type: data.TypeInt, Cells: []any{int64(1)}},
		{Name: "x", Type: data.TypeInt, Cells: []any{int64(2)}},
		{Name: "x", Type: data.TypeInt, Cells: []any{int64(3)}},
	}}
	path := filepath.Join(t.TempDir(), "dup.parquet")
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := writer.For(writer.FormatParquet)
	if err := w.Write(fh, frame, writer.DefaultOptions()); err != nil {
		t.Fatalf("write: %v", err)
	}
	fh.Close()

	src, err := reader.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := reader.For(reader.FormatParquet)
	frames, err := r.Read(src, reader.DefaultOptions())
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := frames[0].Frame
	want := []string{"column_1", "x", "x_2"}
	if !reflect.DeepEqual(got.ColumnNames(), want) {
		t.Errorf("column names = %v, want %v", got.ColumnNames(), want)
	}
	for i, cell := range []any{int64(1), int64(2), int64(3)} {
		if !reflect.DeepEqual(got.Columns[i].Cells, []any{cell}) {
			t.Errorf("column %d cells = %#v, want [%#v]", i, got.Columns[i].Cells, cell)
		}
	}
}
