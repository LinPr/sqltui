package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
)

// stubPageOverlay is a minimal overlay with a configurable fullscreen flag,
// for App.View compositing tests.
type stubPageOverlay struct {
	content    string
	fullscreen bool
}

func (s stubPageOverlay) Update(tea.Msg) (Overlay, tea.Cmd)     { return s, nil }
func (s stubPageOverlay) View(w, h int, th *theme.Theme) string { return s.content }
func (s stubPageOverlay) Fullscreen() bool                      { return s.fullscreen }

func TestFillPageExactSizeAndBlankFill(t *testing.T) {
	got := FillPage("XX\nXX", 6, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(lines))
	}
	want := []string{"      ", "  XX  ", "  XX  ", "      ", "      "}
	for i, l := range lines {
		if l != want[i] {
			t.Fatalf("line %d = %q, want %q", i, l, want[i])
		}
	}
	// Oversized content is truncated to the area.
	got = FillPage(plainBase("X", 12, 9), 6, 4)
	lines = strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("truncated lines = %d, want 4", len(lines))
	}
	for i, l := range lines {
		if ansi.StringWidth(l) != 6 {
			t.Fatalf("truncated line %d width = %d, want 6", i, ansi.StringWidth(l))
		}
	}
}

// TestAppViewFullscreenOverlayReplacesBody: a fullscreen page overlay covers
// the whole body area — nothing of the table behind may leak through — while
// the status bar stays at the bottom.
func TestAppViewFullscreenOverlayReplacesBody(t *testing.T) {
	const w, h = 40, 12
	a := newTestApp(t)
	a.Update(tea.WindowSizeMsg{Width: w, Height: h})

	// Sanity: without an overlay the table content is visible.
	if !strings.Contains(ansi.Strip(a.View().Content), "alice") {
		t.Fatal("sanity: table content not rendered at this size")
	}

	a.Update(PushOverlayMsg{Overlay: stubPageOverlay{content: "PAGE-BOX", fullscreen: true}})
	content := a.View().Content
	plain := ansi.Strip(content)
	if strings.Contains(plain, "alice") {
		t.Fatalf("table content leaked through the fullscreen page:\n%s", plain)
	}
	if !strings.Contains(plain, "PAGE-BOX") {
		t.Fatalf("fullscreen overlay content missing:\n%s", plain)
	}
	// The status bar is still the last line.
	lines := strings.Split(content, "\n")
	if len(lines) != h {
		t.Fatalf("view has %d lines, want %d", len(lines), h)
	}
	if !strings.Contains(ansi.Strip(lines[h-1]), "Tab 1/2") {
		t.Fatalf("status bar missing below the fullscreen page: %q", ansi.Strip(lines[h-1]))
	}
	// Every body line is padded to the exact width: full blank coverage.
	for i, l := range lines[:h-1] {
		if got := ansi.StringWidth(l); got != w {
			t.Fatalf("body line %d width = %d, want %d", i, got, w)
		}
	}
}

// TestAppViewFloatingOverlayKeepsBody: a non-fullscreen overlay (flag false
// or interface absent) keeps today's centered-composite behavior with the
// table visible around it.
func TestAppViewFloatingOverlayKeepsBody(t *testing.T) {
	a := newTestApp(t)
	a.Update(PushOverlayMsg{Overlay: stubPageOverlay{content: "BOX", fullscreen: false}})
	plain := ansi.Strip(a.View().Content)
	if !strings.Contains(plain, "BOX") {
		t.Fatal("floating overlay content missing")
	}
	if !strings.Contains(plain, "alice") {
		t.Fatal("floating overlay must keep the table visible behind it")
	}
}
