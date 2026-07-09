package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
)

// errBox is the built-in error overlay: a red-bordered box showing an error;
// any key closes it.
type errBox struct {
	err error
}

func (b errBox) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return b, CloseOverlay
	}
	return b, nil
}

func (b errBox) View(width, height int, th *theme.Theme) string {
	text := "unknown error"
	if b.err != nil {
		text = b.err.Error()
	}
	boxW := clamp(ansi.StringWidth(text)+4, 20, max(20, width-4))
	inner := boxW - 2
	body := th.Error.Render(padLine(" "+text, inner))
	wrapped := ansi.Wrap(text, inner-2, "")
	if strings.Contains(wrapped, "\n") {
		var lines []string
		for _, l := range strings.Split(wrapped, "\n") {
			lines = append(lines, th.Error.Render(padLine(" "+l, inner)))
		}
		body = strings.Join(lines, "\n")
	}
	hint := th.Subtle.Render(padLine(" press any key to close", inner))
	borderStyle := th.Error
	titleStyle := th.Error.Bold(true)
	return boxWith("error", body+"\n"+hint, boxW, borderStyle, titleStyle)
}
