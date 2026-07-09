// Import wizard overlay: a three-step popup that loads files or URLs into
// new tabs. Step 1 asks for a path (with filename tab-completion), step 2
// picks the format, step 3 configures text-format options. Registered under
// the command name "import".
package popup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

func init() {
	ui.Factories["import"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return newImporter(arg), nil
	}
}

// wizard steps
const (
	importerStepPath = iota
	importerStepFormat
	importerStepOptions
)

// importerAuto is the pseudo-format that resolves via reader.Detect.
const importerAuto = "auto"

// importerBinary marks formats that have no text parsing options; the wizard
// skips step 3 for them and imports straight from the format step.
var importerBinary = map[reader.Format]bool{
	reader.FormatParquet: true,
	reader.FormatSQLite:  true,
	reader.FormatExcel:   true,
}

// importerInferModes are the choices of the infer-schema pick, in order.
var importerInferModes = []reader.InferMode{reader.InferNo, reader.InferFast, reader.InferSafe}

// importerResultMsg carries the outcome of the async load back into the
// wizard (the app routes non-key messages to the topmost overlay). The owner
// field identifies the wizard instance that started the load, so a result
// from a cancelled import is not delivered to a later instance.
type importerResultMsg struct {
	owner  *importer
	frames []reader.NamedFrame
	err    error
}

// option rows on step 3
const (
	importerOptHeader = iota
	importerOptSep
	importerOptInfer
)

type importer struct {
	step int

	// step 1: path input + tab completion
	path     importerInput
	matches  []string
	matchIdx int

	// step 2: format list
	formats   []string
	formatIdx int
	formatOff int
	chosen    reader.Format // resolved when leaving the format step

	// step 3: options
	optFocus int // index into visible option rows
	noHeader bool
	sep      importerInput
	inferIdx int

	loading bool
	errline string
}

func newImporter(arg string) *importer {
	o := &importer{
		formats:  append([]string{importerAuto}, reader.SupportedFormats()...),
		inferIdx: importerIndexOf(importerInferModes, reader.DefaultOptions().InferSchema),
	}
	o.path.set(strings.TrimSpace(arg))
	o.sep.set(",")
	return o
}

func importerIndexOf(modes []reader.InferMode, m reader.InferMode) int {
	for i, v := range modes {
		if v == m {
			return i
		}
	}
	return 0
}

// --- update -------------------------------------------------------------------

func (o *importer) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case importerResultMsg:
		if m.owner != o || !o.loading {
			return o, nil // stale result from a cancelled/closed import
		}
		return o.finish(m)
	case tea.PasteMsg:
		o.handlePaste(m.Content)
		return o, nil
	case tea.KeyPressMsg:
		return o.handleKey(m)
	}
	return o, nil
}

// handlePaste routes bracketed-paste text to whichever input is focused:
// the path field on step 1, the separator field on step 3. Other steps (and
// an in-flight load) ignore pastes.
func (o *importer) handlePaste(content string) {
	if o.loading {
		return
	}
	switch o.step {
	case importerStepPath:
		if o.path.paste(content) {
			o.matches = nil // any edit invalidates the completion cycle
			o.errline = ""
		}
	case importerStepOptions:
		if o.optionRows()[o.optFocus] == importerOptSep && o.sep.paste(strings.TrimSpace(pasteSanitize(content))) {
			o.errline = ""
		}
	}
}

// finish reacts to the async load result: on error the wizard stays open on
// the failing step with an inline message; on success every named frame
// becomes a new tab (in order), followed by a toast and the overlay closing.
func (o *importer) finish(m importerResultMsg) (ui.Overlay, tea.Cmd) {
	o.loading = false
	if m.err != nil {
		o.errline = m.err.Error()
		return o, nil
	}
	if len(m.frames) == 0 {
		o.errline = "no tables found in source"
		return o, nil
	}
	cmds := make([]tea.Cmd, 0, len(m.frames)+2)
	for _, nf := range m.frames {
		am := ui.ApplyFrameMsg{Frame: nf.Frame, Crumb: "table", NewTab: true, TabTitle: nf.Name, RegisterAs: nf.Name}
		cmds = append(cmds, func() tea.Msg { return am })
	}
	toast := fmt.Sprintf("imported %d table(s)", len(m.frames))
	cmds = append(cmds,
		func() tea.Msg { return ui.ToastMsg{Text: toast} },
		ui.CloseOverlay,
	)
	return o, tea.Sequence(cmds...)
}

func (o *importer) handleKey(msg tea.KeyPressMsg) (ui.Overlay, tea.Cmd) {
	if o.loading {
		if msg.String() == "esc" {
			return o, ui.CloseOverlay
		}
		return o, nil
	}
	switch o.step {
	case importerStepPath:
		return o.keyPath(msg)
	case importerStepFormat:
		return o.keyFormat(msg)
	default:
		return o.keyOptions(msg)
	}
}

func (o *importer) keyPath(msg tea.KeyPressMsg) (ui.Overlay, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return o, ui.CloseOverlay
	case "left":
		if o.path.cursor > 0 {
			o.path.key(msg)
			return o, nil
		}
		return o, ui.CloseOverlay // back from step 1 closes
	case "tab":
		o.cycleCompletion()
		return o, nil
	case "enter":
		if strings.TrimSpace(o.path.text) == "" {
			o.errline = "enter a file path or URL"
			return o, nil
		}
		o.errline = ""
		o.matches = nil
		o.step = importerStepFormat
		return o, nil
	}
	if o.path.key(msg) {
		o.matches = nil // any edit invalidates the completion cycle
		o.errline = ""
	}
	return o, nil
}

func (o *importer) keyFormat(msg tea.KeyPressMsg) (ui.Overlay, tea.Cmd) {
	switch msg.String() {
	case "esc", "left":
		o.errline = ""
		o.step = importerStepPath
		return o, nil
	case "up", "k":
		if o.formatIdx > 0 {
			o.formatIdx--
		}
		return o, nil
	case "down", "j":
		if o.formatIdx < len(o.formats)-1 {
			o.formatIdx++
		}
		return o, nil
	case "home", "g":
		o.formatIdx = 0
		return o, nil
	case "end", "G":
		o.formatIdx = len(o.formats) - 1
		return o, nil
	case "enter":
		f, err := o.resolveFormat()
		if err != nil {
			o.errline = err.Error()
			return o, nil
		}
		o.chosen = f
		o.errline = ""
		if importerBinary[f] {
			return o.startImport()
		}
		o.optFocus = 0
		o.step = importerStepOptions
		return o, nil
	}
	return o, nil
}

// resolveFormat turns the current list selection into a concrete format,
// running detection for "auto".
func (o *importer) resolveFormat() (reader.Format, error) {
	sel := o.formats[o.formatIdx]
	if sel != importerAuto {
		return reader.Format(sel), nil
	}
	f := importerDetect(strings.TrimSpace(o.path.text))
	if f == "" {
		return "", fmt.Errorf("cannot detect format from name; pick one explicitly")
	}
	return f, nil
}

func (o *importer) keyOptions(msg tea.KeyPressMsg) (ui.Overlay, tea.Cmd) {
	rows := o.optionRows()
	focused := rows[o.optFocus]
	switch msg.String() {
	case "esc":
		o.errline = ""
		o.step = importerStepFormat
		return o, nil
	case "up", "shift+tab":
		if o.optFocus > 0 {
			o.optFocus--
		}
		return o, nil
	case "down", "tab":
		if o.optFocus < len(rows)-1 {
			o.optFocus++
		}
		return o, nil
	case "enter":
		return o.startImport()
	case "space":
		switch focused {
		case importerOptHeader:
			o.noHeader = !o.noHeader
		case importerOptInfer:
			o.inferIdx = (o.inferIdx + 1) % len(importerInferModes)
		case importerOptSep:
			o.sep.key(msg)
		}
		return o, nil
	case "left":
		switch focused {
		case importerOptSep:
			if o.sep.cursor > 0 {
				o.sep.key(msg)
				return o, nil
			}
		case importerOptInfer:
			o.inferIdx = (o.inferIdx + len(importerInferModes) - 1) % len(importerInferModes)
			return o, nil
		}
		o.errline = ""
		o.step = importerStepFormat // back
		return o, nil
	case "right":
		switch focused {
		case importerOptSep:
			o.sep.key(msg)
		case importerOptInfer:
			o.inferIdx = (o.inferIdx + 1) % len(importerInferModes)
		}
		return o, nil
	}
	if focused == importerOptSep && o.sep.key(msg) {
		o.errline = ""
	}
	return o, nil
}

// optionRows lists the option rows visible for the chosen format; the
// separator input only applies to generic dsv.
func (o *importer) optionRows() []int {
	if o.chosen == reader.FormatDSV {
		return []int{importerOptHeader, importerOptSep, importerOptInfer}
	}
	return []int{importerOptHeader, importerOptInfer}
}

// startImport validates options and kicks off the async load.
func (o *importer) startImport() (ui.Overlay, tea.Cmd) {
	opt := reader.DefaultOptions()
	opt.Format = o.chosen
	opt.NoHeader = o.noHeader
	opt.InferSchema = importerInferModes[o.inferIdx]
	if o.chosen == reader.FormatDSV {
		sep, err := importerParseSep(o.sep.text)
		if err != nil {
			o.errline = err.Error()
			return o, nil
		}
		opt.Separator = sep
	}
	path := strings.TrimSpace(o.path.text)
	format := o.chosen
	o.loading = true
	o.errline = ""
	return o, func() tea.Msg {
		m := importerLoad(path, format, opt)
		m.owner = o
		return m
	}
}

// importerLoad resolves the source (file or URL) and reads all frames from
// it. It runs inside a tea.Cmd, off the update loop.
func importerLoad(path string, format reader.Format, opt reader.Options) importerResultMsg {
	var (
		src *reader.Source
		err error
	)
	if reader.IsURL(path) {
		src, err = reader.FromURL(path)
	} else {
		src, err = reader.FromFile(path)
	}
	if err != nil {
		return importerResultMsg{err: err}
	}
	defer src.Cleanup() // readers fully materialize frames, so the spool file can go
	rd, err := reader.For(format)
	if err != nil {
		return importerResultMsg{err: err}
	}
	frames, err := rd.Read(src, opt)
	if err != nil {
		return importerResultMsg{err: err}
	}
	return importerResultMsg{frames: frames}
}

// importerDetect guesses a format from a path or URL; reader.Detect already
// ignores any URL query string or fragment.
func importerDetect(path string) reader.Format {
	return reader.Detect(path)
}

// importerParseSep parses the separator input: empty means comma, "\t" or
// "tab" mean a tab, otherwise exactly one character is required.
func importerParseSep(s string) (rune, error) {
	switch s {
	case "":
		return ',', nil
	case `\t`, "tab":
		return '\t', nil
	}
	r := []rune(s)
	if len(r) != 1 {
		return 0, fmt.Errorf("separator must be a single character (or \\t)")
	}
	return r[0], nil
}

// --- tab completion -----------------------------------------------------------

// cycleCompletion computes filename matches on first tab and cycles through
// them on subsequent tabs.
func (o *importer) cycleCompletion() {
	if len(o.matches) > 0 {
		o.matchIdx = (o.matchIdx + 1) % len(o.matches)
		o.path.set(o.matches[o.matchIdx])
		return
	}
	o.matches = importerComplete(o.path.text)
	o.matchIdx = 0
	if len(o.matches) > 0 {
		o.path.set(o.matches[0])
	}
	if len(o.matches) == 1 {
		o.matches = nil // fully completed; next tab recomputes (descends into dirs)
	}
}

// importerComplete lists directory entries matching the typed prefix. The
// returned candidates are full paths (directory part preserved); directories
// get a trailing separator. Dotfiles are hidden unless the prefix asks for
// them. URLs complete to nothing.
func importerComplete(text string) []string {
	if reader.IsURL(text) {
		return nil
	}
	dir, prefix := filepath.Split(text)
	searchDir := dir
	if searchDir == "" {
		searchDir = "."
	}
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if !strings.HasPrefix(prefix, ".") && strings.HasPrefix(name, ".") {
			continue
		}
		full := dir + name
		if e.IsDir() {
			full += string(filepath.Separator)
		}
		out = append(out, full)
	}
	sort.Strings(out)
	return out
}

// --- view ---------------------------------------------------------------------

func (o *importer) View(width, height int, th *theme.Theme) string {
	w := min(70, width-4)
	if w < 20 {
		w = max(width, 20)
	}
	var lines []string
	var title, hint string
	switch o.step {
	case importerStepPath:
		title = "import — path (1/3)"
		hint = "enter: next   tab: complete   esc: close"
		lines = o.viewPath(th)
	case importerStepFormat:
		title = "import — format (2/3)"
		hint = "enter: next   esc: back"
		lines = o.viewFormat(th)
	default:
		title = "import — options (3/3)"
		hint = "enter: import   space: toggle   esc: back"
		lines = o.viewOptions(th)
	}

	if o.loading {
		lines = append(lines, "", th.Subtle.Render(" importing …"))
	} else if o.errline != "" {
		lines = append(lines, "", th.Error.Render(" "+o.errline))
	}
	lines = append(lines, "", th.Subtle.Render(" "+hint))
	return ui.Box(title, strings.Join(lines, "\n"), w, th)
}

func (o *importer) viewPath(th *theme.Theme) []string {
	lines := []string{
		th.Text.Render(" file path or http(s) URL:"),
		" " + th.Subtle.Render("> ") + o.path.render(!o.loading, th),
	}
	if len(o.matches) > 0 {
		show := o.matches
		off := 0
		const maxShow = 4
		if len(show) > maxShow {
			off = importerWindow(o.matchIdx, len(show), maxShow)
			show = show[off : off+maxShow]
		}
		for i, m := range show {
			label := " " + filepath.Base(strings.TrimSuffix(m, string(filepath.Separator)))
			if strings.HasSuffix(m, string(filepath.Separator)) {
				label += string(filepath.Separator)
			}
			if off+i == o.matchIdx {
				lines = append(lines, "   "+th.ListSelected.Render(label))
			} else {
				lines = append(lines, "   "+th.ListItem.Render(label))
			}
		}
		if rest := len(o.matches) - off - len(show); rest > 0 {
			lines = append(lines, th.Subtle.Render(fmt.Sprintf("   … %d more", rest)))
		}
	}
	return lines
}

func (o *importer) viewFormat(th *theme.Theme) []string {
	lines := []string{th.Text.Render(" format:")}
	const maxShow = 8
	total := len(o.formats)
	o.formatOff = importerScroll(o.formatOff, o.formatIdx, total, maxShow)
	end := min(o.formatOff+maxShow, total)
	for i := o.formatOff; i < end; i++ {
		name := o.formats[i]
		if name == importerAuto {
			if f := importerDetect(strings.TrimSpace(o.path.text)); f != "" {
				name = fmt.Sprintf("auto (%s)", f)
			}
		}
		if i == o.formatIdx {
			lines = append(lines, "  "+th.ListSelected.Render(" "+name+" "))
		} else {
			lines = append(lines, "  "+th.ListItem.Render(" "+name+" "))
		}
	}
	if total > maxShow {
		lines = append(lines, th.Subtle.Render(fmt.Sprintf("  %d/%d", o.formatIdx+1, total)))
	}
	return lines
}

func (o *importer) viewOptions(th *theme.Theme) []string {
	rows := o.optionRows()
	focused := rows[o.optFocus]
	lines := []string{th.Text.Render(fmt.Sprintf(" options (%s):", o.chosen))}

	mark := func(row int, label string) string {
		if row == focused {
			return " " + th.ListSelected.Render("▸ "+label)
		}
		return " " + th.ListItem.Render("  "+label)
	}

	box := "[ ]"
	if o.noHeader {
		box = "[x]"
	}
	lines = append(lines, mark(importerOptHeader, "no header    "+box))

	for _, row := range rows {
		if row != importerOptSep {
			continue
		}
		prefix := "  separator    "
		if focused == importerOptSep {
			prefix = "▸ separator    "
			lines = append(lines, " "+th.ListItem.Render(prefix)+o.sep.render(true, th))
		} else {
			lines = append(lines, " "+th.ListItem.Render(prefix+o.sep.text))
		}
	}

	var picks []string
	for i, m := range importerInferModes {
		s := " " + string(m) + " "
		if i == o.inferIdx {
			picks = append(picks, th.ListSelected.Render(s))
		} else {
			picks = append(picks, th.Subtle.Render(s))
		}
	}
	inferLabel := "  infer schema "
	if focused == importerOptInfer {
		inferLabel = "▸ infer schema "
	}
	lines = append(lines, " "+th.ListItem.Render(inferLabel)+strings.Join(picks, ""))
	return lines
}

// importerWindow returns the start offset of a size-long window over total
// items that keeps sel visible.
func importerWindow(sel, total, size int) int {
	if total <= size {
		return 0
	}
	off := sel - size/2
	if off < 0 {
		off = 0
	}
	if off > total-size {
		off = total - size
	}
	return off
}

// importerScroll adjusts a scroll offset just enough to keep sel visible.
func importerScroll(off, sel, total, size int) int {
	if total <= size {
		return 0
	}
	if sel < off {
		off = sel
	}
	if sel >= off+size {
		off = sel - size + 1
	}
	if off > total-size {
		off = total - size
	}
	if off < 0 {
		off = 0
	}
	return off
}

// --- line input ----------------------------------------------------------------

// importerInput is a minimal single-line text input (text + cursor over runes).
type importerInput struct {
	text   string
	cursor int // rune index, 0..len(runes)
}

func (in *importerInput) set(s string) {
	in.text = s
	in.cursor = len([]rune(s))
}

// key applies one key press; it reports whether the input consumed the key.
func (in *importerInput) key(msg tea.KeyPressMsg) bool {
	r := []rune(in.text)
	switch msg.String() {
	case "backspace", "ctrl+h":
		if in.cursor > 0 {
			in.text = string(r[:in.cursor-1]) + string(r[in.cursor:])
			in.cursor--
		}
		return true
	case "delete":
		if in.cursor < len(r) {
			in.text = string(r[:in.cursor]) + string(r[in.cursor+1:])
		}
		return true
	case "left":
		if in.cursor > 0 {
			in.cursor--
		}
		return true
	case "right":
		if in.cursor < len(r) {
			in.cursor++
		}
		return true
	case "home", "ctrl+a":
		in.cursor = 0
		return true
	case "end", "ctrl+e":
		in.cursor = len(r)
		return true
	case "ctrl+u":
		in.text = string(r[in.cursor:])
		in.cursor = 0
		return true
	}
	if msg.Text != "" && msg.Mod == 0 {
		in.text = string(r[:in.cursor]) + msg.Text + string(r[in.cursor:])
		in.cursor += len([]rune(msg.Text))
		return true
	}
	return false
}

// paste inserts pasted text at the cursor as a single edit, flattened to one
// line. It reports whether the text changed.
func (in *importerInput) paste(s string) bool {
	t := pasteSanitize(s)
	if t == "" {
		return false
	}
	r := []rune(in.text)
	in.text = string(r[:in.cursor]) + t + string(r[in.cursor:])
	in.cursor += len([]rune(t))
	return true
}

// render draws the input with a block cursor when focused.
func (in *importerInput) render(focused bool, th *theme.Theme) string {
	if !focused {
		return th.Input.Render(in.text)
	}
	r := []rune(in.text)
	before := string(r[:in.cursor])
	cur, after := " ", ""
	if in.cursor < len(r) {
		cur = string(r[in.cursor])
		after = string(r[in.cursor+1:])
	}
	return th.Input.Render(before) + th.ListSelected.Render(cur) + th.Input.Render(after)
}
