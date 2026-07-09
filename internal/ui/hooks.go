package ui

import (
	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/theme"
)

// TabInfo is a summary of one open tab for switchers and schema listings.
type TabInfo struct {
	Title string
	Shape string // e.g. "120 x 5"
}

// AppContext is the read-only view of the application that popup factories
// receive. It exists so popup packages can be wired in without importing the
// app (avoiding cycles).
type AppContext interface {
	CurrentFrame() *data.Frame // top of current pane stack (nil if no tabs)
	CurrentRow() int           // selected row in current table view
	BaseCrumb() string         // current tab title
	Crumbs() []string          // full breadcrumb chain
	ColumnNames() []string
	Engine() *query.Engine // embedded SQL engine (may be nil in db mode)
	TableNames() []string  // engine-registered + backend tables for completion
	Backend() db.Backend   // nil in file mode
	KV() db.KVBackend      // nil unless redis mode
	Theme() *theme.Theme
	ThemeName() string
	ShowBorders() bool
	ShowRowNumbers() bool
	Tabs() []TabInfo
	ActiveTab() int
	ActivePaneID() int // stable identity of the active pane (0 if no tabs)
}

// Factories maps command names to overlay constructors. Popup packages
// register themselves here from init(). A factory may return (nil, nil) for
// pure side-effect commands that need no overlay.
var Factories = map[string]func(ctx AppContext, arg string) (Overlay, error){}
