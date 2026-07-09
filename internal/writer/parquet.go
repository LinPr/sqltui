package writer

import (
	"fmt"
	"io"
	"time"

	pq "github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatParquet, parquetWriter{})
}

// parquetWriter serializes a frame as a parquet file with one optional leaf
// column per frame column.
type parquetWriter struct{}

func (parquetWriter) Write(w io.Writer, f *data.Frame, opt Options) error {
	codec, err := codecFor(opt.Compression)
	if err != nil {
		return err
	}

	names := exportNames(f)
	group := pq.Group{}
	for i, col := range f.Columns {
		group[names[i]] = pq.Optional(nodeFor(col.Type))
	}
	schema := pq.NewSchema("frame", orderedGroup{Group: group, order: names})

	pw := pq.NewGenericWriter[any](w, schema, pq.Compression(codec))
	const batch = 256
	rows := make([]pq.Row, 0, batch)
	for r := 0; r < f.NumRows(); r++ {
		row := make(pq.Row, len(f.Columns))
		for c, col := range f.Columns {
			row[c] = cellValue(col.Cells[r], col.Type, c)
		}
		rows = append(rows, row)
		if len(rows) == batch {
			if _, err := pw.WriteRows(rows); err != nil {
				return err
			}
			rows = rows[:0]
		}
	}
	if len(rows) > 0 {
		if _, err := pw.WriteRows(rows); err != nil {
			return err
		}
	}
	return pw.Close()
}

// orderedGroup is a group node whose fields keep frame column order instead
// of the alphabetical order of pq.Group.
type orderedGroup struct {
	pq.Group
	order []string
}

func (g orderedGroup) Fields() []pq.Field {
	byName := make(map[string]pq.Field, len(g.order))
	for _, f := range g.Group.Fields() {
		byName[f.Name()] = f
	}
	out := make([]pq.Field, 0, len(g.order))
	for _, n := range g.order {
		out = append(out, byName[n])
	}
	return out
}

// exportNames returns non-empty, unique column names for the schema.
func exportNames(f *data.Frame) []string {
	used := map[string]bool{}
	names := make([]string, f.NumCols())
	for i, col := range f.Columns {
		name := col.Name
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		base := name
		for k := 2; used[name]; k++ {
			name = fmt.Sprintf("%s_%d", base, k)
		}
		used[name] = true
		names[i] = name
	}
	return names
}

func nodeFor(dt data.DType) pq.Node {
	switch dt {
	case data.TypeInt:
		return pq.Int(64)
	case data.TypeFloat:
		return pq.Leaf(pq.DoubleType)
	case data.TypeBool:
		return pq.Leaf(pq.BooleanType)
	case data.TypeDate:
		return pq.Date()
	case data.TypeDatetime:
		return pq.Timestamp(pq.Microsecond)
	default:
		return pq.String()
	}
}

func codecFor(c Compression) (compress.Codec, error) {
	switch c {
	case CompressionNone:
		return &pq.Uncompressed, nil
	case CompressionSnappy, "":
		return &pq.Snappy, nil
	case CompressionGzip:
		return &pq.Gzip, nil
	case CompressionZstd:
		return &pq.Zstd, nil
	case CompressionLZ4:
		return &pq.Lz4Raw, nil
	case CompressionBrotli:
		return &pq.Brotli, nil
	default:
		return nil, fmt.Errorf("unsupported parquet compression %q", c)
	}
}

// cellValue converts one cell into a parquet value for an optional leaf at
// the given column index. Cells that cannot be represented in the column
// type are written as null.
func cellValue(v any, dt data.DType, col int) pq.Value {
	if v == nil {
		return pq.NullValue().Level(0, 0, col)
	}
	var pv pq.Value
	ok := true
	switch dt {
	case data.TypeInt:
		switch x := v.(type) {
		case int64:
			pv = pq.Int64Value(x)
		case float64:
			pv = pq.Int64Value(int64(x))
		case bool:
			if x {
				pv = pq.Int64Value(1)
			} else {
				pv = pq.Int64Value(0)
			}
		default:
			ok = false
		}
	case data.TypeFloat:
		switch x := v.(type) {
		case float64:
			pv = pq.DoubleValue(x)
		case int64:
			pv = pq.DoubleValue(float64(x))
		default:
			ok = false
		}
	case data.TypeBool:
		if x, is := v.(bool); is {
			pv = pq.BooleanValue(x)
		} else {
			ok = false
		}
	case data.TypeDate:
		if t, is := v.(time.Time); is {
			pv = pq.Int32Value(int32(daysSinceEpoch(t)))
		} else {
			ok = false
		}
	case data.TypeDatetime:
		if t, is := v.(time.Time); is {
			pv = pq.Int64Value(t.UnixMicro())
		} else {
			ok = false
		}
	default:
		pv = pq.ByteArrayValue([]byte(data.FormatValue(v)))
	}
	if !ok {
		return pq.NullValue().Level(0, 0, col)
	}
	return pv.Level(0, 1, col)
}

func daysSinceEpoch(t time.Time) int64 {
	secs := t.UTC().Unix()
	days := secs / 86400
	if secs%86400 < 0 {
		days--
	}
	return days
}
