package tuiapp

import (
	"github.com/rivo/tview"
)

// type tuiApp struct {
// 	MysqlApp *tview.Application
// 	RedisApp *tview.Application
// }

type TuiApp struct {
	App    *tview.Application
	Pages  *tview.Pages
	Widget []tview.Primitive
}

var (
	// TuiApp *tuiApp
	MysqlTui *TuiApp
	RedisTui *TuiApp
	// Pages    *tview.Pages
	// Widget   []tview.Primitive
)

func init() {
	MysqlTui = &TuiApp{
		App:    tview.NewApplication(),
		Pages:  tview.NewPages(),
		Widget: make([]tview.Primitive, 0),
	}
	RedisTui = &TuiApp{
		App:    tview.NewApplication(),
		Pages:  tview.NewPages(),
		Widget: make([]tview.Primitive, 0),
	}
	// Pages = tview.NewPages()
	// Widget = make([]tview.Primitive, 0)

}

func (ta *TuiApp) GetPage() *tview.Pages {
	return ta.Pages
}

func (ta *TuiApp) AddPage(name string, item tview.Primitive) {
	ta.Pages.AddPage(name, item, true, false)

}

func (ta *TuiApp) ShowPage(name string) {
	ta.Pages.SwitchToPage(name)
}

func (ta *TuiApp) AddWidget(w tview.Primitive) {
	ta.Widget = append(ta.Widget, w)
}

func (ta *TuiApp) GetCurrentFocus() tview.Primitive {
	return ta.App.GetFocus()
}

func (ta *TuiApp) SetNextFocus() {
	wiget := ta.GetCurrentFocus()
	ta.App.SetFocus(ta.NextWigets(wiget))
}

func (ta *TuiApp) PreviousWidgets(curent tview.Primitive) tview.Primitive {
	for i, w := range ta.Widget {
		if w == curent {
			if i-1 >= 0 {
				return ta.Widget[i-1]
			}
			return ta.Widget[len(ta.Widget)-1]
		}
	}
	return ta.Widget[0]
}

func (ta *TuiApp) NextWigets(curent tview.Primitive) tview.Primitive {
	for i, w := range ta.Widget {
		if w == curent {
			if i+1 < len(ta.Widget) {
				return ta.Widget[i+1]
			}
			return ta.Widget[0]
		}
	}
	return ta.Widget[0]
}
