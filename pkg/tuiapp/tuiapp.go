package tuiapp

import (
	"github.com/rivo/tview"
)

type tuiApp struct {
	MysqlApp *tview.Application
	RedisApp *tview.Application
}

var (
	TuiApp *tuiApp
	Pages  *tview.Pages
	Widget []tview.Primitive
)

func init() {
	TuiApp = &tuiApp{}
	Pages = tview.NewPages()
	Widget = make([]tview.Primitive, 0)

}

func GetPageSet() *tview.Pages {
	return Pages
}

func AddPage(name string, item tview.Primitive) {
	Pages.AddPage(name, item, true, false)

}

func ShowPage(name string) {
	Pages.SwitchToPage(name)
}

func AddWidget(w tview.Primitive) {
	Widget = append(Widget, w)
}

func PreviousWidgets(curent tview.Primitive) tview.Primitive {
	for i, w := range Widget {
		if w == curent {
			if i-1 >= 0 {
				return Widget[i-1]
			}
			return Widget[len(Widget)-1]
		}
	}
	return Widget[0]
}

func NextWigets(curent tview.Primitive) tview.Primitive {
	for i, w := range Widget {
		if w == curent {
			if i+1 < len(Widget) {
				return Widget[i+1]
			}
			return Widget[0]
		}
	}
	return Widget[0]
}
