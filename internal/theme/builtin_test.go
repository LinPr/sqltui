package theme

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"testing"
)

// requiredNames is the full set of palettes the package must ship.
var requiredNames = []string{
	"sorbet", "aurora",
	"catppuccin-mocha", "catppuccin-macchiato", "catppuccin-frappe", "catppuccin-latte",
	"dracula", "nord", "gruvbox-dark", "gruvbox-light",
	"solarized-dark", "solarized-light", "tokyo-night", "tokyo-night-storm",
	"monokai", "one-dark", "one-light", "github-dark", "github-light",
	"ayu-dark", "ayu-mirage", "ayu-light",
	"rose-pine", "rose-pine-moon", "rose-pine-dawn",
	"everforest-dark", "everforest-light", "kanagawa", "zenburn",
	"material-dark", "material-light", "night-owl", "palenight",
	"synthwave", "cobalt2", "oceanic-next", "horizon-dark", "iceberg-dark",
	"papercolor-light", "seoul256", "vesper", "flexoki-dark", "flexoki-light",
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func TestRequiredNamesPresent(t *testing.T) {
	have := make(map[string]bool, len(builtins))
	for n := range builtins {
		have[n] = true
	}
	for _, n := range requiredNames {
		if !have[n] {
			t.Errorf("required palette %q is missing", n)
		}
	}
	if len(builtins) < 43 {
		t.Errorf("expected at least 43 built-in palettes, have %d", len(builtins))
	}
}

func TestAllPalettesCompleteAndDerivable(t *testing.T) {
	for _, name := range Names() {
		name := name
		t.Run(name, func(t *testing.T) {
			th, ok := Builtin(name)
			if !ok {
				t.Fatalf("Builtin(%q) returned ok=false", name)
			}
			if th == nil {
				t.Fatalf("Builtin(%q) returned nil theme", name)
			}
			p := th.Palette
			if p.Name != name {
				t.Errorf("Palette.Name = %q, want %q (must match map key)", p.Name, name)
			}
			fields := map[string]string{
				"Bg":        p.Bg,
				"Fg":        p.Fg,
				"BgSoft":    p.BgSoft,
				"FgDim":     p.FgDim,
				"Header":    p.Header,
				"Accent":    p.Accent,
				"AccentFg":  p.AccentFg,
				"Highlight": p.Highlight,
				"Error":     p.Error,
				"Warning":   p.Warning,
				"Success":   p.Success,
			}
			for field, v := range fields {
				if v == "" {
					t.Errorf("field %s is empty", field)
					continue
				}
				if !hexRe.MatchString(v) {
					t.Errorf("field %s = %q, not of form #rrggbb", field, v)
				}
			}
			if len(p.Series) != 6 {
				t.Errorf("Series has %d colors, want 6", len(p.Series))
			}
			for i, s := range p.Series {
				if !hexRe.MatchString(s) {
					t.Errorf("Series[%d] = %q, not of form #rrggbb", i, s)
				}
			}
			// Derived styles must be usable without panicking.
			_ = th.Text.Render("x")
			_ = th.RowSelected.Render("x")
			_ = th.SeriesColor(0)
			_ = th.SeriesColor(11)
		})
	}
}

func TestBuiltinLookup(t *testing.T) {
	tests := []struct {
		name   string
		lookup string
		wantOK bool
		want   string // expected Palette.Name if ok
	}{
		{"exact", "dracula", true, "dracula"},
		{"upper", "DRACULA", true, "dracula"},
		{"mixed case", "Catppuccin-Mocha", true, "catppuccin-mocha"},
		{"surrounding space", "  nord  ", true, "nord"},
		{"unknown", "no-such-theme", false, ""},
		{"empty", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th, ok := Builtin(tt.lookup)
			if ok != tt.wantOK {
				t.Fatalf("Builtin(%q) ok = %v, want %v", tt.lookup, ok, tt.wantOK)
			}
			if !tt.wantOK {
				if th != nil {
					t.Fatalf("Builtin(%q) returned non-nil theme with ok=false", tt.lookup)
				}
				return
			}
			if th.Palette.Name != tt.want {
				t.Errorf("Builtin(%q).Palette.Name = %q, want %q", tt.lookup, th.Palette.Name, tt.want)
			}
		})
	}
}

func TestBuiltinCached(t *testing.T) {
	a, ok := Builtin("nord")
	if !ok {
		t.Fatal("Builtin(nord) failed")
	}
	b, _ := Builtin("NORD")
	if a != b {
		t.Error("repeated lookups should return the cached *Theme")
	}
}

func TestDefault(t *testing.T) {
	th := Default()
	if th == nil {
		t.Fatal("Default() returned nil")
	}
	if th.Palette.Name != DefaultName {
		t.Errorf("Default().Palette.Name = %q, want %q", th.Palette.Name, DefaultName)
	}
	if DefaultName != "sorbet" {
		t.Errorf("DefaultName = %q, want %q", DefaultName, "sorbet")
	}
}

// luminance returns the WCAG relative luminance of a #rrggbb color,
// in [0, 1] where #000000 is 0 and #ffffff is 1.
func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	if !hexRe.MatchString(hex) {
		t.Fatalf("luminance: bad hex color %q", hex)
	}
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("luminance: parsing %q: %v", hex, err)
	}
	lin := func(c int) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// TestDefaultPaletteBrightOnDark asserts the default (sorbet) palette stays
// bright and soft on a dark terminal: its key foregrounds must clear sensible
// relative luminance floors against #000000.
func TestDefaultPaletteBrightOnDark(t *testing.T) {
	p, ok := builtins[DefaultName]
	if !ok {
		t.Fatalf("default palette %q missing", DefaultName)
	}
	checks := []struct {
		field string
		hex   string
		min   float64
	}{
		{"Fg", p.Fg, 0.80},         // near-white body text
		{"Header", p.Header, 0.35}, // vivid but clearly bright header
		{"Accent", p.Accent, 0.20}, // selection fill still pops on black
		{"Highlight", p.Highlight, 0.55},
		{"FgDim", p.FgDim, 0.25}, // dim text must stay readable, not vanish
	}
	for _, c := range checks {
		if got := luminance(t, c.hex); got < c.min {
			t.Errorf("%s = %s has relative luminance %.3f, want >= %.2f (too dim on dark terminals)",
				c.field, c.hex, got, c.min)
		}
	}
	// AccentFg sits on top of Accent and must be near-black for contrast.
	if got := luminance(t, p.AccentFg); got > 0.05 {
		t.Errorf("AccentFg = %s has relative luminance %.3f, want <= 0.05 (must be near-black on Accent)",
			p.AccentFg, got)
	}
}

// rgb parses a #rrggbb color into its channels.
func rgb(t *testing.T, hex string) (r, g, b int) {
	t.Helper()
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("rgb: parsing %q: %v", hex, err)
	}
	return r, g, b
}

// TestSorbetWarmPastel pins the intended character of the sorbet palette:
// warm hues lead (red channel dominates on the yellow/orange roles) and the
// key colors are pastel — bright, with no channel crushed to zero.
func TestSorbetWarmPastel(t *testing.T) {
	p, ok := builtins["sorbet"]
	if !ok {
		t.Fatal("sorbet palette missing")
	}
	// Warm roles: red >= green >= blue (yellow/orange family).
	for _, c := range []struct{ field, hex string }{
		{"Header", p.Header},
		{"Accent", p.Accent},
		{"Highlight", p.Highlight},
		{"Warning", p.Warning},
	} {
		r, g, b := rgb(t, c.hex)
		if r < g || g < b {
			t.Errorf("%s = %s is not a warm yellow/orange (want r >= g >= b, got %d/%d/%d)",
				c.field, c.hex, r, g, b)
		}
	}
	// Pastel means soft: every key foreground keeps all channels well above
	// zero (no fully saturated primaries).
	for _, c := range []struct{ field, hex string }{
		{"Header", p.Header},
		{"Accent", p.Accent},
		{"Highlight", p.Highlight},
		{"Error", p.Error},
		{"Warning", p.Warning},
		{"Success", p.Success},
	} {
		r, g, b := rgb(t, c.hex)
		if min := minInt(r, minInt(g, b)); min < 0x60 {
			t.Errorf("%s = %s is too saturated for a pastel (min channel %d, want >= 0x60)",
				c.field, c.hex, min)
		}
	}
	for i, s := range p.Series {
		r, g, b := rgb(t, s)
		if min := minInt(r, minInt(g, b)); min < 0x60 {
			t.Errorf("Series[%d] = %s is too saturated for a pastel (min channel %d, want >= 0x60)",
				i, s, min)
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestNamesSortedAndContainsDefault(t *testing.T) {
	names := Names()
	if !sort.StringsAreSorted(names) {
		t.Error("Names() is not sorted")
	}
	if len(names) != len(builtins) {
		t.Errorf("Names() returned %d entries, want %d", len(names), len(builtins))
	}
	found := false
	for _, n := range names {
		if n == DefaultName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Names() does not contain DefaultName %q", DefaultName)
	}
}
