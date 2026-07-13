package ui

import (
	"reflect"
	"testing"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
	"github.com/LinPr/sqltui/internal/query"
	"github.com/LinPr/sqltui/internal/reader"
)

// scFrame builds a small two-column frame.
func scFrame() *data.Frame {
	return &data.Frame{Columns: []data.Column{
		{Name: "name", Type: data.TypeString, Cells: []any{"a", "b"}},
		{Name: "age", Type: data.TypeInt, Cells: []any{int64(1), int64(2)}},
	}}
}

// scCountingBackend counts catalog calls so tests can assert laziness and
// caching.
type scCountingBackend struct {
	nsCalls    int
	tblCalls   int
	fetchCalls []string // "ns/table" per FetchTable call
}

func (b *scCountingBackend) Kind() string  { return "sqlite" }
func (b *scCountingBackend) Title() string { return "test" }
func (b *scCountingBackend) Run(string) (db.Result, error) {
	return db.Result{}, nil
}
func (b *scCountingBackend) Namespaces() ([]string, error) {
	b.nsCalls++
	return []string{"main"}, nil
}
func (b *scCountingBackend) Tables(ns string) ([]string, error) {
	b.tblCalls++
	return []string{"users", "orders"}, nil
}
func (b *scCountingBackend) FetchTable(ns, table string, limit int) (*data.Frame, error) {
	b.fetchCalls = append(b.fetchCalls, ns+"/"+table)
	return &data.Frame{Columns: []data.Column{
		{Name: "id", Type: data.TypeInt},
		{Name: "label", Type: data.TypeString},
	}}, nil
}
func (b *scCountingBackend) PrimaryKeys(string, string) ([]string, error) { return nil, nil }
func (b *scCountingBackend) Close() error                                  { return nil }

func TestCompletionSchemaEnginePath(t *testing.T) {
	eng, err := query.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	f := scFrame()
	if err := eng.Register("people", f); err != nil {
		t.Fatal(err)
	}

	a := New(Options{
		Engine: eng,
		Frames: []reader.NamedFrame{{Name: "people", Frame: f}},
	})

	sc := a.CompletionSchema()
	if !reflect.DeepEqual(sc.Current, []string{"name", "age"}) {
		t.Errorf("Current = %v", sc.Current)
	}
	if !reflect.DeepEqual(sc.Tables["people"], []string{"name", "age"}) {
		t.Errorf("Tables[people] = %v", sc.Tables["people"])
	}
	// The engine accessor synced the frame under view as "_".
	if !reflect.DeepEqual(sc.Tables["_"], []string{"name", "age"}) {
		t.Errorf("Tables[_] = %v", sc.Tables["_"])
	}
}

func TestCompletionSchemaBackendPath(t *testing.T) {
	be := &scCountingBackend{}
	a := New(Options{Backend: be})

	// Nothing cached and nothing fetched before a warm-up: the schema is
	// empty and cheap (safe on the update loop).
	sc := a.CompletionSchema()
	if sc.Tables != nil {
		t.Fatalf("tables before warm-up: %v", sc.Tables)
	}
	if be.nsCalls != 0 || be.tblCalls != 0 {
		t.Fatal("CompletionSchema touched the live connection")
	}

	// The plain warm-up lists tables once; columns stay pending.
	a.WarmCompletionSchema()
	sc = a.CompletionSchema()
	if len(sc.Tables) != 2 || sc.Tables["users"] != nil || sc.Tables["orders"] != nil {
		t.Fatalf("tables after listing: %v", sc.Tables)
	}
	a.WarmCompletionSchema()
	if be.nsCalls != 1 || be.tblCalls != 1 {
		t.Errorf("listing not cached: ns=%d tbl=%d", be.nsCalls, be.tblCalls)
	}

	// Per-table columns are fetched on first use only.
	a.WarmCompletionSchema("users")
	sc = a.CompletionSchema()
	if !reflect.DeepEqual(sc.Tables["users"], []string{"id", "label"}) {
		t.Errorf("users columns = %v", sc.Tables["users"])
	}
	if sc.Tables["orders"] != nil {
		t.Errorf("orders fetched eagerly: %v", sc.Tables["orders"])
	}
	a.WarmCompletionSchema("users")
	if !reflect.DeepEqual(be.fetchCalls, []string{"main/users"}) {
		t.Errorf("fetch calls = %v, want exactly one", be.fetchCalls)
	}

	// Unknown tables are ignored.
	a.WarmCompletionSchema("ghost")
	if len(be.fetchCalls) != 1 {
		t.Errorf("fetch calls after unknown table = %v", be.fetchCalls)
	}
}

func TestCompletionSchemaBackendSwapResetsCache(t *testing.T) {
	be1 := &scCountingBackend{}
	a := New(Options{Backend: be1})
	a.WarmCompletionSchema("users")
	if len(a.CompletionSchema().Tables["users"]) == 0 {
		t.Fatal("warm-up did not populate the first connection")
	}

	// A new connection (SetBackendMsg) invalidates the snapshot.
	be2 := &scCountingBackend{}
	a.Update(SetBackendMsg{Backend: be2})
	if tables := a.CompletionSchema().Tables; tables != nil {
		t.Fatalf("stale schema after backend swap: %v", tables)
	}

	a.WarmCompletionSchema("orders")
	sc := a.CompletionSchema()
	if !reflect.DeepEqual(sc.Tables["orders"], []string{"id", "label"}) {
		t.Errorf("orders columns after swap = %v", sc.Tables["orders"])
	}
	if be2.nsCalls != 1 || len(be2.fetchCalls) != 1 {
		t.Errorf("second connection calls: ns=%d fetch=%v", be2.nsCalls, be2.fetchCalls)
	}
	if len(be1.fetchCalls) != 1 {
		t.Errorf("first connection touched after swap: %v", be1.fetchCalls)
	}
}

func TestCompletionSchemaNoFacilities(t *testing.T) {
	a := New(Options{})
	sc := a.CompletionSchema()
	if sc.Current != nil || sc.Tables != nil {
		t.Errorf("schema = %+v, want empty", sc)
	}
	a.WarmCompletionSchema("anything") // must be a no-op, not a panic
}
