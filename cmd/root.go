package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LinPr/sqltui/internal/reader"
)

// fileFlags holds parsing options for file mode; wired to reader.Options.
type fileFlags struct {
	format          string
	separator       string
	quote           string
	noHeader        bool
	ignoreErrors    bool
	inferSchema     string
	inferTypes      []string
	truncateRagged  bool
	widths          []int
	separatorLength int
	noFlexibleWidth bool
	key             string
	multiparts      []string
}

var flags fileFlags

var rootCmd = &cobra.Command{
	Use:   "sqltui [files...]",
	Short: "Terminal viewer and SQL workbench for tabular data and databases",
	Long: `sqltui is a terminal UI for browsing and querying tabular data.

There are two ways to use it:

  Files    Pass data files (or "-" for stdin, or an http(s) URL) directly.
           Supported formats: csv, tsv, dsv, json, jsonl, parquet, excel,
           fwf, logfmt, markdown, html — and sqlite database files
           (.db/.sqlite), whose tables all open at once. Every table becomes
           a tab you can filter, sort, search and query with SQL through the
           embedded engine.

  Servers  Use a subcommand (mysql, postgres, redis) only for server
           databases that need connection credentials. A connection form
           opens first, then you browse the live schema and run statements
           against the server.`,
	Example: `  # open a csv file
  sqltui data.csv

  # several files at once — one tab per table
  sqltui a.csv b.parquet c.xlsx

  # sqlite database files are just files — no subcommand needed
  sqltui app.db

  # read csv from stdin
  cat data.csv | sqltui -

  # open a remote file
  sqltui https://example.com/data.csv

  # force a format when the file extension is misleading
  sqltui export.txt --format csv

  # connect to a MySQL server (opens a connection form)
  sqltui mysql`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFileMode(args)
	},
}

func init() {
	f := rootCmd.Flags()
	f.StringVarP(&flags.format, "format", "f", "", "input format, overriding file-extension detection: "+
		"csv, tsv, dsv, json, jsonl, parquet, fwf, sqlite, excel, logfmt, markdown, html")
	f.StringVar(&flags.separator, "separator", ",", "field separator (dsv only; csv and tsv have fixed separators)")
	f.StringVar(&flags.quote, "quote-char", `"`, "quote character (csv/tsv/dsv only)")
	f.BoolVar(&flags.noHeader, "no-header", false, "treat the first row as data, not column names (csv/tsv/dsv/fwf/excel)")
	f.BoolVar(&flags.ignoreErrors, "ignore-errors", false, "drop unparsable rows instead of failing (any format)")
	f.StringVar(&flags.inferSchema, "infer-schema", "safe", "column type inference for text formats: no, fast, safe")
	f.StringSliceVar(&flags.inferTypes, "infer-types", []string{"int", "float"},
		"types the inference may produce (text formats): all, int, float, boolean, date, datetime")
	f.BoolVar(&flags.truncateRagged, "truncate-ragged-lines", false, "truncate rows longer than the header (csv/tsv/dsv only)")
	f.IntSliceVar(&flags.widths, "widths", nil, "explicit column widths (fwf only)")
	f.IntVar(&flags.separatorLength, "separator-length", 1, "number of characters between columns (fwf only)")
	f.BoolVar(&flags.noFlexibleWidth, "no-flexible-width", false, "enforce strict column widths (fwf only)")
	f.StringVar(&flags.key, "sqlite-key", "", "decryption key for encrypted database files (sqlite only)")
	f.StringSliceVar(&flags.multiparts, "multiparts", nil, "concatenate these files vertically into a single table (any format)")
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// readerOptions converts CLI flags into reader options.
func readerOptions() (reader.Options, error) {
	opt := reader.DefaultOptions()
	if flags.format != "" {
		opt.Format = reader.Format(flags.format)
	}
	sep := []rune(flags.separator)
	if len(sep) != 1 {
		return opt, fmt.Errorf("--separator must be a single character")
	}
	opt.Separator = sep[0]
	quote := []rune(flags.quote)
	if len(quote) != 1 {
		return opt, fmt.Errorf("--quote-char must be a single character")
	}
	opt.Quote = quote[0]
	opt.NoHeader = flags.noHeader
	opt.IgnoreErrors = flags.ignoreErrors
	switch reader.InferMode(flags.inferSchema) {
	case reader.InferNo, reader.InferFast, reader.InferSafe:
		opt.InferSchema = reader.InferMode(flags.inferSchema)
	default:
		return opt, fmt.Errorf("--infer-schema must be one of: no, fast, safe")
	}
	opt.InferTypes = flags.inferTypes
	opt.TruncateRagged = flags.truncateRagged
	opt.Widths = flags.widths
	opt.SeparatorLength = flags.separatorLength
	opt.FlexibleWidth = !flags.noFlexibleWidth
	opt.Key = flags.key
	return opt, nil
}
