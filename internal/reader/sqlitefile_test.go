package reader_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/reader"
)

// writeDatabase creates a database file fixture with two tables covering
// INTEGER/TEXT/REAL/BLOB columns, NULLs, and binary vs utf8 blobs.
func writeDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE people (id INTEGER, name TEXT, height REAL, photo BLOB)`,
		`CREATE TABLE pets (name TEXT)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO people VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		int64(1), "alice", 1.75, []byte{0x00, 0xff},
		int64(2), nil, nil, []byte("plain text"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO pets VALUES ('rex')`); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteFileReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	writeDatabase(t, path)

	src, err := reader.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := reader.For(reader.FormatSQLite)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := r.Read(src, reader.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	if frames[0].Name != "people" || frames[1].Name != "pets" {
		t.Fatalf("frame names = %q, %q, want people, pets", frames[0].Name, frames[1].Name)
	}

	f := frames[0].Frame
	if want := []string{"id", "name", "height", "photo"}; !reflect.DeepEqual(f.ColumnNames(), want) {
		t.Fatalf("columns = %v, want %v", f.ColumnNames(), want)
	}
	wantCols := []struct {
		dtype data.DType
		cells []any
	}{
		{data.TypeInt, []any{int64(1), int64(2)}},
		{data.TypeString, []any{"alice", nil}},
		{data.TypeFloat, []any{1.75, nil}},
		{data.TypeString, []any{"0x00ff", "plain text"}},
	}
	for i, want := range wantCols {
		if f.Columns[i].Type != want.dtype {
			t.Errorf("column %q type = %v, want %v", f.Columns[i].Name, f.Columns[i].Type, want.dtype)
		}
		if !reflect.DeepEqual(f.Columns[i].Cells, want.cells) {
			t.Errorf("column %q cells = %#v, want %#v", f.Columns[i].Name, f.Columns[i].Cells, want.cells)
		}
	}

	pets := frames[1].Frame
	if want := []any{"rex"}; !reflect.DeepEqual(pets.Columns[0].Cells, want) {
		t.Errorf("pets cells = %#v, want %#v", pets.Columns[0].Cells, want)
	}
}

func TestSQLiteEncryptedKeyRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	writeDatabase(t, path)

	src, err := reader.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := reader.For(reader.FormatSQLite)
	opt := reader.DefaultOptions()
	opt.Key = "secret"
	_, err = r.Read(src, opt)
	if err == nil || !strings.Contains(err.Error(), "encrypted") {
		t.Errorf("got %v, want an 'encrypted databases not supported' error", err)
	}
}

func TestSQLiteRejectsNonDatabaseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.db")
	if err := os.WriteFile(path, []byte("this is definitely not a database file"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := reader.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := reader.For(reader.FormatSQLite)
	if _, err := r.Read(src, reader.DefaultOptions()); err == nil {
		t.Error("expected error for a non-database file")
	}
}
