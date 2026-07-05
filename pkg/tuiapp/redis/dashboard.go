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
	helpBar := tuiapp.RenderHelpBar("Tab/Shift+Tab: switch focus | Enter/Ctrl+R: run command | Esc: back to login | Ctrl+Q: quit")

	// register widgets in visual order: tree -> query input -> result view
	tuiapp.RedisTui.AddWidget(treeView)
	tuiapp.RedisTui.AddWidget(queryInput)
	tuiapp.RedisTui.AddWidget(resultTextView)

	mainFlex := tview.NewFlex().
		AddItem(treeView, 30, 1, true).
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
			return nil
		case tcell.KeyBacktab:
			tuiapp.RedisTui.SetPreviousFocus()
			return nil
		case tcell.KeyCtrlR:
			runInputQuery()
			return nil
		case tcell.KeyEscape:
			tuiapp.RedisTui.ShowPage("redis_login")
			return nil
		case tcell.KeyCtrlQ:
			tuiapp.RedisTui.App.Stop()
			return nil
		}
		return event // this event should be returned and not to return nil
	})

	return flex
}
