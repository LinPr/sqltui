package sqlitebe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestBackend(t *testing.T) *Backend {
	t.Helper()

	file := filepath.Join(t.TempDir(), "adapter.db")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}

	be, err := Connect(Config{FilePath: file})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() {
		be.Close()
		DbClinet = nil
	})
	return be
}

func TestConnectRejectsMissingFile(t *testing.T) {
	if _, err := Connect(Config{FilePath: filepath.Join(t.TempDir(), "nope.db")}); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestBackendEndToEnd(t *testing.T) {
	be := newTestBackend(t)

	if got := be.Kind(); got != "sqlite" {
		t.Fatalf("Kind = %q, want sqlite", got)
	}
	if !strings.HasPrefix(be.Title(), "sqlite://") || !strings.HasSuffix(be.Title(), "adapter.db") {
		t.Fatalf("unexpected Title: %q", be.Title())
	}

	// exec: create + insert
	res, err := be.Run(`CREATE TABLE pets (id INTEGER PRIMARY KEY, name TEXT, sound TEXT)`)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.Exec == nil || res.Frame != nil {
		t.Fatalf("CREATE should produce an exec result, got %+v", res)
	}

	res, err = be.Run(`INSERT INTO pets (name, sound) VALUES ('cat', 'meow'), ('dog', 'woof'), ('fish', NULL)`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if res.Exec == nil {
		t.Fatal("INSERT should produce an exec result")
	}
	if res.Exec.RowsAffected != 3 {
		t.Fatalf("RowsAffected = %d, want 3", res.Exec.RowsAffected)
	}
	if !res.Exec.HasLastInsert {
		t.Fatal("sqlite exec results should report HasLastInsert")
	}
	if res.Exec.LastInsertID != 3 {
		t.Fatalf("LastInsertID = %d, want 3", res.Exec.LastInsertID)
	}

	// query
	res, err = be.Run(`SELECT id, name, sound FROM pets ORDER BY id`)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if res.Frame == nil || res.Exec != nil {
		t.Fatalf("SELECT should produce a frame, got %+v", res)
	}
	f := res.Frame
	if f.NumRows() != 3 || f.NumCols() != 3 {
		t.Fatalf("frame shape %dx%d, want 3x3", f.NumRows(), f.NumCols())
	}
	if got := f.ColumnNames(); got[0] != "id" || got[1] != "name" || got[2] != "sound" {
		t.Fatalf("unexpected columns: %v", got)
	}
	if f.Cell(0, 1) != "cat" || f.Cell(1, 2) != "woof" {
		t.Fatalf("unexpected cells: %v / %v", f.Cell(0, 1), f.Cell(1, 2))
	}
	// raw-byte scan renders NULL as an empty string cell
	if f.Cell(2, 2) != "" {
		t.Fatalf("NULL cell = %#v, want empty string", f.Cell(2, 2))
	}

	// namespaces / tables
	ns, err := be.Namespaces()
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if len(ns) != 1 || ns[0] != "" {
		t.Fatalf("Namespaces = %v, want [\"\"]", ns)
	}
	tables, err := be.Tables("ignored")
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	if len(tables) != 1 || tables[0] != "pets" {
		t.Fatalf("Tables = %v, want [pets]", tables)
	}

	// fetch table with limit
	frame, err := be.FetchTable("", "pets", 2)
	if err != nil {
		t.Fatalf("FetchTable: %v", err)
	}
	if frame.NumRows() != 2 || frame.NumCols() != 3 {
		t.Fatalf("FetchTable shape %dx%d, want 2x3", frame.NumRows(), frame.NumCols())
	}
}

func TestFetchTableQuotesIdentifier(t *testing.T) {
	be := newTestBackend(t)

	if _, err := be.Run(`CREATE TABLE "odd ""name" (v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := be.Run(`INSERT INTO "odd ""name" VALUES ('x')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	frame, err := be.FetchTable("", `odd "name`, 10)
	if err != nil {
		t.Fatalf("FetchTable: %v", err)
	}
	if frame.NumRows() != 1 || frame.Cell(0, 0) != "x" {
		t.Fatalf("unexpected frame: rows=%d cell=%v", frame.NumRows(), frame.Cell(0, 0))
	}
}

func TestRunEmptyStatement(t *testing.T) {
	be := newTestBackend(t)

	if _, err := be.Run("   "); err == nil {
		t.Fatal("expected error for empty statement")
	}
}
