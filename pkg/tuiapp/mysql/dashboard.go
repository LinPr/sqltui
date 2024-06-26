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
	tuiapp.MysqlTui.AddWidget(treeView)
	tuiapp.MysqlTui.AddWidget(table)
	// tview.NewTextArea()
	// tview.NewTextView()

	flex := tview.NewFlex().
		AddItem(treeView, 20, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			// AddItem(inputField, 0, 1, false).
			AddItem(queryWidget, 0, 1, false).
			// AddItem(tview.NewBox().SetBorder(true).SetTitle("Middle (3 x height of Top)"), 0, 3, false).
			AddItem(table, 0, 3, false), 0, 2, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			// wiget := tuiapp.TuiApp.MysqlApp.GetFocus()
			// tuiapp.TuiApp.MysqlApp.SetFocus(tuiapp.NextWigets(wiget))
			tuiapp.MysqlTui.SetNextFocus()
		}
		return event // this event should be returned and not to return nil
	})

	// flex.SetBackgroundColor(tcell.ColorBlack)
	tuiapp.MysqlTui.AddPage("mysql_dashboard", flex)

	return flex
}
