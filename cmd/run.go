package cmd

import (
	"fmt"
	"strings"

	"github.com/LinPr/sqltui/internal/config"
	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/reader"
	"github.com/LinPr/sqltui/internal/ui"
	_ "github.com/LinPr/sqltui/internal/ui/popup" // register overlay factories
)

// openSource resolves one CLI argument into a reader source.
func openSource(arg string) (*reader.Source, error) {
	switch {
	case arg == "-":
		return reader.FromStdin()
	case reader.IsURL(arg):
		return reader.FromURL(arg)
	default:
		return reader.FromFile(arg)
	}
}

// detectFormat picks the format for a source: explicit flag first, then
// file-name detection.
func detectFormat(src *reader.Source, opt reader.Options) (reader.Format, error) {
	if opt.Format != "" {
		return opt.Format, nil
	}
	if f := reader.Detect(src.Path); f != "" {
		return f, nil
	}
	return "", fmt.Errorf("cannot detect format of %q; use --format (supported: %s)",
		src.Path, strings.Join(reader.SupportedFormats(), ", "))
}

// readArg loads all frames from one CLI argument.
func readArg(arg string, opt reader.Options) ([]reader.NamedFrame, error) {
	src, err := openSource(arg)
	if err != nil {
		return nil, err
	}
	defer src.Cleanup() // readers fully materialize frames, so the spool file can go
	format, err := detectFormat(src, opt)
	if err != nil {
		return nil, err
	}
	r, err := reader.For(format)
	if err != nil {
		return nil, err
	}
	frames, err := r.Read(src, opt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", arg, err)
	}
	return frames, nil
}

// runFileMode loads the given files and starts the viewer UI.
func runFileMode(paths []string) error {
	if len(paths) == 0 && len(flags.multiparts) == 0 {
		return fmt.Errorf("no input files; see --help")
	}

	opt, err := readerOptions()
	if err != nil {
		return err
	}

	var frames []reader.NamedFrame
	for _, p := range paths {
		nfs, err := readArg(p, opt)
		if err != nil {
			return err
		}
		frames = append(frames, nfs...)
	}

	// --multiparts: concatenate all parts vertically into one frame named
	// after the first part.
	if len(flags.multiparts) > 0 {
		var parts []*data.Frame
		name := ""
		for _, p := range flags.multiparts {
			nfs, err := readArg(p, opt)
			if err != nil {
				return err
			}
			for _, nf := range nfs {
				if name == "" {
					name = nf.Name
				}
				parts = append(parts, nf.Frame)
			}
		}
		if len(parts) == 0 {
			return fmt.Errorf("--multiparts: no tables loaded")
		}
		merged, err := data.Concat(parts...)
		if err != nil {
			return fmt.Errorf("--multiparts: %w", err)
		}
		frames = append(frames, reader.NamedFrame{Name: name, Frame: merged})
	}

	if len(frames) == 0 {
		return fmt.Errorf("no tables loaded")
	}

	engine, err := query.NewEngine()
	if err != nil {
		return err
	}
	defer engine.Close()
	for _, nf := range frames {
		if err := engine.Register(nf.Name, nf.Frame); err != nil {
			return err
		}
	}
	// The active frame is always additionally reachable as "_"; the app
	// re-registers it whenever the active pane or stack changes.
	if err := engine.Register("_", frames[0].Frame); err != nil {
		return err
	}

	uiConf, err := config.ReadUIConfig()
	if err != nil {
		uiConf = config.DefaultUIConfig()
	}

	app := ui.New(ui.Options{
		Frames:         frames,
		Engine:         engine,
		ThemeName:      uiConf.Theme,
		ShowBorders:    uiConf.ShowBorders,
		ShowRowNumbers: uiConf.ShowRowNumbers,
	})
	return ui.Run(app)
}
