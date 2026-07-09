package popup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/ui"
)

func tabTestSwitcher(n, active int) *tabSwitcher {
	tabs := make([]ui.TabInfo, n)
	for i := range tabs {
		tabs[i] = ui.TabInfo{Title: string(rune('a' + i)), Shape: "1 x 1"}
	}
	return &tabSwitcher{tabs: tabs, cursor: active, viewRows: 10}
}

// tabCollect runs cmd (unwrapping batches) and returns the produced messages.
func tabCollect(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch m := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, tabCollect(t, c)...)
		}
	default:
		out = append(out, m)
	}
	return out
}

func TestTabSwitcherNavigation(t *testing.T) {
	o := tabTestSwitcher(3, 1)

	o.tabKey("j")
	if o.cursor != 2 {
		t.Fatalf("cursor after j = %d, want 2", o.cursor)
	}
	o.tabKey("j") // clamp
	if o.cursor != 2 {
		t.Fatalf("cursor clamped = %d, want 2", o.cursor)
	}
	o.tabKey("g")
	if o.cursor != 0 {
		t.Fatalf("cursor after g = %d, want 0", o.cursor)
	}
	o.tabKey("k") // clamp at 0
	if o.cursor != 0 {
		t.Fatalf("cursor after k at top = %d, want 0", o.cursor)
	}
	o.tabKey("G")
	if o.cursor != 2 {
		t.Fatalf("cursor after G = %d, want 2", o.cursor)
	}
}

func TestTabSwitcherEnterJumps(t *testing.T) {
	o := tabTestSwitcher(3, 2)
	cmd, close := o.tabKey("enter")
	if !close {
		t.Fatal("enter must close the overlay")
	}
	msgs := tabCollect(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	jump, ok := msgs[0].(ui.JumpToTabMsg)
	if !ok {
		t.Fatalf("got %T, want ui.JumpToTabMsg", msgs[0])
	}
	if jump.Index != 2 {
		t.Errorf("jump index = %d, want 2", jump.Index)
	}
}

func TestTabSwitcherCloseTab(t *testing.T) {
	for _, key := range []string{"x", "d"} {
		o := tabTestSwitcher(3, 1)
		cmd, close := o.tabKey(key)
		if close {
			t.Fatalf("%q with tabs remaining must not close the overlay", key)
		}
		msgs := tabCollect(t, cmd)
		if len(msgs) != 1 {
			t.Fatalf("got %d messages, want 1", len(msgs))
		}
		ct, ok := msgs[0].(ui.CloseTabMsg)
		if !ok {
			t.Fatalf("got %T, want ui.CloseTabMsg", msgs[0])
		}
		if ct.Index != 1 {
			t.Errorf("close index = %d, want 1", ct.Index)
		}
		if len(o.tabs) != 2 {
			t.Errorf("local tabs = %d, want 2", len(o.tabs))
		}
		if o.tabs[0].Title != "a" || o.tabs[1].Title != "c" {
			t.Errorf("remaining tabs = %v, want a,c", o.tabs)
		}
	}
}

func TestTabSwitcherCloseLastTabClosesOverlay(t *testing.T) {
	o := tabTestSwitcher(1, 0)
	cmd, close := o.tabKey("x")
	if !close {
		t.Fatal("closing the last tab must close the overlay")
	}
	msgs := tabCollect(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(ui.CloseTabMsg); !ok {
		t.Fatalf("got %T, want ui.CloseTabMsg", msgs[0])
	}
}

func TestTabSwitcherCloseLastIndexClampsCursor(t *testing.T) {
	o := tabTestSwitcher(3, 2)
	o.tabKey("x")
	if o.cursor != 1 {
		t.Fatalf("cursor after closing last tab = %d, want 1", o.cursor)
	}
}

func TestTabSwitcherEsc(t *testing.T) {
	o := tabTestSwitcher(2, 0)
	cmd, close := o.tabKey("esc")
	if !close || cmd != nil {
		t.Fatalf("esc: close=%v cmd=%v, want close with no command", close, cmd)
	}
}

func TestTabSwitcherFilterDisabledForFewTabs(t *testing.T) {
	o := tabTestSwitcher(8, 0)
	if o.filterEnabled() {
		t.Fatal("filter must stay off at the threshold (8 tabs)")
	}
	// Letters keep their shortcut meaning.
	o.tabKey("j")
	if o.cursor != 1 || len(o.query) != 0 {
		t.Fatalf("j with filter off: cursor=%d query=%q", o.cursor, string(o.query))
	}
}

func TestTabSwitcherFilterTypeToNarrow(t *testing.T) {
	o := tabTestSwitcher(9, 0) // titles a..i
	if !o.filterEnabled() {
		t.Fatal("filter must be on above the threshold")
	}
	// Letters type into the filter instead of navigating.
	cmd, close := o.tabKey("i")
	if cmd != nil || close {
		t.Fatal("typing must neither close nor emit commands")
	}
	if string(o.query) != "i" {
		t.Fatalf("query = %q, want i", string(o.query))
	}
	if len(o.filtered) != 1 || o.filtered[0] != 8 {
		t.Fatalf("filtered = %v, want [8]", o.filtered)
	}
	// Enter jumps to the original tab index, not the filtered position.
	cmd, close = o.tabKey("enter")
	if !close {
		t.Fatal("enter must close")
	}
	msgs := tabCollect(t, cmd)
	jump, ok := msgs[0].(ui.JumpToTabMsg)
	if !ok || jump.Index != 8 {
		t.Fatalf("enter produced %v, want JumpToTabMsg{8}", msgs[0])
	}
}

func TestTabSwitcherFilterBackspaceAndEsc(t *testing.T) {
	o := tabTestSwitcher(9, 0)
	o.tabKey("i")
	o.tabKey("backspace")
	if string(o.query) != "" || len(o.filtered) != 9 {
		t.Fatalf("backspace: query=%q filtered=%d", string(o.query), len(o.filtered))
	}
	// esc clears a non-empty filter before closing.
	o.tabKey("h")
	if cmd, close := o.tabKey("esc"); close || cmd != nil {
		t.Fatal("esc with a filter set must only clear it")
	}
	if len(o.query) != 0 {
		t.Fatalf("query after esc = %q, want empty", string(o.query))
	}
	if _, close := o.tabKey("esc"); !close {
		t.Fatal("esc with an empty filter must close")
	}
}

func TestTabSwitcherFilterCtrlXCloses(t *testing.T) {
	o := tabTestSwitcher(9, 0)
	o.tabKey("e") // narrow to tab index 4
	if len(o.filtered) != 1 || o.filtered[0] != 4 {
		t.Fatalf("filtered = %v, want [4]", o.filtered)
	}
	cmd, close := o.tabKey("ctrl+x")
	if close {
		t.Fatal("ctrl+x with tabs remaining must not close the overlay")
	}
	msgs := tabCollect(t, cmd)
	ct, ok := msgs[0].(ui.CloseTabMsg)
	if !ok || ct.Index != 4 {
		t.Fatalf("ctrl+x produced %v, want CloseTabMsg{4}", msgs[0])
	}
	if len(o.tabs) != 8 {
		t.Fatalf("local tabs = %d, want 8", len(o.tabs))
	}
	// "x" while a filter is active types instead of closing a tab.
	o2 := tabTestSwitcher(9, 0)
	cmd, close = o2.tabKey("x")
	if cmd != nil || close {
		t.Fatal("x with the filter on must type, not close a tab")
	}
	if string(o2.query) != "x" || len(o2.tabs) != 9 {
		t.Fatalf("x typed: query=%q tabs=%d", string(o2.query), len(o2.tabs))
	}
}

func TestTabSwitcherFilterViewShowsInputLine(t *testing.T) {
	o := tabTestSwitcher(9, 0)
	th := infoTestTheme(t)
	v := o.View(80, 24, th)
	if !strings.Contains(v, "filter") {
		t.Error("filter line missing from view with many tabs")
	}
	few := tabTestSwitcher(3, 0)
	if v := few.View(80, 24, th); strings.Contains(v, "filter") {
		t.Error("filter line must not render with few tabs")
	}
}
