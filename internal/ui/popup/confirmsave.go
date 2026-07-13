// Confirm-save overlay: prompts the user to commit or cancel an in-flight
// cell edit. Dispatches to the reserved "saveedit" / "canceledit" commands so
// the app owns the commit lifecycle.
package popup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["confirmsave"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		pe := ctx.PendingEdit()
		if pe == nil {
			return nil, fmt.Errorf("no pending edit")
		}
		return &confirmSaveOverlay{ctx: ctx, pe: pe}, nil
	}
	addConfirmSaveCommand()
}

// addConfirmSaveCommand lists "confirmsave" in the palette table once.
func addConfirmSaveCommand() {
	for _, c := range Commands {
		if c.Name == "confirmsave" {
			return
		}
	}
	Commands = append(Commands, Command{Name: "confirmsave", Description: "confirm save edit"})
}

// confirmSaveOverlay asks the user whether to commit the pending edit. A
// "y"/enter confirmation closes the box then runs "saveedit"; "n"/esc/ctrl+c
// runs "canceledit" instead.
type confirmSaveOverlay struct {
	ctx ui.AppContext
	pe  *ui.PendingEdit
}

func (o *confirmSaveOverlay) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "y", "Y", "enter":
		return o, tea.Sequence(ui.CloseOverlay, func() tea.Msg {
			return ui.RunCommandMsg{Name: "saveedit"}
		})
	case "n", "N", "esc", "ctrl+c":
		return o, tea.Sequence(ui.CloseOverlay, func() tea.Msg {
			return ui.RunCommandMsg{Name: "canceledit"}
		})
	}
	return o, nil
}

// confirmTrunc clamps a value string to n visible cells for the box body.
func confirmTrunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(s, n, "…")
}

func (o *confirmSaveOverlay) View(width, height int, th *theme.Theme) string {
	boxW := 48
	if boxW > width-4 && width-4 >= 24 {
		boxW = width - 4
	}
	if boxW < 24 {
		boxW = 24
	}
	inner := boxW - 2

	label := func(k, v string) string {
		return " " + th.Subtle.Render(k) + " " + th.Text.Render(confirmTrunc(v, inner-3))
	}

	var lines []string
	lines = append(lines, " "+th.Text.Render("Save changes to "+confirmTrunc(o.pe.ColName, inner-22)+"?"))
	lines = append(lines, label("old:", o.pe.OldValue))
	lines = append(lines, label("new:", o.pe.NewValue))
	lines = append(lines, "")
	lines = append(lines, th.Subtle.Render(" y confirm  •  n/esc cancel"))

	if len(lines) > height-2 {
		lines = lines[:max(1, height-2)]
	}

	return ui.Box("save edit", strings.Join(lines, "\n"), boxW, th)
}

var _ ui.Overlay = (*confirmSaveOverlay)(nil)
