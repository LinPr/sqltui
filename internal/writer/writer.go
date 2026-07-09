// Package writer exports frames to files. Each output format registers
// itself in the format registry via init().
package writer

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/LinPr/sqltui/internal/data"
)

// Format names a supported output format.
type Format string

const (
	FormatCSV      Format = "csv"
	FormatTSV      Format = "tsv"
	FormatJSON     Format = "json"
	FormatJSONL    Format = "jsonl"
	FormatParquet  Format = "parquet"
	FormatMarkdown Format = "markdown"
)

// Compression names parquet compression codecs.
type Compression string

const (
	CompressionNone   Compression = "none"
	CompressionSnappy Compression = "snappy"
	CompressionGzip   Compression = "gzip"
	CompressionZstd   Compression = "zstd"
	CompressionLZ4    Compression = "lz4"
	CompressionBrotli Compression = "brotli"
)

// Options carries all export knobs; format writers pick what applies.
type Options struct {
	Separator   rune        // csv field separator (default ',')
	Quote       rune        // csv quote char (default '"')
	Header      bool        // include header row (default true)
	Pretty      bool        // json: indent output
	Compression Compression // parquet codec (default snappy)
}

// DefaultOptions returns the option defaults shared by all formats.
func DefaultOptions() Options {
	return Options{
		Separator:   ',',
		Quote:       '"',
		Header:      true,
		Compression: CompressionSnappy,
	}
}

// Writer serializes a frame in one format.
type Writer interface {
	Write(w io.Writer, f *data.Frame, opt Options) error
}

var (
	regMu    sync.RWMutex
	registry = map[Format]Writer{}
)

// Register installs a writer for a format; called from init() in format files.
func Register(f Format, w Writer) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[f] = w
}

// For returns the writer registered for a format.
func For(f Format) (Writer, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	if w, ok := registry[f]; ok {
		return w, nil
	}
	return nil, fmt.Errorf("unsupported export format %q (supported: %s)", f, strings.Join(SupportedFormats(), ", "))
}

// SupportedFormats lists registered format names, sorted.
func SupportedFormats() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for f := range registry {
		names = append(names, string(f))
	}
	sort.Strings(names)
	return names
}
