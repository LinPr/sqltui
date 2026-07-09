package dbmode

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/config"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/db/mysqlbe"
	"github.com/LinPr/sqltui/internal/db/postgresbe"
	"github.com/LinPr/sqltui/internal/db/redisbe"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// Field keys shared by field construction, dialing and config saving.
const (
	fkUser     = "user"
	fkPassword = "password"
	fkHost     = "host"
	fkPort     = "port"
	fkDatabase = "database"
	fkSslMode  = "sslmode"
	fkDbNum    = "dbnum"
)

// connField is one editable line of the connection form.
type connField struct {
	key    string
	label  string
	masked bool
	value  []rune
}

// connDoneMsg reports the outcome of an asynchronous connection attempt.
// Exactly one of be/kv is non-nil on success.
type connDoneMsg struct {
	kind string
	vals map[string]string
	be   db.Backend
	kv   db.KVBackend
	err  error
}

// connForm is the connection form overlay, parameterized by engine kind.
// Fields prefill from the saved config; ctrl+s saves, enter (or ctrl+r)
// connects asynchronously from any field.
type connForm struct {
	ctx        ui.AppContext
	kind       string
	fields     []connField
	focus      int
	errText    string
	connecting bool
}

func newConnForm(ctx ui.AppContext, kind string) *connForm {
	return &connForm{ctx: ctx, kind: kind, fields: loadFields(kind)}
}

// loadFields builds the field list for one kind, prefilled from the saved
// per-engine config (best effort: a missing/corrupt config yields empty
// prefills, the readers themselves provide sensible defaults).
func loadFields(kind string) []connField {
	f := func(key, label, val string, masked bool) connField {
		return connField{key: key, label: label, masked: masked, value: []rune(val)}
	}
	switch kind {
	case KindMysql:
		c, err := config.ReadMySqlConfig()
		if err != nil {
			c = &config.MysqlConfig{}
		}
		return []connField{
			f(fkUser, "user", c.UserName, false),
			f(fkPassword, "password", c.Password, true),
			f(fkHost, "host", c.Host, false),
			f(fkPort, "port", c.Port, false),
			f(fkDatabase, "database", c.DbName, false),
		}
	case KindPostgres:
		c, err := config.ReadPostgresConfig()
		if err != nil {
			c = &config.PostgresConfig{}
		}
		return []connField{
			f(fkUser, "user", c.UserName, false),
			f(fkPassword, "password", c.Password, true),
			f(fkHost, "host", c.Host, false),
			f(fkPort, "port", c.Port, false),
			f(fkDatabase, "database", c.DbName, false),
			f(fkSslMode, "sslmode", c.SslMode, false),
		}
	case KindRedis:
		c, err := config.ReadRedisConfig()
		if err != nil {
			c = &config.RedisConfig{}
		}
		return []connField{
			f(fkUser, "user", c.UserName, false),
			f(fkPassword, "password", c.Password, true),
			f(fkHost, "host", c.Host, false),
			f(fkPort, "port", c.Port, false),
			f(fkDbNum, "db number", c.RdbNum, false),
		}
	}
	return nil
}

// values snapshots the current field contents keyed by field key. Masked
// fields (passwords) are passed through verbatim: credentials may
// legitimately contain leading/trailing whitespace.
func (o *connForm) values() map[string]string {
	m := make(map[string]string, len(o.fields))
	for _, fl := range o.fields {
		s := string(fl.value)
		if !fl.masked {
			s = strings.TrimSpace(s)
		}
		m[fl.key] = s
	}
	return m
}

// dialer opens connections for startConnect; a package variable so tests can
// swap in a fake connector and drive the full async connect flow without a
// live server.
var dialer = dial

// dial opens the connection for one kind. Exactly one of the returned
// backends is non-nil on success; the connection is tracked for shutdown.
func dial(kind string, v map[string]string) (db.Backend, db.KVBackend, error) {
	switch kind {
	case KindMysql:
		be, err := mysqlbe.Connect(mysqlbe.Config{
			UserName: v[fkUser], Password: v[fkPassword],
			Host: v[fkHost], Port: v[fkPort], DbName: v[fkDatabase],
		})
		if err != nil {
			return nil, nil, err
		}
		return be, nil, nil
	case KindPostgres:
		be, err := postgresbe.Connect(postgresbe.Config{
			UserName: v[fkUser], Password: v[fkPassword],
			Host: v[fkHost], Port: v[fkPort], DbName: v[fkDatabase],
			SslMode: v[fkSslMode],
		})
		if err != nil {
			return nil, nil, err
		}
		return be, nil, nil
	case KindRedis:
		kv, err := redisbe.Connect(redisbe.Config{
			UserName: v[fkUser], Password: v[fkPassword],
			Host: v[fkHost], Port: v[fkPort], RdbNum: v[fkDbNum],
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, kv, nil
	}
	return nil, nil, fmt.Errorf("unknown database kind: %s", kind)
}

// saveConfig persists the form values with the existing per-engine writers.
func saveConfig(kind string, v map[string]string) error {
	switch kind {
	case KindMysql:
		return config.WriteMysqlConfig(&config.MysqlConfig{
			UserName: v[fkUser], Password: v[fkPassword],
			Host: v[fkHost], Port: v[fkPort], DbName: v[fkDatabase],
		})
	case KindPostgres:
		return config.WritePostgresConfig(&config.PostgresConfig{
			UserName: v[fkUser], Password: v[fkPassword],
			Host: v[fkHost], Port: v[fkPort], DbName: v[fkDatabase],
			SslMode: v[fkSslMode],
		})
	case KindRedis:
		return config.WriteRedisConfig(&config.RedisConfig{
			UserName: v[fkUser], Password: v[fkPassword],
			Host: v[fkHost], Port: v[fkPort], RdbNum: v[fkDbNum],
		})
	}
	return fmt.Errorf("unknown database kind: %s", kind)
}

// Fullscreen marks the form as a full page (ui.FullscreenOverlay): a
// centered box over a fully blanked body. It is the root page of the
// database-mode page stack (login -> browser -> table).
func (o *connForm) Fullscreen() bool { return true }

// startConnect flips into the connecting state and returns the asynchronous
// dial command. A second request while one is in flight is ignored.
func (o *connForm) startConnect() tea.Cmd {
	if o.connecting {
		return nil
	}
	o.connecting = true
	o.errText = ""
	kind, vals := o.kind, o.values()
	return func() tea.Msg {
		be, kv, err := dialer(kind, vals)
		if err == nil {
			if be != nil {
				conns.track(be)
			}
			if kv != nil {
				conns.track(kv)
			}
		}
		return connDoneMsg{kind: kind, vals: vals, be: be, kv: kv, err: err}
	}
}

// successCmds is the ordered message sequence after a successful connect:
// close the form, attach the connection to the app, open the browser
// (schema for SQL engines, key browser via the wrapped schema factory for
// redis) and toast the connection title.
func (o *connForm) successCmds(m connDoneMsg) []tea.Cmd {
	title := ""
	if m.be != nil {
		title = m.be.Title()
	}
	if m.kv != nil {
		title = m.kv.Title()
	}
	be, kv := m.be, m.kv
	return []tea.Cmd{
		ui.CloseOverlay,
		func() tea.Msg { return ui.SetBackendMsg{Backend: be, KV: kv} },
		func() tea.Msg { return ui.RunCommandMsg{Name: "schema"} },
		func() tea.Msg { return ui.ToastMsg{Text: "connected: " + title} },
	}
}

func (o *connForm) Update(msg tea.Msg) (ui.Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case connDoneMsg:
		o.connecting = false
		if m.err != nil {
			o.errText = m.err.Error()
			return o, nil
		}
		_ = saveConfig(m.kind, m.vals) // legacy behavior: persist on success
		return o, tea.Sequence(o.successCmds(m)...)

	case tea.PasteMsg:
		o.insertText(m.Content)
		return o, nil

	case tea.KeyPressMsg:
		return o.handleKey(m)
	}
	return o, nil
}

// insertText appends printable text to the focused field. Newlines (and other
// control characters) are stripped so multi-line pastes land as one value.
func (o *connForm) insertText(s string) {
	if len(o.fields) == 0 {
		return
	}
	fl := &o.fields[o.focus]
	for _, r := range s {
		if r >= ' ' && r != 0x7f {
			fl.value = append(fl.value, r)
		}
	}
}

func (o *connForm) handleKey(key tea.KeyPressMsg) (ui.Overlay, tea.Cmd) {
	switch key.String() {
	case "esc":
		// While a dial is in flight the form must stay up: dismissing it
		// would drop the connDoneMsg (the freshly dialed backend replaces —
		// and closes — the one the app is still using), and reopening the
		// form could start a second concurrent dial.
		if o.connecting {
			return o, nil
		}
		// The form is the root page of the database-mode page stack: esc
		// always quits, connected or not.
		return o, tea.Quit
	case "ctrl+c":
		return o, tea.Quit
	case "tab", "down":
		o.focus = (o.focus + 1) % len(o.fields)
		return o, nil
	case "shift+tab", "up":
		o.focus = (o.focus - 1 + len(o.fields)) % len(o.fields)
		return o, nil
	case "enter", "ctrl+r":
		return o, o.startConnect()
	case "ctrl+s":
		if err := saveConfig(o.kind, o.values()); err != nil {
			o.errText = "save config: " + err.Error()
			return o, nil
		}
		return o, func() tea.Msg { return ui.ToastMsg{Text: "config saved"} }
	case "backspace":
		fl := &o.fields[o.focus]
		if n := len(fl.value); n > 0 {
			fl.value = fl.value[:n-1]
		}
		return o, nil
	case "ctrl+u":
		o.fields[o.focus].value = nil
		return o, nil
	}
	if t := key.Text; t != "" {
		o.insertText(t)
	}
	return o, nil
}

func (o *connForm) View(width, height int, th *theme.Theme) string {
	w := width - 4
	if w > 60 {
		w = 60
	}
	if w < 30 {
		w = 30
	}
	inner := w - 2

	labelW := 0
	for _, fl := range o.fields {
		if n := len(fl.label); n > labelW {
			labelW = n
		}
	}

	var lines []string
	for i, fl := range o.fields {
		val := string(fl.value)
		if fl.masked {
			val = strings.Repeat("*", len(fl.value))
		}
		label := fmt.Sprintf(" %-*s ", labelW, fl.label)
		room := inner - ansi.StringWidth(label) - 1
		if room < 1 {
			room = 1
		}
		// Keep the tail visible while typing long values.
		if r := []rune(val); len(r) > room {
			val = "…" + string(r[len(r)-room+1:])
		}
		if i == o.focus {
			cur := th.ListSelected.Render(" ")
			lines = append(lines, th.Text.Render(label)+th.Input.Render(val)+cur)
		} else {
			lines = append(lines, th.Subtle.Render(label)+th.Text.Render(val))
		}
	}

	lines = append(lines, "")
	switch {
	case o.connecting:
		lines = append(lines, th.Warning.Render(" connecting..."))
	case o.errText != "":
		for _, l := range strings.Split(ansi.Wrap(o.errText, inner-2, ""), "\n") {
			lines = append(lines, th.Error.Render(" "+l))
		}
	default:
		lines = append(lines, th.Placeholder.Render(" fill in the connection parameters"))
	}

	lines = append(lines, th.Placeholder.Render(" enter connect · tab/↑↓ move · ctrl+s save · esc quit"))

	return ui.Box("connect "+o.kind, strings.Join(lines, "\n"), w, th)
}

// compile-time interface checks
var (
	_ ui.Overlay           = (*connForm)(nil)
	_ ui.FullscreenOverlay = (*connForm)(nil)
)
