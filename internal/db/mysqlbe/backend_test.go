package mysqlbe

import (
	"strings"
	"testing"
)

func TestConfigDsn(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "typical",
			cfg:  Config{UserName: "root", Password: "secret", Host: "127.0.0.1", Port: "3306", DbName: "shop"},
			want: "root:secret@tcp(127.0.0.1:3306)/shop?charset=utf8&parseTime=true",
		},
		{
			name: "empty password",
			cfg:  Config{UserName: "u", Password: "", Host: "db.local", Port: "3307", DbName: "d"},
			want: "u:@tcp(db.local:3307)/d?charset=utf8&parseTime=true",
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

func TestCurrentNamespace(t *testing.T) {
	b := &Backend{db: &DB{dbName: "shop"}}
	if got := b.CurrentNamespace(); got != "shop" {
		t.Errorf("CurrentNamespace() = %q, want shop", got)
	}
	if got := (&Backend{}).CurrentNamespace(); got != "" {
		t.Errorf("CurrentNamespace() without connection = %q, want empty", got)
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"users", "`users`"},
		{"odd`name", "`odd``name`"},
	}
	for _, tt := range tests {
		if got := quoteIdent(tt.in); got != tt.want {
			t.Errorf("quoteIdent(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHasReturningClause(t *testing.T) {
	tests := []struct {
		stmt string
		want bool
	}{
		{"INSERT INTO t (a) VALUES (1) RETURNING a", true},
		{"delete from t where a = 1 returning *", true},
		{"UPDATE t SET a = 2", false},
		{"INSERT INTO returning_log (a) VALUES (1)", false},
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
	pks, err := b.PrimaryKeys("shop", "users")
	if err == nil {
		t.Fatalf("expected error from PrimaryKeys with no connection, got pks=%v", pks)
	}
}
