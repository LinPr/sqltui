package popup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// themeTestCtx is a minimal AppContext for exercising the factory.
type themeTestCtx struct{ themeName string }

func (c themeTestCtx) CurrentFrame() *data.Frame { return nil }
func (c themeTestCtx) CurrentRow() int           { return 0 }
func (c themeTestCtx) BaseCrumb() string         { return "" }
func (c themeTestCtx) Crumbs() []string          { return nil }
func (c themeTestCtx) ColumnNames() []string     { return nil }
func (c themeTestCtx) Engine() *query.Engine     { return nil }
func (c themeTestCtx) TableNames() []string      { return nil }
func (c themeTestCtx) Backend() db.Backend       { return nil }
func (c themeTestCtx) KV() db.KVBackend          { return nil }
func (c themeTestCtx) Theme() *theme.Theme       { return theme.Default() }
func (c themeTestCtx) ThemeName() string         { return c.themeName }
func (c themeTestCtx) ShowBorders() bool         { return false }
func (c themeTestCtx) ShowRowNumbers() bool      { return false }
func (c themeTestCtx) Tabs() []ui.TabInfo        { return nil }
func (c themeTestCtx) ActiveTab() int            { return 0 }
func (c themeTestCtx) ActivePaneID() int         { return 0 }

// themeCollectMsgs runs a command (flattening batches) and returns every
// message it produces.
func themeCollectMsgs(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, themeCollectMsgs(t, c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func themeKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func themeType(t *testing.T, ov ui.Overlay, text string) (ui.Overlay, []tea.Msg) {
	t.Helper()
	var msgs []tea.Msg
	for _, r := range text {
		var cmd tea.Cmd
		ov, cmd = ov.Update(themeKey(string(r)))
		msgs = append(msgs, themeCollectMsgs(t, cmd)...)
	}
	return ov, msgs
}

func TestThemeFactoryStartsOnActiveTheme(t *testing.T) {
	f, ok := ui.Factories["theme"]
	if !ok {
		t.Fatal(`ui.Factories["theme"] not registered`)
	}
	ov, err := f(themeTestCtx{themeName: "dracula"}, "")
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	ts, ok := ov.(*themeSelect)
	if !ok {
		t.Fatalf("factory returned %T, want *themeSelect", ov)
	}
	if got := ts.highlighted(); got != "dracula" {
		t.Errorf("initial highlight = %q, want dracula", got)
	}
	if len(ts.filtered) != len(theme.Names()) {
		t.Errorf("initial list has %d entries, want %d", len(ts.filtered), len(theme.Names()))
	}
}

func TestThemeMoveEmitsLivePreview(t *testing.T) {
	ts := newThemeSelect(theme.Names()[0], "")
	ov, cmd := ts.Update(themeKey("down"))
	msgs := themeCollectMsgs(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	set, ok := msgs[0].(ui.SetThemeMsg)
	if !ok {
		t.Fatalf("got %T, want ui.SetThemeMsg", msgs[0])
	}
	if want := theme.Names()[1]; set.Name != want {
		t.Errorf("preview theme = %q, want %q", set.Name, want)
	}
	// Moving up past the top must not re-emit the same preview.
	ov, cmd = ov.Update(themeKey("up"))
	themeCollectMsgs(t, cmd) // back to first name: one emission
	_, cmd = ov.Update(themeKey("up"))
	if msgs := themeCollectMsgs(t, cmd); len(msgs) != 0 {
		t.Errorf("cursor at top re-emitted preview: %v", msgs)
	}
}

func TestThemeFilterNarrowsAndPreviews(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, msgs := themeType(t, ts, "dracula")
	got := ov.(*themeSelect)
	if len(got.filtered) == 0 || got.filtered[0] != "dracula" {
		t.Fatalf("filtered = %v, want dracula first", got.filtered)
	}
	var last string
	for _, m := range msgs {
		if set, ok := m.(ui.SetThemeMsg); ok {
			last = set.Name
		}
	}
	if last != "dracula" {
		t.Errorf("last preview = %q, want dracula", last)
	}
}

func TestThemeEnterKeepsHighlighted(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, _ := themeType(t, ts, "gruvbox")
	_, cmd := ov.Update(themeKey("enter"))
	msgs := themeCollectMsgs(t, cmd)
	var set *ui.SetThemeMsg
	var closed bool
	for _, m := range msgs {
		switch m := m.(type) {
		case ui.SetThemeMsg:
			set = &m
		case ui.CloseOverlayMsg:
			closed = true
		}
	}
	if set == nil || !strings.HasPrefix(set.Name, "gruvbox") {
		t.Errorf("enter kept %v, want a gruvbox theme", set)
	}
	if !closed {
		t.Error("enter did not close the overlay")
	}
}

func TestThemeEscReverts(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, _ := ts.Update(themeKey("down"))
	ov, _ = ov.Update(themeKey("down"))
	_, cmd := ov.Update(themeKey("esc"))
	msgs := themeCollectMsgs(t, cmd)
	var set *ui.SetThemeMsg
	var closed bool
	for _, m := range msgs {
		switch m := m.(type) {
		case ui.SetThemeMsg:
			set = &m
		case ui.CloseOverlayMsg:
			closed = true
		}
	}
	if set == nil || set.Name != "nord" {
		t.Errorf("esc reverted to %v, want nord", set)
	}
	if !closed {
		t.Error("esc did not close the overlay")
	}
}

func TestThemeEnterWithNoMatchReverts(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, _ := themeType(t, ts, "zzzznotatheme")
	_, cmd := ov.Update(themeKey("enter"))
	msgs := themeCollectMsgs(t, cmd)
	var set *ui.SetThemeMsg
	for _, m := range msgs {
		if s, ok := m.(ui.SetThemeMsg); ok {
			set = &s
		}
	}
	if set == nil || set.Name != "nord" {
		t.Errorf("enter with empty match set %v, want revert to nord", set)
	}
}

func TestThemeBackspaceRestoresList(t *testing.T) {
	ts := newThemeSelect("nord", "")
	applied := "nord" // last theme the popup applied via SetThemeMsg
	track := func(msgs []tea.Msg) {
		for _, m := range msgs {
			if set, ok := m.(ui.SetThemeMsg); ok {
				applied = set.Name
			}
		}
	}
	ov, msgs := themeType(t, ts, "zz")
	track(msgs)
	if got := ov.(*themeSelect).filtered; len(got) != 0 {
		t.Fatalf("filter zz matched %v, expected nothing", got)
	}
	var cmd tea.Cmd
	ov, cmd = ov.Update(themeKey("backspace"))
	track(themeCollectMsgs(t, cmd))
	ov, cmd = ov.Update(themeKey("backspace"))
	track(themeCollectMsgs(t, cmd))
	got := ov.(*themeSelect)
	if len(got.filtered) != len(theme.Names()) {
		t.Errorf("after clearing filter list has %d entries, want %d",
			len(got.filtered), len(theme.Names()))
	}
	// The highlight must sit on whichever theme is actually applied behind
	// the popup, so the visible state never lies.
	if got.highlighted() != applied {
		t.Errorf("highlight after clearing = %q, want applied theme %q",
			got.highlighted(), applied)
	}
}

func TestThemeHighlightFollowsPreviewAfterClearing(t *testing.T) {
	// A filter that matches something moves the live preview; clearing the
	// filter keeps the highlight on the theme that is actually applied.
	ts := newThemeSelect("nord", "")
	ov, msgs := themeType(t, ts, "dracula")
	var applied string
	for _, m := range msgs {
		if set, ok := m.(ui.SetThemeMsg); ok {
			applied = set.Name
		}
	}
	if applied != "dracula" {
		t.Fatalf("applied preview = %q, want dracula", applied)
	}
	var cmd tea.Cmd
	for range "dracula" {
		ov, cmd = ov.Update(themeKey("backspace"))
		themeCollectMsgs(t, cmd)
	}
	got := ov.(*themeSelect)
	if got.highlighted() != "dracula" {
		t.Errorf("highlight after clearing = %q, want dracula (the applied preview)",
			got.highlighted())
	}
}

func TestThemeInputCursorEditing(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, _ := themeType(t, ts, "nrd")
	ov, _ = ov.Update(themeKey("left"))
	ov, _ = ov.Update(themeKey("left"))
	ov, _ = themeType(t, ov, "o")
	got := ov.(*themeSelect)
	if s := string(got.input); s != "nord" {
		t.Errorf("input = %q, want nord", s)
	}
	if len(got.filtered) == 0 || got.filtered[0] != "nord" {
		t.Errorf("filtered = %v, want nord first", got.filtered)
	}
}

func TestThemeScrollKeepsCursorVisible(t *testing.T) {
	ts := newThemeSelect(theme.Names()[0], "")
	var ov ui.Overlay = ts
	for range theme.Names() { // move past the end; must clamp
		ov, _ = ov.Update(themeKey("down"))
	}
	got := ov.(*themeSelect)
	if got.sel != len(theme.Names())-1 {
		t.Fatalf("sel = %d, want %d", got.sel, len(theme.Names())-1)
	}
	view := got.View(80, 24, theme.Default())
	last := theme.Names()[len(theme.Names())-1]
	if !strings.Contains(view, last) {
		t.Errorf("view does not show the highlighted last theme %q", last)
	}
	rows := themeVisibleRows(24, len(got.filtered))
	if got.sel < got.offset || got.sel >= got.offset+rows {
		t.Errorf("sel %d outside window [%d,%d)", got.sel, got.offset, got.offset+rows)
	}
}

func TestThemeViewSmallSizes(t *testing.T) {
	ts := newThemeSelect("nord", "")
	for _, wh := range [][2]int{{80, 24}, {30, 8}, {10, 4}, {200, 60}} {
		view := ts.View(wh[0], wh[1], theme.Default())
		if view == "" {
			t.Errorf("empty view at %dx%d", wh[0], wh[1])
		}
	}
}

func TestThemeIgnoresNonKeyMsgs(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, cmd := ts.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if ov != ui.Overlay(ts) || cmd != nil {
		t.Error("non-key message changed state or produced a command")
	}
}

func TestThemePasteFiltersAndPreviews(t *testing.T) {
	ts := newThemeSelect("nord", "")
	ov, cmd := ts.Update(tea.PasteMsg{Content: "dracula"})
	ts = ov.(*themeSelect)
	if string(ts.input) != "dracula" {
		t.Fatalf("input = %q, want dracula", string(ts.input))
	}
	if ts.highlighted() != "dracula" {
		t.Fatalf("highlighted = %q, want dracula", ts.highlighted())
	}
	msgs := themeCollectMsgs(t, cmd)
	found := false
	for _, m := range msgs {
		if sm, ok := m.(ui.SetThemeMsg); ok && sm.Name == "dracula" {
			found = true
		}
	}
	if !found {
		t.Fatalf("paste did not emit a preview: %v", msgs)
	}

	// Newlines flatten; empty paste is a no-op with no command.
	ts = newThemeSelect("nord", "")
	ov, _ = ts.Update(tea.PasteMsg{Content: "gruv\r\nbox"})
	ts = ov.(*themeSelect)
	if string(ts.input) != "gruv box" {
		t.Fatalf("flattened input = %q", string(ts.input))
	}
	ov, cmd = ts.Update(tea.PasteMsg{Content: "\r"})
	if ov != ui.Overlay(ts) || cmd != nil {
		t.Fatal("empty paste changed state or produced a command")
	}
}
