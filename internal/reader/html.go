package reader

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"github.com/LinPr/sqltui/internal/data"
)

// htmlReader extracts every <table> element from an HTML document. The
// header comes from the first row containing <th> cells, or the first row.
type htmlReader struct{}

func init() { Register(FormatHTML, htmlReader{}) }

func (htmlReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	f, err := src.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := html.Parse(stripBOM(f))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var frames []NamedFrame
	for _, tbl := range collectTables(doc) {
		frame := tableToFrame(tbl)
		if frame == nil {
			continue
		}
		frame = applyInfer(frame, opt)
		frames = append(frames, NamedFrame{Frame: frame})
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no tables found in %s", src.Path)
	}

	stem := src.Name()
	if len(frames) == 1 {
		frames[0].Name = stem
	} else {
		for i := range frames {
			frames[i].Name = fmt.Sprintf("%s [%d]", stem, i+1)
		}
	}
	return frames, nil
}

// collectTables walks the document and returns every table element,
// including nested ones (each is extracted independently).
func collectTables(doc *html.Node) []*html.Node {
	var tables []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables = append(tables, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables
}

type htmlCell struct {
	text string
	th   bool
}

// tableToFrame converts one table element into a frame, or nil when the
// table holds no rows.
func tableToFrame(tbl *html.Node) *data.Frame {
	rows := tableRows(tbl)
	if len(rows) == 0 {
		return nil
	}
	headerCells := rows[0]
	dataRows := rows[1:]
	header := make([]string, len(headerCells))
	for i, c := range headerCells {
		if c.text != "" {
			header[i] = c.text
		} else {
			header[i] = fmt.Sprintf("column_%d", i+1)
		}
	}
	var out [][]any
	for _, r := range dataRows {
		row := make([]any, len(header))
		for i := range header {
			if i < len(r) {
				row[i] = r[i].text
			} else {
				row[i] = nil
			}
		}
		out = append(out, row)
	}
	return buildFrame(header, out)
}

// tableRows gathers the tr rows of a table without descending into nested
// tables. Rows whose cells are all <th> sort before data rows so the header
// row is rows[0].
func tableRows(tbl *html.Node) [][]htmlCell {
	var rows [][]htmlCell
	var headerIdx = -1
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				if c.Data == "table" {
					continue // nested table handled separately
				}
				if c.Data == "tr" {
					cells := rowCells(c)
					if len(cells) == 0 {
						continue
					}
					if headerIdx < 0 && isHeaderRow(cells) {
						headerIdx = len(rows)
					}
					rows = append(rows, cells)
					continue
				}
			}
			walk(c)
		}
	}
	walk(tbl)
	// move the <th> row to the front so it becomes the header
	if headerIdx > 0 {
		hdr := rows[headerIdx]
		rows = append(rows[:headerIdx], rows[headerIdx+1:]...)
		rows = append([][]htmlCell{hdr}, rows...)
	}
	return rows
}

func isHeaderRow(cells []htmlCell) bool {
	for _, c := range cells {
		if !c.th {
			return false
		}
	}
	return true
}

// rowCells collects the th/td cells of one tr element.
func rowCells(tr *html.Node) []htmlCell {
	var cells []htmlCell
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, htmlCell{text: textContent(c), th: c.Data == "th"})
		}
	}
	return cells
}

// textContent returns the whitespace-collapsed text of a node subtree.
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
