package popup

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
	"github.com/LinPr/sqltui/internal/ui"
)

// schemaTestBackend is a stub db.Backend for schema overlay tests. It does
// NOT implement the optional CurrentNamespace interface (see
// schemaTestBackendNS for the variant that does).
type schemaTestBackend struct {
	namespaces []string
	tables     map[string][]string
	nsErr      error
	tblErr     error
	tblCalls   []string // namespaces Tables() was asked for
}

func (b *schemaTestBackend) Kind() string  { return "stub" }
func (b *schemaTestBackend) Title() string { return "stub://db" }
func (b *schemaTestBackend) Run(string) (db.Result, error) {
	return db.Result{}, errors.New("not implemented")
}
func (b *schemaTestBackend) Namespaces() ([]string, error) { return b.namespaces, b.nsErr }
func (b *schemaTestBackend) Tables(ns string) ([]string, error) {
	b.tblCalls = append(b.tblCalls, ns)
	if b.tblErr != nil {
		return nil, b.tblErr
	}
	return b.tables[ns], nil
}
func (b *schemaTestBackend) FetchTable(string, string, int) (*data.Frame, error) {
	return nil, errors.New("not implemented")
}
func (b *schemaTestBackend) PrimaryKeys(string, string) ([]string, error) { return nil, nil }
func (b *schemaTestBackend) Close() error                                  { return nil }

// schemaTestBackendNS additionally reports a connected namespace through the
// optional CurrentNamespace interface.
type schemaTestBackendNS struct {
	*schemaTestBackend
	current string
}

func (b *schemaTestBackendNS) CurrentNamespace() string { return b.current }

// schemaTestCtx is a minimal AppContext stub for schema overlay tests.
type schemaTestCtx struct {
	tabs    []ui.TabInfo
	active  int
	backend db.Backend
}

func (c schemaTestCtx) CurrentFrame() *data.Frame { return nil }
func (c schemaTestCtx) CurrentRow() int           { return 0 }
func (c schemaTestCtx) SheetFieldCursor() int     { return 0 }
func (c schemaTestCtx) CurrentTableNamespace() string { return "" }
func (c schemaTestCtx) BaseCrumb() string         { return "" }
func (c schemaTestCtx) Crumbs() []string          { return nil }
func (c schemaTestCtx) ColumnNames() []string     { return nil }
func (c schemaTestCtx) Engine() *query.Engine     { return nil }
func (c schemaTestCtx) TableNames() []string      { return nil }
func (c schemaTestCtx) Backend() db.Backend       { return c.backend }
func (c schemaTestCtx) KV() db.KVBackend          { return nil }
func (c schemaTestCtx) Theme() *theme.Theme       { return nil }
func (c schemaTestCtx) ThemeName() string         { return "" }
func (c schemaTestCtx) ShowBorders() bool         { return false }
func (c schemaTestCtx) ShowRowNumbers() bool      { return false }
func (c schemaTestCtx) Tabs() []ui.TabInfo        { return c.tabs }
func (c schemaTestCtx) ActiveTab() int            { return c.active }
func (c schemaTestCtx) ActivePaneID() int         { return 0 }
func (c schemaTestCtx) PendingEdit() *ui.PendingEdit   { return nil }
func (c schemaTestCtx) PendingDelete() *ui.PendingDelete { return nil }

// schemaTestMsgs runs a cmd (possibly a batch) and collects all messages.
func schemaTestMsgs(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, schemaTestMsgs(t, c)...)
		}
		return out
	}
	// tea.Sequence yields an unexported []tea.Cmd-based message; unwrap it
	// reflectively so ordered command chains can be inspected too.
	if rv := reflect.ValueOf(msg); rv.IsValid() && rv.Kind() == reflect.Slice &&
		strings.Contains(rv.Type().String(), "sequenceMsg") {
		var out []tea.Msg
		for i := 0; i < rv.Len(); i++ {
			if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
				out = append(out, schemaTestMsgs(t, c)...)
			}
		}
		return out
	}
	return []tea.Msg{msg}
}

// schemaOpen builds the overlay and synchronously delivers its async
// namespace listing, as the app does after pushing it (Init cmd -> Update).
func schemaOpen(ctx ui.AppContext) *schemaOverlay {
	o := newSchemaOverlay(ctx)
	if cmd := o.Init(); cmd != nil {
		ov, _ := o.Update(cmd())
		o = ov.(*schemaOverlay)
	}
	return o
}

// schemaType feeds one printable string into the overlay's filter input.
func schemaType(o *schemaOverlay, s string) {
	for _, r := range s {
		o.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// schemaLiveLabels collects the live-table rows of the current listing.
func schemaLiveLabels(o *schemaOverlay) []string {
	var out []string
	for _, r := range o.rows() {
		if r.kind == schemaLive {
			out = append(out, r.label)
		}
	}
	return out
}

func TestSchemaFactoryRegistered(t *testing.T) {
	f, ok := ui.Factories["schema"]
	if !ok {
		t.Fatal("schema factory not registered")
	}
	ov, err := f(schemaTestCtx{}, "")
	if err != nil || ov == nil {
		t.Fatalf("factory: overlay=%v err=%v", ov, err)
	}
}

func TestSchemaEntriesFileMode(t *testing.T) {
	ctx := schemaTestCtx{
		tabs:   []ui.TabInfo{{Title: "a", Shape: "3 x 2"}, {Title: "b", Shape: "5 x 1"}},
		active: 1,
	}
	o := schemaOpen(ctx)
	rows := o.rows()
	// header + 2 tabs, no live section without a backend
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].kind != schemaHeader {
		t.Errorf("rows[0].kind = %v, want header", rows[0].kind)
	}
	if !strings.Contains(rows[1].label, "a") || !strings.Contains(rows[1].label, "3 x 2") {
		t.Errorf("tab row label = %q", rows[1].label)
	}
	// Cursor starts on the active tab.
	if o.cursor != 2 || rows[o.cursor].tab != 1 {
		t.Errorf("cursor = %d (tab %d), want row for active tab 1", o.cursor, rows[o.cursor].tab)
	}
}

func TestSchemaConnectedNamespaceEagerOthersCollapsed(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"aux", "main", "sys"},
			tables: map[string][]string{
				"main": {"users", "orders"}, "aux": {"log"}, "sys": {"cfg"},
			},
		},
		current: "main",
	}
	ctx := schemaTestCtx{tabs: []ui.TabInfo{{Title: "t", Shape: "1 x 1"}}, backend: be}
	o := schemaOpen(ctx)

	// Only the connected namespace was listed eagerly.
	if len(be.tblCalls) != 1 || be.tblCalls[0] != "main" {
		t.Fatalf("Tables() called for %v, want [main]", be.tblCalls)
	}
	// The connected group comes first, is open and loaded.
	if len(o.groups) != 3 || o.groups[0].ns != "main" {
		t.Fatalf("groups = %+v, want main first of 3", o.groups)
	}
	if !o.groups[0].open || !o.groups[0].loaded || !o.groups[0].connected {
		t.Errorf("connected group state = %+v", o.groups[0])
	}
	for _, g := range o.groups[1:] {
		if g.open || g.loaded {
			t.Errorf("group %q must stay collapsed and unloaded: %+v", g.ns, g)
		}
	}
	// Visible live rows: only the connected namespace's tables.
	if got := schemaLiveLabels(o); len(got) != 2 || got[0] != "users" || got[1] != "orders" {
		t.Errorf("live rows = %v, want [users orders]", got)
	}
	// Collapsed groups render as expandable headers.
	var collapsed []string
	for _, r := range o.rows() {
		if r.kind == schemaGroup && strings.Contains(r.label, "enter to expand") {
			collapsed = append(collapsed, r.label)
		}
	}
	if len(collapsed) != 2 {
		t.Errorf("collapsed group headers = %v, want 2", collapsed)
	}
	for _, l := range collapsed {
		if !strings.HasPrefix(l, "▸ ") {
			t.Errorf("collapsed header %q missing ▸ marker", l)
		}
	}
}

func TestSchemaNoCurrentNamespaceAllCollapsed(t *testing.T) {
	// Without the optional CurrentNamespace interface and with several
	// namespaces, nothing is loaded eagerly.
	be := &schemaTestBackend{
		namespaces: []string{"a", "b"},
		tables:     map[string][]string{"a": {"t1"}, "b": {"t2"}},
	}
	o := schemaOpen(schemaTestCtx{backend: be})
	if len(be.tblCalls) != 0 {
		t.Fatalf("Tables() called for %v, want none", be.tblCalls)
	}
	if got := schemaLiveLabels(o); len(got) != 0 {
		t.Errorf("live rows = %v, want none before expansion", got)
	}
	for _, g := range o.groups {
		if g.open || g.loaded {
			t.Errorf("group %q must start collapsed: %+v", g.ns, g)
		}
	}
}

func TestSchemaSingleNamespaceLoadsEagerly(t *testing.T) {
	// A sole namespace is listed eagerly even without CurrentNamespace.
	be := &schemaTestBackend{namespaces: []string{"main"}, tables: map[string][]string{"main": {"x"}}}
	o := schemaOpen(schemaTestCtx{backend: be})
	if got := schemaLiveLabels(o); len(got) != 1 || got[0] != "x" {
		t.Errorf("live rows = %v, want [x]", got)
	}
}

func TestSchemaBackendEmptyNamespaceFlat(t *testing.T) {
	// Engines without namespaces report a single empty string; the listing
	// stays flat: no group headers, labels without a leading dot.
	be := &schemaTestBackend{namespaces: []string{""}, tables: map[string][]string{"": {"kv"}}}
	o := schemaOpen(schemaTestCtx{backend: be})
	if !o.flat {
		t.Fatal("single empty namespace must use the flat layout")
	}
	found := false
	for _, r := range o.rows() {
		if r.kind == schemaGroup {
			t.Errorf("flat layout must not render group headers: %+v", r)
		}
		if r.kind == schemaLive {
			found = true
			if r.label != "kv" || r.ns != "" || r.table != "kv" {
				t.Errorf("live row = %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("no live row built")
	}
}

func TestSchemaBackendError(t *testing.T) {
	be := &schemaTestBackend{nsErr: errors.New("connection lost")}
	o := schemaOpen(schemaTestCtx{backend: be})
	found := false
	for _, r := range o.rows() {
		if r.kind == schemaNote && strings.Contains(r.label, "connection lost") {
			found = true
		}
	}
	if !found {
		t.Error("namespace error not surfaced as a note row")
	}
}

func TestSchemaLazyExpandAndCollapse(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"main", "aux"},
			tables:     map[string][]string{"main": {"users"}, "aux": {"log", "trace"}},
		},
		current: "main",
	}
	o := schemaOpen(schemaTestCtx{backend: be})

	// Move the cursor onto the collapsed "aux" group header.
	rows := o.rows()
	target := -1
	for i, r := range rows {
		if r.kind == schemaGroup && o.groups[r.group].ns == "aux" {
			target = i
		}
	}
	if target < 0 {
		t.Fatal("no aux group header")
	}
	o.cursor = target
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on unloaded group must start the lazy load")
	}
	if !o.groups[1].open || !o.groups[1].loading {
		t.Fatalf("group state after enter = %+v, want open+loading", o.groups[1])
	}
	// The listing arrives asynchronously and expands in place.
	o.Update(cmd())
	if !o.groups[1].loaded || o.groups[1].loading {
		t.Fatalf("group state after load = %+v, want loaded", o.groups[1])
	}
	got := schemaLiveLabels(o)
	want := []string{"users", "log", "trace"}
	if len(got) != len(want) {
		t.Fatalf("live rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("live[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Enter on the (now open) header collapses it without a rescan.
	rows = o.rows()
	for i, r := range rows {
		if r.kind == schemaGroup && o.groups[r.group].ns == "aux" {
			o.cursor = i
		}
	}
	before := len(be.tblCalls)
	_, cmd = o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || o.groups[1].open {
		t.Fatal("enter on open group must collapse it without commands")
	}
	// Re-expanding a loaded group does not hit the backend again.
	rows = o.rows()
	for i, r := range rows {
		if r.kind == schemaGroup && o.groups[r.group].ns == "aux" {
			o.cursor = i
		}
	}
	o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(be.tblCalls) != before {
		t.Errorf("re-expanding a loaded group rescanned: calls %v", be.tblCalls)
	}
}

func TestSchemaGroupLoadError(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{namespaces: []string{"main", "aux"}},
		current:           "main",
	}
	o := schemaOpen(schemaTestCtx{backend: be})
	be.tblErr = errors.New("access denied")
	for i, r := range o.rows() {
		if r.kind == schemaGroup && o.groups[r.group].ns == "aux" {
			o.cursor = i
		}
	}
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("no load command")
	}
	o.Update(cmd())
	found := false
	for _, r := range o.rows() {
		if r.kind == schemaNote && strings.Contains(r.label, "access denied") {
			found = true
		}
	}
	if !found {
		t.Error("group load error not surfaced as a note row")
	}
}

func TestSchemaFilterNarrowsTabsAndTables(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"main", "aux"},
			tables:     map[string][]string{"main": {"users", "orders"}, "aux": {"log"}},
		},
		current: "main",
	}
	ctx := schemaTestCtx{
		tabs:    []ui.TabInfo{{Title: "users_csv", Shape: "9 x 3"}, {Title: "notes", Shape: "2 x 2"}},
		backend: be,
	}
	o := schemaOpen(ctx)
	schemaType(o, "user")

	rows := o.rows()
	var tabRows, liveRows []string
	for _, r := range rows {
		switch r.kind {
		case schemaTab:
			tabRows = append(tabRows, r.label)
		case schemaLive:
			liveRows = append(liveRows, r.label)
		}
	}
	// In db mode the tabs section is skipped: no tab rows even when a tab
	// title matches the filter.
	if len(tabRows) != 0 {
		t.Errorf("filtered tab rows = %v, want none in db mode", tabRows)
	}
	if len(liveRows) != 1 || liveRows[0] != "users" {
		t.Errorf("filtered live rows = %v, want [users]", liveRows)
	}
	// The unloaded aux namespace does not match "user": it stays hidden but
	// is flagged so the user knows it was not searched.
	note := false
	for _, r := range rows {
		if r.kind == schemaNote && strings.Contains(r.label, "not loaded") {
			note = true
		}
	}
	if !note {
		t.Error("unloaded namespaces not flagged while filtering")
	}
	// The cursor landed on a selectable row.
	if o.cursor < 0 || !rows[o.cursor].selectable() {
		t.Errorf("cursor = %d, not on a selectable row", o.cursor)
	}
}

func TestSchemaFilterMatchesUnloadedNamespaceName(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"main", "analytics"},
			tables:     map[string][]string{"main": {"users"}, "analytics": {"events"}},
		},
		current: "main",
	}
	o := schemaOpen(schemaTestCtx{backend: be})
	schemaType(o, "analytics")

	rows := o.rows()
	target := -1
	for i, r := range rows {
		if r.kind == schemaGroup && strings.Contains(r.label, "analytics") {
			if !strings.Contains(r.label, "not loaded") {
				t.Errorf("unloaded group header = %q, want a not-loaded note", r.label)
			}
			target = i
		}
	}
	if target < 0 {
		t.Fatal("matching unloaded namespace not shown while filtering")
	}
	// Enter on it loads the namespace.
	o.cursor = target
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on unloaded group must start the lazy load")
	}
	o.Update(cmd())
	if !o.groups[1].loaded {
		t.Fatal("group not loaded after expansion")
	}
}

func TestSchemaFilterSearchesLoadedCollapsedGroups(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"main", "aux"},
			tables:     map[string][]string{"main": {"users"}, "aux": {"audit_log"}},
		},
		current: "main",
	}
	o := schemaOpen(schemaTestCtx{backend: be})
	// Load aux, then collapse it again.
	for i, r := range o.rows() {
		if r.kind == schemaGroup && o.groups[r.group].ns == "aux" {
			o.cursor = i
		}
	}
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	o.Update(cmd())
	for i, r := range o.rows() {
		if r.kind == schemaGroup && o.groups[r.group].ns == "aux" {
			o.cursor = i
		}
	}
	o.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // collapse
	if o.groups[1].open {
		t.Fatal("aux should be collapsed")
	}
	// Filtering still finds its (already loaded) tables.
	schemaType(o, "audit")
	if got := schemaLiveLabels(o); len(got) != 1 || got[0] != "audit_log" {
		t.Errorf("filtered rows = %v, want [audit_log]", got)
	}
}

func TestSchemaEscClearsFilterThenCloses(t *testing.T) {
	o := schemaOpen(schemaTestCtx{tabs: []ui.TabInfo{{Title: "a", Shape: "1 x 1"}}})
	schemaType(o, "zzz")
	if o.input.String() != "zzz" {
		t.Fatalf("filter input = %q, want zzz", o.input.String())
	}
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("first esc must clear the filter, not close")
	}
	if o.input.String() != "" {
		t.Fatalf("filter after esc = %q, want empty", o.input.String())
	}
	_, cmd = o.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("second esc must close")
	}
	if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
		t.Errorf("esc cmd msg = %T, want CloseOverlayMsg", cmd())
	}
}

// TestSchemaEscDbModeAlwaysChainsConnect pins the page-stack rule: in db
// mode the browser is a page between the connection form and the table, so
// esc steps back to the form whether or not tabs are open.
func TestSchemaEscDbModeAlwaysChainsConnect(t *testing.T) {
	for name, tabs := range map[string][]ui.TabInfo{
		"zero tabs": nil,
		"with tabs": {{Title: "a", Shape: "1 x 1"}, {Title: "b", Shape: "2 x 2"}},
	} {
		be := &schemaTestBackend{namespaces: []string{"m"}, tables: map[string][]string{"m": {"x"}}}
		o := schemaOpen(schemaTestCtx{tabs: tabs, backend: be})
		_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if cmd == nil {
			t.Fatalf("%s: esc produced no command", name)
		}
		var closed, connected bool
		for _, m := range schemaTestMsgs(t, cmd) {
			switch v := m.(type) {
			case ui.CloseOverlayMsg:
				closed = true
			case ui.RunCommandMsg:
				connected = true
				if v.Name != "connect" {
					t.Errorf("%s: RunCommandMsg.Name = %q, want connect", name, v.Name)
				}
			}
		}
		if !closed || !connected {
			t.Errorf("%s: closed=%v connect=%v, want both", name, closed, connected)
		}
	}
}

func TestSchemaEscFileModeJustCloses(t *testing.T) {
	// Without a backend there is no connection form: esc simply closes.
	ctx := schemaTestCtx{tabs: []ui.TabInfo{{Title: "a", Shape: "1 x 1"}}}
	o := schemaOpen(ctx)
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msgs := schemaTestMsgs(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("esc in file mode produced %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(ui.CloseOverlayMsg); !ok {
		t.Fatalf("esc msg = %T, want CloseOverlayMsg", msgs[0])
	}
}

func TestSchemaFullscreenOnlyInDbMode(t *testing.T) {
	be := &schemaTestBackend{namespaces: []string{"m"}}
	if o := newSchemaOverlay(schemaTestCtx{backend: be}); !o.Fullscreen() {
		t.Error("db mode: schema browser must be a fullscreen page")
	}
	if o := newSchemaOverlay(schemaTestCtx{}); o.Fullscreen() {
		t.Error("file mode: schema browser must stay a floating box")
	}
}

// TestSchemaFullscreenViewFillsPage: in db mode the frame is sized to fill
// most of the page (width-8 x body height-2) even with few entries, so the
// table behind can never show through; the file-mode box stays compact.
func TestSchemaFullscreenViewFillsPage(t *testing.T) {
	be := &schemaTestBackend{namespaces: []string{"m"}, tables: map[string][]string{"m": {"x"}}}
	o := schemaOpen(schemaTestCtx{backend: be})
	th := infoTestTheme(t)

	const w, bodyH = 80, 23
	v := o.View(w, bodyH, th)
	lines := strings.Split(v, "\n")
	if len(lines) != bodyH-2 {
		t.Fatalf("fullscreen frame height = %d lines, want %d", len(lines), bodyH-2)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w-8 {
			t.Fatalf("fullscreen frame line %d width = %d, want %d", i, got, w-8)
		}
	}

	of := schemaOpen(schemaTestCtx{tabs: []ui.TabInfo{{Title: "a", Shape: "1 x 1"}}})
	if got := len(strings.Split(of.View(w, bodyH, th), "\n")); got >= bodyH-2 {
		t.Errorf("file-mode box height = %d lines, must stay compact (< %d)", got, bodyH-2)
	}
}

func TestSchemaCtrlCCloses(t *testing.T) {
	o := schemaOpen(schemaTestCtx{tabs: []ui.TabInfo{{Title: "a", Shape: "1 x 1"}}})
	schemaType(o, "abc")
	_, cmd := o.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c: no cmd")
	}
	if _, ok := cmd().(ui.CloseOverlayMsg); !ok {
		t.Errorf("ctrl+c cmd msg = %T, want CloseOverlayMsg", cmd())
	}
}

func TestSchemaNavigationSkipsHeaders(t *testing.T) {
	be := &schemaTestBackend{namespaces: []string{"m"}, tables: map[string][]string{"m": {"x"}}}
	ctx := schemaTestCtx{tabs: []ui.TabInfo{{Title: "a", Shape: "1 x 1"}}, backend: be}
	o := schemaOpen(ctx)
	// db mode skips the tabs section; rows: [header tables][group m][live x]
	if o.cursor != 1 {
		t.Fatalf("initial cursor = %d, want 1", o.cursor)
	}
	o.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if o.cursor != 2 {
		t.Errorf("cursor after down = %d, want 2", o.cursor)
	}
	o.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if o.cursor != 2 {
		t.Errorf("cursor after down at bottom = %d, want 2", o.cursor)
	}
	o.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if o.cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", o.cursor)
	}
	o.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if o.cursor != 1 {
		t.Errorf("cursor after up at top = %d, want 1", o.cursor)
	}
}

func TestSchemaEnterOnTab(t *testing.T) {
	ctx := schemaTestCtx{
		tabs:   []ui.TabInfo{{Title: "a", Shape: "1 x 1"}, {Title: "b", Shape: "2 x 2"}},
		active: 0,
	}
	o := schemaOpen(ctx)
	o.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // move to tab b
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msgs := schemaTestMsgs(t, cmd)

	var jumped, closed bool
	for _, m := range msgs {
		switch v := m.(type) {
		case ui.JumpToTabMsg:
			jumped = true
			if v.Index != 1 {
				t.Errorf("JumpToTabMsg.Index = %d, want 1", v.Index)
			}
		case ui.CloseOverlayMsg:
			closed = true
		}
	}
	if !jumped || !closed {
		t.Errorf("enter on tab: jumped=%v closed=%v, want both", jumped, closed)
	}
}

func TestSchemaEnterOnLiveTable(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"m", "other"},
			tables:     map[string][]string{"m": {"users"}},
		},
		current: "m",
	}
	o := schemaOpen(schemaTestCtx{backend: be})
	rows := o.rows()
	for i, r := range rows {
		if r.kind == schemaLive {
			o.cursor = i
		}
	}
	if o.cursor < 0 || rows[o.cursor].kind != schemaLive {
		t.Fatalf("cursor = %d, not on live row", o.cursor)
	}
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msgs := schemaTestMsgs(t, cmd)

	var ran, closed bool
	for _, m := range msgs {
		switch v := m.(type) {
		case ui.RunCommandMsg:
			ran = true
			if v.Name != "opentable" || v.Arg != "m\tusers" {
				t.Errorf("RunCommandMsg = %+v, want opentable m\\tusers", v)
			}
		case ui.CloseOverlayMsg:
			closed = true
		}
	}
	if !ran || !closed {
		t.Errorf("enter on live table: ran=%v closed=%v, want both", ran, closed)
	}
}

func TestSchemaEnterNoSelectable(t *testing.T) {
	o := schemaOpen(schemaTestCtx{}) // no tabs, no backend
	if o.cursor != -1 {
		t.Fatalf("cursor = %d, want -1 with nothing selectable", o.cursor)
	}
	_, cmd := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter with nothing selectable should produce no cmd")
	}
}

func TestSchemaScrollWindow(t *testing.T) {
	var tabs []ui.TabInfo
	for i := 0; i < 30; i++ {
		tabs = append(tabs, ui.TabInfo{Title: string(rune('a' + i%26)), Shape: "1 x 1"})
	}
	o := schemaOpen(schemaTestCtx{tabs: tabs, active: 0})
	th := infoTestTheme(t)
	o.View(80, 15, th) // establish viewLen

	rows := o.rows()
	if o.viewLen >= len(rows) {
		t.Fatalf("viewLen = %d, expected scrolling window over %d rows", o.viewLen, len(rows))
	}
	// Page down repeatedly; the window must always contain the cursor.
	for i := 0; i < 6; i++ {
		o.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if o.cursor != len(rows)-1 {
		t.Fatalf("cursor after paging = %d, want %d", o.cursor, len(rows)-1)
	}
	if o.cursor < o.offset || o.cursor >= o.offset+o.viewLen {
		t.Errorf("cursor %d outside window [%d, %d)", o.cursor, o.offset, o.offset+o.viewLen)
	}
	// And back to the top.
	for i := 0; i < 6; i++ {
		o.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	}
	if o.cursor != 1 {
		t.Errorf("cursor after paging up = %d, want 1", o.cursor)
	}
	if o.offset > o.cursor {
		t.Errorf("offset %d beyond cursor %d", o.offset, o.cursor)
	}
}

func TestSchemaViewContent(t *testing.T) {
	be := &schemaTestBackendNS{
		schemaTestBackend: &schemaTestBackend{
			namespaces: []string{"m", "aux"},
			tables:     map[string][]string{"m": {"users"}},
		},
		current: "m",
	}
	ctx := schemaTestCtx{tabs: []ui.TabInfo{{Title: "people", Shape: "3 x 2"}}, backend: be}
	o := schemaOpen(ctx)
	th := infoTestTheme(t)
	v := o.View(80, 24, th)
	for _, want := range []string{"filter", "users", "stub://db",
		"(connected)", "enter to expand"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
	// In db mode the tabs section is skipped: the header and the tab
	// title/shape must not appear.
	for _, absent := range []string{"tabs", "people", "3 x 2"} {
		if strings.Contains(v, absent) {
			t.Errorf("view contains %q, want tabs section absent in db mode", absent)
		}
	}
}
