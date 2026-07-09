// Package theme defines the color system for the UI. A theme is derived
// from a compact Palette so that built-in themes are pure data.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette is the raw color definition of a theme. All colors are hex
// strings like "#1e1e2e".
type Palette struct {
	Name string

	Bg     string // main background (kept for contrast fills, e.g. text on tags)
	Fg     string // main text
	BgSoft string // alternating row / status-bar / subtle panel background
	FgDim  string // dimmed text (gutter, placeholders, borders)

	Header    string // table header foreground
	Accent    string // primary accent (selection background)
	AccentFg  string // text on top of Accent
	Highlight string // search-match / emphasis foreground

	Error   string
	Warning string
	Success string

	// Series colors used for status-bar tags and plot groups; at least 4.
	Series []string
}

// Theme holds ready-to-use styles derived from a Palette.
type Theme struct {
	Palette Palette

	// base
	Text    lipgloss.Style
	Subtle  lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Success lipgloss.Style

	// table
	Header      lipgloss.Style
	RowEven     lipgloss.Style
	RowOdd      lipgloss.Style
	RowSelected lipgloss.Style
	Gutter      lipgloss.Style
	Border      lipgloss.Style
	Match       lipgloss.Style // search-match cell emphasis

	// status bar
	StatusBar lipgloss.Style

	// popups / prompts
	PopupBorder  lipgloss.Style
	PopupTitle   lipgloss.Style
	ListItem     lipgloss.Style
	ListSelected lipgloss.Style
	Input        lipgloss.Style
	Placeholder  lipgloss.Style
}

// New derives a full Theme from a palette.
//
// Base styles carry no background so the terminal's own background shows
// through; explicit backgrounds are kept only where the fill itself carries
// meaning (selection, the odd-row stripe, the status bar).
func New(p Palette) *Theme {
	fg := lipgloss.Color(p.Fg)
	bgSoft := lipgloss.Color(p.BgSoft)
	fgDim := lipgloss.Color(p.FgDim)
	accent := lipgloss.Color(p.Accent)
	accentFg := lipgloss.Color(p.AccentFg)

	t := &Theme{Palette: p}
	t.Text = lipgloss.NewStyle().Foreground(fg)
	t.Subtle = lipgloss.NewStyle().Foreground(fgDim)
	t.Error = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Error))
	t.Warning = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Warning))
	t.Success = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Success))

	t.Header = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Header)).Bold(true)
	t.RowEven = lipgloss.NewStyle().Foreground(fg)
	t.RowOdd = lipgloss.NewStyle().Foreground(fg).Background(bgSoft)
	t.RowSelected = lipgloss.NewStyle().Foreground(accentFg).Background(accent)
	t.Gutter = lipgloss.NewStyle().Foreground(fgDim)
	t.Border = lipgloss.NewStyle().Foreground(fgDim)
	t.Match = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Highlight)).Bold(true)

	t.StatusBar = lipgloss.NewStyle().Foreground(fg).Background(bgSoft)

	t.PopupBorder = lipgloss.NewStyle().Foreground(accent)
	t.PopupTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Header)).Bold(true)
	t.ListItem = lipgloss.NewStyle().Foreground(fg)
	t.ListSelected = lipgloss.NewStyle().Foreground(accentFg).Background(accent).Bold(true)
	t.Input = lipgloss.NewStyle().Foreground(fg)
	t.Placeholder = lipgloss.NewStyle().Foreground(fgDim)
	return t
}

// SeriesColor returns the i-th series color, cycling.
func (t *Theme) SeriesColor(i int) color.Color {
	s := t.Palette.Series
	if len(s) == 0 {
		return lipgloss.Color(t.Palette.Accent)
	}
	return lipgloss.Color(s[i%len(s)])
}
