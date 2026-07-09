package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/theme"
)

// SearchBar implements live search over the current frame. While active it
// keeps a preview frame (the filtered rows) that the app displays instead of
// the pane's top frame; the stack is only touched on commit.
type SearchBar struct {
	active bool
	fuzzy  bool
	query  string

	base    *data.Frame // frame being searched (top of stack at activation)
	preview *data.Frame // live filtered result
}

func (s *SearchBar) Active() bool  { return s.active }
func (s *SearchBar) Fuzzy() bool   { return s.fuzzy }
func (s *SearchBar) Query() string { return s.query }

// Start activates the bar over frame f.
func (s *SearchBar) Start(f *data.Frame, fuzzy bool) {
	s.active = true
	s.fuzzy = fuzzy
	s.query = ""
	s.base = f
	s.preview = f
}

// Type appends printable input and recomputes the preview.
func (s *SearchBar) Type(text string) {
	if !s.active || text == "" {
		return
	}
	s.query += text
	s.recompute()
}

// Paste appends pasted text as a single edit (bracketed paste delivers the
// whole chunk in one message, not per-rune key presses), flattened to one
// line first.
func (s *SearchBar) Paste(text string) {
	s.Type(flattenPaste(text))
}

// flattenPaste makes pasted text safe for the single-line search input:
// newlines and tabs become single spaces, carriage returns and other control
// characters are dropped.
func flattenPaste(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(' ')
		case r < ' ' || r == 0x7f:
			// drop \r and other control characters
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Backspace removes the last rune and recomputes the preview.
func (s *SearchBar) Backspace() {
	if !s.active || s.query == "" {
		return
	}
	r := []rune(s.query)
	s.query = string(r[:len(r)-1])
	s.recompute()
}

func (s *SearchBar) recompute() {
	if s.base == nil {
		return
	}
	if s.query == "" {
		s.preview = s.base
		return
	}
	var rows []int
	if s.fuzzy {
		rows = data.SearchFuzzy(s.base, s.query)
	} else {
		rows = data.SearchExact(s.base, s.query)
	}
	s.preview = s.base.Select(rows)
}

// Preview returns the frame to display while the bar is active (never nil
// while active with a base frame).
func (s *SearchBar) Preview() *data.Frame {
	if s.preview != nil {
		return s.preview
	}
	return s.base
}

// Commit deactivates the bar and returns the filtered frame plus a crumb
// label. ok is false when there is nothing to commit (empty query).
func (s *SearchBar) Commit() (f *data.Frame, crumb string, ok bool) {
	f, q, fuzzy := s.preview, s.query, s.fuzzy
	s.Cancel()
	if q == "" || f == nil {
		return nil, "", false
	}
	if fuzzy {
		return f, "fuzzy", true
	}
	return f, "search", true
}

// Cancel deactivates the bar and drops the preview.
func (s *SearchBar) Cancel() {
	s.active = false
	s.query = ""
	s.base = nil
	s.preview = nil
}

// View renders the input line shown at the bottom of the screen.
func (s *SearchBar) View(width int, th *theme.Theme) string {
	label := "search"
	if s.fuzzy {
		label = "fuzzy"
	}
	n := 0
	if s.preview != nil {
		n = s.preview.NumRows()
	}
	line := " " + label + " > " + s.query + "▏"
	tail := ""
	if s.base != nil {
		tail = strconv.Itoa(n) + " rows "
	}
	pad := width - ansi.StringWidth(line) - ansi.StringWidth(tail)
	if pad < 0 {
		pad = 0
	}
	return th.Input.Render(padLine(line+strings.Repeat(" ", pad)+tail, width))
}
