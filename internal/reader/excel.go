package reader

import (
	"fmt"

	"github.com/xuri/excelize/v2"

	"github.com/LinPr/sqltui/internal/data"
)

func init() {
	Register(FormatExcel, excelReader{})
}

type excelReader struct{}

func (excelReader) Read(src *Source, opt Options) ([]NamedFrame, error) {
	wb, err := excelize.OpenFile(src.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer wb.Close()

	var out []NamedFrame
	for _, sheet := range wb.GetSheetList() {
		rows, err := wb.GetRows(sheet)
		if err != nil {
			if opt.IgnoreErrors {
				continue
			}
			return nil, fmt.Errorf("sheet %q: %w", sheet, err)
		}
		if len(rows) == 0 {
			continue
		}
		frame := frameFromSheet(rows, opt)
		frame = data.InferFrame(frame, string(opt.InferSchema), opt.InferTypes)
		out = append(out, NamedFrame{Name: sheet, Frame: frame})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("workbook has no data")
	}
	if len(out) == 1 {
		out[0].Name = src.Name()
	}
	return out, nil
}

// frameFromSheet converts the string matrix of one sheet into a frame.
// Ragged rows are padded with nil cells.
func frameFromSheet(rows [][]string, opt Options) *data.Frame {
	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}

	var names []string
	var body [][]string
	if opt.NoHeader {
		names = synthesizeHeader(width)
		body = rows
	} else {
		names = make([]string, width)
		for i := 0; i < width; i++ {
			if i < len(rows[0]) && rows[0][i] != "" {
				names[i] = rows[0][i]
			} else {
				names[i] = fmt.Sprintf("column_%d", i+1)
			}
		}
		body = rows[1:]
	}

	frame := data.New(names...)
	for _, r := range body {
		cells := make([]any, width)
		for i := 0; i < width; i++ {
			if i < len(r) {
				cells[i] = r[i]
			}
		}
		frame.AppendRow(cells)
	}
	return frame
}
