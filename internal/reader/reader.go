// Package reader loads tabular data files into frames. Each supported file
// format registers itself in the format registry via init().
package reader

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/LinPr/sqltui/internal/data"
)

// Format names a supported input format.
type Format string

const (
	FormatDSV      Format = "dsv"
	FormatCSV      Format = "csv"
	FormatTSV      Format = "tsv"
	FormatJSON     Format = "json"
	FormatJSONL    Format = "jsonl"
	FormatParquet  Format = "parquet"
	FormatFWF      Format = "fwf"
	FormatSQLite   Format = "sqlite"
	FormatExcel    Format = "excel"
	FormatLogfmt   Format = "logfmt"
	FormatMarkdown Format = "markdown"
	FormatHTML     Format = "html"
)

// InferMode controls schema/type inference while parsing.
type InferMode string

const (
	InferNo   InferMode = "no"   // everything stays string
	InferFast InferMode = "fast" // sample first 128 rows
	InferSafe InferMode = "safe" // full scan
)

// Options carries all parsing knobs; format readers pick what applies.
type Options struct {
	Format          Format
	Separator       rune      // dsv field separator (default ',')
	Quote           rune      // dsv quote char (default '"')
	NoHeader        bool      // first row is data, synthesize column names
	IgnoreErrors    bool      // drop unparsable rows instead of failing
	InferSchema     InferMode // default InferSafe
	InferTypes      []string  // subset of: int float boolean date datetime, or "all"
	TruncateRagged  bool      // truncate rows longer than the header
	Widths          []int     // fwf column widths ([] = auto-detect)
	SeparatorLength int       // fwf separator width (default 1)
	FlexibleWidth   bool      // fwf: allow last column to overflow
	Key             string    // decryption key for encrypted database files
}

// DefaultOptions returns the option defaults shared by all formats.
func DefaultOptions() Options {
	return Options{
		Separator:       ',',
		Quote:           '"',
		InferSchema:     InferSafe,
		InferTypes:      []string{"int", "float"},
		SeparatorLength: 1,
		FlexibleWidth:   true,
	}
}

// NamedFrame is one table loaded from a source; multi-table sources
// (spreadsheets, database files) produce several.
type NamedFrame struct {
	Name  string // tab title, e.g. file stem or sheet/table name
	Frame *data.Frame
}

// Reader parses one format.
type Reader interface {
	// Read consumes the source and returns one or more frames.
	Read(src *Source, opt Options) ([]NamedFrame, error)
}

var (
	regMu    sync.RWMutex
	registry = map[Format]Reader{}
)

// Register installs a reader for a format; called from init() in format files.
func Register(f Format, r Reader) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[f] = r
}

// For returns the reader registered for a format.
func For(f Format) (Reader, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	if r, ok := registry[f]; ok {
		return r, nil
	}
	return nil, fmt.Errorf("unsupported format %q (supported: %s)", f, strings.Join(SupportedFormats(), ", "))
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

// Detect guesses the format from a file name; empty string when unknown.
// For URLs the query string and fragment are ignored, so presigned or
// tokenized links like "https://host/data.csv?token=..." still detect.
func Detect(path string) Format {
	if IsURL(path) {
		if i := strings.IndexAny(path, "?#"); i > 0 {
			path = path[:i]
		}
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return FormatCSV
	case strings.HasSuffix(lower, ".tsv"):
		return FormatTSV
	case strings.HasSuffix(lower, ".json"):
		return FormatJSON
	case strings.HasSuffix(lower, ".jsonl") || strings.HasSuffix(lower, ".ndjson"):
		return FormatJSONL
	case strings.HasSuffix(lower, ".parquet") || strings.HasSuffix(lower, ".pqt"):
		return FormatParquet
	case strings.HasSuffix(lower, ".fwf"):
		return FormatFWF
	case strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3"):
		return FormatSQLite
	case strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls") ||
		strings.HasSuffix(lower, ".xlsm") || strings.HasSuffix(lower, ".xlsb"):
		return FormatExcel
	case strings.HasSuffix(lower, ".logfmt"):
		return FormatLogfmt
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		return FormatMarkdown
	case strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm"):
		return FormatHTML
	}
	return ""
}
