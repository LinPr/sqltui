package redis

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func RenderDashBoardPage() *tview.Flex {

	treeView := RenderKeyTreeView()
	queryWidget := RenderQueryWidget()
	resultTextView := RenderResultTextView()
	helpBar := RenderHelpBar()
	tuiapp.RedisTui.AddWidget(treeView)
	tuiapp.RedisTui.AddWidget(resultTextView)

	mainFlex := tview.NewFlex().
		AddItem(treeView, 20, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryWidget, 0, 1, false).
			AddItem(resultTextView, 0, 3, false), 0, 2, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainFlex, 0, 1, true).
		AddItem(helpBar, 1, 0, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			tuiapp.RedisTui.SetNextFocus()
		case tcell.KeyBacktab:
			tuiapp.RedisTui.SetPreviousFocus()
		case tcell.KeyEscape:
			tuiapp.RedisTui.ShowPage("redis_login")
			return nil
		}
		return event // this event should be returned and not to return nil
	})

	tuiapp.RedisTui.AddPage("redis_dashboard", flex)
	return flex
}

func RenderHelpBar() *tview.TextView {
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[green]Tab/Shift+Tab: switch focus | Enter: run command | Esc: back to login | Ctrl+C: quit")
	return helpBar
}
