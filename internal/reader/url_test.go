package reader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"http://example.com/a.csv", true},
		{"https://example.com/a.csv", true},
		{"HTTPS://EXAMPLE.COM/A.CSV", true},
		{"ftp://example.com/a.csv", false},
		{"/tmp/a.csv", false},
		{"a.csv", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsURL(tt.in); got != tt.want {
			t.Errorf("IsURL(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFromURL(t *testing.T) {
	const body = "a,b\n1,2\n"
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		w.Write([]byte(body))
	}))
	defer srv.Close()

	src, err := FromURL(srv.URL + "/data.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(src.LocalPath)

	if gotUA != "sqltui" {
		t.Errorf("user agent = %q, want %q", gotUA, "sqltui")
	}
	if !src.URL || src.Stdin {
		t.Errorf("source flags = %+v, want URL=true Stdin=false", src)
	}
	if src.Path != srv.URL+"/data.csv" {
		t.Errorf("Path = %q", src.Path)
	}
	b, err := src.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != body {
		t.Errorf("downloaded = %q, want %q", b, body)
	}
	if src.Name() != "data" {
		t.Errorf("Name() = %q, want %q", src.Name(), "data")
	}
}

func TestFromURLHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if _, err := FromURL(srv.URL + "/missing.csv"); err == nil {
		t.Fatal("want error on 404")
	}
}

func TestFromURLFollowsRedirect(t *testing.T) {
	const body = "x\n1\n"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, srv.URL+"/final", http.StatusFound)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()

	src, err := FromURL(srv.URL + "/start")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(src.LocalPath)
	b, _ := src.Bytes()
	if string(b) != body {
		t.Errorf("downloaded = %q, want %q", b, body)
	}
}

func TestDetectURLWithQuery(t *testing.T) {
	tests := []struct {
		path string
		want Format
	}{
		{"https://example.com/data.csv?token=abc", FormatCSV},
		{"https://example.com/data.parquet?a=1&b=2", FormatParquet},
		{"https://example.com/data.json#frag", FormatJSON},
		{"https://example.com/data", ""},
		{"local.csv?token=x", ""}, // local names keep their '?' verbatim
		{"plain.csv", FormatCSV},
	}
	for _, tt := range tests {
		if got := Detect(tt.path); got != tt.want {
			t.Errorf("Detect(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestSourceCleanup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("x\n1\n"))
	}))
	defer srv.Close()

	src, err := FromURL(srv.URL + "/data.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src.LocalPath); err != nil {
		t.Fatalf("spool file missing before cleanup: %v", err)
	}
	src.Cleanup()
	if _, err := os.Stat(src.LocalPath); !os.IsNotExist(err) {
		t.Errorf("spool file still present after cleanup: %v", err)
	}

	// Cleanup on a plain file source must not delete the file.
	local := fixtureFileForCleanup(t)
	fsrc, err := FromFile(local)
	if err != nil {
		t.Fatal(err)
	}
	fsrc.Cleanup()
	if _, err := os.Stat(local); err != nil {
		t.Errorf("local file removed by cleanup: %v", err)
	}
}

func fixtureFileForCleanup(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "keep.csv")
	if err := os.WriteFile(p, []byte("a\n1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
