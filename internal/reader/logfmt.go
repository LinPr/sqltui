package reader

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// logfmtReader parses key=value logfmt lines. Columns are the union of keys
// in first-appearance order; keys missing on a line yield nulls.
type logfmtReader struct{}

func init() { Register(FormatLogfmt, logfmtReader{}) }

func (logfmtReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	f, err := src.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(stripBOM(f))
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)

	rt := newRecordTable()
	lineno := 0
	for sc.Scan() {
		lineno++
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		keys, vals, err := parseLogfmtLine(line)
		if err != nil {
			if opt.IgnoreErrors {
				continue
			}
			return nil, fmt.Errorf("parse logfmt line %d: %w", lineno, err)
		}
		rt.add(keys, vals)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	frame := rt.frame()
	frame = applyInfer(frame, opt)
	return []NamedFrame{{Name: src.Name(), Frame: frame}}, nil
}

// parseLogfmtLine splits one line into key=value pairs. Values may be
// double-quoted with Go-style escapes; a bare key without '=' means "true".
func parseLogfmtLine(line string) ([]string, map[string]any, error) {
	var keys []string
	vals := map[string]any{}
	i := 0
	for i < len(line) {
		// skip whitespace between pairs
		if line[i] == ' ' || line[i] == '\t' {
			i++
			continue
		}
		// key: up to '=' or whitespace
		start := i
		for i < len(line) && line[i] != '=' && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		key := line[start:i]
		if key == "" { // stray '=' — skip it
			i++
			continue
		}
		var val any
		if i < len(line) && line[i] == '=' {
			i++
			if i < len(line) && line[i] == '"' {
				j := i + 1
				for j < len(line) && line[j] != '"' {
					if line[j] == '\\' {
						j++ // skip the escaped char
					}
					j++
				}
				if j >= len(line) {
					return nil, nil, fmt.Errorf("unterminated quoted value for key %q", key)
				}
				unq, err := strconv.Unquote(line[i : j+1])
				if err != nil {
					return nil, nil, fmt.Errorf("bad quoted value for key %q: %w", key, err)
				}
				val = unq
				i = j + 1
			} else {
				start = i
				for i < len(line) && line[i] != ' ' && line[i] != '\t' {
					i++
				}
				val = line[start:i]
			}
		} else {
			val = "true" // bare key means boolean true
		}
		if _, dup := vals[key]; !dup {
			keys = append(keys, key)
		}
		vals[key] = val
	}
	return keys, vals, nil
}
