package dbmode

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/db/redisbe"
	"github.com/LinPr/sqltui/internal/ui"
)

// --- prompt completion -----------------------------------------------------------

func TestRedisPromptCompletionPrefixMatch(t *testing.T) {
	p := newRedisPrompt(&fakeCtx{kv: &fakeKV{}}, "")
	if len(p.sugg) != 0 {
		t.Fatalf("empty input must have no suggestions, got %d", len(p.sugg))
	}

	typeText(t, p, "hge")
	if len(p.sugg) != 2 { // HGET, HGETALL
		t.Fatalf("suggestions for hge = %d, want 2", len(p.sugg))
	}
	for _, s := range p.sugg {
		if !strings.HasPrefix(s.Command, "HGE") {
			t.Fatalf("suggestion %q does not match prefix", s.Command)
		}
	}
}

func TestRedisPromptCompletionCap(t *testing.T) {
	p := newRedisPrompt(&fakeCtx{kv: &fakeKV{}}, "")
	typeText(t, p, "ge") // GEO* + GET* > 8 commands in the reference
	if len(p.sugg) != maxPromptSuggestions {
		t.Fatalf("suggestions capped at %d, got %d", maxPromptSuggestions, len(p.sugg))
	}
}

func TestRedisPromptTabAppliesAndCycles(t *testing.T) {
	p := newRedisPrompt(&fakeCtx{kv: &fakeKV{}}, "")
	typeText(t, p, "get")
	if len(p.sugg) < 2 {
		t.Fatalf("need at least 2 suggestions, got %d", len(p.sugg))
	}
	first, second := p.sugg[0].Command, p.sugg[1].Command

	p.Update(keyPress("tab"))
	if got := string(p.input); got != first+" " {
		t.Fatalf("first tab applied %q, want %q", got, first+" ")
	}
	p.Update(keyPress("tab"))
	if got := string(p.input); got != second+" " {
		t.Fatalf("second tab applied %q, want %q", got, second+" ")
	}
	// Editing after cycling recomputes from the new text.
	p.Update(keyPress("backspace"))
	if p.cycling {
		t.Fatal("editing must stop the tab cycle")
	}
}

func TestRedisPromptSuggestionsRendered(t *testing.T) {
	p := newRedisPrompt(&fakeCtx{kv: &fakeKV{}}, "")
	typeText(t, p, "hgetall")
	view := p.View(100, 30, testTheme())
	if !strings.Contains(view, "HGETALL") {
		t.Fatal("suggestion command not rendered")
	}
	var help redisbe.CommandHelp
	for _, h := range redisbe.CommandHelps {
		if h.Command == "HGETALL" {
			help = h
		}
	}
	if !strings.Contains(view, help.Args) {
		t.Fatal("suggestion args not rendered")
	}
}

// --- prompt execution ------------------------------------------------------------

func TestRedisPromptEnterRunsCommand(t *testing.T) {
	kv := &fakeKV{doOut: `"PONG"`}
	p := newRedisPrompt(&fakeCtx{kv: kv}, "")
	typeText(t, p, "ping extra")

	_, cmd := p.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatal("enter did not start the command")
	}
	if !p.running {
		t.Fatal("prompt not in running state")
	}
	msg := runCmd(cmd)
	done, ok := msg.(redisDoneMsg)
	if !ok {
		t.Fatalf("run produced %T, want redisDoneMsg", msg)
	}
	if len(kv.doArgs) != 2 || kv.doArgs[0] != "ping" || kv.doArgs[1] != "extra" {
		t.Fatalf("Do called with %v, want [ping extra]", kv.doArgs)
	}

	// Success replaces the prompt with a value viewer titled by the command.
	ov, _ := p.Update(done)
	viewer, ok := ov.(*valueViewer)
	if !ok {
		t.Fatalf("success returned %T, want *valueViewer", ov)
	}
	if viewer.title != "ping extra" || viewer.text != `"PONG"` {
		t.Fatalf("viewer = %q / %q", viewer.title, viewer.text)
	}
}

func TestRedisPromptEnterEmptyIsNoop(t *testing.T) {
	p := newRedisPrompt(&fakeCtx{kv: &fakeKV{}}, "")
	if _, cmd := p.Update(keyPress("enter")); cmd != nil {
		t.Fatal("enter on empty input must be a no-op")
	}
}

func TestRedisPromptErrorStaysOpen(t *testing.T) {
	kv := &fakeKV{doErr: fmt.Errorf("WRONGTYPE")}
	p := newRedisPrompt(&fakeCtx{kv: kv}, "get k")

	_, cmd := p.Update(keyPress("enter"))
	done := runCmd(cmd).(redisDoneMsg)
	ov, cmd := p.Update(done)
	if cmd != nil {
		t.Fatal("error must not emit follow-up commands")
	}
	if ov != ui.Overlay(p) {
		t.Fatal("error must keep the prompt open")
	}
	if p.running || p.errText == "" {
		t.Fatal("error state not recorded")
	}
	if !strings.Contains(p.View(80, 24, testTheme()), "WRONGTYPE") {
		t.Fatal("inline error not rendered")
	}
}

func TestRedisPromptEscCloses(t *testing.T) {
	p := newRedisPrompt(&fakeCtx{kv: &fakeKV{}}, "")
	_, cmd := p.Update(keyPress("esc"))
	if _, ok := runCmd(cmd).(ui.CloseOverlayMsg); !ok {
		t.Fatal("esc must close the prompt")
	}
}

// --- key browser -----------------------------------------------------------------

func TestKeyBrowserLazyLoadAndOpenKey(t *testing.T) {
	kv := &fakeKV{keys: map[string][]string{
		"string": {"greeting", "counter"},
		"hash":   {"user:1"},
	}}
	b := newKeyBrowser(kv, 1)
	if len(b.sections) != len(redisbe.KeyTypes) {
		t.Fatalf("sections = %d, want %d", len(b.sections), len(redisbe.KeyTypes))
	}

	// Init opens and scans the first section only.
	cmd := b.Init()
	if cmd == nil {
		t.Fatal("Init did not scan the first section")
	}
	if !b.sections[0].open || !b.sections[0].loading {
		t.Fatal("first section not opening")
	}
	for _, s := range b.sections[1:] {
		if s.open || s.loading {
			t.Fatal("later sections must stay lazy")
		}
	}

	msg := runCmd(cmd)
	b.Update(msg)
	if got := b.sections[0].keys; len(got) != 2 {
		t.Fatalf("loaded keys = %v", got)
	}

	// Navigate to the first key and open it: a value viewer is pushed.
	b.Update(keyPress("down")) // header -> first key
	rows := b.rows()
	if rows[b.cursor].key != "greeting" {
		t.Fatalf("cursor on %q, want greeting", rows[b.cursor].key)
	}
	_, cmd = b.Update(keyPress("enter"))
	push, ok := runCmd(cmd).(ui.PushOverlayMsg)
	if !ok {
		t.Fatalf("enter on key produced %T, want PushOverlayMsg", runCmd(cmd))
	}
	viewer, ok := push.Overlay.(*valueViewer)
	if !ok {
		t.Fatalf("pushed %T, want *valueViewer", push.Overlay)
	}
	if viewer.title != "greeting" {
		t.Fatalf("viewer title = %q, want greeting", viewer.title)
	}
	// The viewer loads the value on Init.
	kv.value = "hello"
	loaded := runCmd(viewer.Init())
	viewer.Update(loaded)
	if viewer.loading || viewer.text != "hello" {
		t.Fatalf("viewer did not load: loading=%v text=%q", viewer.loading, viewer.text)
	}
}

func TestKeyBrowserSectionToggleAndRescan(t *testing.T) {
	kv := &fakeKV{keys: map[string][]string{"string": {"a"}}}
	b := newKeyBrowser(kv, 1)
	b.Update(runCmd(b.Init())) // load section 0

	// Enter on the open header closes it; enter again re-opens without a
	// rescan (already loaded).
	_, cmd := b.Update(keyPress("enter"))
	if cmd != nil || b.sections[0].open {
		t.Fatal("enter on open header must close it without commands")
	}
	_, cmd = b.Update(keyPress("enter"))
	if cmd != nil || !b.sections[0].open {
		t.Fatal("re-opening a loaded section must not rescan")
	}

	// "r" rescans open sections.
	kv.keys["string"] = []string{"a", "b", "c"}
	_, cmd = b.Update(keyPress("r"))
	if cmd == nil {
		t.Fatal("r did not rescan")
	}
	b.Update(runCmd(cmd))
	if got := len(b.sections[0].keys); got != 3 {
		t.Fatalf("rescan loaded %d keys, want 3", got)
	}
}

func TestKeyBrowserScanError(t *testing.T) {
	kv := &fakeKV{scanErr: fmt.Errorf("connection lost")}
	b := newKeyBrowser(kv, 1)
	b.Update(runCmd(b.Init()))
	if b.sections[0].errText == "" {
		t.Fatal("scan error not recorded")
	}
	if !strings.Contains(b.View(80, 24, testTheme()), "connection lost") {
		t.Fatal("scan error not rendered")
	}
}

func TestKeyBrowserQWithTabsJustCloses(t *testing.T) {
	// q keeps the close semantics: with open tabs it just closes the
	// overlay, back to the table page.
	b := newKeyBrowser(&fakeKV{}, 1)
	_, cmd := b.Update(keyPress("q"))
	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("q with tabs open produced %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(ui.CloseOverlayMsg); !ok {
		t.Fatalf("q must close the key browser, got %T", msgs[0])
	}
}

// assertChainsConnect checks that a command closes the overlay AND reopens
// the connection form (the page-back chain).
func assertChainsConnect(t *testing.T, label string, cmd tea.Cmd) {
	t.Helper()
	var closed, connected bool
	for _, m := range collectMsgs(cmd) {
		switch v := m.(type) {
		case ui.CloseOverlayMsg:
			closed = true
		case ui.RunCommandMsg:
			connected = true
			if v.Name != "connect" {
				t.Errorf("%s: RunCommandMsg.Name = %q, want connect", label, v.Name)
			}
		}
	}
	if !closed || !connected {
		t.Errorf("%s: closed=%v connect=%v, want both", label, closed, connected)
	}
}

func TestKeyBrowserEscAlwaysChainsConnect(t *testing.T) {
	// The browser is a page in the login -> browser -> table stack: esc goes
	// back to the connection form regardless of how many tabs are open.
	for _, tabs := range []int{0, 1, 3} {
		b := newKeyBrowser(&fakeKV{}, tabs)
		_, cmd := b.Update(keyPress("esc"))
		assertChainsConnect(t, fmt.Sprintf("esc with %d tabs", tabs), cmd)
	}
}

func TestKeyBrowserQZeroTabsChainsConnect(t *testing.T) {
	// With no open tabs, closing the browser would leave a blank workspace:
	// q goes back to the connection form instead.
	b := newKeyBrowser(&fakeKV{}, 0)
	_, cmd := b.Update(keyPress("q"))
	assertChainsConnect(t, "q with zero tabs", cmd)
}

func TestRedisOverlaysFullscreen(t *testing.T) {
	// The key browser and value viewer only exist in database mode and
	// always render as full pages.
	for name, ov := range map[string]ui.Overlay{
		"key browser":  newKeyBrowser(&fakeKV{}, 0),
		"value viewer": newValueViewerText("t", "x"),
	} {
		fs, ok := ov.(ui.FullscreenOverlay)
		if !ok || !fs.Fullscreen() {
			t.Errorf("%s must be a fullscreen page overlay", name)
		}
	}
}

func TestValueViewerEscAndQReturnToBrowser(t *testing.T) {
	// The value viewer pops back to the key browser page only — no chain.
	for _, k := range []string{"esc", "q"} {
		v := newValueViewerText("t", "x")
		_, cmd := v.Update(keyPress(k))
		msgs := collectMsgs(cmd)
		if len(msgs) != 1 {
			t.Fatalf("%s produced %d messages, want 1", k, len(msgs))
		}
		if _, ok := msgs[0].(ui.CloseOverlayMsg); !ok {
			t.Fatalf("%s must close the viewer only, got %T", k, msgs[0])
		}
	}
}

// collectMsgs runs a command and flattens batch/sequence chains into the
// produced messages (nil-safe).
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	// tea.Sequence yields an unexported []tea.Cmd-based message; unwrap it
	// reflectively so ordered command chains can be inspected too.
	if rv := reflect.ValueOf(msg); rv.IsValid() && rv.Kind() == reflect.Slice &&
		strings.Contains(rv.Type().String(), "sequenceMsg") {
		var out []tea.Msg
		for i := 0; i < rv.Len(); i++ {
			if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
				out = append(out, collectMsgs(c)...)
			}
		}
		return out
	}
	return []tea.Msg{msg}
}

// browserKeys collects the key rows of the current listing.
func browserKeys(b *keyBrowser) []string {
	var out []string
	for _, r := range b.rows() {
		if r.key != "" {
			out = append(out, r.key)
		}
	}
	return out
}

func TestKeyBrowserFilterNarrowsKeys(t *testing.T) {
	kv := &fakeKV{keys: map[string][]string{"string": {"greeting", "counter", "user:1"}}}
	b := newKeyBrowser(kv, 1)
	b.Update(runCmd(b.Init()))

	// "/" focuses the filter; typed text narrows the loaded keys.
	b.Update(keyPress("/"))
	if !b.filtering {
		t.Fatal("/ must focus the filter input")
	}
	typeText(t, b, "count")
	if got := browserKeys(b); len(got) != 1 || got[0] != "counter" {
		t.Fatalf("filtered keys = %v, want [counter]", got)
	}
	// Section headers survive the filter.
	rows := b.rows()
	if len(rows) == 0 || rows[0].key != "" || rows[0].note != "" {
		t.Fatalf("first row = %+v, want a section header", rows[0])
	}
	// The filter line and match render.
	v := b.View(80, 24, testTheme())
	if !strings.Contains(v, "count") {
		t.Error("filter text not rendered")
	}
	if strings.Contains(v, "greeting") {
		t.Error("non-matching key still rendered")
	}

	// enter leaves the input focused mode but keeps the narrowed list; the
	// remaining key can be opened.
	b.Update(keyPress("enter"))
	if b.filtering {
		t.Fatal("enter must return keys to navigation")
	}
	if got := browserKeys(b); len(got) != 1 {
		t.Fatalf("filter dropped on enter: %v", got)
	}
	b.Update(keyPress("down")) // header -> counter
	if rows := b.rows(); rows[b.cursor].key != "counter" {
		t.Fatalf("cursor on %q, want counter", rows[b.cursor].key)
	}
	_, cmd := b.Update(keyPress("enter"))
	push, ok := runCmd(cmd).(ui.PushOverlayMsg)
	if !ok {
		t.Fatalf("enter on filtered key produced %T, want PushOverlayMsg", runCmd(cmd))
	}
	if viewer := push.Overlay.(*valueViewer); viewer.title != "counter" {
		t.Fatalf("viewer title = %q, want counter", viewer.title)
	}
}

func TestKeyBrowserFilterCaseInsensitiveAndNoMatch(t *testing.T) {
	kv := &fakeKV{keys: map[string][]string{"string": {"User:1", "session"}}}
	b := newKeyBrowser(kv, 1)
	b.Update(runCmd(b.Init()))
	b.Update(keyPress("/"))
	typeText(t, b, "user")
	if got := browserKeys(b); len(got) != 1 || got[0] != "User:1" {
		t.Fatalf("filtered keys = %v, want [User:1]", got)
	}
	typeText(t, b, "zzz")
	if got := browserKeys(b); len(got) != 0 {
		t.Fatalf("filtered keys = %v, want none", got)
	}
	found := false
	for _, r := range b.rows() {
		if strings.Contains(r.note, "no matching keys") {
			found = true
		}
	}
	if !found {
		t.Error("empty match must show a note in the section")
	}
}

func TestKeyBrowserFilterEscBehavior(t *testing.T) {
	kv := &fakeKV{keys: map[string][]string{"string": {"a", "b"}}}
	b := newKeyBrowser(kv, 1)
	b.Update(runCmd(b.Init()))

	// esc while typing clears the filter and stays open.
	b.Update(keyPress("/"))
	typeText(t, b, "a")
	_, cmd := b.Update(keyPress("esc"))
	if cmd != nil || b.filtering || len(b.filter) != 0 {
		t.Fatalf("esc while typing: cmd=%v filtering=%v filter=%q",
			cmd, b.filtering, string(b.filter))
	}
	if got := browserKeys(b); len(got) != 2 {
		t.Fatalf("keys after clear = %v, want both", got)
	}

	// A committed filter (enter) is cleared by esc before a second esc
	// steps back to the connection form.
	b.Update(keyPress("/"))
	typeText(t, b, "a")
	b.Update(keyPress("enter"))
	_, cmd = b.Update(keyPress("esc"))
	if cmd != nil || len(b.filter) != 0 {
		t.Fatal("esc with a committed filter must only clear it")
	}
	_, cmd = b.Update(keyPress("esc"))
	assertChainsConnect(t, "esc with no filter", cmd)
}

func TestKeyBrowserFilterTypingDoesNotTriggerShortcuts(t *testing.T) {
	kv := &fakeKV{keys: map[string][]string{"string": {"radar"}}}
	b := newKeyBrowser(kv, 1)
	b.Update(runCmd(b.Init()))
	b.Update(keyPress("/"))
	// "r" must type into the filter, not rescan; "q" must not close.
	ov, cmd := b.Update(keyPress("r"))
	if cmd != nil {
		t.Fatal("r while filtering must not rescan")
	}
	ov, cmd = ov.Update(keyPress("q"))
	if cmd != nil {
		t.Fatal("q while filtering must not close")
	}
	if string(b.filter) != "rq" {
		t.Fatalf("filter = %q, want rq", string(b.filter))
	}
	// backspace edits the filter.
	ov.Update(keyPress("backspace"))
	if string(b.filter) != "r" {
		t.Fatalf("filter after backspace = %q, want r", string(b.filter))
	}
}
