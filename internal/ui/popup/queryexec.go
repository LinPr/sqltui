// Package popup contains the modal overlays (palette, query editor,
// exporter, ...) that plug into the app through the ui.Factories registry.
//
// This file holds the shared SQL-execution plumbing used by the query editor
// (queryeditor.go) and the inline select/filter/order prompts (prompts.go):
// routing between the live database backend and the embedded engine, the
// inline-prompt statement builders, and a small line-input + suggestion-list
// widget pair. Everything is unexported and prefixed "qx".
package popup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// qxExecInfo describes the outcome of a statement that returned no rows.
type qxExecInfo struct {
	rowsAffected int64
}

// qxRun executes sqlText through whichever facility the app has: a live
// database backend takes precedence (database mode), otherwise the embedded
// engine (file mode). Exactly one of frame/info is non-nil on success.
func qxRun(ctx ui.AppContext, sqlText string) (*data.Frame, *qxExecInfo, error) {
	if be := ctx.Backend(); be != nil {
		res, err := be.Run(sqlText)
		if err != nil {
			return nil, nil, err
		}
		if res.Frame != nil {
			return res.Frame, nil, nil
		}
		if res.Exec != nil {
			return nil, &qxExecInfo{rowsAffected: res.Exec.RowsAffected}, nil
		}
		return nil, &qxExecInfo{}, nil
	}
	if eng := ctx.Engine(); eng != nil {
		f, err := eng.Query(sqlText)
		if err != nil {
			return nil, nil, err
		}
		return f, nil, nil
	}
	return nil, nil, fmt.Errorf("no database connection and no embedded SQL engine available")
}

// qxRunCmd wraps qxRun in a tea.Cmd, mapping the outcome to app messages:
// row sets go through onFrame, exec outcomes become a toast and errors
// become the error popup.
func qxRunCmd(ctx ui.AppContext, sqlText string, onFrame func(*data.Frame) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		frame, info, err := qxRun(ctx, sqlText)
		switch {
		case err != nil:
			return ui.ErrorMsg{Err: err}
		case frame != nil:
			return onFrame(frame)
		case info != nil:
			return ui.ToastMsg{Text: fmt.Sprintf("%d rows affected", info.rowsAffected)}
		default:
			return ui.ToastMsg{Text: "ok"}
		}
	}
}

// qxQuoteIdent wraps an identifier in double quotes, doubling embedded
// quote characters.
func qxQuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// qxTarget returns the quoted table name the inline prompts run against. In
// file mode the app keeps the frame under view registered as "_", so the
// prompts always target that alias. In live database mode there is no "_"
// alias; we fall back to the current tab title, which by convention is the
// name of the table the tab was loaded from. That is a heuristic: tabs opened
// from full query results carry a statement fragment as their title and the
// resulting SQL will simply fail with a clear engine error. Quoting is
// dialect-aware: mysql rejects double-quoted identifiers by default (they are
// string literals without ANSI_QUOTES), so it gets backticks.
func qxTarget(ctx ui.AppContext) string {
	be := ctx.Backend()
	if be == nil && ctx.Engine() != nil {
		return qxQuoteIdent("_")
	}
	if be != nil && be.Kind() == "mysql" {
		return "`" + strings.ReplaceAll(ctx.BaseCrumb(), "`", "``") + "`"
	}
	return qxQuoteIdent(ctx.BaseCrumb())
}

// qxSelectSQL expands the inline select prompt: SELECT <arg> FROM <target>.
func qxSelectSQL(ctx ui.AppContext, arg string) string {
	return "SELECT " + arg + " FROM " + qxTarget(ctx)
}

// qxFilterSQL expands the inline filter prompt: SELECT * FROM <target> WHERE <arg>.
func qxFilterSQL(ctx ui.AppContext, arg string) string {
	return "SELECT * FROM " + qxTarget(ctx) + " WHERE " + arg
}

// qxOrderSQL expands the inline order prompt: SELECT * FROM <target> ORDER BY <arg>.
func qxOrderSQL(ctx ui.AppContext, arg string) string {
	return "SELECT * FROM " + qxTarget(ctx) + " ORDER BY " + arg
}

// --- completion schema access --------------------------------------------------

// qxSchemaProvider is the optional AppContext extension that supplies a
// full completion schema (tables with columns + current frame columns)
// without blocking on the live connection.
type qxSchemaProvider interface {
	CompletionSchema() query.Schema
}

// qxSchemaWarmer is the optional AppContext extension that fills the
// live-connection completion cache. It blocks on catalog queries, so it
// must only be called from inside a tea.Cmd goroutine.
type qxSchemaWarmer interface {
	WarmCompletionSchema(tables ...string)
}

// qxSchemaOf snapshots the completion schema for ctx. Contexts without
// CompletionSchema (test stubs) fall back to the legacy pair of accessors:
// current columns plus bare table names (no per-table columns). The
// fallback's TableNames may issue live catalog queries, so treat this like
// the warmer: fine inside a tea.Cmd, not on the update loop.
func qxSchemaOf(ctx ui.AppContext) query.Schema {
	if sp, ok := ctx.(qxSchemaProvider); ok {
		return sp.CompletionSchema()
	}
	sc := query.Schema{Current: ctx.ColumnNames()}
	if ts := ctx.TableNames(); len(ts) > 0 {
		sc.Tables = make(map[string][]string, len(ts))
		for _, t := range ts {
			sc.Tables[t] = nil
		}
	}
	return sc
}

// qxWarm runs the schema warm-up when ctx supports it (blocking; tea.Cmd
// goroutines only).
func qxWarm(ctx ui.AppContext, tables ...string) {
	if w, ok := ctx.(qxSchemaWarmer); ok {
		w.WarmCompletionSchema(tables...)
	}
}

// qxShortTitle condenses a statement into a tab title: whitespace collapsed,
// truncated to 20 characters.
func qxShortTitle(sqlText string) string {
	fields := strings.Fields(sqlText)
	s := strings.Join(fields, " ")
	r := []rune(s)
	if len(r) > 20 {
		return string(r[:20])
	}
	if s == "" {
		return "query"
	}
	return s
}

// --- paste handling ------------------------------------------------------------

// pasteSanitize flattens bracketed-paste text for a single-line input:
// newlines and tabs become single spaces (so multi-line SQL keeps its token
// boundaries), while carriage returns and all other control characters are
// dropped. Every popup input funnels tea.PasteMsg content through this.
func pasteSanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(' ')
		case r < ' ' || r == 0x7f:
			// drop \r and other control characters
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- line input --------------------------------------------------------------

// qxInput is a minimal single-line editor: rune slice plus cursor position
// (rune index, 0..len).
type qxInput struct {
	text   []rune
	cursor int
}

func qxNewInput(prefill string) qxInput {
	r := []rune(prefill)
	return qxInput{text: r, cursor: len(r)}
}

func (in *qxInput) String() string { return string(in.text) }

// byteCursor converts the rune cursor to the byte offset expected by
// query.Suggest.
func (in *qxInput) byteCursor() int { return len(string(in.text[:in.cursor])) }

// handle processes an editing key. It reports whether the key was consumed
// and whether the text changed. Keys with app-level meaning (enter, tab,
// esc, up, down) must be intercepted by the caller before this is reached.
func (in *qxInput) handle(msg tea.KeyPressMsg) (handled, changed bool) {
	switch msg.String() {
	case "backspace":
		if in.cursor > 0 {
			in.text = append(in.text[:in.cursor-1], in.text[in.cursor:]...)
			in.cursor--
			return true, true
		}
		return true, false
	case "delete":
		if in.cursor < len(in.text) {
			in.text = append(in.text[:in.cursor], in.text[in.cursor+1:]...)
			return true, true
		}
		return true, false
	case "left":
		if in.cursor > 0 {
			in.cursor--
		}
		return true, false
	case "right":
		if in.cursor < len(in.text) {
			in.cursor++
		}
		return true, false
	case "home", "ctrl+a":
		in.cursor = 0
		return true, false
	case "end", "ctrl+e":
		in.cursor = len(in.text)
		return true, false
	case "ctrl+u":
		if in.cursor > 0 {
			in.text = append([]rune{}, in.text[in.cursor:]...)
			in.cursor = 0
			return true, true
		}
		return true, false
	case "ctrl+w":
		if in.cursor == 0 {
			return true, false
		}
		start := in.cursor
		for start > 0 && in.text[start-1] == ' ' {
			start--
		}
		for start > 0 && in.text[start-1] != ' ' {
			start--
		}
		in.text = append(in.text[:start], in.text[in.cursor:]...)
		in.cursor = start
		return true, true
	}
	if t := msg.Text; t != "" {
		ins := make([]rune, 0, len(t))
		for _, r := range t {
			if r >= ' ' && r != 0x7f {
				ins = append(ins, r)
			}
		}
		if len(ins) == 0 {
			return false, false
		}
		in.text = append(in.text[:in.cursor], append(ins, in.text[in.cursor:]...)...)
		in.cursor += len(ins)
		return true, true
	}
	return false, false
}

// paste inserts pasted text at the cursor as one edit (bracketed paste
// arrives as a single tea.PasteMsg, not per-rune key presses). The text is
// flattened to a single line first. It reports whether the text changed.
func (in *qxInput) paste(s string) bool {
	ins := []rune(pasteSanitize(s))
	if len(ins) == 0 {
		return false
	}
	in.text = append(in.text[:in.cursor], append(ins, in.text[in.cursor:]...)...)
	in.cursor += len(ins)
	return true
}

// applySuggestion replaces the identifier token ending at the cursor with s
// and places the cursor after it.
func (in *qxInput) applySuggestion(s string) {
	text := in.String()
	b := in.byteCursor()
	start := b
	for start > 0 && qxIdentByte(text[start-1]) {
		start--
	}
	head := []rune(text[:start] + s)
	in.text = append(append([]rune{}, head...), []rune(text[b:])...)
	in.cursor = len(head)
}

// qxIdentByte mirrors the completer's notion of an identifier byte.
func qxIdentByte(b byte) bool {
	return b == '_' ||
		('a' <= b && b <= 'z') ||
		('A' <= b && b <= 'Z') ||
		('0' <= b && b <= '9')
}

// view renders the input with a visible cursor cell, horizontally scrolled
// so the cursor stays inside width w (assumes narrow runes, which holds for
// SQL text).
func (in *qxInput) view(w int, th *theme.Theme) string {
	if w < 2 {
		w = 2
	}
	start := 0
	if in.cursor > w-1 {
		start = in.cursor - (w - 1)
	}
	before := string(in.text[start:in.cursor])
	curCh := " "
	after := ""
	if in.cursor < len(in.text) {
		curCh = string(in.text[in.cursor])
		rest := in.text[in.cursor+1:]
		if maxAfter := w - (in.cursor - start) - 1; len(rest) > maxAfter {
			if maxAfter < 0 {
				maxAfter = 0
			}
			rest = rest[:maxAfter]
		}
		after = string(rest)
	}
	return th.Input.Render(before) + th.Input.Reverse(true).Render(curCh) + th.Input.Render(after)
}

// --- suggestion list ---------------------------------------------------------

// qxMaxSuggestions caps how many completion candidates are visible at once;
// longer lists scroll as the highlight moves.
const qxMaxSuggestions = 8

// qxSuggest is the completion dropdown state. An empty items slice means the
// list is closed.
type qxSuggest struct {
	items []query.Suggestion
	sel   int
}

func (s *qxSuggest) open() bool { return len(s.items) > 0 }

func (s *qxSuggest) selected() query.Suggestion {
	if !s.open() {
		return query.Suggestion{}
	}
	return s.items[s.sel]
}

// recompute refreshes the candidate list for the token at cursor.
func (s *qxSuggest) recompute(input string, cursor int, sc query.Schema) {
	s.items = query.Suggest(input, cursor, sc)
	s.sel = 0
}

// cycle moves the highlight by d, wrapping.
func (s *qxSuggest) cycle(d int) {
	if n := len(s.items); n > 0 {
		s.sel = ((s.sel+d)%n + n) % n
	}
}

func (s *qxSuggest) clear() {
	s.items = nil
	s.sel = 0
}

// qxKindMarker abbreviates a suggestion kind for the dropdown gutter.
func qxKindMarker(kind string) string {
	switch kind {
	case query.KindColumn:
		return "col"
	case query.KindTable:
		return "tbl"
	case query.KindFunction:
		return "fn"
	case query.KindKeyword:
		return "kw"
	}
	return kind
}

// view renders one line per visible candidate: the replacement text plus a
// dimmed kind marker and detail (e.g. the owning table). At most
// qxMaxSuggestions lines show at once; the window follows the highlight and
// a dimmed position line appears when the list scrolls.
func (s *qxSuggest) view(th *theme.Theme) []string {
	n := len(s.items)
	if n == 0 {
		return nil
	}
	off := 0
	if s.sel >= qxMaxSuggestions {
		off = s.sel - qxMaxSuggestions + 1
	}
	end := off + qxMaxSuggestions
	if end > n {
		end = n
	}
	lines := make([]string, 0, end-off+1)
	for i := off; i < end; i++ {
		it := s.items[i]
		marker := qxKindMarker(it.Kind)
		if it.Detail != "" {
			marker += " " + it.Detail
		}
		if i == s.sel {
			lines = append(lines, th.ListSelected.Render(" "+it.Text+"  "+marker))
		} else {
			lines = append(lines, th.ListItem.Render(" "+it.Text+"  ")+th.Placeholder.Render(marker))
		}
	}
	if n > qxMaxSuggestions {
		lines = append(lines, th.Placeholder.Render(fmt.Sprintf(" %d/%d", s.sel+1, n)))
	}
	return lines
}

// qxBoxWidth picks a sensible centered-box width for the given window width.
func qxBoxWidth(width int) int {
	w := width - 4
	if w > 70 {
		w = 70
	}
	if w < 24 {
		w = 24
	}
	return w
}
