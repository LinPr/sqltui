package dbmode

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/config"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/ui"
)

// fakeDialer swaps the package dialer for the test's lifetime so the async
// connect flow can be driven without a live server.
func fakeDialer(t *testing.T, fn func(kind string, v map[string]string) (db.Backend, db.KVBackend, error)) {
	t.Helper()
	old := dialer
	dialer = fn
	t.Cleanup(func() { dialer = old })
}

func TestConnFormFieldsPerKind(t *testing.T) {
	useTempConfig(t)
	for _, tc := range []struct {
		kind string
		want []string
	}{
		{KindMysql, []string{fkUser, fkPassword, fkHost, fkPort, fkDatabase}},
		{KindPostgres, []string{fkUser, fkPassword, fkHost, fkPort, fkDatabase, fkSslMode}},
		{KindRedis, []string{fkUser, fkPassword, fkHost, fkPort, fkDbNum}},
	} {
		f := newConnForm(nil, tc.kind)
		if len(f.fields) != len(tc.want) {
			t.Fatalf("%s: %d fields, want %d", tc.kind, len(f.fields), len(tc.want))
		}
		for i, k := range tc.want {
			if f.fields[i].key != k {
				t.Errorf("%s field %d = %q, want %q", tc.kind, i, f.fields[i].key, k)
			}
		}
	}
}

func TestConnFormPrefillsFromConfig(t *testing.T) {
	useTempConfig(t)
	if err := config.WriteMysqlConfig(&config.MysqlConfig{
		UserName: "alice", Password: "secret", Host: "db.internal", Port: "3307", DbName: "shop",
	}); err != nil {
		t.Fatal(err)
	}

	f := newConnForm(nil, KindMysql)
	got := f.values()
	want := map[string]string{
		fkUser: "alice", fkPassword: "secret",
		fkHost: "db.internal", fkPort: "3307", fkDatabase: "shop",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("prefill %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestConnFormNavigationAndEditing(t *testing.T) {
	useTempConfig(t)
	f := newConnForm(nil, KindMysql)

	// tab / down move forward, wrapping.
	f.Update(keyPress("tab"))
	if f.focus != 1 {
		t.Fatalf("focus after tab = %d, want 1", f.focus)
	}
	f.Update(keyPress("down"))
	if f.focus != 2 {
		t.Fatalf("focus after down = %d, want 2", f.focus)
	}
	// shift+tab / up move backward, wrapping past the first field.
	f.Update(keyPress("shift+tab"))
	f.Update(keyPress("up"))
	f.Update(keyPress("up"))
	if want := len(f.fields) - 1; f.focus != want {
		t.Fatalf("focus after wrap = %d, want %d", f.focus, want)
	}

	// printable keys edit the focused field, backspace deletes.
	f.focus = 1
	typeText(t, f, "abc")
	if got := string(f.fields[1].value); got != "abc" {
		t.Fatalf("typed value = %q, want abc", got)
	}
	f.Update(keyPress("backspace"))
	if got := string(f.fields[1].value); got != "ab" {
		t.Fatalf("after backspace = %q, want ab", got)
	}
	// ctrl+u clears the focused field.
	f.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if got := string(f.fields[1].value); got != "" {
		t.Fatalf("after ctrl+u = %q, want empty", got)
	}

	// password field renders masked.
	f.fields[1].value = []rune("hunter2")
	view := f.View(80, 24, testTheme())
	if strings.Contains(view, "hunter2") {
		t.Fatal("password rendered in clear text")
	}
	if !strings.Contains(view, "*******") {
		t.Fatal("password not masked with asterisks")
	}
}

// TestConnFormEnterConnectsFromAnyField covers the key UX rule: enter always
// attempts a connection immediately, never moves focus.
func TestConnFormEnterConnectsFromAnyField(t *testing.T) {
	useTempConfig(t)
	for _, focus := range []int{0, 2, 4} {
		dialed := false
		fakeDialer(t, func(kind string, v map[string]string) (db.Backend, db.KVBackend, error) {
			dialed = true
			return nil, nil, fmt.Errorf("refused")
		})

		f := newConnForm(nil, KindMysql)
		f.focus = focus
		_, cmd := f.Update(keyPress("enter"))
		if cmd == nil {
			t.Fatalf("enter on field %d did not start the connect", focus)
		}
		if f.focus != focus {
			t.Fatalf("enter moved focus from %d to %d", focus, f.focus)
		}
		if !f.connecting {
			t.Fatalf("enter on field %d did not flip the connecting state", focus)
		}
		runCmd(cmd)
		if !dialed {
			t.Fatalf("enter on field %d never dialed", focus)
		}
	}
}

func TestConnFormPasteIntoFocusedField(t *testing.T) {
	useTempConfig(t)
	f := newConnForm(nil, KindMysql)
	f.focus = 2 // host

	_, cmd := f.Update(tea.PasteMsg{Content: "db-1.internal\r\ndb-2.internal\n"})
	if cmd != nil {
		t.Fatal("paste must not emit commands")
	}
	if got := string(f.fields[2].value); got != "db-1.internaldb-2.internal" {
		t.Fatalf("pasted value = %q, want newlines stripped", got)
	}

	// Paste appends to what is already typed, into the focused field only.
	f.Update(tea.PasteMsg{Content: ":3306"})
	if got := string(f.fields[2].value); got != "db-1.internaldb-2.internal:3306" {
		t.Fatalf("second paste = %q, want appended", got)
	}
	for i := range f.fields {
		if i != 2 && len(f.fields[i].value) != 0 {
			t.Fatalf("paste leaked into field %d (%q)", i, string(f.fields[i].value))
		}
	}
}

func TestConnFormSaveConfig(t *testing.T) {
	file := useTempConfig(t)
	f := newConnForm(nil, KindMysql)
	for i := range f.fields {
		if f.fields[i].key == fkHost {
			f.fields[i].value = []rune("db.example.internal")
		}
	}

	_, cmd := f.Update(keyPress("ctrl+s"))
	if msg := runCmd(cmd); msg == nil {
		t.Fatal("ctrl+s produced no toast")
	} else if _, ok := msg.(ui.ToastMsg); !ok {
		t.Fatalf("ctrl+s produced %T, want ToastMsg", msg)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if !strings.Contains(string(raw), "db.example.internal") {
		t.Fatalf("saved config missing host: %s", raw)
	}
}

// TestConnFormEscAlwaysQuits pins the page-stack rule: the connection form
// is the root page of database mode, so esc quits the app whether or not a
// live connection exists (reaching the form via the browser's esc included).
func TestConnFormEscAlwaysQuits(t *testing.T) {
	useTempConfig(t)
	for name, f := range map[string]*connForm{
		"not connected": newConnForm(&fakeCtx{}, KindMysql),
		"connected kv":  newConnForm(&fakeCtx{kv: &fakeKV{}}, KindRedis),
		"connected sql": newConnForm(&fakeCtx{be: &fakeBackend{}}, KindMysql),
	} {
		_, cmd := f.Update(keyPress("esc"))
		if _, ok := runCmd(cmd).(tea.QuitMsg); !ok {
			t.Errorf("%s: esc on the connection form must quit the app", name)
		}
	}
}

// TestConnFormFullscreenPage: the form renders as a full page (a centered
// box over a blanked body), not a floating popup.
func TestConnFormFullscreenPage(t *testing.T) {
	useTempConfig(t)
	var ov ui.Overlay = newConnForm(&fakeCtx{}, KindMysql)
	fs, ok := ov.(ui.FullscreenOverlay)
	if !ok || !fs.Fullscreen() {
		t.Fatal("connection form must be a fullscreen page overlay")
	}
}

// TestConnFormFooterHint: the hint no longer distinguishes connected from
// not connected — esc always quits.
func TestConnFormFooterHint(t *testing.T) {
	useTempConfig(t)
	for name, f := range map[string]*connForm{
		"not connected": newConnForm(&fakeCtx{}, KindMysql),
		"connected":     newConnForm(&fakeCtx{kv: &fakeKV{}}, KindRedis),
	} {
		v := f.View(80, 24, testTheme())
		if !strings.Contains(v, "esc quit") {
			t.Errorf("%s: footer hint missing %q", name, "esc quit")
		}
		if strings.Contains(v, "esc close") {
			t.Errorf("%s: stale connected-mode hint still rendered", name)
		}
	}
}

// TestConnFormConnectSuccess drives the full asynchronous connect flow with a
// fake connector wired in through the dialer seam.
func TestConnFormConnectSuccess(t *testing.T) {
	file := useTempConfig(t)
	t.Cleanup(conns.closeAll)

	be := &fakeBackend{}
	fakeDialer(t, func(kind string, v map[string]string) (db.Backend, db.KVBackend, error) {
		if kind != KindMysql {
			t.Fatalf("dialed kind %q, want %q", kind, KindMysql)
		}
		if v[fkHost] != "db.internal" {
			t.Fatalf("dialed host %q, want db.internal", v[fkHost])
		}
		return be, nil, nil
	})

	f := newConnForm(nil, KindMysql)
	for i := range f.fields {
		if f.fields[i].key == fkHost {
			f.fields[i].value = []rune("db.internal")
		}
	}

	// enter starts the async connect from any field.
	_, cmd := f.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatal("enter did not start the connect")
	}
	if !f.connecting {
		t.Fatal("form not in connecting state")
	}
	// A second request while in flight is ignored.
	if _, again := f.Update(keyPress("ctrl+r")); again != nil {
		t.Fatal("second connect while in flight must be ignored")
	}

	msg := runCmd(cmd)
	done, ok := msg.(connDoneMsg)
	if !ok {
		t.Fatalf("connect produced %T, want connDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("connect failed: %v", done.err)
	}
	if done.be != be || done.kv != nil {
		t.Fatalf("connect: be=%v kv=%v, want the fake SQL backend only", done.be, done.kv)
	}

	// Feeding the success message saves the config and yields the follow-up
	// sequence (close form, attach backend, open schema, toast).
	_, cmd = f.Update(done)
	if f.connecting {
		t.Fatal("still connecting after success")
	}
	if cmd == nil {
		t.Fatal("success produced no follow-up commands")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("config not saved on success: %v", err)
	}
	if !strings.Contains(string(raw), "db.internal") {
		t.Fatal("saved config missing the connected host")
	}

	// The success command sequence carries the expected messages.
	cmds := f.successCmds(done)
	if len(cmds) != 4 {
		t.Fatalf("successCmds returned %d commands, want 4", len(cmds))
	}
	if _, ok := runCmd(cmds[0]).(ui.CloseOverlayMsg); !ok {
		t.Fatal("first success command must close the form")
	}
	sb, ok := runCmd(cmds[1]).(ui.SetBackendMsg)
	if !ok || sb.Backend == nil {
		t.Fatal("second success command must attach the backend")
	}
	rc, ok := runCmd(cmds[2]).(ui.RunCommandMsg)
	if !ok || rc.Name != "schema" {
		t.Fatal("third success command must open the schema browser")
	}
	if _, ok := runCmd(cmds[3]).(ui.ToastMsg); !ok {
		t.Fatal("fourth success command must toast")
	}
}

// TestConnFormConnectFailure checks the inline-error path: the form stays
// open and shows the error.
func TestConnFormConnectFailure(t *testing.T) {
	useTempConfig(t)
	fakeDialer(t, func(kind string, v map[string]string) (db.Backend, db.KVBackend, error) {
		return nil, nil, fmt.Errorf("connection refused")
	})

	f := newConnForm(nil, KindMysql)
	_, cmd := f.Update(keyPress("ctrl+r"))
	if cmd == nil {
		t.Fatal("ctrl+r did not start the connect")
	}
	done, ok := runCmd(cmd).(connDoneMsg)
	if !ok || done.err == nil {
		t.Fatalf("expected failing connDoneMsg, got %#v", done)
	}

	ov, cmd := f.Update(done)
	if cmd != nil {
		t.Fatal("failure must not emit follow-up commands")
	}
	if ov != ui.Overlay(f) {
		t.Fatal("failure must keep the form open")
	}
	if f.errText == "" {
		t.Fatal("failure did not set the inline error")
	}
	view := f.View(80, 24, testTheme())
	if !strings.Contains(view, strings.Split(f.errText, " ")[0]) {
		t.Fatal("inline error not rendered")
	}
}

func TestConnFormEscIgnoredWhileConnecting(t *testing.T) {
	useTempConfig(t)
	f := newConnForm(&fakeCtx{}, KindMysql)
	f.connecting = true
	// Dismissing the form mid-dial would drop the connDoneMsg (orphaning the
	// new backend after the app closed the old one) and allow a second
	// concurrent dial; esc must be a no-op until the dial resolves.
	_, cmd := f.Update(keyPress("esc"))
	if cmd != nil {
		t.Fatal("esc while connecting must be ignored")
	}
}

func TestConnFormPasswordKeepsWhitespace(t *testing.T) {
	useTempConfig(t)
	f := newConnForm(nil, KindMysql)
	for i := range f.fields {
		switch f.fields[i].key {
		case fkPassword:
			f.fields[i].value = []rune("  p@ss ")
		case fkUser:
			f.fields[i].value = []rune("  alice ")
		}
	}
	v := f.values()
	if v[fkPassword] != "  p@ss " {
		t.Errorf("password = %q, want whitespace preserved", v[fkPassword])
	}
	if v[fkUser] != "alice" {
		t.Errorf("user = %q, want trimmed", v[fkUser])
	}
}
