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
	MysqlTui    *TuiApp
	SqliteTui   *TuiApp
	RedisTui    *TuiApp
	PostgresTui *TuiApp
)

func init() {
	MysqlTui = &TuiApp{
		App:    tview.NewApplication(),
		Pages:  tview.NewPages(),
		Widget: make([]tview.Primitive, 0),
	}
	SqliteTui = &TuiApp{
		App:    tview.NewApplication(),
		Pages:  tview.NewPages(),
		Widget: make([]tview.Primitive, 0),
	}
	RedisTui = &TuiApp{
		App:    tview.NewApplication(),
		Pages:  tview.NewPages(),
		Widget: make([]tview.Primitive, 0),
	}
	PostgresTui = &TuiApp{
		App:    tview.NewApplication(),
		Pages:  tview.NewPages(),
		Widget: make([]tview.Primitive, 0),
	}
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

func (ta *TuiApp) SetPreviousFocus() {
	wiget := ta.GetCurrentFocus()
	ta.App.SetFocus(ta.PreviousWidgets(wiget))
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

// RenderHelpBar builds the one-line help bar shown at the bottom of every
// dashboard, with a single shared color scheme and alignment.
func RenderHelpBar(text string) *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]" + text)
}

// EscapeArgs escapes tview style tags in every string/error argument so that
// dynamic values coming from the database or user input are rendered
// verbatim instead of being interpreted as color/region tags.
func EscapeArgs(a []any) []any {
	escaped := make([]any, len(a))
	for i, arg := range a {
		switch v := arg.(type) {
		case string:
			escaped[i] = tview.Escape(v)
		case error:
			escaped[i] = tview.Escape(v.Error())
		default:
			escaped[i] = arg
		}
	}
	return escaped
}
