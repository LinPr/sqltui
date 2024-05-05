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
	tuiapp.RedisTui.AddWidget(treeView)
	tuiapp.RedisTui.AddWidget(resultTextView)
	// tview.NewTextArea()
	// tview.NewTextView()

	flex := tview.NewFlex().
		AddItem(treeView, 20, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			// AddItem(inputField, 0, 1, false).
			AddItem(queryWidget, 0, 1, false).
			// AddItem(tview.NewBox().SetBorder(true).SetTitle("Middle (3 x height of Top)"), 0, 3, false).
			AddItem(resultTextView, 0, 3, false), 0, 2, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			// wiget := tuiapp.TuiApp.MysqlApp.GetFocus()
			// tuiapp.TuiApp.MysqlApp.SetFocus(tuiapp.NextWigets(wiget))
			tuiapp.RedisTui.SetNextFocus()
		}
		return event // this event should be returned and not to return nil
	})

	// flex.SetBackgroundColor(tcell.ColorDefault)
	tuiapp.RedisTui.AddPage("redis_dashboard", flex)
	return flex
}
