package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
)

func fullStatusInfo() statusInfo {
	return statusInfo{
		Tab: 1, NTabs: 2,
		Row: 3, NRows: 100, NCols: 5,
		Crumbs: []string{"tbl", "filter"},
		Col:    "name",
		Mode:   "wide",
		Conn:   "mysql://localhost/db",
	}
}

func TestStatusBarAllSegmentsWide(t *testing.T) {
	line := renderStatusBar(fullStatusInfo(), 120, theme.Default())
	plain := ansi.Strip(line)
	for _, want := range []string{
		"Tab 1/2", "Row 3", "100 x 5", // core tags
		"col: name", "[wide]", // column + mode tags
		"tbl > filter",             // breadcrumb, table first
		"mysql://localhost/db",     // connection title (db mode)
		appName + " " + appVersion, // app label at the right edge
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("status bar at width 120 missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(line, "\n") {
		t.Fatal("status bar must be a single line")
	}
	if w := ansi.StringWidth(line); w > 120 {
		t.Fatalf("status bar width = %d, want <= 120", w)
	}
}

func TestStatusBarDropsRightmostSegmentsFirst(t *testing.T) {
	// At 42 cells there is room for the core tags and the app label, but the
	// mode, column and connection segments must be dropped (in that order).
	line := renderStatusBar(fullStatusInfo(), 42, theme.Default())
	plain := ansi.Strip(line)
	for _, want := range []string{"Tab 1/2", "Row 3", "100 x 5", appName} {
		if !strings.Contains(plain, want) {
			t.Errorf("width 42: missing core segment %q:\n%s", want, plain)
		}
	}
	for _, gone := range []string{"[wide]", "col:", "mysql"} {
		if strings.Contains(plain, gone) {
			t.Errorf("width 42: segment %q should be dropped:\n%s", gone, plain)
		}
	}
	if w := ansi.StringWidth(line); w > 42 {
		t.Fatalf("width = %d, want <= 42", w)
	}
}

func TestStatusBarVeryNarrowKeepsCoreTags(t *testing.T) {
	line := renderStatusBar(fullStatusInfo(), 20, theme.Default())
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "Tab 1/2") || !strings.Contains(plain, "Row 3") {
		t.Fatalf("width 20: tab/row tags must survive:\n%s", plain)
	}
	for _, gone := range []string{"100 x 5", appName, "col:", "[wide]", "mysql"} {
		if strings.Contains(plain, gone) {
			t.Errorf("width 20: segment %q should be dropped:\n%s", gone, plain)
		}
	}
	if w := ansi.StringWidth(line); w > 20 {
		t.Fatalf("width = %d, want <= 20", w)
	}
	if strings.Contains(line, "\n") {
		t.Fatal("status bar must stay a single line")
	}
}

func TestStatusBarFitModeAndNoConn(t *testing.T) {
	info := fullStatusInfo()
	info.Mode = "fit"
	info.Conn = ""
	line := renderStatusBar(info, 100, theme.Default())
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "[fit]") {
		t.Fatalf("compact mode should show the [fit] tag:\n%s", plain)
	}
	if strings.Contains(plain, "mysql") {
		t.Fatalf("no connection title expected in file mode:\n%s", plain)
	}
}

func TestStatusBarZeroWidth(t *testing.T) {
	if got := renderStatusBar(fullStatusInfo(), 0, theme.Default()); got != "" {
		t.Fatalf("zero width should render nothing, got %q", got)
	}
}
