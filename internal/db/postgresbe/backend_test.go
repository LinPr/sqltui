package postgresbe

import (
	"strings"
	"testing"
	"time"
)

func TestConfigDsn(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "typical",
			cfg:  Config{UserName: "postgres", Password: "pw", Host: "127.0.0.1", Port: "5432", DbName: "app", SslMode: "disable"},
			want: "postgres://postgres:pw@127.0.0.1:5432/app?sslmode=disable",
		},
		{
			name: "empty sslmode falls back to disable",
			cfg:  Config{UserName: "u", Password: "p", Host: "h", Port: "5433", DbName: "d"},
			want: "postgres://u:p@h:5433/d?sslmode=disable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Dsn(); got != tt.want {
				t.Fatalf("Dsn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentNamespaceFallback(t *testing.T) {
	// Without a live connection the schema falls back to the default.
	if got := (&Backend{}).CurrentNamespace(); got != "public" {
		t.Errorf("CurrentNamespace() without connection = %q, want public", got)
	}
	if got := (&Backend{db: &DB{}}).CurrentNamespace(); got != "public" {
		t.Errorf("CurrentNamespace() with dead handle = %q, want public", got)
	}
}

func TestCellValue(t *testing.T) {
	now := time.Date(2024, 5, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"nil stays nil", nil, nil},
		{"bytes become string", []byte("hi"), "hi"},
		{"time is kept", now, now},
		{"string passes through", "s", "s"},
		{"int64 passes through", int64(42), int64(42)},
		{"float64 passes through", 1.5, 1.5},
		{"bool passes through", true, true},
		{"other types rendered as string", int32(7), "7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cellValue(tt.in); got != tt.want {
				t.Fatalf("cellValue(%#v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestHasReturningClause(t *testing.T) {
	tests := []struct {
		stmt string
		want bool
	}{
		{"insert into t values (1) returning id", true},
		{"UPDATE t SET x = 1 RETURNING *", true},
		{"delete from t where id = 1", false},
	}
	for _, tt := range tests {
		words := strings.Fields(tt.stmt)
		if got := hasReturningClause(words); got != tt.want {
			t.Errorf("hasReturningClause(%q) = %v, want %v", tt.stmt, got, tt.want)
		}
	}
}

// TestPrimaryKeysWithoutConnection exercises the method on a Backend whose
// underlying handle is nil: it must return an error rather than panic.
func TestPrimaryKeysWithoutConnection(t *testing.T) {
	b := &Backend{db: &DB{}}
	pks, err := b.PrimaryKeys("public", "users")
	if err == nil {
		t.Fatalf("expected error from PrimaryKeys with no connection, got pks=%v", pks)
	}
}

// TestColumnsMetaWithoutConnection exercises the method on a Backend whose
// underlying handle is nil: it must return an error rather than panic.
func TestColumnsMetaWithoutConnection(t *testing.T) {
	b := &Backend{db: &DB{}}
	cols, err := b.ColumnsMeta("public", "users")
	if err == nil {
		t.Fatalf("expected error from ColumnsMeta with no connection, got cols=%v", cols)
	}
}
