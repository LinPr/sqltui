package writer

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatJSON, jsonWriter{})
}

// jsonWriter serializes a frame as a JSON array of row objects with typed
// values, preserving column order.
type jsonWriter struct{}

func (jsonWriter) Write(w io.Writer, f *data.Frame, opt Options) error {
	keys := jsonKeys(f)
	rows := make([]json.RawMessage, f.NumRows())
	for r := range rows {
		rows[r] = rowObject(f, keys, r)
	}
	var out []byte
	var err error
	if opt.Pretty {
		out, err = json.MarshalIndent(rows, "", "  ")
	} else {
		out, err = json.Marshal(rows)
	}
	if err != nil {
		return err
	}
	out = append(out, '\n')
	_, err = w.Write(out)
	return err
}

// jsonKeys returns one pre-marshaled object key per column. Duplicate column
// names are made unique (a, a -> a, a_2, same scheme as the other writers)
// because JSON parsers keep only one of two identical keys.
func jsonKeys(f *data.Frame) [][]byte {
	names := exportNames(f)
	keys := make([][]byte, len(names))
	for i, n := range names {
		keys[i], _ = json.Marshal(n)
	}
	return keys
}

// rowObject renders one row as a compact JSON object in column order.
func rowObject(f *data.Frame, keys [][]byte, row int) []byte {
	buf := &bytes.Buffer{}
	buf.WriteByte('{')
	for c := range f.Columns {
		if c > 0 {
			buf.WriteByte(',')
		}
		buf.Write(keys[c])
		buf.WriteByte(':')
		buf.Write(jsonValue(f.Columns[c].Cells[row]))
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// jsonValue encodes one cell: numbers and booleans stay typed, nil becomes
// null, times are formatted strings.
func jsonValue(v any) []byte {
	switch x := v.(type) {
	case nil:
		return []byte("null")
	case int64:
		return strconv.AppendInt(nil, x, 10)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return []byte("null")
		}
		return strconv.AppendFloat(nil, x, 'g', -1, 64)
	case bool:
		return strconv.AppendBool(nil, x)
	case time.Time:
		b, _ := json.Marshal(data.FormatValue(x))
		return b
	default:
		b, err := json.Marshal(data.FormatValue(v))
		if err != nil {
			return []byte("null")
		}
		return b
	}
}
