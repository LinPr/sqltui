package popup

import (
	"strings"
	"testing"

	"github.com/LinPr/sqltui/internal/ui"
)

func TestHelpBuildLines(t *testing.T) {
	lines := helpBuildLines()
	if len(lines) == 0 {
		t.Fatal("no help lines built")
	}

	var sections []string
	entries := 0
	for _, l := range lines {
		if l.Section != "" {
			sections = append(sections, l.Section)
		} else {
			entries++
		}
	}

	want := []string{"Global", "Table", "Sheet"}
	if len(Commands) > 0 {
		want = append(want, "Commands (:)")
	}
	if len(sections) != len(want) {
		t.Fatalf("sections = %v, want %v", sections, want)
	}
	for i, s := range want {
		if sections[i] != s {
			t.Errorf("section[%d] = %q, want %q", i, sections[i], s)
		}
	}

	wantEntries := len(ui.GlobalBindings) + len(ui.TableBindings) + len(ui.SheetBindings) + len(Commands)
	if entries != wantEntries {
		t.Errorf("entries = %d, want %d", entries, wantEntries)
	}
}

func TestHelpBuildLinesJoinsKeys(t *testing.T) {
	lines := helpBuildLines()
	// GlobalBindings[0] is the first entry after the "Global" header.
	b := ui.GlobalBindings[0]
	got := lines[1]
	if got.Keys != strings.Join(b.Keys, ", ") {
		t.Errorf("first entry keys = %q, want %q", got.Keys, strings.Join(b.Keys, ", "))
	}
	if got.Text != b.Help {
		t.Errorf("first entry text = %q, want %q", got.Text, b.Help)
	}
}

func TestHelpScrollAndClose(t *testing.T) {
	o := newHelpOverlay()
	o.viewRows = 5

	if o.helpKey("j") {
		t.Fatal("j must not close")
	}
	if o.offset != 1 {
		t.Fatalf("offset after j = %d, want 1", o.offset)
	}
	o.helpKey("k")
	o.helpKey("k") // clamp at 0
	if o.offset != 0 {
		t.Fatalf("offset after k,k = %d, want 0", o.offset)
	}

	o.helpKey("G")
	maxOff := max(0, len(o.lines)-o.viewRows)
	if o.offset != maxOff {
		t.Fatalf("offset after G = %d, want %d", o.offset, maxOff)
	}
	o.helpKey("pgdown") // clamped
	if o.offset != maxOff {
		t.Fatalf("offset after pgdown at bottom = %d, want %d", o.offset, maxOff)
	}
	o.helpKey("g")
	if o.offset != 0 {
		t.Fatalf("offset after g = %d, want 0", o.offset)
	}

	for _, k := range []string{"q", "esc", "f1"} {
		if !newHelpOverlay().helpKey(k) {
			t.Errorf("key %q must close the overlay", k)
		}
	}
	if newHelpOverlay().helpKey("z") {
		t.Error("unbound key must not close the overlay")
	}
}
