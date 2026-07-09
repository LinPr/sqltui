package reader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/LinPr/sqltui/internal/data"
)

// jsonReader parses a file holding a JSON array of objects (or one object).
type jsonReader struct{}

func init() { Register(FormatJSON, jsonReader{}) }

func (jsonReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	f, err := src.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(stripBOM(f))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	rt := newRecordTable()
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil, fmt.Errorf("json: top-level value must be an object or an array of objects")
	}
	switch delim {
	case '[':
		for dec.More() {
			keys, vals, err := decodeJSONObject(dec)
			if err != nil {
				return nil, fmt.Errorf("parse json: %w", err)
			}
			rt.add(keys, vals)
		}
		if _, err := dec.Token(); err != nil { // consume ']'
			return nil, fmt.Errorf("parse json: %w", err)
		}
	case '{':
		keys, vals, err := decodeJSONObjectBody(dec)
		if err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		rt.add(keys, vals)
	default:
		return nil, fmt.Errorf("json: top-level value must be an object or an array of objects")
	}

	return []NamedFrame{{Name: src.Name(), Frame: rt.frame()}}, nil
}

// decodeJSONObject expects the next value in the stream to be an object and
// decodes it, preserving key order.
func decodeJSONObject(dec *json.Decoder) ([]string, map[string]any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, fmt.Errorf("expected an object, got %v", tok)
	}
	return decodeJSONObjectBody(dec)
}

// decodeJSONObjectBody decodes the members of an object whose opening '{'
// has already been consumed, and consumes the closing '}'.
func decodeJSONObjectBody(dec *json.Decoder) ([]string, map[string]any, error) {
	var keys []string
	vals := map[string]any{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("expected object key, got %v", tok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}
		v, err := jsonCellValue(raw)
		if err != nil {
			return nil, nil, err
		}
		if _, dup := vals[key]; !dup {
			keys = append(keys, key)
		}
		vals[key] = v
	}
	if _, err := dec.Token(); err != nil { // consume '}'
		return nil, nil, err
	}
	return keys, vals, nil
}

// jsonCellValue maps one raw JSON value onto a frame cell:
// string→string, integral number→int64, other number→float64, bool→bool,
// null→nil, array/object→compact JSON text.
func jsonCellValue(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	switch trimmed[0] {
	case 'n': // null
		return nil, nil
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(trimmed, &b); err != nil {
			return nil, err
		}
		return b, nil
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return nil, err
		}
		return s, nil
	case '{', '[':
		var buf bytes.Buffer
		if err := json.Compact(&buf, trimmed); err != nil {
			return nil, err
		}
		return buf.String(), nil
	default: // number
		num := json.Number(trimmed)
		if i, err := num.Int64(); err == nil {
			return i, nil
		}
		f, err := num.Float64()
		if err != nil {
			return nil, err
		}
		// float64(math.MaxInt64) rounds up to 2^63, which overflows int64,
		// so the upper bound must be strict. float64(math.MinInt64) is
		// exactly -2^63, so the lower bound may be inclusive.
		if f == math.Trunc(f) && f >= math.MinInt64 && f < 9223372036854775808.0 {
			return int64(f), nil
		}
		return f, nil
	}
}

// recordTable accumulates key/value records and produces a frame whose
// column order follows first appearance of each key.
type recordTable struct {
	order []string
	seen  map[string]bool
	rows  []map[string]any
}

func newRecordTable() *recordTable {
	return &recordTable{seen: map[string]bool{}}
}

func (t *recordTable) add(keys []string, vals map[string]any) {
	for _, k := range keys {
		if !t.seen[k] {
			t.seen[k] = true
			t.order = append(t.order, k)
		}
	}
	t.rows = append(t.rows, vals)
}

func (t *recordTable) frame() *data.Frame {
	f := data.New(t.order...)
	row := make([]any, len(t.order))
	for _, vals := range t.rows {
		for i, k := range t.order {
			row[i] = vals[k] // missing keys yield nil
		}
		f.AppendRow(row)
	}
	return f
}
