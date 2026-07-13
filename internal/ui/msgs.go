package ui

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
)

// SetBackendMsg attaches a live database connection to a running app.
// Database mode starts the UI with no connection and wires one in after the
// connection form succeeds. Nil fields are left untouched.
type SetBackendMsg struct {
	Backend db.Backend
	KV      db.KVBackend
}

// ApplyFrameMsg pushes a derived frame onto a pane's stack, or opens it as a
// new tab when NewTab is set. Crumb is a short operation label ("filter",
// "query", "sort") shown in the breadcrumb.
type ApplyFrameMsg struct {
	Frame    *data.Frame
	Crumb    string
	NewTab   bool
	TabTitle string
	// PaneID identifies the pane the operation was started from (see
	// AppContext.ActivePaneID). When set (non-zero) and NewTab is false the
	// frame is pushed onto that pane even if the user has switched tabs while
	// the command ran; if the pane is gone the frame opens as a new tab.
	PaneID int
	// RegisterAs, when non-empty, also registers Frame in the embedded SQL
	// engine under this name (no-op in db mode), so imported tables are
	// queryable by name like CLI-loaded ones.
	RegisterAs string
}

// ReplaceBaseMsg swaps the base frame of the pane identified by PaneID
// (or the active pane when zero), used by the refresh command to reload
// the current table without opening a new tab.
type ReplaceBaseMsg struct {
	Frame  *data.Frame
	PaneID int
}

// RunCommandMsg invokes a named command. Built-in names (quit, reset,
// toggleborders, togglerownumbers, reloadconfig) are handled by the app
// directly; everything else is resolved through the Factories registry.
type RunCommandMsg struct {
	Name string
	Arg  string
}

// SetThemeMsg switches the active theme by built-in name.
type SetThemeMsg struct{ Name string }

// ToggleBordersMsg flips the table border toggle.
type ToggleBordersMsg struct{}

// ToggleRowNumbersMsg flips the row-number gutter toggle.
type ToggleRowNumbersMsg struct{}

// ResetStackMsg pops the current pane back to its base frame.
type ResetStackMsg struct{}

// JumpToRowMsg selects a row (0-based) in the current table view.
type JumpToRowMsg struct{ Row int }

// JumpToTabMsg activates the tab at Index (0-based, clamped).
type JumpToTabMsg struct{ Index int }

// CloseTabMsg closes the tab at Index; closing the last tab quits.
type CloseTabMsg struct{ Index int }

// RegisterTableMsg registers the current frame in the SQL engine under Name.
type RegisterTableMsg struct{ Name string }

// ExecProcessMsg runs an external program with the terminal released and
// resumes the app afterwards. OnDone (optional) converts the process error
// into a follow-up message.
type ExecProcessMsg struct {
	Cmd    *exec.Cmd
	OnDone func(error) tea.Msg
}

// CopyTextMsg copies text to the system clipboard via OSC52.
type CopyTextMsg struct{ Text string }
