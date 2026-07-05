package mysql

import (
	"github.com/rivo/tview"
)

func RenderQueryWidget() *tview.Flex {
	inputField := RenderInputFiedl()
	textView := RenderTextView()

	queryWidget := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 2, 1, false).
		AddItem(textView, 0, 1, false)
	queryWidget.SetBorder(true).SetTitle("[green]Query (Ctrl+R: run)")

	return queryWidget
}
