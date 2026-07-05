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
	helpBar := RenderHelpBar()
	tuiapp.MysqlTui.AddWidget(treeView)
	tuiapp.MysqlTui.AddWidget(table)

	body := tview.NewFlex().
		AddItem(treeView, 20, 1, true).
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
		case tcell.KeyBacktab:
			tuiapp.MysqlTui.SetPreviousFocus()
		case tcell.KeyEscape:
			tuiapp.MysqlTui.ShowPage("mysql_login")
			return nil
		case tcell.KeyCtrlQ:
			tuiapp.MysqlTui.App.Stop()
			return nil
		}
		return event // this event should be returned and not to return nil
	})

	tuiapp.MysqlTui.AddPage("mysql_dashboard", flex)

	return flex
}

func RenderHelpBar() *tview.TextView {
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]Tab[white]/[yellow]Shift+Tab[white]: switch focus | [yellow]Enter[white]: run query / select | [yellow]Esc[white]: back to login | [yellow]Ctrl+Q[white]: quit")
	return helpBar
}
