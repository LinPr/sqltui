package redis

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/rivo/tview"
)

func RenderQueryWidget() *tview.Flex {

	inputField := RenderInputFiedl()
	textView := RenderErrTextView()

	tuiapp.RedisTui.AddWidget(inputField)

	queryWidget := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 2, 1, false).
		AddItem(textView, 0, 1, false)
	queryWidget.SetBorder(true).SetTitle("[green]Query")

	return queryWidget
}
