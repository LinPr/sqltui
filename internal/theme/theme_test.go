package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// hasBackground reports whether a style sets an explicit background color.
func hasBackground(s lipgloss.Style) bool {
	_, unset := s.GetBackground().(lipgloss.NoColor)
	return !unset
}

// TestBaseStylesTransparent asserts that base styles carry no background so
// the terminal's own background shows through.
func TestBaseStylesTransparent(t *testing.T) {
	for _, name := range Names() {
		th, ok := Builtin(name)
		if !ok {
			t.Fatalf("Builtin(%q) returned ok=false", name)
		}
		transparent := map[string]lipgloss.Style{
			"Text":        th.Text,
			"Subtle":      th.Subtle,
			"Error":       th.Error,
			"Warning":     th.Warning,
			"Success":     th.Success,
			"Header":      th.Header,
			"RowEven":     th.RowEven,
			"Gutter":      th.Gutter,
			"Border":      th.Border,
			"Match":       th.Match,
			"PopupBorder": th.PopupBorder,
			"PopupTitle":  th.PopupTitle,
			"ListItem":    th.ListItem,
			"Input":       th.Input,
			"Placeholder": th.Placeholder,
		}
		for field, s := range transparent {
			if hasBackground(s) {
				t.Errorf("%s: style %s must not set a background (terminal bg shows through), got %v",
					name, field, s.GetBackground())
			}
		}
	}
}

// TestContrastStylesKeepBackground asserts that the styles whose whole point
// is a contrasting fill still set one.
func TestContrastStylesKeepBackground(t *testing.T) {
	for _, name := range Names() {
		th, ok := Builtin(name)
		if !ok {
			t.Fatalf("Builtin(%q) returned ok=false", name)
		}
		p := th.Palette
		filled := map[string]struct {
			style lipgloss.Style
			want  string
		}{
			"RowSelected":  {th.RowSelected, p.Accent},
			"ListSelected": {th.ListSelected, p.Accent},
			"RowOdd":       {th.RowOdd, p.BgSoft},
			"StatusBar":    {th.StatusBar, p.BgSoft},
		}
		for field, f := range filled {
			if !hasBackground(f.style) {
				t.Errorf("%s: style %s must keep an explicit background", name, field)
				continue
			}
			if got, want := f.style.GetBackground(), lipgloss.Color(f.want); got != want {
				t.Errorf("%s: style %s background = %v, want %v (%s)", name, field, got, want, f.want)
			}
		}
	}
}
