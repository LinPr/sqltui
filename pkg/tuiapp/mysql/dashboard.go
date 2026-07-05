package mysql

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func RenderDashBoardPage() *tview.Flex {
	treeView := RenderTreeView()
	queryWidget := RenderQueryWidget()
	table := RenderTable()
	helpBar := tuiapp.RenderHelpBar("Tab/Shift+Tab: switch focus | Enter/Ctrl+R: run query | Esc: back to login | Ctrl+Q: quit")

	// register widgets in visual order: tree -> query input -> result table
	tuiapp.MysqlTui.AddWidget(treeView)
	tuiapp.MysqlTui.AddWidget(queryInput)
	tuiapp.MysqlTui.AddWidget(table)

	body := tview.NewFlex().
		AddItem(treeView, 30, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryWidget, 0, 1, false).
			AddItem(table, 0, 3, false), 0, 2, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(helpBar, 1, 0, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			tuiapp.MysqlTui.SetNextFocus()
			return nil
		case tcell.KeyBacktab:
			tuiapp.MysqlTui.SetPreviousFocus()
			return nil
		case tcell.KeyCtrlR:
			runInputQuery()
			return nil
		case tcell.KeyEscape:
			tuiapp.MysqlTui.ShowPage("mysql_login")
			return nil
		case tcell.KeyCtrlQ:
			tuiapp.MysqlTui.App.Stop()
			return nil
		}
		return event // this event should be returned and not to return nil
	})

	return flex
}
