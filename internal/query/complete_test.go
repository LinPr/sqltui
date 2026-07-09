package query

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/LinPr/sqltui/internal/data"
)

// completionSchema is the fixture most Suggest tests run against.
func completionSchema() Schema {
	return Schema{
		Tables: map[string][]string{
			"users":  {"id", "name", "email"},
			"orders": {"id", "user_id", "total"},
			"_":      {"name", "age"},
		},
		Current: []string{"name", "age"},
	}
}

// texts projects suggestions onto their replacement texts.
func texts(sug []Suggestion) []string {
	if len(sug) == 0 {
		return nil
	}
	out := make([]string, len(sug))
	for i, s := range sug {
		out[i] = s.Text
	}
	return out
}

func TestSuggestTableContext(t *testing.T) {
	sc := completionSchema()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty token after FROM suggests all tables", "select * from ", []string{"_", "orders", "users"}},
		{"prefix after FROM", "select * from u", []string{"users"}},
		{"after JOIN", "select * from users join or", []string{"orders"}},
		{"after INTO", "insert into u", []string{"users"}},
		{"after UPDATE", "update o", []string{"orders"}},
		{"after TABLE", "drop table use", []string{"users"}},
		{"underscore table", "select * from _", []string{"_"}},
		{"no match", "select * from zz", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Suggest(tc.input, len(tc.input), sc)
			if !reflect.DeepEqual(texts(got), tc.want) {
				t.Errorf("Suggest(%q) = %v, want %v", tc.input, texts(got), tc.want)
			}
			for _, s := range got {
				if s.Kind != KindTable {
					t.Errorf("%q: kind = %q, want table", s.Text, s.Kind)
				}
			}
		})
	}
}

func TestSuggestTableContextDetailShowsColumnCount(t *testing.T) {
	sc := completionSchema()
	got := Suggest("from users", len("from users"), sc)
	if len(got) != 1 || got[0].Detail != "3 cols" {
		t.Errorf("got %+v, want users with detail \"3 cols\"", got)
	}

	// Unknown column sets (live tables not fetched yet) carry no detail.
	sc.Tables["pending"] = nil
	got = Suggest("from pend", len("from pend"), sc)
	if len(got) != 1 || got[0].Detail != "" {
		t.Errorf("got %+v, want pending with empty detail", got)
	}
}

func TestSuggestDotCompletion(t *testing.T) {
	sc := completionSchema()
	tests := []struct {
		name   string
		input  string
		cursor int // -1 = end
		want   []string
		detail string
	}{
		{"table dot empty token", "select users. from users", len("select users."), []string{"email", "id", "name"}, "users"},
		{"table dot with prefix", "select users.na from users", len("select users.na"), []string{"name"}, "users"},
		{"alias without AS", "select u.na from users u", len("select u.na"), []string{"name"}, "users"},
		{"alias with AS", "select u.em from users as u", len("select u.em"), []string{"email"}, "users"},
		{"alias declared after cursor", "select o.to from users u join orders o on o.user_id = u.id", len("select o.to"), []string{"total"}, "orders"},
		{"comma-separated FROM list", "select b.tot from users a, orders b", len("select b.tot"), []string{"total"}, "orders"},
		{"quoted table with alias", `select u.na from "users" u`, len("select u.na"), []string{"name"}, "users"},
		{"qualifier case-insensitive", "select USERS.i from users", len("select USERS.i"), []string{"id"}, "users"},
		{"unknown qualifier", "select x.na from users u", len("select x.na"), nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Suggest(tc.input, tc.cursor, sc)
			if !reflect.DeepEqual(texts(got), tc.want) {
				t.Fatalf("Suggest(%q) = %v, want %v", tc.input, texts(got), tc.want)
			}
			for _, s := range got {
				if s.Kind != KindColumn || s.Detail != tc.detail {
					t.Errorf("%q: kind=%q detail=%q, want column of %q", s.Text, s.Kind, s.Detail, tc.detail)
				}
			}
		})
	}
}

func TestSuggestColumnContext(t *testing.T) {
	sc := completionSchema()

	// After SELECT: columns first, then functions, then keywords.
	got := Suggest("select n", len("select n"), sc)
	want := []string{"name", "nullif(", "not", "null"}
	if !reflect.DeepEqual(texts(got), want) {
		t.Fatalf("after SELECT: %v, want %v", texts(got), want)
	}
	kinds := []string{KindColumn, KindFunction, KindKeyword, KindKeyword}
	for i, s := range got {
		if s.Kind != kinds[i] {
			t.Errorf("%q kind = %q, want %q", s.Text, s.Kind, kinds[i])
		}
	}

	// The same ranking holds after WHERE / AND / comma / open paren.
	for _, input := range []string{
		"select * from users where n",
		"select * from users where id = 1 and n",
		"select name, n",
		"select count(n",
	} {
		got := Suggest(input, len(input), sc)
		if !reflect.DeepEqual(texts(got), want) {
			t.Errorf("Suggest(%q) = %v, want %v", input, texts(got), want)
		}
	}

	// Table names are not offered in a column position.
	got = Suggest("select u", len("select u"), sc)
	for _, s := range got {
		if s.Kind == KindTable {
			t.Errorf("table %q offered in column context", s.Text)
		}
	}
}

func TestSuggestColumnContextEmptyTokenListsColumnsOnly(t *testing.T) {
	sc := completionSchema()
	got := Suggest("select * from users where ", len("select * from users where "), sc)
	// Current columns first (sorted), then per-table columns (deduped).
	want := []string{"age", "name", "id", "total", "user_id", "email"}
	if !reflect.DeepEqual(texts(got), want) {
		t.Fatalf("empty token after WHERE: %v, want %v", texts(got), want)
	}
	for _, s := range got {
		if s.Kind != KindColumn {
			t.Errorf("%q: kind = %q, want column only", s.Text, s.Kind)
		}
	}
}

func TestSuggestColumnDetailNamesOwningTable(t *testing.T) {
	sc := completionSchema()
	got := Suggest("select tot", len("select tot"), sc)
	if len(got) == 0 || got[0].Text != "total" || got[0].Detail != "orders" {
		t.Errorf("got %+v, want total owned by orders", got)
	}
	// Current-frame columns carry no owning table.
	got = Suggest("select ag", len("select ag"), sc)
	if len(got) == 0 || got[0].Text != "age" || got[0].Detail != "" {
		t.Errorf("got %+v, want age with empty detail", got)
	}
}

func TestSuggestOtherContext(t *testing.T) {
	sc := completionSchema()

	// Bare token: columns > tables > functions > keywords.
	got := Suggest("u", 1, sc)
	want := []string{"user_id", "users", "upper(", "union", "update", "using"}
	if !reflect.DeepEqual(texts(got), want) {
		t.Fatalf("bare token: %v, want %v", texts(got), want)
	}
	kinds := []string{KindColumn, KindTable, KindFunction, KindKeyword, KindKeyword, KindKeyword}
	for i, s := range got {
		if s.Kind != kinds[i] {
			t.Errorf("%q kind = %q, want %q", s.Text, s.Kind, kinds[i])
		}
	}

	// Empty token in an unclassified position yields nothing.
	for _, input := range []string{"", "select * ", "where a = "} {
		if got := Suggest(input, len(input), sc); got != nil {
			t.Errorf("Suggest(%q) = %v, want nil", input, texts(got))
		}
	}
}

func TestSuggestCaseFollowsTypedPrefix(t *testing.T) {
	sc := completionSchema()
	tests := []struct {
		input string
		want  string
		kind  string
	}{
		{"SEL", "SELECT", KindKeyword},
		{"sel", "select", KindKeyword},
		{"Sel", "SELECT", KindKeyword}, // any uppercase -> uppercase
		{"select COU", "COUNT(", KindFunction},
		{"select cou", "count(", KindFunction},
		{"select NA", "name", KindColumn}, // identifiers stay verbatim
		{"from USER", "users", KindTable}, // ... tables too
	}
	for _, tc := range tests {
		got := Suggest(tc.input, len(tc.input), sc)
		if len(got) == 0 || got[0].Text != tc.want || got[0].Kind != tc.kind {
			t.Errorf("Suggest(%q) = %+v, want first %q (%s)", tc.input, got, tc.want, tc.kind)
		}
	}
}

func TestSuggestFunctionsCarryTrailingParen(t *testing.T) {
	got := Suggest("select coal", len("select coal"), Schema{})
	if len(got) != 1 || got[0].Text != "coalesce(" || got[0].Kind != KindFunction {
		t.Errorf("got %+v, want coalesce(", got)
	}
}

func TestSuggestCursorHandling(t *testing.T) {
	sc := completionSchema()

	// Cursor mid-token completes the prefix before it.
	input := "select nameless from users"
	got := Suggest(input, len("select nam"), sc)
	if len(got) == 0 || got[0].Text != "name" {
		t.Errorf("mid-token: %v", texts(got))
	}

	// Out-of-range cursors clamp instead of panicking.
	if got := Suggest("se", -5, sc); got != nil {
		t.Errorf("negative cursor: %v", texts(got))
	}
	if got := Suggest("se", 99, sc); len(got) == 0 || got[0].Text != "select" {
		t.Errorf("cursor past end: %v", texts(got))
	}
}

func TestSuggestDedupes(t *testing.T) {
	sc := completionSchema()
	// "name" exists in Current, "users" and "_"; it must appear exactly once
	// (the Current entry, without an owning table).
	got := Suggest("select nam", len("select nam"), sc)
	count := 0
	for _, s := range got {
		if s.Text == "name" {
			count++
			if s.Detail != "" {
				t.Errorf("dedupe kept table entry %+v over the current-frame one", s)
			}
		}
	}
	if count != 1 {
		t.Errorf("name appears %d times", count)
	}
}

func TestDotTable(t *testing.T) {
	sc := completionSchema()
	tests := []struct {
		name   string
		input  string
		cursor int
		want   string
		ok     bool
	}{
		{"direct table", "select users. from users", len("select users."), "users", true},
		{"alias", "select u.na from users u", len("select u.na"), "users", true},
		{"unknown qualifier", "select x. from users", len("select x."), "", false},
		{"not a dot position", "select na", len("select na"), "", false},
		{"start of input", "x", 0, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DotTable(tc.input, tc.cursor, sc)
			if got != tc.want || ok != tc.ok {
				t.Errorf("DotTable(%q) = %q,%v want %q,%v", tc.input, got, ok, tc.want, tc.ok)
			}
		})
	}

	// Canonical schema spelling wins over the typed qualifier.
	mixed := Schema{Tables: map[string][]string{"Users": {"id"}}}
	if got, ok := DotTable("select users.i", len("select users."), mixed); !ok || got != "Users" {
		t.Errorf("canonical spelling: %q,%v", got, ok)
	}
}

func TestScanAliases(t *testing.T) {
	got := scanAliases("SELECT * FROM users AS u, extra e JOIN main.orders o ON o.user_id = u.id WHERE u.id > 0")
	want := map[string]string{"u": "users", "o": "orders", "e": "extra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scanAliases = %v, want %v", got, want)
	}

	// Keywords never become aliases; string literals are skipped.
	got = scanAliases("select * from users where name = 'join fake f'")
	if len(got) != 0 {
		t.Errorf("aliases from keyword/literal: %v", got)
	}
}

func TestKeywords(t *testing.T) {
	kw := Keywords()
	if len(kw) == 0 {
		t.Fatal("Keywords returned nothing")
	}
	if !sort.StringsAreSorted(kw) {
		t.Error("Keywords not sorted")
	}
	for _, want := range []string{"SELECT", "FROM", "WHERE", "GROUP", "ORDER", "LIMIT", "JOIN", "CASE", "CAST", "COUNT", "AVG"} {
		found := false
		for _, k := range kw {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Keywords missing %q", want)
		}
	}
	// Returned slice must be a copy.
	kw[0] = "MUTATED"
	if Keywords()[0] == "MUTATED" {
		t.Error("Keywords returns shared backing array")
	}
}

// TestEngineTableColumns lives here rather than in engine_test.go because it
// exists for the completion schema: Suggest consumes its output.
func TestEngineTableColumns(t *testing.T) {
	e := newTestEngine(t)

	if got, err := e.TableColumns(); err != nil || len(got) != 0 {
		t.Fatalf("empty engine: %v, %v", got, err)
	}

	if err := e.Register("people", &data.Frame{Columns: []data.Column{
		{Name: "name", Type: data.TypeString, Cells: []any{"a"}},
		{Name: "age", Type: data.TypeInt, Cells: []any{int64(1)}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := e.Register("_", fixtureFrame()); err != nil {
		t.Fatal(err)
	}

	got, err := e.TableColumns()
	if err != nil {
		t.Fatalf("TableColumns: %v", err)
	}
	want := map[string][]string{
		"people": {"name", "age"}, // declaration order, not sorted
		"_":      {"s", "i", "f", "b", "d", "dt", `weird "name`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TableColumns = %v, want %v", got, want)
	}

	// Feeding the result to Suggest wires dot completion to real tables.
	sc := Schema{Tables: got}
	sug := Suggest("select people.a from people", len("select people.a"), sc)
	if len(sug) != 1 || sug[0].Text != "age" {
		t.Errorf("dot completion over engine schema: %v", texts(sug))
	}

	if err := e.Unregister("people"); err != nil {
		t.Fatal(err)
	}
	got, err = e.TableColumns()
	if err != nil {
		t.Fatal(err)
	}
	if _, still := got["people"]; still {
		t.Error("unregistered table still reported")
	}
}

func TestSuggestKeywordListsSorted(t *testing.T) {
	if !sort.StringsAreSorted(sqlKeywords) {
		t.Error("sqlKeywords not sorted")
	}
	if !sort.StringsAreSorted(sqlFunctions) {
		t.Error("sqlFunctions not sorted")
	}
	for _, f := range sqlFunctions {
		if strings.ToUpper(f) != f {
			t.Errorf("function %q not uppercase", f)
		}
	}
}
