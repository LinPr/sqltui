package reader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeTemp spools content to a temp file and wraps it in a Source.
func writeTemp(t *testing.T, name, content string) *Source {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestMarkdownMultipleTables(t *testing.T) {
	r, _ := For(FormatMarkdown)
	frames, err := r.Read(fixture(t, "tables.md"), noInferOpts(FormatMarkdown))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d tables, want 2", len(frames))
	}
	if frames[0].Name != "tables [1]" || frames[1].Name != "tables [2]" {
		t.Errorf("names = %q, %q; want %q, %q", frames[0].Name, frames[1].Name, "tables [1]", "tables [2]")
	}
	checkGrid(t, frames[0].Frame,
		[]string{"id", "name"},
		[][]any{
			{"1", "alice"},
			{"2", "bo|b"}, // escaped pipe
		})
	checkGrid(t, frames[1].Frame,
		[]string{"col_a", "col_b"},
		[][]any{
			{"x", "10"},
			{"y", "20"},
		})
}

func TestMarkdownSingleTableKeepsStem(t *testing.T) {
	r, _ := For(FormatMarkdown)
	frames, err := r.Read(fixture(t, "single.md"), noInferOpts(FormatMarkdown))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].Name != "single" {
		t.Fatalf("frames = %+v, want one frame named %q", frames, "single")
	}
	checkGrid(t, frames[0].Frame, []string{"k", "v"}, [][]any{{"a", "1"}})
}

func TestMarkdownNoTables(t *testing.T) {
	src := writeTemp(t, "prose.md", "# heading\n\njust text\n")
	r, _ := For(FormatMarkdown)
	if _, err := r.Read(src, noInferOpts(FormatMarkdown)); err == nil {
		t.Fatal("want error when file has no tables")
	}
}

func TestSplitPipeRow(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"| a | b |", []string{"a", "b"}},
		{"a | b", []string{"a", "b"}},
		{`| x \| y | z |`, []string{"x | y", "z"}},
		{"|---|:--:|", []string{"---", ":--:"}},
		{"| a |  | c |", []string{"a", "", "c"}},
	}
	for _, tt := range tests {
		if got := splitPipeRow(tt.line); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitPipeRow(%q) = %#v, want %#v", tt.line, got, tt.want)
		}
	}
}
