package reader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// urlTimeout bounds a whole download.
const urlTimeout = 60 * time.Second

// IsURL reports whether s looks like an http(s) URL.
func IsURL(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// FromURL downloads an http(s) URL to a temporary file and wraps it in a
// Source. Redirects are followed; the request carries a plain user agent.
func FromURL(rawurl string) (*Source, error) {
	client := &http.Client{Timeout: urlTimeout}
	req, err := http.NewRequest(http.MethodGet, rawurl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sqltui")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", rawurl, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: %s", rawurl, resp.Status)
	}

	tmp, err := os.CreateTemp("", "sqltui-url-*")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, fmt.Errorf("download %s: %w", rawurl, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	return &Source{Path: rawurl, LocalPath: tmp.Name(), URL: true}, nil
}
