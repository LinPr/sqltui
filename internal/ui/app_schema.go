// Completion-schema plumbing for the app: CompletionSchema assembles the
// query.Schema the SQL completer works from, WarmCompletionSchema fills the
// live-connection side of it off the update loop.
package ui

import (
	"sync"

	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
)

// completionCaches memoizes one backend-schema snapshot per app. The App
// struct lives in app.go (owned by other concerns), so instead of a new
// field the cache hangs off this package-level map keyed by the *App
// pointer. Each app owns at most one entry, overwritten in place when its
// connection changes, and apps live for the process lifetime, so the map
// stays bounded and leak-free.
var completionCaches sync.Map // *App -> *schemaCache

// schemaCache is the lazily-filled snapshot of a live connection's catalog.
// The backend field identifies the connection the snapshot was built from;
// a SetBackendMsg swaps a.backend, which the accessors detect and answer by
// resetting the entry, so no explicit invalidation hook is needed.
type schemaCache struct {
	mu      sync.Mutex
	backend db.Backend            // connection the snapshot belongs to
	listed  bool                  // namespace/table listing completed
	ns      map[string]string     // table -> namespace (for column fetches)
	cols    map[string][]string   // table -> columns (nil until fetched)
	meta    map[string][]db.ColumnMeta // table -> full column metadata
	fetched map[string]bool       // column fetch attempted (even if empty)
}

// completionCache returns the app's cache entry, creating it on first use.
func (a *App) completionCache() *schemaCache {
	if v, ok := completionCaches.Load(a); ok {
		return v.(*schemaCache)
	}
	v, _ := completionCaches.LoadOrStore(a, &schemaCache{})
	return v.(*schemaCache)
}

// resetFor rebinds the cache to be (locked) and clears any snapshot taken
// from a previous connection. Callers must hold c.mu.
func (c *schemaCache) resetFor(be db.Backend) {
	if c.backend == be {
		return
	}
	c.backend = be
	c.listed = false
	c.ns = make(map[string]string)
	c.cols = make(map[string][]string)
	c.meta = make(map[string][]db.ColumnMeta)
	c.fetched = make(map[string]bool)
}

// CompletionSchema builds the completion schema without ever blocking on
// the live connection: engine tables come from PRAGMA lookups against the
// in-memory database (cheap, safe on the update loop), live tables come
// from whatever WarmCompletionSchema has cached so far, and Current is the
// column set of the frame under view.
func (a *App) CompletionSchema() query.Schema {
	sc := query.Schema{Current: a.ColumnNames()}
	tables := make(map[string][]string)

	if eng := a.Engine(); eng != nil {
		if m, err := eng.TableColumns(); err == nil {
			for t, cols := range m {
				tables[t] = cols
			}
		}
	}

	if be := a.backend; be != nil {
		c := a.completionCache()
		c.mu.Lock()
		if c.backend == be {
			for t, cols := range c.cols {
				tables[t] = append([]string(nil), cols...)
			}
		}
		c.mu.Unlock()
	}

	if len(tables) > 0 {
		sc.Tables = tables
	}
	return sc
}

// WarmCompletionSchema populates the live-connection side of the completion
// cache. It issues catalog queries and therefore blocks: call it only from
// inside a tea.Cmd goroutine, never on the update loop. With no arguments
// it fetches the namespace/table listing (once per connection); given table
// names it additionally fetches those tables' columns on first use. A
// replaced connection resets the cache automatically. File mode (no
// backend) is a no-op. Errors are swallowed: completion degrades to
// whatever is cached rather than interrupting typing.
func (a *App) WarmCompletionSchema(tables ...string) {
	be := a.backend
	if be == nil {
		return
	}
	c := a.completionCache()

	c.mu.Lock()
	c.resetFor(be)
	needList := !c.listed
	c.mu.Unlock()

	if needList {
		ns := make(map[string]string)
		if nss, err := be.Namespaces(); err == nil {
			for _, n := range nss {
				ts, err := be.Tables(n)
				if err != nil {
					continue
				}
				for _, t := range ts {
					if _, dup := ns[t]; !dup {
						ns[t] = n
					}
				}
			}
		}
		c.mu.Lock()
		if c.backend == be && !c.listed {
			c.listed = true
			for t, n := range ns {
				c.ns[t] = n
				if _, ok := c.cols[t]; !ok {
					c.cols[t] = nil // table known, columns pending
				}
			}
		}
		c.mu.Unlock()
	}

	for _, t := range tables {
		c.mu.Lock()
		nsName, known := c.ns[t]
		skip := c.backend != be || !known || c.fetched[t]
		c.mu.Unlock()
		if skip {
			continue
		}

		// Prefer the richer ColumnsMeta call: it yields both the column
		// names and the full metadata the sheet type line consumes. Only when
		// the engine can't describe the table do we fall back to a 1-row
		// FetchTable for the names alone (meta stays nil).
		var cols []string
		var meta []db.ColumnMeta
		if cm, err := be.ColumnsMeta(nsName, t); err == nil && cm != nil {
			cols = make([]string, len(cm))
			for i, m := range cm {
				cols[i] = m.Name
			}
			meta = cm
		} else if f, err := be.FetchTable(nsName, t, 1); err == nil && f != nil {
			cols = f.ColumnNames()
		}

		c.mu.Lock()
		if c.backend == be {
			c.fetched[t] = true
			c.cols[t] = cols
			c.meta[t] = meta
		}
		c.mu.Unlock()
	}
}

// columnMetaFor returns the cached full column metadata for table, or nil when
// no snapshot is available for the current connection (file mode, a table the
// cache hasn't warmed, or a backend whose ColumnsMeta errored). The caller is
// expected to degrade gracefully.
func (a *App) columnMetaFor(table string) []db.ColumnMeta {
	c := a.completionCache()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.backend == a.backend {
		return c.meta[table]
	}
	return nil
}

// cursorFieldMeta resolves the metadata of the active sheet field for the type
// line. It looks up the cached column metadata for the current pane's table
// and matches the column NAME at the cursor index (the frame's column order
// need not match the engine's ordinal order). When no meta is available (file
// mode, a not-yet-warmed table, or a nil pane/frame) it falls back to the
// frame DType string for dataType and empty strings for the rest.
func (a *App) cursorFieldMeta() (dataType, notNull, defaultVal, comment string) {
	p := a.pane()
	if p == nil {
		return "", "", "", ""
	}
	f := p.Current()
	if f == nil {
		return "", "", "", ""
	}
	// When a sheet field filter is active, SheetField is an index into the
	// matched column list, not a raw column index. Resolve the real column
	// via the matched set so the type/metadata matches the field under the
	// cursor.
	field := clamp(p.SheetField, 0, f.NumCols()-1)
	if matched := sheetMatchedCols(f, string(p.SheetFilter)); len(matched) > 0 {
		idx := clamp(p.SheetField, 0, len(matched)-1)
		field = matched[idx]
	}
	name := f.Columns[field].Name

	if meta := a.columnMetaFor(p.Title); meta != nil {
		for _, m := range meta {
			if m.Name == name {
				dataType = m.DataType
				notNull = m.IsNullable
				defaultVal = m.Default
				comment = m.Comment
				return
			}
		}
	}
	// Fall back to the frame's own DType.
	dataType = f.Columns[field].Type.String()
	return
}
