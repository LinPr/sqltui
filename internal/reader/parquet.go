package reader

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LinPr/sqltui/internal/data"
	pq "github.com/parquet-go/parquet-go"
)

func init() {
	Register(FormatParquet, parquetReader{})
}

type parquetReader struct{}

// parquetLeaf describes one leaf column of the file schema, in leaf order.
type parquetLeaf struct {
	name     string // dotted path for nested fields
	typ      pq.Type
	dtype    data.DType
	repeated bool
}

func (parquetReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	fh, err := os.Open(src.LocalPath)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil {
		return nil, err
	}
	pf, err := pq.OpenFile(fh, info.Size())
	if err != nil {
		return nil, fmt.Errorf("open parquet file: %w", err)
	}

	leaves := collectLeaves(nil, pq.Node(pf.Schema()), false, nil)
	frame := &data.Frame{Columns: make([]data.Column, len(leaves))}
	for i, lf := range leaves {
		frame.Columns[i] = data.Column{Name: lf.name, Type: lf.dtype}
	}

	for _, rg := range pf.RowGroups() {
		if err := readRowGroup(frame, leaves, rg); err != nil {
			if opt.IgnoreErrors {
				continue
			}
			return nil, err
		}
	}
	return []NamedFrame{{Name: src.Name(), Frame: frame}}, nil
}

// collectLeaves walks the schema depth-first, mirroring parquet leaf-column
// ordering, and records the display name and frame dtype of every leaf.
func collectLeaves(path []string, node pq.Node, repeated bool, acc []parquetLeaf) []parquetLeaf {
	if node.Leaf() {
		typ := node.Type()
		return append(acc, parquetLeaf{
			name:     strings.Join(path, "."),
			typ:      typ,
			dtype:    leafDType(typ),
			repeated: repeated || node.Repeated(),
		})
	}
	for _, f := range node.Fields() {
		acc = collectLeaves(append(path, f.Name()), f, repeated || f.Repeated(), acc)
	}
	return acc
}

// leafDType maps a parquet physical/logical type to a frame column type.
func leafDType(t pq.Type) data.DType {
	if lt := t.LogicalType(); lt != nil {
		switch {
		case lt.UTF8 != nil, lt.Enum != nil, lt.Json != nil, lt.Bson != nil, lt.UUID != nil:
			return data.TypeString
		case lt.Date != nil:
			return data.TypeDate
		case lt.Timestamp != nil:
			return data.TypeDatetime
		case lt.Integer != nil:
			return data.TypeInt
		case lt.Decimal != nil:
			return data.TypeFloat
		}
	}
	switch t.Kind() {
	case pq.Boolean:
		return data.TypeBool
	case pq.Int32, pq.Int64:
		return data.TypeInt
	case pq.Float, pq.Double:
		return data.TypeFloat
	default: // byte arrays, int96, anything exotic
		return data.TypeString
	}
}

func readRowGroup(frame *data.Frame, leaves []parquetLeaf, rg pq.RowGroup) error {
	rows := rg.Rows()
	defer rows.Close()

	buf := make([]pq.Row, 128)
	for {
		n, err := rows.ReadRows(buf)
		for _, row := range buf[:n] {
			cells := make([]any, len(leaves))
			row.Range(func(col int, vals []pq.Value) bool {
				if col < 0 || col >= len(leaves) {
					return true
				}
				cells[col] = leafCell(leaves[col], vals)
				return true
			})
			frame.AppendRow(cells)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read parquet rows: %w", err)
		}
		if n == 0 {
			return nil
		}
	}
}

// leafCell converts the value(s) of one leaf column in one row to a cell.
// Repeated fields are rendered as a comma-joined string.
func leafCell(leaf parquetLeaf, vals []pq.Value) any {
	if len(vals) == 0 {
		return nil
	}
	if !leaf.repeated && len(vals) == 1 {
		return convertParquetValue(vals[0], leaf)
	}
	parts := make([]string, 0, len(vals))
	empty := true
	for _, v := range vals {
		cell := convertParquetValue(v, leaf)
		if cell != nil {
			empty = false
		}
		parts = append(parts, data.FormatValue(cell))
	}
	if empty {
		return nil
	}
	return strings.Join(parts, ",")
}

var dateEpoch = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)

func convertParquetValue(v pq.Value, leaf parquetLeaf) any {
	if v.IsNull() {
		return nil
	}
	switch leaf.dtype {
	case data.TypeBool:
		return v.Boolean()
	case data.TypeInt:
		if v.Kind() == pq.Int32 {
			return int64(v.Int32())
		}
		return v.Int64()
	case data.TypeFloat:
		if lt := leaf.typ.LogicalType(); lt != nil && lt.Decimal != nil {
			return decimalFloat(v, lt.Decimal.Scale)
		}
		if v.Kind() == pq.Float {
			return float64(v.Float())
		}
		return v.Double()
	case data.TypeDate:
		return dateEpoch.AddDate(0, 0, int(v.Int32()))
	case data.TypeDatetime:
		n := v.Int64()
		if lt := leaf.typ.LogicalType(); lt != nil && lt.Timestamp != nil {
			switch {
			case lt.Timestamp.Unit.Millis != nil:
				return time.UnixMilli(n).UTC()
			case lt.Timestamp.Unit.Micros != nil:
				return time.UnixMicro(n).UTC()
			default:
				return time.Unix(0, n).UTC()
			}
		}
		return time.UnixMicro(n).UTC()
	default:
		switch v.Kind() {
		case pq.ByteArray, pq.FixedLenByteArray:
			b := v.ByteArray()
			if utf8.Valid(b) {
				return string(b)
			}
			return "0x" + hex.EncodeToString(b)
		default:
			return fmt.Sprint(v)
		}
	}
}

// decimalFloat converts a DECIMAL-annotated value to float64.
func decimalFloat(v pq.Value, scale int32) float64 {
	div := 1.0
	for i := int32(0); i < scale; i++ {
		div *= 10
	}
	switch v.Kind() {
	case pq.Int32:
		return float64(v.Int32()) / div
	case pq.Int64:
		return float64(v.Int64()) / div
	default: // fixed-len byte array big-endian two's complement
		b := v.ByteArray()
		var n int64
		if len(b) > 0 && b[0]&0x80 != 0 {
			n = -1
		}
		for _, c := range b {
			n = n<<8 | int64(c)
		}
		return float64(n) / div
	}
}
