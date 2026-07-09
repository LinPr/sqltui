package popup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
	"github.com/LinPr/sqltui/internal/writer"
)

func init() {
	ui.Factories["edit"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		frame := ctx.CurrentFrame()
		if frame == nil {
			return nil, errors.New("no data to edit")
		}

		tmp, err := os.CreateTemp("", "sqltui-edit-*.csv")
		if err != nil {
			return nil, fmt.Errorf("create temp file: %w", err)
		}
		cw, err := writer.For(writer.FormatCSV)
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return nil, err
		}
		werr := cw.Write(tmp, frame, writer.DefaultOptions())
		if cerr := tmp.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(tmp.Name())
			return nil, fmt.Errorf("write temp file: %w", werr)
		}

		return &editRunner{cmd: editEditorCmd(tmp.Name()), tmp: tmp.Name(), paneID: ctx.ActivePaneID()}, nil
	}
}

// editEditorCmd builds the editor invocation for the temp file, honoring a
// multi-word $EDITOR ("code -w") and falling back to vi.
func editEditorCmd(path string) *exec.Cmd {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	return exec.Command(parts[0], append(parts[1:], path)...)
}

// editRunner is an invisible auto-run overlay: factories cannot dispatch
// commands, so it implements ui.OverlayIniter and emits the ExecProcessMsg
// (batched with its own close) the moment it is pushed. Update stays as a
// fallback fire path for a message that races in before the close.
type editRunner struct {
	cmd     *exec.Cmd
	tmp     string
	paneID  int
	started bool
}

// Init fires the editor process as soon as the overlay is pushed.
func (e *editRunner) Init() tea.Cmd {
	if e.started {
		return nil
	}
	e.started = true
	tmp, paneID := e.tmp, e.paneID
	exe := ui.ExecProcessMsg{
		Cmd:    e.cmd,
		OnDone: func(err error) tea.Msg { return editReload(tmp, paneID, err) },
	}
	return tea.Batch(
		ui.CloseOverlay,
		func() tea.Msg { return exe },
	)
}

func (e *editRunner) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	return e, e.Init()
}

func (e *editRunner) View(width, height int, th *theme.Theme) string { return "" }

var _ ui.OverlayIniter = (*editRunner)(nil)

// editReload re-reads the edited temp CSV and turns it into the follow-up
// app message: ApplyFrameMsg (routed to the originating pane) on success,
// ErrorMsg otherwise.
func editReload(tmp string, paneID int, procErr error) tea.Msg {
	defer os.Remove(tmp)
	if procErr != nil {
		return ui.ErrorMsg{Err: fmt.Errorf("editor failed: %w", procErr)}
	}
	r, err := reader.For(reader.FormatCSV)
	if err != nil {
		return ui.ErrorMsg{Err: err}
	}
	src, err := reader.FromFile(tmp)
	if err != nil {
		return ui.ErrorMsg{Err: fmt.Errorf("reload edited data: %w", err)}
	}
	opt := reader.DefaultOptions()
	opt.Format = reader.FormatCSV
	frames, err := r.Read(src, opt)
	if err != nil {
		return ui.ErrorMsg{Err: fmt.Errorf("reload edited data: %w", err)}
	}
	if len(frames) == 0 || frames[0].Frame == nil {
		return ui.ErrorMsg{Err: errors.New("edited file produced no data")}
	}
	return ui.ApplyFrameMsg{Frame: frames[0].Frame, Crumb: "edit", PaneID: paneID}
}
