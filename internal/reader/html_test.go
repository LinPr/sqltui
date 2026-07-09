package reader

import "testing"

func TestHTMLMultipleTables(t *testing.T) {
	r, _ := For(FormatHTML)
	frames, err := r.Read(fixture(t, "tables.html"), noInferOpts(FormatHTML))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d tables, want 2", len(frames))
	}
	if frames[0].Name != "tables [1]" || frames[1].Name != "tables [2]" {
		t.Errorf("names = %q, %q", frames[0].Name, frames[1].Name)
	}
	// first table: header from <th> cells, nested markup flattened to text
	checkGrid(t, frames[0].Frame,
		[]string{"id", "name"},
		[][]any{
			{"1", "alice"},
			{"2", "bob jr"},
		})
	// second table: no <th>, first row becomes the header
	checkGrid(t, frames[1].Frame,
		[]string{"key", "val"},
		[][]any{
			{"x", "10"},
			{"y", "20"},
		})
}

func TestHTMLSingleTableKeepsStem(t *testing.T) {
	src := writeTemp(t, "one.html",
		`<table><tr><th>a</th></tr><tr><td>1</td></tr></table>`)
	r, _ := For(FormatHTML)
	frames, err := r.Read(src, noInferOpts(FormatHTML))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].Name != "one" {
		t.Fatalf("frames = %+v, want one frame named %q", frames, "one")
	}
	checkGrid(t, frames[0].Frame, []string{"a"}, [][]any{{"1"}})
}

func TestHTMLNoTables(t *testing.T) {
	src := writeTemp(t, "plain.html", "<html><body><p>nothing</p></body></html>")
	r, _ := For(FormatHTML)
	if _, err := r.Read(src, noInferOpts(FormatHTML)); err == nil {
		t.Fatal("want error when document has no tables")
	}
}

func TestHTMLRaggedRowPadded(t *testing.T) {
	src := writeTemp(t, "ragged.html",
		`<table>
			<tr><th>a</th><th>b</th></tr>
			<tr><td>1</td></tr>
			<tr><td>2</td><td>3</td><td>4</td></tr>
		</table>`)
	r, _ := For(FormatHTML)
	frames, err := r.Read(src, noInferOpts(FormatHTML))
	if err != nil {
		t.Fatal(err)
	}
	checkGrid(t, frames[0].Frame,
		[]string{"a", "b"},
		[][]any{
			{"1", nil},
			{"2", "3"}, // extra cell dropped
		})
}
