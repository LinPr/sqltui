package sqlite

import "github.com/rivo/tview"

func RenderDashBoardPage() tview.Primitive {

	flex := tview.NewFlex().
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false).
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false)

	return flex
}
