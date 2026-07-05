package mysql

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var TableRecords *tview.Table

func RenderTable() *tview.Table {
	table := tview.NewTable().
		SetBorders(true).
		SetSeparator('|').
		SetFixed(1, 0).
		SetSelectable(true, false).
		Select(0, 0)

	table.SetBorder(true).SetTitle("[green]Result Table")

	TableRecords = table

	return TableRecords
}

func ClearTableRecords() {
	TableRecords.Clear()
}

func FillTableWithQueryResult(fields []string, result [][]string) {
	TableRecords.Clear()
	// 1. fill the first line with field names (fixed, non-selectable header)
	startRow := 0
	if len(fields) > 0 {
		for j, field := range fields {
			cell := tview.NewTableCell(tview.Escape(field)).
				SetTextColor(tcell.ColorYellow).
				SetAttributes(tcell.AttrBold).
				SetAlign(tview.AlignCenter).
				SetSelectable(false)
			TableRecords.SetCell(startRow, j, cell)
		}
		startRow += 1
	}

	// 2. fill result
	for i, row := range result {
		for j, cell := range row {
			SetCell(startRow+i, j, cell, tcell.ColorWhite)
		}
	}

	TableRecords.ScrollToBeginning()
	if len(result) > 0 {
		TableRecords.Select(startRow, 0)
	}
}

func SetCell(row int, column int, text string, color tcell.Color) {
	cell := tview.NewTableCell(tview.Escape(text)).
		SetTextColor(color).
		SetAlign(tview.AlignCenter)
	TableRecords.SetCell(row, column, cell)
}

// SetBorderStyle sets a new global border style and returns the old one.
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
