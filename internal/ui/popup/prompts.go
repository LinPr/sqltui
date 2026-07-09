package popup

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// qxPromptBuilders maps a prompt kind to its statement builder (queryexec.go).
var qxPromptBuilders = map[string]func(ui.AppContext, string) string{
	"select": qxSelectSQL,
	"filter": qxFilterSQL,
	"order":  qxOrderSQL,
}

func init() {
	for kind := range qxPromptBuilders {
		kind := kind
		ui.Factories[kind] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
			return qxNewPrompt(ctx, kind, arg), nil
		}
	}
}

// qxNewPrompt builds the overlay for one inline prompt invocation. With an
// empty arg it returns the interactive prompt; with a non-empty arg the
// statement runs immediately (see qxAutoRun for the mechanism).
func qxNewPrompt(ctx ui.AppContext, kind, arg string) ui.Overlay {
	if a := strings.TrimSpace(arg); a != "" {
		sqlText := qxPromptBuilders[kind](ctx, a)
		return &qxAutoRun{label: sqlText, cmd: qxPromptRunCmd(ctx, kind, sqlText)}
	}
	return &qxPrompt{
		ctx:  ctx,
		kind: kind,
		// The title shows the expansion template with … marking the input slot.
		title: qxPromptBuilders[kind](ctx, "…"),
	}
}

// qxPromptRunCmd runs sqlText and pushes a row-set result onto the
// originating pane's stack under the prompt kind's crumb. The pane identity
// is captured at command-creation time so a slow query cannot land on a tab
// the user switched to while it ran.
func qxPromptRunCmd(ctx ui.AppContext, kind, sqlText string) tea.Cmd {
	paneID := ctx.ActivePaneID()
	return qxRunCmd(ctx, sqlText, func(f *data.Frame) tea.Msg {
		return ui.ApplyFrameMsg{Frame: f, Crumb: kind, NewTab: false, PaneID: paneID}
	})
}

// qxPrompt is the reusable inline prompt overlay behind the "select",
// "filter" and "order" commands: one input line completed against the
// current frame's columns (plus SQL keywords), expanded into a full
// statement on enter.
type qxPrompt struct {
	ctx   ui.AppContext
	kind  string
	title string
	in    qxInput
	sug   qxSuggest
}

// refresh recomputes the suggestion list. The prompt input is a fragment
// against one table, so the schema is stripped of table names: columns of
// the current frame, functions and keywords complete, table names do not.
func (p *qxPrompt) refresh() {
	sc := query.Schema{Current: p.ctx.ColumnNames()}
	if sp, ok := p.ctx.(qxSchemaProvider); ok {
		sc = sp.CompletionSchema()
		sc.Tables = nil
	}
	p.sug.recompute(p.in.String(), p.in.byteCursor(), sc)
}

func (p *qxPrompt) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if pm, ok := msg.(tea.PasteMsg); ok {
		if p.in.paste(pm.Content) {
			p.refresh()
		}
		return p, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}

	switch key.String() {
	case "esc":
		if p.sug.open() {
			p.sug.clear()
			return p, nil
		}
		return p, ui.CloseOverlay

	case "tab", "down":
		if p.sug.open() {
			p.sug.cycle(1)
		} else {
			p.refresh()
		}
		return p, nil

	case "up":
		if p.sug.open() {
			p.sug.cycle(-1)
		}
		return p, nil

	case "enter":
		if p.sug.open() {
			p.in.applySuggestion(p.sug.selected().Text)
			p.sug.clear()
			return p, nil
		}
		arg := strings.TrimSpace(p.in.String())
		if arg == "" {
			return p, ui.CloseOverlay
		}
		sqlText := qxPromptBuilders[p.kind](p.ctx, arg)
		return p, tea.Batch(ui.CloseOverlay, qxPromptRunCmd(p.ctx, p.kind, sqlText))
	}

	handled, changed := p.in.handle(key)
	if changed {
		p.refresh()
	} else if handled {
		p.sug.clear()
	}
	return p, nil
}

func (p *qxPrompt) View(width, height int, th *theme.Theme) string {
	w := qxBoxWidth(width)
	inner := w - 2

	lines := []string{" " + p.in.view(inner-2, th)}
	lines = append(lines, p.sug.view(th)...)
	lines = append(lines, th.Placeholder.Render(" enter run · tab complete · esc close"))
	return ui.Box(p.title, strings.Join(lines, "\n"), w, th)
}

// qxAutoRun exists because factories return overlays, not commands: when an
// inline prompt is invoked with its argument already spelled out ("filter
// price > 10") the statement should run immediately, with no prompt shown.
// It implements ui.OverlayIniter, so pushing it fires the statement (batched
// with the overlay's own close) at once — no follow-up message is needed.
// Update stays as a fallback fire path for a message that races in before
// CloseOverlayMsg pops the overlay.
type qxAutoRun struct {
	label string
	cmd   tea.Cmd
	fired bool
}

// Init fires the statement as soon as the overlay is pushed.
func (a *qxAutoRun) Init() tea.Cmd {
	if a.fired {
		return nil
	}
	a.fired = true
	return tea.Batch(ui.CloseOverlay, a.cmd)
}

func (a *qxAutoRun) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	return a, a.Init()
}

func (a *qxAutoRun) View(width, height int, th *theme.Theme) string {
	w := qxBoxWidth(width)
	return ui.Box("running", th.Text.Render(" "+a.label), w, th)
}

var _ ui.OverlayIniter = (*qxAutoRun)(nil)
