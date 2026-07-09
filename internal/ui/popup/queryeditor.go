package popup

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["query"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return qxNewEditor(ctx, arg), nil
	}
}

// qxEditor is the full SQL editor overlay ("query" command): a single input
// line with context-aware SQL autocompletion. Row-set results open in a new
// tab; non-query results show a rows-affected toast. A non-empty factory arg
// prefills the line but never auto-executes.
type qxEditor struct {
	ctx ui.AppContext
	in  qxInput
	sug qxSuggest
	// sc is the completion schema. Contexts implementing qxSchemaProvider
	// refresh it live on every keystroke (cheap: engine PRAGMAs or a cache
	// read); everything else keeps the async snapshot taken by Init, because
	// the fallback path may issue live catalog queries that must not run on
	// the update loop.
	sc query.Schema
	// warmed remembers which live tables a column warm-up was already kicked
	// for, so a dot completion on an unknown/empty table cannot re-fetch on
	// every keystroke.
	warmed map[string]bool
}

// qxSchemaMsg delivers an async completion-schema snapshot to the editor.
// reopen recomputes the suggestion list even when it is closed — set by the
// dot warm-up so the columns pop in once they arrive.
type qxSchemaMsg struct {
	sc     query.Schema
	reopen bool
}

func qxNewEditor(ctx ui.AppContext, arg string) *qxEditor {
	return &qxEditor{ctx: ctx, in: qxNewInput(arg), warmed: make(map[string]bool)}
}

// Init warms the live table catalog and snapshots the schema, both off the
// update loop (ui.OverlayIniter).
func (e *qxEditor) Init() tea.Cmd {
	ctx := e.ctx
	return func() tea.Msg {
		qxWarm(ctx)
		return qxSchemaMsg{sc: qxSchemaOf(ctx)}
	}
}

// refresh recomputes the suggestion list for the token at the cursor.
func (e *qxEditor) refresh() {
	if sp, ok := e.ctx.(qxSchemaProvider); ok {
		e.sc = sp.CompletionSchema()
	} else {
		e.sc.Current = e.ctx.ColumnNames()
	}
	e.sug.recompute(e.in.String(), e.in.byteCursor(), e.sc)
}

// warmDotCmd kicks an async column fetch when the cursor sits in a
// "table." completion whose columns are not cached yet. The fetch happens
// inside the command goroutine; the resulting schema snapshot reopens the
// suggestion list. At most one fetch per table per editor session.
func (e *qxEditor) warmDotCmd() tea.Cmd {
	w, ok := e.ctx.(qxSchemaWarmer)
	if !ok {
		return nil
	}
	tbl, ok := query.DotTable(e.in.String(), e.in.byteCursor(), e.sc)
	if !ok || e.warmed[tbl] || len(e.sc.Tables[tbl]) > 0 {
		return nil
	}
	e.warmed[tbl] = true
	ctx := e.ctx
	return func() tea.Msg {
		w.WarmCompletionSchema(tbl)
		return qxSchemaMsg{sc: qxSchemaOf(ctx), reopen: true}
	}
}

func (e *qxEditor) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if sm, ok := msg.(qxSchemaMsg); ok {
		e.sc = sm.sc
		if sm.reopen || e.sug.open() {
			e.refresh()
		}
		return e, nil
	}
	if pm, ok := msg.(tea.PasteMsg); ok {
		if e.in.paste(pm.Content) {
			e.refresh()
			return e, e.warmDotCmd()
		}
		return e, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return e, nil
	}

	switch key.String() {
	case "esc":
		if e.sug.open() {
			e.sug.clear()
			return e, nil
		}
		return e, ui.CloseOverlay

	case "tab", "down":
		if e.sug.open() {
			e.sug.cycle(1)
		} else {
			e.refresh()
			return e, e.warmDotCmd()
		}
		return e, nil

	case "up":
		if e.sug.open() {
			e.sug.cycle(-1)
		}
		return e, nil

	case "enter":
		if e.sug.open() {
			e.in.applySuggestion(e.sug.selected().Text)
			e.sug.clear()
			return e, nil
		}
		sqlText := strings.TrimSpace(e.in.String())
		if sqlText == "" {
			return e, ui.CloseOverlay
		}
		run := qxRunCmd(e.ctx, sqlText, func(f *data.Frame) tea.Msg {
			return ui.ApplyFrameMsg{
				Frame:    f,
				Crumb:    "query",
				NewTab:   true,
				TabTitle: qxShortTitle(sqlText),
			}
		})
		return e, tea.Batch(ui.CloseOverlay, run)
	}

	handled, changed := e.in.handle(key)
	if changed {
		e.refresh()
		return e, e.warmDotCmd()
	} else if handled {
		// Pure cursor movement invalidates the token context; close the list.
		e.sug.clear()
	}
	return e, nil
}

func (e *qxEditor) View(width, height int, th *theme.Theme) string {
	w := qxBoxWidth(width)
	inner := w - 2

	lines := []string{" " + e.in.view(inner-2, th)}
	lines = append(lines, e.sug.view(th)...)
	lines = append(lines, th.Placeholder.Render(" enter run · tab complete · esc close"))
	return ui.Box("query", strings.Join(lines, "\n"), w, th)
}

var _ ui.OverlayIniter = (*qxEditor)(nil)
