package popup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/ui"
)

func importerKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func importerType(o *importer, s string) {
	for _, r := range s {
		o.Update(importerKey(string(r)))
	}
}

// --- line input ----------------------------------------------------------------

func TestImporterInputEditing(t *testing.T) {
	var in importerInput
	for _, r := range "héllo" {
		in.key(importerKey(string(r)))
	}
	if in.text != "héllo" || in.cursor != 5 {
		t.Fatalf("insert: got %q cursor %d", in.text, in.cursor)
	}
	in.key(importerKey("left"))
	in.key(importerKey("left"))
	in.key(importerKey("backspace")) // delete second 'l'... actually the 'l' before cursor
	if in.text != "hélo" || in.cursor != 2 {
		t.Fatalf("backspace: got %q cursor %d", in.text, in.cursor)
	}
	in.key(importerKey("x"))
	if in.text != "héxlo" || in.cursor != 3 {
		t.Fatalf("mid insert: got %q cursor %d", in.text, in.cursor)
	}
	in.key(importerKey("home"))
	if in.cursor != 0 {
		t.Fatalf("home: cursor %d", in.cursor)
	}
	in.key(tea.KeyPressMsg{Code: tea.KeyDelete})
	if in.text != "éxlo" {
		t.Fatalf("delete: got %q", in.text)
	}
	in.key(importerKey("end"))
	if in.cursor != 4 {
		t.Fatalf("end: cursor %d", in.cursor)
	}
	// modified printables must not be inserted
	if in.key(tea.KeyPressMsg{Code: 'c', Text: "c", Mod: tea.ModCtrl}) {
		t.Fatalf("ctrl-modified key should not be consumed as text")
	}
}

// --- separator parsing -----------------------------------------------------------

func TestImporterParseSep(t *testing.T) {
	cases := []struct {
		in   string
		want rune
		err  bool
	}{
		{"", ',', false},
		{";", ';', false},
		{`\t`, '\t', false},
		{"tab", '\t', false},
		{"ab", 0, true},
	}
	for _, c := range cases {
		got, err := importerParseSep(c.in)
		if (err != nil) != c.err || got != c.want {
			t.Errorf("importerParseSep(%q) = %q, %v; want %q err=%v", c.in, got, err, c.want, c.err)
		}
	}
}

// --- completion ------------------------------------------------------------------

func TestImporterComplete(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"alpha.csv", "album.json", "beta.tsv", ".hidden"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "aldir"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := importerComplete(filepath.Join(dir, "al"))
	want := []string{
		filepath.Join(dir, "album.json"),
		filepath.Join(dir, "aldir") + string(filepath.Separator),
		filepath.Join(dir, "alpha.csv"),
	}
	if len(got) != len(want) {
		t.Fatalf("matches = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("matches[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// empty prefix hides dotfiles
	all := importerComplete(dir + string(filepath.Separator))
	for _, m := range all {
		if strings.Contains(m, ".hidden") {
			t.Fatalf("dotfile leaked into %v", all)
		}
	}

	// dot prefix reveals them (filepath.Join would clean the trailing dot)
	dot := importerComplete(dir + string(filepath.Separator) + ".")
	if len(dot) != 1 || !strings.HasSuffix(dot[0], ".hidden") {
		t.Fatalf("dot prefix matches = %v", dot)
	}

	// URLs never complete
	if m := importerComplete("https://example.com/da"); m != nil {
		t.Fatalf("URL completion = %v, want nil", m)
	}
}

func TestImporterTabCycles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"aa.csv", "ab.csv"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := newImporter(filepath.Join(dir, "a"))
	o.Update(importerKey("tab"))
	if o.path.text != filepath.Join(dir, "aa.csv") {
		t.Fatalf("first tab: %q", o.path.text)
	}
	o.Update(importerKey("tab"))
	if o.path.text != filepath.Join(dir, "ab.csv") {
		t.Fatalf("second tab: %q", o.path.text)
	}
	o.Update(importerKey("tab")) // wraps
	if o.path.text != filepath.Join(dir, "aa.csv") {
		t.Fatalf("third tab: %q", o.path.text)
	}
	// editing resets the cycle
	o.Update(importerKey("backspace"))
	if o.matches != nil {
		t.Fatalf("edit should clear completion state")
	}
}

// --- detection helpers -------------------------------------------------------------

func TestImporterDetect(t *testing.T) {
	if f := importerDetect("https://example.com/data.csv?token=1"); f != reader.FormatCSV {
		t.Fatalf("url detect = %q", f)
	}
	if f := importerDetect("/tmp/thing.parquet"); f != reader.FormatParquet {
		t.Fatalf("file detect = %q", f)
	}
	if f := importerDetect("noext"); f != "" {
		t.Fatalf("detect(noext) = %q, want empty", f)
	}
}

// --- wizard flow -------------------------------------------------------------------

func TestImporterWizardFlow(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "people.csv")
	if err := os.WriteFile(csv, []byte("name,age\nann,3\nbob,4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fac, ok := ui.Factories["import"]
	if !ok {
		t.Fatal("factory 'import' not registered")
	}
	ov, err := fac(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	o := ov.(*importer)

	// step 1: empty path rejected inline
	_, cmd := o.Update(importerKey("enter"))
	if cmd != nil || o.errline == "" || o.step != importerStepPath {
		t.Fatalf("empty path: step=%d err=%q cmd=%v", o.step, o.errline, cmd)
	}

	importerType(o, csv)
	o.Update(importerKey("enter"))
	if o.step != importerStepFormat || o.errline != "" {
		t.Fatalf("after path: step=%d err=%q", o.step, o.errline)
	}

	// step 2: "auto" resolves to csv, a text format -> options step
	o.Update(importerKey("enter"))
	if o.step != importerStepOptions || o.chosen != reader.FormatCSV {
		t.Fatalf("after format: step=%d chosen=%q", o.step, o.chosen)
	}
	// csv hides the separator row
	rows := o.optionRows()
	if len(rows) != 2 {
		t.Fatalf("csv option rows = %v", rows)
	}

	// toggle no-header and cycle infer
	o.Update(importerKey("space"))
	if !o.noHeader {
		t.Fatal("space should toggle no-header")
	}
	o.Update(importerKey("space"))
	o.Update(importerKey("down"))
	before := o.inferIdx
	o.Update(importerKey("right"))
	if o.inferIdx == before {
		t.Fatal("right should cycle infer mode")
	}
	o.Update(importerKey("left"))
	if o.inferIdx != before {
		t.Fatal("left should cycle infer mode back")
	}

	// enter starts the async load
	_, cmd = o.Update(importerKey("enter"))
	if cmd == nil || !o.loading {
		t.Fatalf("enter on options: cmd=%v loading=%v", cmd, o.loading)
	}

	// run the load command and feed the result back
	res, ok := cmd().(importerResultMsg)
	if !ok {
		t.Fatalf("load cmd returned %T", cmd())
	}
	if res.err != nil {
		t.Fatalf("load err: %v", res.err)
	}
	if len(res.frames) != 1 || res.frames[0].Frame.NumRows() != 2 {
		t.Fatalf("frames = %+v", res.frames)
	}
	_, cmd = o.Update(res)
	if cmd == nil {
		t.Fatal("successful result must yield apply/toast/close commands")
	}
	if o.loading {
		t.Fatal("loading flag should clear")
	}
}

func TestImporterErrorKeepsWizardOpen(t *testing.T) {
	o := newImporter("/no/such/file.csv")
	o.Update(importerKey("enter")) // -> format
	o.Update(importerKey("enter")) // auto -> csv (text) -> options
	_, cmd := o.Update(importerKey("enter"))
	if cmd == nil {
		t.Fatal("expected load command")
	}
	res := cmd()
	_, cmd = o.Update(res)
	if cmd != nil {
		t.Fatalf("error result must not emit commands, got %v", cmd)
	}
	if o.errline == "" || o.step != importerStepOptions {
		t.Fatalf("error should stay inline on the failing step: err=%q step=%d", o.errline, o.step)
	}
}

func TestImporterBinarySkipsOptions(t *testing.T) {
	o := newImporter("/no/such/file.parquet")
	o.Update(importerKey("enter")) // -> format
	_, cmd := o.Update(importerKey("enter"))
	if cmd == nil || !o.loading {
		t.Fatalf("binary format should import from step 2: cmd=%v loading=%v step=%d", cmd, o.loading, o.step)
	}
}

func TestImporterBackNavigation(t *testing.T) {
	o := newImporter("x.csv")
	o.Update(importerKey("enter"))
	o.Update(importerKey("enter"))
	if o.step != importerStepOptions {
		t.Fatalf("step = %d", o.step)
	}
	o.Update(importerKey("esc"))
	if o.step != importerStepFormat {
		t.Fatalf("esc from options: step = %d", o.step)
	}
	o.Update(importerKey("left"))
	if o.step != importerStepPath {
		t.Fatalf("left from format: step = %d", o.step)
	}
	// left with cursor > 0 edits, does not close
	_, cmd := o.Update(importerKey("left"))
	if cmd != nil {
		t.Fatal("left mid-text must not close")
	}
	// esc on step 1 closes
	_, cmd = o.Update(importerKey("esc"))
	if cmd == nil {
		t.Fatal("esc on step 1 must close")
	}
	if msg := cmd(); msg != (ui.CloseOverlayMsg{}) {
		t.Fatalf("close msg = %#v", msg)
	}
}

func TestImporterAutoUndetectable(t *testing.T) {
	o := newImporter("mystery.bin")
	o.Update(importerKey("enter"))
	o.Update(importerKey("enter")) // auto cannot detect .bin
	if o.step != importerStepFormat || o.errline == "" {
		t.Fatalf("undetectable auto: step=%d err=%q", o.step, o.errline)
	}
	// picking an explicit format proceeds (first entry after "auto" is a
	// text format, so the wizard moves to the options step)
	o.Update(importerKey("down"))
	o.Update(importerKey("enter"))
	if o.step == importerStepFormat && !o.loading {
		t.Fatalf("explicit format should proceed: step=%d err=%q", o.step, o.errline)
	}
}

func TestImporterWindow(t *testing.T) {
	if off := importerWindow(0, 10, 4); off != 0 {
		t.Fatalf("window start = %d", off)
	}
	if off := importerWindow(9, 10, 4); off != 6 {
		t.Fatalf("window end = %d", off)
	}
	if off := importerWindow(2, 3, 4); off != 0 {
		t.Fatalf("window small = %d", off)
	}
}

func TestImporterScroll(t *testing.T) {
	off := 0
	off = importerScroll(off, 7, 13, 8)
	if off != 0 {
		t.Fatalf("scroll in-window = %d", off)
	}
	off = importerScroll(off, 8, 13, 8)
	if off != 1 {
		t.Fatalf("scroll down = %d", off)
	}
	off = importerScroll(off, 0, 13, 8)
	if off != 0 {
		t.Fatalf("scroll up = %d", off)
	}
}

func TestImporterIgnoresStaleResult(t *testing.T) {
	// A result from a cancelled importer instance must not be delivered to a
	// later instance (the app routes stray messages to the top overlay).
	old := newImporter("a.csv")
	cur := newImporter("b.csv")
	stale := importerResultMsg{owner: old, frames: []reader.NamedFrame{{Name: "a"}}}
	if _, cmd := cur.Update(stale); cmd != nil {
		t.Fatal("stale result from another instance produced commands")
	}

	// A result arriving after esc cancelled the load is dropped too.
	cur.loading = true
	cur.loading = false // simulates the cancel path clearing/closing
	own := importerResultMsg{owner: cur, frames: []reader.NamedFrame{{Name: "b"}}}
	if _, cmd := cur.Update(own); cmd != nil {
		t.Fatal("result after cancel produced commands")
	}
}

func TestImporterPaste(t *testing.T) {
	o := newImporter("")

	// Step 1: paste lands in the path input as one edit, flattened.
	o.Update(tea.PasteMsg{Content: "data\r\n.csv"})
	if o.path.text != "data .csv" {
		t.Fatalf("path after paste = %q", o.path.text)
	}

	// A paste invalidates a pending completion cycle and the error line.
	o = newImporter("")
	o.matches = []string{"stale"}
	o.errline = "old error"
	o.Update(tea.PasteMsg{Content: "file.csv"})
	if o.matches != nil || o.errline != "" {
		t.Fatalf("paste kept stale state: matches=%v errline=%q", o.matches, o.errline)
	}
	if o.path.text != "file.csv" {
		t.Fatalf("path = %q", o.path.text)
	}

	// Step 2 (format list) ignores pastes.
	o.step = importerStepFormat
	o.Update(tea.PasteMsg{Content: "zzz"})
	if o.path.text != "file.csv" || o.formatIdx != 0 {
		t.Fatalf("format step consumed paste: path=%q idx=%d", o.path.text, o.formatIdx)
	}

	// Step 3: paste reaches the separator input when it has focus and is
	// trimmed so a trailing clipboard newline still yields one character.
	o.step = importerStepOptions
	o.chosen = reader.FormatDSV
	o.optFocus = 1 // separator row
	o.sep.set("")
	o.Update(tea.PasteMsg{Content: "|\n"})
	if o.sep.text != "|" {
		t.Fatalf("sep after paste = %q, want |", o.sep.text)
	}

	// Step 3 with a non-separator row focused ignores pastes.
	o.optFocus = 0
	o.Update(tea.PasteMsg{Content: ";"})
	if o.sep.text != "|" {
		t.Fatalf("unfocused sep changed: %q", o.sep.text)
	}

	// While loading, pastes are swallowed.
	o.step = importerStepPath
	o.loading = true
	o.Update(tea.PasteMsg{Content: "x"})
	if o.path.text != "file.csv" {
		t.Fatalf("loading wizard consumed paste: %q", o.path.text)
	}
}

func TestImporterInputPaste(t *testing.T) {
	var in importerInput
	in.set("ab")
	in.cursor = 1
	if !in.paste("X\nY") {
		t.Fatal("paste should report a change")
	}
	if in.text != "aX Yb" || in.cursor != 4 {
		t.Fatalf("after paste: %q cursor %d", in.text, in.cursor)
	}
	if in.paste("\r") {
		t.Fatal("control-only paste must not report a change")
	}
}
