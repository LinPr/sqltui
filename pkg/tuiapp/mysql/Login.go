package mysql

import (
	"fmt"
	"log"

	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func RenderLoginPage() *tview.Flex {
	form := renderLoginForm()
	textView := renderLoginErrTextView()

	flex := tview.NewFlex().
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 1, false).
			AddItem(form, 15, 3, true).
			AddItem(textView, 0, 1, false), 50, 3, true).
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false)

	tuiapp.AddPage("mysql_login", form)

	return flex
}

func renderLoginForm() *tview.Form {
	form := tview.NewForm().
		AddInputField("username:", "root", 20, nil, nil).
		AddInputField("password:", "123456", 20, nil, nil).
		AddInputField("    host:", "127.0.0.1", 20, nil, nil).
		AddInputField("    port:", "3306", 20, nil, nil).
		// AddInputField("  dbname:", "ngx_test", 20, nil, nil).
		AddInputField("  dbname:", "testdb", 20, nil, nil).
		SetFieldBackgroundColor(tcell.ColorGray)
	// AddDropDown(" charset:", []string{"utf8", "ascall", "unicode"}, 0, nil)

	form.AddButton("Connect", ConnectCallback(form)).
		AddButton("Save", SaveCallback(form)).
		AddButton("Quit", QuitCallback(tuiapp.TuiApp.MysqlApp)).
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tcell.ColorGray).
		SetButtonTextColor(tcell.ColorLightGoldenrodYellow)

	form.SetBorder(true).SetBorderColor(tcell.ColorWhite)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		log.Println("event: ", event.Key())
		switch event.Key() {
		case tcell.KeyCtrlS:
			SaveCallback(form)()

		case tcell.KeyEnter:
			ConnectCallback(form)()

		}
		return event
	})

	return form
}

var LoginErrOut *tview.TextView

func printfLoginErrOut(format string, a ...any) {
	LoginErrOut.Clear()
	erMsg := fmt.Sprintf(format, a...)
	LoginErrOut.SetText(erMsg)
}

func renderLoginErrTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true)

	LoginErrOut = textView
	return textView
}

func ConnectCallback(form *tview.Form) func() {
	return func() {
		count := form.GetFormItemCount()
		for i := 0; i < count; i++ {
			log.Println(form.GetFormItem(i).GetLabel())
			log.Println(form.GetFormItem(i).(*tview.InputField).GetText())
		}
		username := form.GetFormItem(0).(*tview.InputField).GetText()
		password := form.GetFormItem(1).(*tview.InputField).GetText()
		host := form.GetFormItem(2).(*tview.InputField).GetText()
		port := form.GetFormItem(3).(*tview.InputField).GetText()
		dbname := form.GetFormItem(4).(*tview.InputField).GetText()

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", username, password, host, port, dbname)
		log.Println(dsn)

		dbc, err := NewDB(dsn) // mmust init database client here
		if err != nil {
			printfLoginErrOut("[red]" + err.Error())
			return
		}
		fields, err := dbc.FetchTableFields("urls")
		if err != nil {
			log.Println("FetchTableFields error: ", err)
		}
		log.Printf("fields: %+v", fields)
		recods, err := dbc.FetchTableRecords("urls")
		if err != nil {
			log.Println("FetchTableRecords error: ", err)
		}
		log.Printf("recods: %+v", recods)

		SetRootTreeNodeName(GetDbName())
		tuiapp.ShowPage("mysql_dashboard")
	}
}

func SaveCallback(form *tview.Form) func() {
	return func() {
		log.Println("call SaveCallback")
	}
}

func QuitCallback(app *tview.Application) func() {
	return func() {
		app.Stop()
		// os.Remove("sqltui.log")
	}
}
