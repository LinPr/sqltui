package mysql

import (
	// "strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var TableRecords *tview.Table

func RenderTable() *tview.Table {
	// SetBorderStyle()

	table := tview.NewTable().
		SetBorders(true).
		SetSeparator('|').
		SetFixed(1, 0).
		Select(0, 0)

	table.SetDoneFunc(
		func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				table.SetSelectable(true, true)
			}
		})

	table.SetSelectedFunc(func(row int, column int) {
		table.GetCell(row, column).SetTextColor(tcell.ColorRed)
		table.SetSelectable(false, false)
	})

	table.SetBorder(true).SetTitle("[green]Result Table")

	TableRecords = table

	// lorem := strings.Split("Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet. Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet.", " ")
	// cols, rows := 10, 40
	// word := 0
	// for r := 0; r < rows; r++ {
	// 	for c := 0; c < cols; c++ {
	// 		color := tcell.ColorWhite
	// 		if r == 0 {
	// 			color = tcell.ColorYellow
	// 		}
	// 		SetCell(r, c, lorem[word], color)
	// 		word = (word + 1) % len(lorem)
	// 	}
	// }

	return TableRecords
}

func ClearTableRecords() {
	TableRecords.Clear()
}

func FillTableWithQueryResult(fields []string, result [][]string) {
	TableRecords.Clear()
	// 1. fill the first line with field names
	startRow := 0
	if fields != nil {
		for j, field := range fields {
			SetCell(startRow, j, field, tcell.ColorYellow)
		}
		startRow += 1
	}

	// 2. fill result
	if result != nil {
		for i, row := range result {
			for j, cell := range row {
				SetCell(startRow+i, j, cell, tcell.ColorWhite)
			}
		}
	}
}

func SetCell(row int, column int, text string, color tcell.Color) {
	cell := tview.NewTableCell(text).
		SetTextColor(color).
		SetAlign(tview.AlignCenter)
	TableRecords.SetCell(row, column, cell)
}

// set new border style and return old one
func SetBorderStyle() *struct {
	Horizontal  rune
	Vertical    rune
	TopLeft     rune
	TopRight    rune
	BottomLeft  rune
	BottomRight rune

	LeftT   rune
	RightT  rune
	TopT    rune
	BottomT rune
	Cross   rune

	HorizontalFocus  rune
	VerticalFocus    rune
	TopLeftFocus     rune
	TopRightFocus    rune
	BottomLeftFocus  rune
	BottomRightFocus rune
} {

	oldBorder := tview.Borders
	tview.Borders.Vertical = '|'
	tview.Borders.Horizontal = '-'
	tview.Borders.TopLeft = '+'
	tview.Borders.TopRight = '+'
	tview.Borders.Cross = '+'
	tview.Borders.TopT = '+'
	tview.Borders.BottomT = '+'
	tview.Borders.BottomLeft = '+'
	tview.Borders.BottomRight = '+'
	tview.Borders.LeftT = '+'
	tview.Borders.RightT = '+'

	return &oldBorder
}
