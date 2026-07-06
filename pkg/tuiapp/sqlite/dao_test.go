package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()

	file := filepath.Join(t.TempDir(), "test.db")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}

	db, err := NewDB(file)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		DbClinet = nil
	})

	if _, err := db.RawExec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.RawExec(`INSERT INTO users (name) VALUES ('alice'), ('bob')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return db
}

func TestNewDBRejectsMissingFile(t *testing.T) {
	if _, err := NewDB(filepath.Join(t.TempDir(), "no-such.db")); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestListTables(t *testing.T) {
	db := newTestDB(t)

	tables, err := db.ListTables()
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if len(tables) != 1 || tables[0] != "users" {
		t.Fatalf("expected [users], got %v", tables)
	}
}

func TestFetchTableFields(t *testing.T) {
	db := newTestDB(t)

	fields, err := db.FetchTableFields("users")
	if err != nil {
		t.Fatalf("FetchTableFields: %v", err)
	}
	if len(fields) != 2 || fields[0] != "id" || fields[1] != "name" {
		t.Fatalf("expected [id name], got %v", fields)
	}
}

func TestRawSqlCommandRoutesDQLAndExec(t *testing.T) {
	db := newTestDB(t)

	res, err := db.RawSqlCommand("SELECT id, name FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if !res.IsDQL {
		t.Fatal("SELECT should be routed as DQL")
	}
	if len(res.Fields) != 2 || len(res.Records) != 2 || res.Records[0][1] != "alice" {
		t.Fatalf("unexpected result: fields=%v records=%v", res.Fields, res.Records)
	}

	res, err = db.RawSqlCommand("UPDATE users SET name = 'carol' WHERE name = 'bob'")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res.IsDQL {
		t.Fatal("UPDATE should be routed as exec")
	}
	if n, _ := res.Result.RowsAffected(); n != 1 {
		t.Fatalf("expected 1 row affected, got %d", n)
	}
}

func TestFetchTableRecordsQuotedIdentifier(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.RawExec(`CREATE TABLE "odd name" (v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.RawExec(`INSERT INTO "odd name" VALUES ('x')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	records, err := db.FetchTableRecords("odd name")
	if err != nil {
		t.Fatalf("FetchTableRecords: %v", err)
	}
	if len(records) != 1 || records[0][0] != "x" {
		t.Fatalf("unexpected records: %v", records)
	}
}
