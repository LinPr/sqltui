// Package dbmode wires the live-database mode into the UI: the mysql /
// postgres / redis subcommands start an empty workspace with a connection
// form overlay on top, then browse schemas, tables and keys through the
// shared overlay system.
package dbmode

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/LinPr/sqltui/internal/config"
	"github.com/LinPr/sqltui/internal/ui"
	"github.com/LinPr/sqltui/internal/ui/popup" // register the base overlay factories
)

// Engine kinds accepted by Run.
const (
	KindMysql    = "mysql"
	KindPostgres = "postgres"
	KindRedis    = "redis"
)

// Run starts the UI in database mode for one engine kind: no tabs, no
// embedded engine, a connection form as the initial overlay. It returns when
// the UI exits and closes any connection made during the session.
func Run(kind string) error {
	switch kind {
	case KindMysql, KindPostgres, KindRedis:
	default:
		return fmt.Errorf("unknown database kind: %s", kind)
	}

	uiConf, err := config.ReadUIConfig()
	if err != nil {
		uiConf = config.DefaultUIConfig()
	}

	RegisterFactories(kind)

	app := ui.New(ui.Options{
		ThemeName:      uiConf.Theme,
		ShowBorders:    uiConf.ShowBorders,
		ShowRowNumbers: uiConf.ShowRowNumbers,
	})
	app.PushOverlay(newConnForm(app, kind))

	runErr := ui.Run(app)
	conns.closeAll()
	return runErr
}

// --- factory registration ------------------------------------------------------

var (
	registerOnce sync.Once

	// Pristine factories captured before any wrapping, so repeated
	// registration can never nest wrappers.
	origQuery  func(ui.AppContext, string) (ui.Overlay, error)
	origSchema func(ui.AppContext, string) (ui.Overlay, error)
)

// RegisterFactories installs the database-mode overlay factories:
//
//   - "opentable": load one live table into a new tab (ns "\t" table arg,
//     matching the schema browser's dispatch encoding),
//   - "connect": reopen the connection form for the current kind,
//   - redis only: "query" and "schema" are wrapped so that, with a live
//     key-value connection, they open the raw-command prompt and the key
//     browser instead; without one they delegate to the originals.
func RegisterFactories(kind string) {
	registerOnce.Do(func() {
		origQuery = ui.Factories["query"]
		origSchema = ui.Factories["schema"]
	})

	ui.Factories["opentable"] = openTableFactory
	ui.Factories["connect"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		return newConnForm(ctx, kind), nil
	}
	addPaletteCommand("connect", "open the database connection form")

	if kind != KindRedis {
		return
	}
	ui.Factories["query"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		if ctx.KV() != nil {
			return newRedisPrompt(ctx, arg), nil
		}
		if origQuery != nil {
			return origQuery(ctx, arg)
		}
		return nil, fmt.Errorf("query: no connection")
	}
	ui.Factories["schema"] = func(ctx ui.AppContext, arg string) (ui.Overlay, error) {
		if kv := ctx.KV(); kv != nil {
			return newKeyBrowser(kv, len(ctx.Tabs())), nil
		}
		if origSchema != nil {
			return origSchema(ctx, arg)
		}
		return nil, fmt.Errorf("schema: no connection")
	}
}

// addPaletteCommand lists a command in the palette table (once). Dispatch by
// exact name works regardless; this only makes the command visible.
func addPaletteCommand(name, desc string) {
	for _, c := range popup.Commands {
		if c.Name == name {
			return
		}
	}
	popup.Commands = append(popup.Commands, popup.Command{Name: name, Description: desc})
}

// --- connection tracking --------------------------------------------------------

// conns remembers every live connection opened by the connection form so Run
// can close them when the UI exits.
var conns = &connTracker{}

type connTracker struct {
	mu   sync.Mutex
	open []io.Closer
}

func (t *connTracker) track(c io.Closer) {
	if c == nil {
		return
	}
	t.mu.Lock()
	t.open = append(t.open, c)
	t.mu.Unlock()
}

func (t *connTracker) closeAll() {
	t.mu.Lock()
	open := t.open
	t.open = nil
	t.mu.Unlock()
	for _, c := range open {
		_ = c.Close() // best effort on shutdown
	}
}

// splitTableArg decodes the "ns\ttable" argument produced by the schema
// browser; a plain table name (no tab) is accepted with an empty namespace.
func splitTableArg(arg string) (ns, table string) {
	if i := strings.IndexByte(arg, '\t'); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return "", arg
}
