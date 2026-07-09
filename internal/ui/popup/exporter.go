// Package popup implements the modal overlays (palette, wizards, prompts)
// that plug into the app through the ui.Factories registry.
package popup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
	"github.com/LinPr/sqltui/internal/writer"
)

func init() {
	ui.Factories["export"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		if ctx.CurrentFrame() == nil {
			return nil, errors.New("no data to export")
		}
		formats := writer.SupportedFormats()
		if len(formats) == 0 {
			return nil, errors.New("no export formats registered")
		}
		e := newExporter(ctx, formats)
		if arg != "" {
			for i, f := range formats {
				if f == arg {
					e.fmtSel = i
					e.enterOptionsOrPath()
					break
				}
			}
		}
		return e, nil
	}
}

// exportStep enumerates the wizard pages.
type exportStep int

const (
	exportStepFormat exportStep = iota
	exportStepOptions
	exportStepPath
	exportStepConfirm
)

var exportCompressions = []string{"none", "snappy", "gzip", "zstd", "lz4", "brotli"}

// exporter is a multi-step export wizard: format -> options -> path -> write.
type exporter struct {
	ctx     ui.AppContext
	formats []string
	step    exportStep

	fmtSel int // selected format index

	// format-specific options
	optSel  int  // focused option row (csv/tsv)
	sep     rune // csv separator
	header  bool // csv/tsv: include header row
	pretty  bool // json: indent output
	compSel int  // parquet compression index

	// destination path input
	path    []rune
	cursor  int
	confirm string // absolute path pending overwrite confirmation

	errText string // inline validation hint
}

func newExporter(ctx ui.AppContext, formats []string) *exporter {
	e := &exporter{
		ctx:     ctx,
		formats: formats,
		sep:     ',',
		header:  true,
		compSel: 1, // snappy, the writer default
	}
	return e
}

func (e *exporter) format() string { return e.formats[e.fmtSel] }

// exportExt maps a format name to its conventional file extension.
func exportExt(format string) string {
	if format == "markdown" {
		return "md"
	}
	return format
}

// exportHasOptions reports whether a format gets an option page.
func exportHasOptions(format string) bool {
	switch format {
	case "csv", "tsv", "json", "parquet":
		return true
	}
	return false
}

// exportExpandPath expands a leading ~ to the user's home directory.
func exportExpandPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func (e *exporter) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	if pm, ok := msg.(tea.PasteMsg); ok {
		e.paste(pm.Content)
		return e, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return e, nil
	}
	return e, e.handleKey(key.String(), key.Text)
}

// paste routes bracketed-paste text to the focused input: the destination
// path on the path page, or the csv/tsv separator (single character) on the
// option page. Other pages ignore pastes.
func (e *exporter) paste(content string) {
	text := pasteSanitize(content)
	if text == "" {
		return
	}
	switch e.step {
	case exportStepPath:
		e.errText = ""
		r := []rune(text)
		e.path = append(e.path[:e.cursor], append(r, e.path[e.cursor:]...)...)
		e.cursor += len(r)
	case exportStepOptions:
		format := e.format()
		if (format == "csv" || format == "tsv") && e.optSel == 0 {
			if r := []rune(strings.TrimSpace(text)); len(r) == 1 {
				e.sep = r[0]
			}
		}
	}
}

// handleKey processes one key; key is the chord string, text the printable
// runes (empty for special keys). Split out from Update for testability.
func (e *exporter) handleKey(key, text string) tea.Cmd {
	e.errText = ""
	switch e.step {
	case exportStepFormat:
		return e.keyFormat(key)
	case exportStepOptions:
		return e.keyOptions(key, text)
	case exportStepPath:
		return e.keyPath(key, text)
	case exportStepConfirm:
		return e.keyConfirm(key)
	}
	return nil
}

func (e *exporter) keyFormat(key string) tea.Cmd {
	switch key {
	case "esc", "left", "q":
		return ui.CloseOverlay
	case "up", "k":
		if e.fmtSel > 0 {
			e.fmtSel--
		}
	case "down", "j":
		if e.fmtSel < len(e.formats)-1 {
			e.fmtSel++
		}
	case "home", "g":
		e.fmtSel = 0
	case "end", "G":
		e.fmtSel = len(e.formats) - 1
	case "enter":
		e.enterOptionsOrPath()
	}
	return nil
}

// enterOptionsOrPath advances from the format page to the option page when
// the format has one, otherwise straight to the path page.
func (e *exporter) enterOptionsOrPath() {
	e.optSel = 0
	if exportHasOptions(e.format()) {
		e.step = exportStepOptions
		return
	}
	e.enterPath()
}

// enterPath moves to the destination page, prefilling "<tab title>.<ext>".
func (e *exporter) enterPath() {
	e.step = exportStepPath
	if len(e.path) == 0 {
		base := e.ctx.BaseCrumb()
		if base == "" {
			base = "export"
		}
		e.path = []rune(base + "." + exportExt(e.format()))
	}
	e.cursor = len(e.path)
}

func (e *exporter) keyOptions(key, text string) tea.Cmd {
	switch key {
	case "esc", "left":
		e.step = exportStepFormat
		return nil
	case "enter":
		e.enterPath()
		return nil
	}
	switch e.format() {
	case "csv", "tsv":
		switch key {
		case "up", "shift+tab":
			e.optSel = 0
		case "down", "tab":
			e.optSel = 1
		default:
			if e.optSel == 1 { // header toggle row
				if key == "space" || key == "y" || key == "n" || key == "h" || key == "l" || key == "right" {
					e.header = !e.header
				}
			} else if r := []rune(text); len(r) == 1 { // separator row
				e.sep = r[0]
			}
		}
	case "json":
		if key == "space" || key == "y" || key == "n" || key == "right" {
			e.pretty = !e.pretty
		}
	case "parquet":
		switch key {
		case "up", "k":
			if e.compSel > 0 {
				e.compSel--
			}
		case "down", "j":
			if e.compSel < len(exportCompressions)-1 {
				e.compSel++
			}
		}
	}
	return nil
}

func (e *exporter) keyPath(key, text string) tea.Cmd {
	switch key {
	case "esc":
		if exportHasOptions(e.format()) {
			e.step = exportStepOptions
		} else {
			e.step = exportStepFormat
		}
		return nil
	case "enter":
		return e.submitPath()
	case "backspace":
		if e.cursor > 0 {
			e.path = append(e.path[:e.cursor-1], e.path[e.cursor:]...)
			e.cursor--
		}
	case "delete":
		if e.cursor < len(e.path) {
			e.path = append(e.path[:e.cursor], e.path[e.cursor+1:]...)
		}
	case "left":
		if e.cursor > 0 {
			e.cursor--
		}
	case "right":
		if e.cursor < len(e.path) {
			e.cursor++
		}
	case "home", "ctrl+a":
		e.cursor = 0
	case "end", "ctrl+e":
		e.cursor = len(e.path)
	case "ctrl+u":
		e.path = e.path[:0]
		e.cursor = 0
	default:
		if text != "" {
			r := []rune(text)
			e.path = append(e.path[:e.cursor], append(r, e.path[e.cursor:]...)...)
			e.cursor += len(r)
		}
	}
	return nil
}

// submitPath validates the destination and either asks for overwrite
// confirmation or writes immediately.
func (e *exporter) submitPath() tea.Cmd {
	p := strings.TrimSpace(string(e.path))
	if p == "" {
		e.errText = "destination path is required"
		return nil
	}
	dst := exportExpandPath(p)
	if info, err := os.Stat(dst); err == nil {
		if info.IsDir() {
			e.errText = fmt.Sprintf("%s is a directory", dst)
			return nil
		}
		e.confirm = dst
		e.step = exportStepConfirm
		return nil
	}
	return e.doWrite(dst)
}

func (e *exporter) keyConfirm(key string) tea.Cmd {
	switch key {
	case "y", "Y", "enter":
		dst := e.confirm
		e.confirm = ""
		return e.doWrite(dst)
	case "n", "N", "esc", "left":
		e.confirm = ""
		e.step = exportStepPath
	}
	return nil
}

// doWrite serializes the current frame to dst. On failure the wizard stays
// on the path page so the user can fix the destination.
func (e *exporter) doWrite(dst string) tea.Cmd {
	frame := e.ctx.CurrentFrame()
	if frame == nil {
		return tea.Batch(exportErrCmd(errors.New("no data to export")), ui.CloseOverlay)
	}
	w, err := writer.For(writer.Format(e.format()))
	if err != nil {
		e.step = exportStepFormat
		return exportErrCmd(err)
	}
	opt := writer.DefaultOptions()
	opt.Separator = e.sep
	opt.Header = e.header
	opt.Pretty = e.pretty
	opt.Compression = writer.Compression(exportCompressions[e.compSel])

	// Write to a temp file in the destination directory and rename into
	// place only on success, so a failed write (disk full, encoding error)
	// never destroys an existing file or leaves a partial one behind.
	mode := os.FileMode(0o644)
	if info, err := os.Stat(dst); err == nil {
		mode = info.Mode().Perm() // preserve the existing file's permissions
	}
	dir, base := filepath.Dir(dst), filepath.Base(dst)
	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		e.step = exportStepPath
		return exportErrCmd(err)
	}
	werr := w.Write(tmp, frame, opt)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), mode) // CreateTemp defaults to 0600
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), dst)
	}
	if werr != nil {
		os.Remove(tmp.Name())
		e.step = exportStepPath
		return exportErrCmd(fmt.Errorf("export failed: %w", werr))
	}
	return tea.Batch(
		func() tea.Msg { return ui.ToastMsg{Text: "exported to " + dst} },
		ui.CloseOverlay,
	)
}

func exportErrCmd(err error) tea.Cmd {
	return func() tea.Msg { return ui.ErrorMsg{Err: err} }
}

func (e *exporter) View(width, height int, th *theme.Theme) string {
	w := min(64, width-4)
	if w < 20 {
		w = 20
	}
	var lines []string
	title := "Export"
	switch e.step {
	case exportStepFormat:
		title = "Export — format"
		for i, f := range e.formats {
			if i == e.fmtSel {
				lines = append(lines, th.ListSelected.Render(" "+f+" "))
			} else {
				lines = append(lines, th.ListItem.Render("  "+f))
			}
		}
		lines = append(lines, "", th.Subtle.Render(" enter select · esc cancel"))
	case exportStepOptions:
		title = "Export — " + e.format() + " options"
		lines = e.viewOptions(th)
		lines = append(lines, "", th.Subtle.Render(" enter continue · esc back"))
	case exportStepPath:
		title = "Export — " + e.format() + " destination"
		lines = append(lines, th.Text.Render(" Path: ")+e.viewInput(th))
		if e.errText != "" {
			lines = append(lines, th.Error.Render(" "+e.errText))
		}
		lines = append(lines, "", th.Subtle.Render(" enter write · esc back"))
	case exportStepConfirm:
		title = "Export — overwrite?"
		lines = append(lines,
			th.Warning.Render(" "+e.confirm+" exists."),
			th.Text.Render(" Overwrite? (y/n)"),
		)
	}
	return ui.Box(title, strings.Join(lines, "\n"), w, th)
}

// viewOptions renders the format-specific option rows.
func (e *exporter) viewOptions(th *theme.Theme) []string {
	mark := func(on bool) string {
		if on {
			return "yes"
		}
		return "no"
	}
	row := func(focused bool, label, value string) string {
		if focused {
			return th.ListSelected.Render(" " + label + ": " + value + " ")
		}
		return th.ListItem.Render("  " + label + ": " + value)
	}
	switch e.format() {
	case "csv", "tsv":
		return []string{
			row(e.optSel == 0, "separator", string(e.sep)),
			row(e.optSel == 1, "header", mark(e.header)),
			th.Subtle.Render(" type to set separator · space toggles header"),
		}
	case "json":
		return []string{
			row(true, "pretty", mark(e.pretty)),
			th.Subtle.Render(" space toggles"),
		}
	case "parquet":
		out := make([]string, 0, len(exportCompressions))
		for i, c := range exportCompressions {
			if i == e.compSel {
				out = append(out, th.ListSelected.Render(" "+c+" "))
			} else {
				out = append(out, th.ListItem.Render("  "+c))
			}
		}
		return out
	}
	return nil
}

// viewInput renders the path line input with a block cursor.
func (e *exporter) viewInput(th *theme.Theme) string {
	before := string(e.path[:e.cursor])
	var at, after string
	if e.cursor < len(e.path) {
		at = string(e.path[e.cursor])
		after = string(e.path[e.cursor+1:])
	} else {
		at = " "
	}
	return th.Input.Render(before) + th.ListSelected.Render(at) + th.Input.Render(after)
}
