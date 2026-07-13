// Refresh overlay: re-fetches the table the active pane shows and swaps it
// into the pane's base frame in place (no new tab). Registered under the
// command name "refreshtable" and dispatched by the ActRefresh key binding in
// database mode.
package popup

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// refreshLimit caps how many rows a refresh re-fetches, matching the initial
// open-table load so the preview stays the same size after a refresh.
const refreshLimit = 200

func init() {
	ui.Factories["refreshtable"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		be := ctx.Backend()
		if be == nil {
			return nil, fmt.Errorf("refreshtable: no live database connection")
		}
		return &refreshTableOverlay{ctx: ctx, be: be, ns: ctx.CurrentTableNamespace(), table: ctx.BaseCrumb()}, nil
	}
	addRefreshCommand()
}

// addRefreshCommand lists "refreshtable" in the palette table once, so the
// command is discoverable (the dispatch path itself works regardless).
func addRefreshCommand() {
	for _, c := range Commands {
		if c.Name == "refreshtable" {
			return
		}
	}
	Commands = append(Commands, Command{Name: "refreshtable", Description: "refresh the current table"})
}

// refreshTableMsg carries the re-fetched rows (or the fetch error) back to
// the loading overlay.
type refreshTableMsg struct {
	frame *data.Frame
	err   error
}

// refreshTableOverlay is a transient overlay: it kicks off the fetch on Init
// and closes itself, emitting a ReplaceBaseMsg on success or an ErrorMsg on
// failure.
type refreshTableOverlay struct {
	ctx   ui.AppContext
	be    db.Backend
	ns    string
	table string
}

func (o *refreshTableOverlay) Init() tea.Cmd {
	be, ns, table := o.be, o.ns, o.table
	return func() tea.Msg {
		f, err := be.FetchTable(ns, table, refreshLimit)
		return refreshTableMsg{frame: f, err: err}
	}
}

func (o *refreshTableOverlay) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case refreshTableMsg:
		if m.err != nil {
			err := m.err
			return o, tea.Sequence(ui.CloseOverlay, func() tea.Msg { return ui.ErrorMsg{Err: err} })
		}
		paneID := o.ctx.ActivePaneID()
		frame := m.frame
		return o, tea.Sequence(
			ui.CloseOverlay,
			func() tea.Msg { return ui.ReplaceBaseMsg{Frame: frame, PaneID: paneID} },
			func() tea.Msg { return ui.ToastMsg{Text: "refreshed"} },
		)
	case tea.KeyPressMsg:
		switch m.String() {
		case "esc", "q", "ctrl+c":
			return o, ui.CloseOverlay
		}
	}
	return o, nil
}

func (o *refreshTableOverlay) View(width, height int, th *theme.Theme) string {
	w := len(o.table) + 24
	if w > width-4 {
		w = width - 4
	}
	if w < 24 {
		w = 24
	}
	body := th.Text.Render(" refreshing " + o.table + " ...")
	return ui.Box("refresh", body, w, th)
}

// compile-time interface checks
var (
	_ ui.Overlay       = (*refreshTableOverlay)(nil)
	_ ui.OverlayIniter = (*refreshTableOverlay)(nil)
)
