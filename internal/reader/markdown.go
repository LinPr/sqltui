package reader

import (
	"fmt"
	"regexp"
	"strings"
)

// markdownReader extracts every well-formed pipe table from a markdown file.
// One table keeps the file stem as its name; several are numbered
// "<stem> [1]", "<stem> [2]", ...
type markdownReader struct{}

func init() { Register(FormatMarkdown, markdownReader{}) }

var mdAlignCell = regexp.MustCompile(`^:?-+:?$`)

func (markdownReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	b, err := src.Bytes()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(stripBOMBytes(b)), "\r\n", "\n"), "\n")

	var frames []NamedFrame
	i := 0
	for i < len(lines) {
		if !isPipeRow(lines[i]) || i+1 >= len(lines) || !isAlignmentRow(lines[i+1]) {
			i++
			continue
		}
		header := splitPipeRow(lines[i])
		align := splitPipeRow(lines[i+1])
		if len(header) == 0 || len(align) != len(header) {
			i++ // not well-formed: column counts disagree
			continue
		}
		i += 2
		var rows [][]any
		for i < len(lines) && isPipeRow(lines[i]) {
			cells := splitPipeRow(lines[i])
			row := make([]any, len(header))
			for c := range header {
				if c < len(cells) {
					row[c] = cells[c]
				} else {
					row[c] = nil // pad short rows
				}
			}
			rows = append(rows, row)
			i++
		}
		frame := buildFrame(header, rows)
		frame = applyInfer(frame, opt)
		frames = append(frames, NamedFrame{Frame: frame})
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no pipe tables found in %s", src.Path)
	}

	stem := src.Name()
	if len(frames) == 1 {
		frames[0].Name = stem
	} else {
		for idx := range frames {
			frames[idx].Name = fmt.Sprintf("%s [%d]", stem, idx+1)
		}
	}
	return frames, nil
}

// isPipeRow reports whether a line looks like a table row (contains an
// unescaped pipe and is not blank).
func isPipeRow(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return false
	}
	esc := false
	for _, r := range s {
		if esc {
			esc = false
			continue
		}
		switch r {
		case '\\':
			esc = true
		case '|':
			return true
		}
	}
	return false
}

// isAlignmentRow reports whether a line is a table alignment row like
// | --- | :---: | ---: |
func isAlignmentRow(line string) bool {
	if !isPipeRow(line) {
		return false
	}
	cells := splitPipeRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !mdAlignCell.MatchString(strings.ReplaceAll(c, " ", "")) {
			return false
		}
	}
	return true
}

// splitPipeRow splits a table row into trimmed cells, honoring leading and
// trailing pipes and `\|` escapes.
func splitPipeRow(line string) []string {
	s := strings.TrimSpace(line)
	var cells []string
	var cur strings.Builder
	esc := false
	for _, r := range s {
		if esc {
			// `\|` and `\\` decode to the literal character; any other
			// backslash sequence is kept verbatim.
			if r != '|' && r != '\\' {
				cur.WriteRune('\\')
			}
			cur.WriteRune(r)
			esc = false
			continue
		}
		switch r {
		case '\\':
			esc = true
		case '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if esc {
		cur.WriteRune('\\')
	}
	cells = append(cells, cur.String())
	// drop the empty fragments produced by a leading / trailing pipe
	if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" && strings.HasPrefix(s, "|") {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" && strings.HasSuffix(s, "|") {
		cells = cells[:len(cells)-1]
	}
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}
