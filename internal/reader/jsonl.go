package reader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonlReader parses newline-delimited JSON: one object per line.
type jsonlReader struct{}

func init() { Register(FormatJSONL, jsonlReader{}) }

const maxLineBytes = 64 * 1024 * 1024

func (jsonlReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
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
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		keys, vals, err := decodeJSONObject(dec)
		if err != nil {
			if opt.IgnoreErrors {
				continue
			}
			return nil, fmt.Errorf("parse jsonl line %d: %w", lineno, err)
		}
		rt.add(keys, vals)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return []NamedFrame{{Name: src.Name(), Frame: rt.frame()}}, nil
}
