package sqlitebe

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

func TestRawSqlCommandReturningIsDQL(t *testing.T) {
	db := newTestDB(t)

	res, err := db.RawSqlCommand("INSERT INTO users (name) VALUES ('x'), ('y') RETURNING id, name")
	if err != nil {
		t.Fatalf("insert returning: %v", err)
	}
	if !res.IsDQL {
		t.Fatal("INSERT ... RETURNING must be routed as DQL (result set)")
	}
	if len(res.Records) != 2 || res.Records[0][1] != "x" || res.Records[1][1] != "y" {
		t.Fatalf("returning rows = %v", res.Records)
	}

	res, err = db.RawSqlCommand("DELETE FROM users WHERE name = 'y' RETURNING id")
	if err != nil {
		t.Fatalf("delete returning: %v", err)
	}
	if !res.IsDQL || len(res.Records) != 1 {
		t.Fatalf("DELETE ... RETURNING: IsDQL=%v records=%v", res.IsDQL, res.Records)
	}
}

func TestListPrimaryKeys(t *testing.T) {
	db := newTestDB(t)

	// users.id is declared INTEGER PRIMARY KEY in newTestDB.
	pks, err := db.ListPrimaryKeys("users")
	if err != nil {
		t.Fatalf("ListPrimaryKeys(users): %v", err)
	}
	if len(pks) != 1 || pks[0] != "id" {
		t.Fatalf("expected [id], got %v", pks)
	}

	// A table without a primary key yields an empty list.
	if _, err := db.RawExec(`CREATE TABLE logs (msg TEXT)`); err != nil {
		t.Fatalf("create logs: %v", err)
	}
	pks, err = db.ListPrimaryKeys("logs")
	if err != nil {
		t.Fatalf("ListPrimaryKeys(logs): %v", err)
	}
	if len(pks) != 0 {
		t.Fatalf("expected no primary keys for logs, got %v", pks)
	}

	// A composite primary key comes back in pk-ordinal order.
	if _, err := db.RawExec(`CREATE TABLE kv (k TEXT, v TEXT, PRIMARY KEY (k, v))`); err != nil {
		t.Fatalf("create kv: %v", err)
	}
	pks, err = db.ListPrimaryKeys("kv")
	if err != nil {
		t.Fatalf("ListPrimaryKeys(kv): %v", err)
	}
	if len(pks) != 2 || pks[0] != "k" || pks[1] != "v" {
		t.Fatalf("expected [k v], got %v", pks)
	}
}

func TestColumnsMeta(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.RawExec(`CREATE TABLE meta_demo (
		id INTEGER NOT NULL PRIMARY KEY,
		email TEXT NOT NULL DEFAULT 'unknown',
		note TEXT
	)`); err != nil {
		t.Fatalf("create meta_demo: %v", err)
	}

	cols, err := db.ColumnsMeta("meta_demo")
	if err != nil {
		t.Fatalf("ColumnsMeta(meta_demo): %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d (%v)", len(cols), cols)
	}

	// id: NOT NULL INTEGER PRIMARY KEY, no default literal.
	id := cols[0]
	if id.Name != "id" || id.DataType != "INTEGER" || id.IsNullable != "NO" || id.Default != "" || id.Comment != "" {
		t.Fatalf("id column = %+v", id)
	}

	// email: NOT NULL with a default literal.
	email := cols[1]
	if email.Name != "email" || email.DataType != "TEXT" || email.IsNullable != "NO" || email.Default != "'unknown'" || email.Comment != "" {
		t.Fatalf("email column = %+v", email)
	}

	// note: nullable, no default.
	note := cols[2]
	if note.Name != "note" || note.DataType != "TEXT" || note.IsNullable != "YES" || note.Default != "" || note.Comment != "" {
		t.Fatalf("note column = %+v", note)
	}
}
