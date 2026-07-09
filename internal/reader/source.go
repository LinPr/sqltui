package reader

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// utf8BOM is the UTF-8 byte-order mark some tools prepend to text files.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM returns r with a leading UTF-8 BOM (EF BB BF) removed, if present.
func stripBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && bytes.Equal(b, utf8BOM) {
		br.Discard(3)
	}
	return br
}

// stripBOMBytes returns b with a leading UTF-8 BOM removed, if present.
func stripBOMBytes(b []byte) []byte {
	return bytes.TrimPrefix(b, utf8BOM)
}

// Source is one input to load: a local file, standard input, or a URL that
// has already been downloaded to a temporary file.
type Source struct {
	// Path is the original user-supplied location (file path or URL).
	Path string
	// LocalPath is the on-disk file to read. For plain files it equals Path;
	// for stdin and URLs it points at a temporary spool file.
	LocalPath string
	// Stdin marks input piped from standard input ("-").
	Stdin bool
	// URL marks input downloaded from http(s).
	URL bool
}

// Name returns a short display name for tab titles.
func (s *Source) Name() string {
	if s.Stdin {
		return "stdin"
	}
	base := filepath.Base(s.Path)
	if i := strings.Index(base, "?"); i > 0 {
		base = base[:i]
	}
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// Open opens the underlying local file.
func (s *Source) Open() (io.ReadCloser, error) {
	return os.Open(s.LocalPath)
}

// Bytes reads the whole source into memory.
func (s *Source) Bytes() ([]byte, error) {
	return os.ReadFile(s.LocalPath)
}

// Cleanup removes the temporary spool file backing a stdin or URL source.
// It is a no-op for plain local files. Safe to call once every reader is
// done: readers fully materialize their frames before returning.
func (s *Source) Cleanup() {
	if s.Stdin || s.URL {
		os.Remove(s.LocalPath)
	}
}

// FromFile builds a Source for a local file path.
func FromFile(path string) (*Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}
	return &Source{Path: path, LocalPath: path}, nil
}

// FromStdin spools standard input to a temporary file.
func FromStdin() (*Source, error) {
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, os.Stdin); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "sqltui-stdin-*")
	if err != nil {
		return nil, err
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	return &Source{Path: "-", LocalPath: tmp.Name(), Stdin: true}, nil
}
