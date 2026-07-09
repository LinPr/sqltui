package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/theme"
)

// Overlay is a modal component rendered on top of the main view. The app
// keeps a stack of overlays; only the top one receives key input.
type Overlay interface {
	// Update handles a message and returns the (possibly replaced) overlay.
	Update(msg tea.Msg) (Overlay, tea.Cmd)
	// View renders the overlay box for the given available area.
	View(width, height int, th *theme.Theme) string
}

// FullscreenOverlay marks an overlay that renders as a full page: it
// covers the whole body area instead of floating over it. Database mode
// uses it to make the connection form, the schema/key browsers and the
// value viewer feel like a page stack (login -> browser -> table) rather
// than popups over the table.
type FullscreenOverlay interface{ Fullscreen() bool }

// OverlayIniter is an optional extension of Overlay: an overlay that needs
// to kick off an asynchronous command as soon as it is shown (loading data,
// scanning keys, ...) implements Init. The app runs the returned command
// right after pushing the overlay.
type OverlayIniter interface {
	Init() tea.Cmd
}

// CloseOverlayMsg pops the top overlay off the stack.
type CloseOverlayMsg struct{}

// CloseOverlay is a convenience command that emits CloseOverlayMsg.
func CloseOverlay() tea.Msg { return CloseOverlayMsg{} }

// PushOverlayMsg pushes a new overlay onto the stack.
type PushOverlayMsg struct{ Overlay Overlay }

// ErrorMsg surfaces an error to the user in the error popup.
type ErrorMsg struct{ Err error }

// ToastMsg shows a transient success/info message at the bottom.
type ToastMsg struct{ Text string }
