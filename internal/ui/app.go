package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/config"
	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/theme"
)

const toastDuration = 2500 * time.Millisecond

// Options configures a new App.
type Options struct {
	Frames  []reader.NamedFrame // initial tabs (already engine-registered)
	Engine  *query.Engine       // embedded SQL engine (nil in db mode)
	Backend db.Backend          // live SQL connection (nil in file mode)
	KV      db.KVBackend        // live redis connection (nil otherwise)

	ThemeName      string
	ShowBorders    bool
	ShowRowNumbers bool
}

// App is the root model: tabs of panes, an overlay stack, the search bar and
// the bottom status line.
type App struct {
	tabs     []*Pane
	active   int
	overlays []Overlay

	th        *theme.Theme
	themeName string

	showBorders    bool
	showRowNumbers bool

	width, height int

	toastText string
	toastID   int

	search SearchBar

	engine  *query.Engine
	backend db.Backend
	kv      db.KVBackend

	// pendingEdit/pendingDelete hold an in-flight cell edit / row delete that
	// a confirm popup is about to commit. They are set by the dispatch cases
	// (ActEdit for Agent E, ActDelete here) and cleared by the commit/cancel
	// built-ins (saveedit/canceledit/deleterows/canceldelete).
	pendingEdit   *PendingEdit
	pendingDelete *PendingDelete

	// syncedFrame caches the frame last registered as "_" (see ensureSynced).
	syncedFrame *data.Frame

	exiting bool
}

// toastExpiredMsg clears a toast after its timer fires.
type toastExpiredMsg struct{ id int }

// New builds the app from options. Each frame becomes a tab.
func New(opts Options) *App {
	th, ok := theme.Builtin(opts.ThemeName)
	name := opts.ThemeName
	if !ok {
		th = theme.Default()
		name = th.Palette.Name
	}
	a := &App{
		th:             th,
		themeName:      name,
		showBorders:    opts.ShowBorders,
		showRowNumbers: opts.ShowRowNumbers,
		engine:         opts.Engine,
		backend:        opts.Backend,
		kv:             opts.KV,
		width:          80,
		height:         24,
	}
	for _, nf := range opts.Frames {
		a.tabs = append(a.tabs, NewPane(nf.Name, nf.Frame))
	}
	return a
}

// Run drives the app to completion on the terminal.
func Run(app *App) error {
	_, err := tea.NewProgram(app).Run()
	return err
}

// --- tea.Model ---------------------------------------------------------------

func (a *App) Init() tea.Cmd { return nil }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil

	case PushOverlayMsg:
		if m.Overlay != nil {
			return a, a.pushOverlay(m.Overlay)
		}
		return a, nil

	case SetBackendMsg:
		if m.Backend != nil {
			a.backend = m.Backend
		}
		if m.KV != nil {
			a.kv = m.KV
		}
		return a, nil

	case CloseOverlayMsg:
		if n := len(a.overlays); n > 0 {
			a.overlays = a.overlays[:n-1]
		}
		return a, nil

	case ErrorMsg:
		a.overlays = append(a.overlays, errBox{err: m.Err})
		return a, nil

	case ToastMsg:
		return a, a.setToast(m.Text)

	case toastExpiredMsg:
		if m.id == a.toastID {
			a.toastText = ""
		}
		return a, nil

	case ApplyFrameMsg:
		return a, a.applyFrame(m)

	case ReplaceBaseMsg:
		return a, a.replaceBase(m)

	case RunCommandMsg:
		return a, a.runCommand(m.Name, m.Arg)

	case SetThemeMsg:
		th, ok := theme.Builtin(m.Name)
		if !ok {
			a.overlays = append(a.overlays, errBox{err: fmt.Errorf("unknown theme: %s", m.Name)})
			return a, nil
		}
		a.th, a.themeName = th, m.Name
		a.persistUI()
		return a, a.setToast("theme: " + m.Name)

	case ToggleBordersMsg:
		a.showBorders = !a.showBorders
		a.persistUI()
		return a, nil

	case ToggleRowNumbersMsg:
		a.showRowNumbers = !a.showRowNumbers
		a.persistUI()
		return a, nil

	case ResetStackMsg:
		if p := a.pane(); p != nil {
			p.Reset()
		}
		return a, nil

	case JumpToRowMsg:
		if p := a.pane(); p != nil && p.Current() != nil {
			p.Table.JumpTo(m.Row, p.Current().NumRows())
		}
		return a, nil

	case JumpToTabMsg:
		if len(a.tabs) > 0 {
			a.active = clamp(m.Index, 0, len(a.tabs)-1)
		}
		return a, nil

	case CloseTabMsg:
		return a, a.closeTab(m.Index)

	case RegisterTableMsg:
		return a, a.registerTable(m.Name)

	case ExecProcessMsg:
		if m.Cmd == nil {
			return a, nil
		}
		return a, tea.ExecProcess(m.Cmd, tea.ExecCallback(m.OnDone))

	case CopyTextMsg:
		return a, tea.Batch(tea.SetClipboard(m.Text), a.setToast("copied to clipboard"))

	case columnMetaMsg:
		// An async ColumnsMeta fetch finished. On success store the metadata
		// into the completion cache so the next View shows the real type
		// instead of the frame DType fallback. The cache may be unbound when
		// WarmCompletionSchema never ran (cold open); bind it to the current
		// backend in that case. A cache bound to a different backend
		// (connection swapped mid-flight) rejects the store.
		if m.err == nil && m.meta != nil && a.backend != nil {
			c := a.completionCache()
			c.mu.Lock()
			if c.backend == nil {
				c.backend = a.backend
				c.ns = make(map[string]string)
				c.cols = make(map[string][]string)
				c.meta = make(map[string][]db.ColumnMeta)
				c.fetched = make(map[string]bool)
			}
			if c.backend == a.backend {
				c.meta[m.table] = m.meta
				c.fetched[m.table] = true
				if c.cols[m.table] == nil {
					cols := make([]string, len(m.meta))
					for i, cm := range m.meta {
						cols[i] = cm.Name
					}
					c.cols[m.table] = cols
				}
			}
			c.mu.Unlock()
		}
		return a, nil

	case tea.PasteMsg:
		// Bracketed paste follows key routing: the top overlay swallows it;
		// with no overlay open it feeds the active search bar.
		if n := len(a.overlays); n > 0 {
			ov, cmd := a.overlays[n-1].Update(msg)
			a.overlays[n-1] = ov
			return a, cmd
		}
		if a.search.Active() {
			a.search.Paste(m.Content)
			if p := a.pane(); p != nil {
				p.Table.ClampTo(a.search.Preview())
			}
		}
		return a, nil

	case cellSavedMsg:
		// An async UPDATE finished. On success toast and refresh the table so
		// the edit is visible; on error surface the error overlay.
		if m.err != nil {
			a.overlays = append(a.overlays, errBox{err: m.err})
			return a, nil
		}
		cmds := []tea.Cmd{a.setToast(fmt.Sprintf("updated %d row(s)", m.rows))}
		if a.backend != nil {
			cmds = append(cmds, a.runCommand("refreshtable", ""))
		}
		return a, tea.Batch(cmds...)

	case rowsDeletedMsg:
		// An async DELETE finished. On success toast, refresh the table and
		// clear the multi-select set; on error surface the error overlay.
		if m.err != nil {
			a.overlays = append(a.overlays, errBox{err: m.err})
			return a, nil
		}
		if p := a.pane(); p != nil {
			p.Table.ClearSelect()
		}
		cmds := []tea.Cmd{a.setToast(fmt.Sprintf("deleted %d row(s)", m.rows))}
		if a.backend != nil {
			cmds = append(cmds, a.runCommand("refreshtable", ""))
		}
		return a, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		return a.handleKey(m)

	default:
		// Anything else (ticks, async results, ...) goes to the top overlay
		// so self-updating overlays keep working.
		if n := len(a.overlays); n > 0 {
			ov, cmd := a.overlays[n-1].Update(msg)
			a.overlays[n-1] = ov
			return a, cmd
		}
		return a, nil
	}
}

func (a *App) View() tea.View {
	w, h := a.width, a.height
	bodyH := max(1, h-1)

	bottom := a.renderBottom(w)

	var content string
	if n := len(a.overlays); n > 0 {
		top := a.overlays[n-1]
		box := top.View(w, bodyH, a.th)
		if fs, ok := top.(FullscreenOverlay); ok && fs.Fullscreen() {
			// Page-style overlay: its view replaces the whole body area (the
			// status bar stays at the bottom). The blank fill guarantees
			// nothing of the view behind it shows through.
			content = FillPage(box, w, bodyH) + "\n" + bottom
		} else {
			content = Composite(a.renderBody(w, bodyH)+"\n"+bottom, box, w, h)
		}
	} else {
		content = a.renderBody(w, bodyH) + "\n" + bottom
	}

	v := tea.NewView(content)
	v.AltScreen = true
	// No BackgroundColor: the terminal's own background shows through so the
	// app blends with the user's terminal theme.
	return v
}

// --- rendering ----------------------------------------------------------------

func (a *App) renderBody(w, h int) string {
	p := a.pane()
	if p == nil {
		msg := "no data loaded"
		if a.backend != nil || a.kv != nil {
			msg = "no tables open - press : for commands"
		}
		lines := strings.Split(blankLines(w, h, a.th), "\n")
		mid := h / 2
		if mid < len(lines) {
			pad := max(0, (w-len(msg))/2)
			lines[mid] = a.th.Subtle.Render(padLine(strings.Repeat(" ", pad)+msg, w))
		}
		return strings.Join(lines, "\n")
	}

	frame := p.Current()
	if a.search.Active() {
		frame = a.search.Preview()
	}

	if p.Mode == ModeSheet && frame != nil && frame.NumRows() > 0 {
		if p.SheetEditing {
			keyW, valW := sheetTableGeometry(frame, w)
			matched := sheetMatchedCols(frame, string(p.SheetFilter))
			actualCol := p.SheetField
			if len(matched) > 0 {
				actualCol = matched[clamp(p.SheetField, 0, len(matched)-1)]
			}
			leftContent := renderSheetKeyPanel(frame, p.Table.Sel(), p.SheetOff, keyW, h, p.SheetField,
				string(p.SheetFilter), p.SheetFiltering, a.th)
			rightContent := renderSheetDrawer(frame, p.Table.Sel(), actualCol,
				p.SheetEdit, p.SheetEditCur, p.SheetValOff, valW, h, a.cursorFieldType(), a.th)
			return joinDrawerSplit(leftContent, rightContent, keyW, valW, h, a.th)
		}
		return renderSheet(frame, p.Table.Sel(), p.SheetOff, w, h, p.SheetField,
			false, nil, 0, p.SheetValOff,
			string(p.SheetFilter), p.SheetFiltering, a.th)
	}

	return p.Table.Render(frame, RenderOpts{
		Width:          w,
		Height:         h,
		Theme:          a.th,
		ShowBorders:    a.showBorders,
		ShowRowNumbers: a.showRowNumbers,
	})
}

func (a *App) renderBottom(w int) string {
	if a.search.Active() {
		return a.search.View(w, a.th)
	}
	if a.toastText != "" {
		return a.th.Success.Render(padLine(" "+a.toastText, w))
	}
	info := statusInfo{Tab: 0, NTabs: len(a.tabs), Row: 0}
	if p := a.pane(); p != nil {
		info.Tab = a.active + 1
		info.Crumbs = p.Crumbs()
		if f := p.Current(); f != nil {
			info.Row = p.Table.Sel() + 1
			info.NRows = f.NumRows()
			info.NCols = f.NumCols()
			if f.NumCols() > 0 {
				info.Mode = "fit"
				if p.Table.Expanded() {
					info.Mode = "wide"
				}
				// The column cursor is live in both modes (y-copy targets it).
				info.Col = f.Columns[clamp(p.Table.ColCursor(), 0, f.NumCols()-1)].Name
			}
		}
	}
	switch {
	case a.backend != nil:
		info.Conn = a.backend.Title()
	case a.kv != nil:
		info.Conn = a.kv.Title()
	}
	return renderStatusBar(info, w, a.th)
}

// --- key handling ---------------------------------------------------------------

func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Modal overlays swallow every key.
	if n := len(a.overlays); n > 0 {
		ov, cmd := a.overlays[n-1].Update(msg)
		a.overlays[n-1] = ov
		return a, cmd
	}

	if a.search.Active() {
		return a.handleSearchKey(msg)
	}

	key := msg.String()
	if key == "ctrl+c" {
		a.exiting = true
		return a, tea.Quit
	}

	p := a.pane()
	if p != nil && p.Mode == ModeSheet {
		// While inline-editing a sheet field, keys go to the edit handler
		// before the normal SheetBindings dispatch: j/k and other nav keys
		// must not move the cursor mid-edit.
		if p.SheetEditing {
			return a.handleSheetEditKey(msg)
		}
		// While the field-filter input is active, keys go to the filter
		// handler instead of the normal dispatch so "/" can pre-type the
		// first rune and esc clears the filter instead of leaving sheet mode.
		if p.SheetFiltering {
			return a.handleSheetFilterKey(msg)
		}
		return a, a.dispatch(actionFor(key, SheetBindings, GlobalBindings), key)
	}
	return a, a.dispatch(actionFor(key, TableBindings, GlobalBindings), key)
}

// handleSheetFilterKey routes keys while the sheet field-filter input is
// active. esc/ctrl+c clear the filter and exit filter input; enter commits the
// current pattern (filter stays applied, input closes); backspace pops the
// last rune; ctrl+u clears the text; up/down/j/k move the cursor within the
// matched set; printable runes append and reset the cursor to the first
// match. After each change the matched set is recomputed and SheetField is
// clamped, with the left list edge-followed.
func (a *App) handleSheetFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := a.pane()
	if p == nil {
		return a, nil
	}
	f := p.Current()
	if f == nil {
		return a, nil
	}
	key := msg.String()
	switch key {
	case "esc", "ctrl+c":
		p.SheetFiltering = false
		p.SheetFilter = nil
		p.SheetField = 0
		p.SheetOff = 0
		return a, nil
	case "enter":
		p.SheetFiltering = false
		// Keep the filter applied. Clamp the cursor to the matched set and
		// edge-follow so the entry stays visible.
		a.clampSheetToMatched(f, p)
		return a, nil
	case "backspace", "ctrl+h":
		if len(p.SheetFilter) > 0 {
			p.SheetFilter = p.SheetFilter[:len(p.SheetFilter)-1]
		}
		p.SheetField = 0
		p.SheetOff = 0
		a.clampSheetToMatched(f, p)
		return a, nil
	case "ctrl+u":
		p.SheetFilter = nil
		p.SheetField = 0
		p.SheetOff = 0
		return a, nil
	case "up", "k":
		if p.SheetField > 0 {
			p.SheetField--
		}
		p.SheetValOff = 0
		a.edgeFollowSheet(f, p)
		return a, nil
	case "down", "j":
		matched := sheetMatchedCols(f, string(p.SheetFilter))
		if len(matched) > 0 && p.SheetField < len(matched)-1 {
			p.SheetField++
		}
		p.SheetValOff = 0
		a.edgeFollowSheet(f, p)
		return a, nil
	default:
		if t := msg.Key().Text; t != "" && msg.Mod == 0 {
			p.SheetFilter = append(p.SheetFilter, []rune(t)...)
			p.SheetField = 0
			p.SheetOff = 0
			a.clampSheetToMatched(f, p)
		}
		return a, nil
	}
}

// clampSheetToMatched ensures SheetField is a valid index into the current
// matched set and re-runs the left-list edge-follow. Called after every
// filter mutation that can shrink the matched set.
func (a *App) clampSheetToMatched(f *data.Frame, p *Pane) {
	matched := sheetMatchedCols(f, string(p.SheetFilter))
	if len(matched) == 0 {
		p.SheetField = 0
		p.SheetOff = 0
		return
	}
	if p.SheetField >= len(matched) {
		p.SheetField = len(matched) - 1
	}
	if p.SheetField < 0 {
		p.SheetField = 0
	}
	a.edgeFollowSheet(f, p)
}

// edgeFollowSheet advances SheetOff so the cursored entry stays visible in
// the left list, mirroring renderSheet's geometry. It assumes SheetField is a
// valid index into the current matched set.
func (a *App) edgeFollowSheet(f *data.Frame, p *Pane) {
	matched := sheetMatchedCols(f, string(p.SheetFilter))
	if len(matched) == 0 {
		p.SheetOff = 0
		return
	}
	keyW, _ := sheetTableGeometry(f, a.width)
	bodyH := max(1, a.height-1)
	// Mirror renderSheet's header accounting: row header, optional filter
	// line, and the Key|Value column header.
	headers := 2
	if p.SheetFiltering || strings.TrimSpace(string(p.SheetFilter)) != "" {
		headers = 3
	}
	visible := bodyH - headers
	if visible < 1 {
		visible = 1
	}
	maxOff := max(0, sheetKeyLineCountCols(f, matched, keyW)-visible)
	curStart := sheetKeyLineOffsetCols(f, matched, p.SheetField, keyW)
	curEnd := curStart + sheetKeyEntryHeightCols(f, matched, p.SheetField, keyW) - 1
	if curStart < p.SheetOff {
		p.SheetOff = curStart
	} else if curEnd >= p.SheetOff+visible {
		p.SheetOff = curEnd - visible + 1
		if p.SheetOff > curStart {
			p.SheetOff = curStart
		}
	}
	p.SheetOff = clamp(p.SheetOff, 0, maxOff)
}

// handleSheetEditKey routes keys while a sheet field is being inline-edited.
// esc/ctrl+c cancel (keeping the original value); ctrl+s commits via the
// "confirmsave" command; printable runes insert at the cursor; backspace
// deletes before the cursor; left/right/home/end/ctrl+a/ctrl+e move the
// cursor; ctrl+u clears the buffer. After each mutation the right-pane
// value scroll is re-clamped. Any other key is swallowed so the field does
// not move.
func (a *App) handleSheetEditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := a.pane()
	if p == nil {
		return a, nil
	}
	f := p.Current()
	if f == nil {
		return a, nil
	}
	key := msg.String()
	switch key {
	case "esc", "ctrl+c":
		p.SheetEditing = false
		p.SheetEdit = nil
		p.SheetEditCur = 0
		return a, nil
	case "ctrl+s":
		if p.SheetField < 0 || p.SheetField >= f.NumCols() {
			return a, nil
		}
		a.pendingEdit = &PendingEdit{
			Frame:     f,
			Row:       p.Table.Sel(),
			Col:       p.SheetField,
			ColName:   f.Columns[p.SheetField].Name,
			OldValue:  f.CellString(p.Table.Sel(), p.SheetField),
			NewValue:  string(p.SheetEdit),
			Table:     a.BaseCrumb(),
			Namespace: a.CurrentTableNamespace(),
		}
		return a, a.runCommand("confirmsave", "")
	case "backspace", "ctrl+h":
		if p.SheetEditCur > 0 {
			p.SheetEdit = append(p.SheetEdit[:p.SheetEditCur-1], p.SheetEdit[p.SheetEditCur:]...)
			p.SheetEditCur--
		}
	case "delete":
		if p.SheetEditCur < len(p.SheetEdit) {
			p.SheetEdit = append(p.SheetEdit[:p.SheetEditCur], p.SheetEdit[p.SheetEditCur+1:]...)
		}
	case "left":
		if p.SheetEditCur > 0 {
			p.SheetEditCur--
		}
	case "right":
		if p.SheetEditCur < len(p.SheetEdit) {
			p.SheetEditCur++
		}
	case "home", "ctrl+a":
		p.SheetEditCur = 0
	case "end", "ctrl+e":
		p.SheetEditCur = len(p.SheetEdit)
	case "ctrl+u":
		p.SheetEdit = p.SheetEdit[p.SheetEditCur:]
		p.SheetEditCur = 0
	default:
		// Printable rune insert at the cursor. Modifier combos (ctrl+x) are
		// ignored so they never land as literal text.
		if t := msg.Key().Text; t != "" && msg.Mod == 0 {
			r := []rune(t)
			p.SheetEdit = append(p.SheetEdit[:p.SheetEditCur], append(r, p.SheetEdit[p.SheetEditCur:]...)...)
			p.SheetEditCur += len(r)
		}
	}
	// Re-clamp the right-pane value scroll to the edit buffer length.
	_, valW := sheetTableGeometry(f, a.width)
	// drawer header: fieldname(type) + separator + hint = 3 lines
	visible := max(1, max(1, a.height-1)-3)
	count := sheetValueLineCount(f, p.Table.Sel(), p.SheetField, valW)
	if editing := len(p.SheetEdit); editing > 0 {
		count = strings.Count(ansi.Wrap(string(p.SheetEdit), valW, ""), "\n") + 1
	}
	maxOff := max(0, count-visible)
	// Keep the cursor line in view while typing.
	curLine, _ := sheetEditLineOf(p.SheetEdit, p.SheetEditCur, valW)
	if curLine < p.SheetValOff {
		p.SheetValOff = curLine
	} else if curLine >= p.SheetValOff+visible {
		p.SheetValOff = curLine - visible + 1
	}
	p.SheetValOff = clamp(p.SheetValOff, 0, maxOff)
	return a, nil
}

func (a *App) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		a.search.Cancel()
		if p := a.pane(); p != nil {
			p.Table.ClampTo(p.Current())
		}
		return a, nil
	case "enter":
		f, crumb, ok := a.search.Commit()
		if p := a.pane(); ok && p != nil {
			p.Push(f, crumb)
		} else if p != nil {
			p.Table.ClampTo(p.Current())
		}
		return a, nil
	case "backspace":
		a.search.Backspace()
	default:
		if t := msg.Key().Text; t != "" {
			a.search.Type(t)
		}
	}
	if p := a.pane(); p != nil {
		p.Table.ClampTo(a.search.Preview())
	}
	return a, nil
}

// dispatch executes a semantic action from the keymap.
func (a *App) dispatch(action, key string) tea.Cmd {
	p := a.pane()
	var f *data.Frame
	if p != nil {
		f = p.Current()
	}
	nrows, ncols := 0, 0
	if f != nil {
		nrows, ncols = f.NumRows(), f.NumCols()
	}

	switch action {
	// global
	case ActQuit:
		a.exiting = true
		return tea.Quit
	case ActHelp:
		return a.runCommand("help", "")
	case ActPalette:
		return a.runCommand("palette", "")
	case ActTabSwitch:
		return a.runCommand("tabs", "")
	case ActPrevTab:
		if len(a.tabs) > 1 {
			a.active = (a.active - 1 + len(a.tabs)) % len(a.tabs)
		}
	case ActNextTab:
		if len(a.tabs) > 1 {
			a.active = (a.active + 1) % len(a.tabs)
		}
	case ActEscBack:
		// Handled before the pane check: on an empty db-mode workspace esc
		// reopens the connection form.
		return a.escBack()
	}

	if p == nil || f == nil {
		return nil
	}

	switch action {
	// table navigation
	case ActUp:
		p.Table.Move(-1, nrows)
	case ActDown:
		p.Table.Move(1, nrows)
	case ActTop:
		p.Table.Top()
	case ActBottom:
		p.Table.Bottom(nrows)
	case ActHalfUp:
		p.Table.HalfPageUp(nrows)
	case ActHalfDown:
		p.Table.HalfPageDown(nrows)
	case ActPageUp:
		p.Table.PageUp(nrows)
	case ActPageDown:
		p.Table.PageDown(nrows)
	case ActRandom:
		p.Table.Random(nrows)
	case ActLeft:
		p.Table.PrevCol(ncols)
	case ActRight:
		p.Table.NextCol(ncols)
	case ActFirstCol:
		p.Table.FirstCol()
	case ActLastCol:
		p.Table.LastCol(ncols)
	case ActExpand:
		p.Table.ToggleExpanded()
	case ActEdit:
		// Toggle inline edit on the cursored sheet field. Only meaningful in
		// sheet mode; in table mode e is unbound. While already editing, e is
		// a no-op (esc cancels, ctrl+s commits).
		if p.Mode != ModeSheet || f == nil || nrows == 0 || ncols == 0 {
			return nil
		}
		if p.SheetEditing {
			return nil
		}
		val := f.CellString(p.Table.Sel(), p.SheetField)
		p.SheetEdit = []rune(val)
		p.SheetEditCur = len(p.SheetEdit)
		p.SheetEditing = true
		p.SheetValOff = 0
		return nil
	case ActRefresh:
		if a.backend != nil {
			return a.runCommand("refreshtable", "")
		}
		if a.kv != nil {
			return a.setToast("use r in the key browser to rescan")
		}
		return a.setToast("use :import to reload") // file mode fallback
	case ActToggleSelect:
		if p != nil && f != nil && nrows > 0 {
			p.Table.ToggleSelect(p.Table.Sel())
		}
		return nil
	case ActDelete:
		return a.startDelete()

	// mode switches
	case ActSheet:
		if nrows > 0 {
			p.Mode = ModeSheet
			p.SheetOff = 0
			p.SheetField = 0
			p.SheetValOff = 0
			p.SheetEditing = false
			p.SheetEdit = nil
			p.SheetEditCur = 0
			p.SheetFilter = nil
			p.SheetFiltering = false
			// Warm the column-metadata cache for this table so the sheet shows
			// the real engine type (e.g. "varchar(255)") instead of the frame
			// DType fallback ("str" on mysql). Skipped when the cache is already
			// populated or in file mode (no backend -> warmColumnMeta is nil).
			if a.backend != nil && a.columnMetaFor(p.Title) == nil {
				return a.warmColumnMeta(p.Title)
			}
		}
	case ActSheetFilter:
		// "/" enters filter input with a fresh buffer. While already filtering
		// this is never reached: handleKey routes to handleSheetFilterKey
		// before dispatch, so a literal "/" lands as a printable rune.
		p.SheetFilter = nil
		p.SheetFiltering = true
		return nil
	case ActBack:
		p.Mode = ModeTable
	case ActSheetDown, ActSheetUp:
		matched := sheetMatchedCols(f, string(p.SheetFilter))
		if len(matched) == 0 {
			return nil
		}
		delta := 1
		if action == ActSheetUp {
			delta = -1
		}
		p.SheetField = clamp(p.SheetField+delta, 0, len(matched)-1)
		// A new field shows its value from the top, and cancels any in-flight
		// inline edit (the edit buffer belongs to the previous field).
		p.SheetValOff = 0
		if p.SheetEditing {
			p.SheetEditing = false
			p.SheetEdit = nil
			p.SheetEditCur = 0
		}
		a.edgeFollowSheet(f, p)
	case ActCopy:
		text := rowClipboardText(f, p.Table.Sel())
		return tea.Batch(tea.SetClipboard(text), a.setToast("copied to clipboard"))
	case ActCopyCell:
		if nrows == 0 || ncols == 0 {
			return nil
		}
		c := clamp(p.Table.ColCursor(), 0, ncols-1)
		r := clamp(p.Table.Sel(), 0, nrows-1)
		return tea.Batch(
			tea.SetClipboard(f.CellString(r, c)),
			a.setToast("copied cell "+f.Columns[c].Name),
		)
	case ActCopyRow:
		if nrows == 0 {
			return nil
		}
		r := clamp(p.Table.Sel(), 0, nrows-1)
		return tea.Batch(
			tea.SetClipboard(rowClipboardText(f, r)),
			a.setToast(fmt.Sprintf("copied row %d", r+1)),
		)

	// searches / commands
	case ActFuzzy:
		a.search.Start(f, true)
	case ActExact:
		a.search.Start(f, false)
	case ActInfo:
		return a.runCommand("info", "")
	case ActGoto:
		return a.runCommand("goto", key)

	// stack / tabs
	case ActPop:
		return a.pop()
	}
	return nil
}

// pop implements "q": sheet -> table, stack pop, close tab, quit on last.
func (a *App) pop() tea.Cmd {
	p := a.pane()
	if p == nil {
		a.exiting = true
		return tea.Quit
	}
	if p.Mode == ModeSheet {
		p.Mode = ModeTable
		return nil
	}
	if p.Pop() {
		return nil
	}
	return a.closeTab(a.active)
}

// startDelete gathers the rows to delete (the multi-select set, or the cursor
// row when nothing is selected) and hands off to the confirm-delete popup. In
// database mode it first resolves the table's primary keys: a table without a
// primary key cannot be safely deleted and is rejected with a toast. File mode
// always has an implicit identity (row index) so it skips the PK check.
func (a *App) startDelete() tea.Cmd {
	p := a.pane()
	if p == nil {
		return nil
	}
	f := p.Current()
	if f == nil {
		return nil
	}
	nrows := f.NumRows()
	if nrows == 0 {
		return nil
	}
	rows := p.Table.SelectedRows()
	if len(rows) == 0 {
		rows = []int{clamp(p.Table.Sel(), 0, nrows-1)}
	}
	if a.backend != nil {
		table := a.BaseCrumb()
		ns := a.CurrentTableNamespace()
		pks, err := a.backend.PrimaryKeys(ns, table)
		if err != nil {
			e := err
			return func() tea.Msg { return ErrorMsg{Err: e} }
		}
		if len(pks) == 0 {
			return a.setToast("no primary key, cannot delete")
		}
		a.pendingDelete = &PendingDelete{Frame: f, Rows: rows, Table: table, Namespace: ns}
		return a.runCommand("confirmdelete", "")
	}
	// file mode
	a.pendingDelete = &PendingDelete{Frame: f, Rows: rows}
	return a.runCommand("confirmdelete", "")
}

// escBack implements esc's "go back one level" in table mode. Unlike pop it
// never closes a tab and never quits: sheet -> table, stack pop, and on the
// base frame it steps back OUT to the browser overlay (db mode) or the
// connection form (empty db-mode workspace). In file mode the base frame is
// the bottom of the hierarchy and esc does nothing.
func (a *App) escBack() tea.Cmd {
	dbMode := a.backend != nil || a.kv != nil
	p := a.pane()
	if p == nil {
		// Empty workspace: back to the connection form (registered by the
		// db-mode integration as the "connect" factory).
		if dbMode {
			return a.runCommand("connect", "")
		}
		return nil
	}
	if p.Mode == ModeSheet {
		p.Mode = ModeTable
		return nil
	}
	if p.Pop() {
		return nil
	}
	// Base frame: one level up is the schema browser (SQL backends) or the
	// key browser (the db-mode integration wraps the "schema" factory to
	// serve it when a key-value connection is live).
	if dbMode {
		return a.runCommand("schema", "")
	}
	return nil
}

// --- commands ---------------------------------------------------------------------

// runCommand resolves a command name: reserved built-ins first, then the
// popup factory registry.
func (a *App) runCommand(name, arg string) tea.Cmd {
	switch name {
	case "quit":
		a.exiting = true
		return tea.Quit
	case "reset":
		if p := a.pane(); p != nil {
			p.Reset()
		}
		return nil
	case "toggleborders":
		a.showBorders = !a.showBorders
		a.persistUI()
		return nil
	case "togglerownumbers":
		a.showRowNumbers = !a.showRowNumbers
		a.persistUI()
		return nil
	case "reloadconfig":
		uc, err := config.ReadUIConfig()
		if err != nil {
			a.overlays = append(a.overlays, errBox{err: fmt.Errorf("reload config: %w", err)})
			return nil
		}
		if th, ok := theme.Builtin(uc.Theme); ok {
			a.th, a.themeName = th, uc.Theme
		}
		a.showBorders = uc.ShowBorders
		a.showRowNumbers = uc.ShowRowNumbers
		return a.setToast("config reloaded")
	case "saveedit":
		if a.pendingEdit == nil {
			return nil
		}
		pe := *a.pendingEdit
		a.pendingEdit = nil
		if p := a.pane(); p != nil {
			p.SheetEditing = false
		}
		return commitCellEdit(a, pe)
	case "canceledit":
		a.pendingEdit = nil
		if p := a.pane(); p != nil {
			p.SheetEditing = false
		}
		return a.setToast("edit canceled")
	case "deleterows":
		if a.pendingDelete == nil {
			return nil
		}
		pd := *a.pendingDelete
		a.pendingDelete = nil
		if p := a.pane(); p != nil {
			p.Table.ClearSelect()
		}
		return commitRowDelete(a, pd)
	case "canceldelete":
		a.pendingDelete = nil
		if p := a.pane(); p != nil {
			p.Table.ClearSelect()
		}
		return nil
	}

	factory, ok := Factories[name]
	if !ok {
		// The search bar is part of the app itself; serve these directly
		// when no popup has claimed the names.
		switch name {
		case "search", "fuzzysearch":
			if f := a.CurrentFrame(); f != nil {
				a.search.Start(f, name == "fuzzysearch")
			}
			return nil
		}
		a.overlays = append(a.overlays, errBox{err: fmt.Errorf("unknown command: %s", name)})
		return nil
	}
	ov, err := factory(a, arg)
	if err != nil {
		a.overlays = append(a.overlays, errBox{err: err})
		return nil
	}
	if ov != nil {
		return a.pushOverlay(ov)
	}
	return nil
}

// pushOverlay appends an overlay to the stack and starts its Init command
// when it has one (OverlayIniter).
func (a *App) pushOverlay(ov Overlay) tea.Cmd {
	a.overlays = append(a.overlays, ov)
	if in, ok := ov.(OverlayIniter); ok {
		return in.Init()
	}
	return nil
}

// PushOverlay adds an overlay before the program starts. Database mode uses
// it to show the connection form on top of the empty workspace.
func (a *App) PushOverlay(ov Overlay) {
	if ov != nil {
		a.overlays = append(a.overlays, ov)
	}
}

// applyFrame handles ApplyFrameMsg: push onto the originating pane's stack or
// open a new tab.
func (a *App) applyFrame(m ApplyFrameMsg) tea.Cmd {
	if m.Frame == nil {
		return nil
	}
	if m.RegisterAs != "" && a.engine != nil {
		_ = a.engine.Register(m.RegisterAs, m.Frame) // best effort, mirrors CLI load
	}
	newTab := m.NewTab || len(a.tabs) == 0
	var target *Pane
	if !newTab {
		if m.PaneID != 0 {
			// Route the result back to the pane the command was run from; the
			// user may have switched tabs while it executed. A vanished pane
			// (tab closed mid-flight) falls back to opening a new tab.
			target = a.paneByID(m.PaneID)
			newTab = target == nil
		} else {
			target = a.pane()
			newTab = target == nil
		}
	}
	if newTab {
		title := m.TabTitle
		if title == "" {
			title = m.Crumb
		}
		if title == "" {
			title = "result"
		}
		a.tabs = append(a.tabs, NewPane(title, m.Frame))
		a.active = len(a.tabs) - 1
	} else {
		crumb := m.Crumb
		if crumb == "" {
			crumb = "op"
		}
		target.Push(m.Frame, crumb)
	}
	return nil
}

// replaceBase handles ReplaceBaseMsg: swap the base frame of the target pane
// (used by the refresh command to reload the current table in place).
func (a *App) replaceBase(m ReplaceBaseMsg) tea.Cmd {
	var target *Pane
	if m.PaneID != 0 {
		target = a.paneByID(m.PaneID)
	}
	if target == nil {
		target = a.pane()
	}
	if target == nil || m.Frame == nil {
		return nil
	}
	target.ReplaceBase(m.Frame)
	return nil
}

// paneByID finds an open pane by its stable ID; nil when it was closed.
func (a *App) paneByID(id int) *Pane {
	for _, p := range a.tabs {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

func (a *App) closeTab(i int) tea.Cmd {
	if len(a.tabs) == 0 {
		a.exiting = true
		return tea.Quit
	}
	i = clamp(i, 0, len(a.tabs)-1)
	a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
	if len(a.tabs) == 0 {
		a.exiting = true
		return tea.Quit
	}
	// Closing a tab before the active one shifts the slice left; follow the
	// active pane so the user keeps looking at the same tab.
	if i < a.active {
		a.active--
	}
	if a.active >= len(a.tabs) {
		a.active = len(a.tabs) - 1
	}
	return nil
}

func (a *App) registerTable(name string) tea.Cmd {
	if a.engine == nil {
		a.overlays = append(a.overlays, errBox{err: fmt.Errorf("no embedded SQL engine in this mode")})
		return nil
	}
	f := a.CurrentFrame()
	if f == nil {
		a.overlays = append(a.overlays, errBox{err: fmt.Errorf("no frame to register")})
		return nil
	}
	if err := a.engine.Register(name, f); err != nil {
		a.overlays = append(a.overlays, errBox{err: err})
		return nil
	}
	return a.setToast("registered as " + name)
}

// ensureSynced re-registers the active frame under "_" so that
// `select * from _` targets what the user is looking at. The registration is
// O(rows), so it runs lazily — only when SQL is about to execute (Engine())
// and only if the frame under view changed since the last sync (frames are
// immutable by convention, so pointer identity is a valid cache key).
func (a *App) ensureSynced() {
	if a.engine == nil {
		return
	}
	if f := a.CurrentFrame(); f != nil && f != a.syncedFrame {
		if a.engine.Register("_", f) == nil { // best effort
			a.syncedFrame = f
		}
	}
}

func (a *App) setToast(text string) tea.Cmd {
	a.toastText = text
	a.toastID++
	id := a.toastID
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastExpiredMsg{id: id}
	})
}

func (a *App) persistUI() {
	_ = config.WriteUIConfig(&config.UIConfig{ // best effort
		Theme:          a.themeName,
		ShowBorders:    a.showBorders,
		ShowRowNumbers: a.showRowNumbers,
	})
}

func (a *App) pane() *Pane {
	if len(a.tabs) == 0 {
		return nil
	}
	if a.active >= len(a.tabs) {
		a.active = len(a.tabs) - 1
	}
	return a.tabs[a.active]
}

// --- AppContext ----------------------------------------------------------------

func (a *App) CurrentFrame() *data.Frame {
	if p := a.pane(); p != nil {
		return p.Current()
	}
	return nil
}

func (a *App) CurrentRow() int {
	if p := a.pane(); p != nil {
		return p.Table.Sel()
	}
	return 0
}

// SheetFieldCursor reports the selected field index in sheet mode. It is
// always a valid column index for the current frame, or 0 when no pane is
// open.
func (a *App) SheetFieldCursor() int {
	if p := a.pane(); p != nil {
		return p.SheetField
	}
	return 0
}

// CurrentTableNamespace reports the namespace the active table was loaded
// from, so the refresh command can re-fetch the same table. It consults the
// completion cache's table-to-namespace map first, then falls back to the
// backend's connected namespace (when the backend exposes one), and finally
// returns "" (the backend then treats it as the default namespace).
func (a *App) CurrentTableNamespace() string {
	if a.backend == nil {
		return ""
	}
	if p := a.pane(); p != nil {
		c := a.completionCache()
		c.mu.Lock()
		ns := c.ns[p.Title]
		c.mu.Unlock()
		if ns != "" {
			return ns
		}
	}
	if cn, ok := a.backend.(interface{ CurrentNamespace() string }); ok {
		return cn.CurrentNamespace()
	}
	return ""
}

// warmColumnMeta returns a tea.Cmd that fetches the real column metadata for
// table off the update loop. The resulting columnMetaMsg populates the
// completion cache, so the next View shows the engine-native type inline with
// the value instead of the frame DType fallback. The cache is touched under
// lock only when the connection still matches by the time the fetch returns.
// Returns nil in file mode (no backend).
func (a *App) warmColumnMeta(table string) tea.Cmd {
	be := a.backend
	if be == nil || table == "" {
		return nil
	}
	ns := a.CurrentTableNamespace()
	tableC := table
	return func() tea.Msg {
		m, err := be.ColumnsMeta(ns, tableC)
		return columnMetaMsg{table: tableC, meta: m, err: err}
	}
}

// cursorFieldType returns just the data-type token of the active sheet field
// (e.g. "int", "varchar(255)", "i64"). It reuses cursorFieldMeta so the same
// cache-then-DType fallback path applies: real engine type when the cache is
// warm, the frame DType otherwise. Empty when no pane/frame is active.
func (a *App) cursorFieldType() string {
	dt, _, _, _ := a.cursorFieldMeta()
	return dt
}

func (a *App) BaseCrumb() string {
	if p := a.pane(); p != nil {
		return p.Title
	}
	return ""
}

func (a *App) Crumbs() []string {
	if p := a.pane(); p != nil {
		return p.Crumbs()
	}
	return nil
}

func (a *App) ColumnNames() []string {
	if f := a.CurrentFrame(); f != nil {
		return f.ColumnNames()
	}
	return nil
}

func (a *App) Engine() *query.Engine {
	a.ensureSynced()
	return a.engine
}

func (a *App) TableNames() []string {
	var names []string
	if a.engine != nil {
		if ts, err := a.engine.Tables(); err == nil {
			names = append(names, ts...)
		}
	}
	if a.backend != nil {
		if nss, err := a.backend.Namespaces(); err == nil {
			for _, ns := range nss {
				if ts, err := a.backend.Tables(ns); err == nil {
					names = append(names, ts...)
				}
			}
		}
	}
	return names
}

func (a *App) Backend() db.Backend { return a.backend }
func (a *App) KV() db.KVBackend    { return a.kv }

func (a *App) Theme() *theme.Theme { return a.th }
func (a *App) ThemeName() string   { return a.themeName }

func (a *App) ShowBorders() bool    { return a.showBorders }
func (a *App) ShowRowNumbers() bool { return a.showRowNumbers }

func (a *App) Tabs() []TabInfo {
	out := make([]TabInfo, len(a.tabs))
	for i, p := range a.tabs {
		shape := "0 x 0"
		if f := p.Current(); f != nil {
			shape = fmt.Sprintf("%d x %d", f.NumRows(), f.NumCols())
		}
		out[i] = TabInfo{Title: p.Title, Shape: shape}
	}
	return out
}

func (a *App) ActiveTab() int { return a.active }

func (a *App) ActivePaneID() int {
	if p := a.pane(); p != nil {
		return p.ID()
	}
	return 0
}

// PendingEdit reports the in-flight cell edit awaiting commit, or nil when
// no edit is in progress. Confirm popups read it to render the proposed
// change; the "saveedit" command consumes it.
func (a *App) PendingEdit() *PendingEdit { return a.pendingEdit }

// PendingDelete reports the in-flight row delete awaiting commit, or nil
// when no delete is in progress.
func (a *App) PendingDelete() *PendingDelete { return a.pendingDelete }

// compile-time interface checks
var (
	_ tea.Model  = (*App)(nil)
	_ AppContext = (*App)(nil)
)
