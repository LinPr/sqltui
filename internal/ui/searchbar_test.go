package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
)

func searchFrame() *data.Frame {
	f := data.New("name", "city")
	f.AppendRow([]any{"alice", "amsterdam"})
	f.AppendRow([]any{"bob", "berlin"})
	f.AppendRow([]any{"carol", "chicago"})
	return f
}

func TestSearchPreviewLive(t *testing.T) {
	f := searchFrame()
	var s SearchBar
	s.Start(f, false)

	if !s.Active() {
		t.Fatal("bar should be active after Start")
	}
	if s.Preview() != f {
		t.Fatal("empty query should preview the base frame")
	}

	s.Type("a")
	p := s.Preview()
	if p.NumRows() != 2 { // "alice amsterdam" and "carol chicago" contain "a"
		t.Fatalf("preview rows = %d, want 2", p.NumRows())
	}

	s.Type("li")
	if s.Query() != "ali" {
		t.Fatalf("query = %q", s.Query())
	}
	if s.Preview().NumRows() != 1 {
		t.Fatalf("preview rows = %d, want 1", s.Preview().NumRows())
	}

	s.Backspace()
	if s.Query() != "al" {
		t.Fatalf("query after backspace = %q", s.Query())
	}
	if s.Preview().NumRows() != 1 { // still only alice
		t.Fatalf("preview rows after backspace = %d, want 1", s.Preview().NumRows())
	}
}

func TestSearchCommit(t *testing.T) {
	f := searchFrame()
	var s SearchBar
	s.Start(f, false)
	s.Type("bob")

	got, crumb, ok := s.Commit()
	if !ok {
		t.Fatal("commit with a query should succeed")
	}
	if crumb != "search" {
		t.Fatalf("crumb = %q, want search", crumb)
	}
	if got.NumRows() != 1 || got.CellString(0, 0) != "bob" {
		t.Fatalf("committed frame wrong: %d rows", got.NumRows())
	}
	if s.Active() {
		t.Fatal("bar should deactivate after commit")
	}
}

func TestSearchCommitEmptyQuery(t *testing.T) {
	var s SearchBar
	s.Start(searchFrame(), true)
	if _, _, ok := s.Commit(); ok {
		t.Fatal("commit with empty query should report not-ok")
	}
	if s.Active() {
		t.Fatal("bar should deactivate")
	}
}

func TestSearchCancel(t *testing.T) {
	var s SearchBar
	s.Start(searchFrame(), true)
	s.Type("xyz")
	s.Cancel()
	if s.Active() || s.Query() != "" || s.Preview() != nil {
		t.Fatal("cancel should clear all state")
	}
}

func TestFlattenPaste(t *testing.T) {
	if got := flattenPaste("a\r\nb\tc\x01d"); got != "a b cd" {
		t.Fatalf("flattenPaste = %q, want %q", got, "a b cd")
	}
	if got := flattenPaste("plain"); got != "plain" {
		t.Fatalf("flattenPaste passthrough = %q", got)
	}
	if got := flattenPaste("\r\x7f"); got != "" {
		t.Fatalf("control-only paste = %q, want empty", got)
	}
}

func TestSearchPaste(t *testing.T) {
	f := searchFrame()
	var s SearchBar
	s.Start(f, false)

	// A multi-character paste lands as one edit and updates the preview.
	s.Paste("bob")
	if s.Query() != "bob" {
		t.Fatalf("query = %q, want bob", s.Query())
	}
	if s.Preview().NumRows() != 1 {
		t.Fatalf("preview rows = %d, want 1", s.Preview().NumRows())
	}

	// Newlines and CR are flattened; empty paste is a no-op.
	s.Cancel()
	s.Start(f, false)
	s.Paste("a\r\nli")
	if s.Query() != "a li" {
		t.Fatalf("flattened query = %q, want %q", s.Query(), "a li")
	}
	s.Paste("")
	if s.Query() != "a li" {
		t.Fatalf("empty paste changed query: %q", s.Query())
	}

	// An inactive bar ignores pastes.
	s.Cancel()
	s.Paste("x")
	if s.Query() != "" {
		t.Fatalf("inactive bar accepted paste: %q", s.Query())
	}
}

// pasteRecorder is a stub overlay that records pasted content.
type pasteRecorder struct{ got string }

func (r *pasteRecorder) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	if pm, ok := msg.(tea.PasteMsg); ok {
		r.got += pm.Content
	}
	return r, nil
}

func (r *pasteRecorder) View(w, h int, th *theme.Theme) string { return "" }

func TestAppPasteRouting(t *testing.T) {
	a := newTestApp(t)

	// No overlay, no search: the paste is dropped without side effects.
	a.Update(tea.PasteMsg{Content: "zzz"})
	if a.search.Active() {
		t.Fatal("stray paste must not activate search")
	}

	// Active search bar receives pasted text, flattened.
	a.Update(key("/"))
	a.Update(tea.PasteMsg{Content: "row\n1"})
	if got := a.search.Query(); got != "row 1" {
		t.Fatalf("search query after paste = %q, want %q", got, "row 1")
	}

	// With an overlay open, the paste goes to the top overlay, not the bar.
	rec := &pasteRecorder{}
	a.Update(PushOverlayMsg{Overlay: rec})
	a.Update(tea.PasteMsg{Content: "abc"})
	if rec.got != "abc" {
		t.Fatalf("overlay paste = %q, want abc", rec.got)
	}
	if got := a.search.Query(); got != "row 1" {
		t.Fatalf("search bar stole overlay paste: %q", got)
	}
}

func TestSearchFuzzyCrumb(t *testing.T) {
	var s SearchBar
	s.Start(searchFrame(), true)
	s.Type("crl")
	got, crumb, ok := s.Commit()
	if !ok || crumb != "fuzzy" {
		t.Fatalf("crumb = %q ok=%v, want fuzzy", crumb, ok)
	}
	if got.NumRows() != 1 || got.CellString(0, 0) != "carol" {
		t.Fatalf("fuzzy result wrong: %d rows", got.NumRows())
	}
}
