package mysql

import (
	"fmt"
	"log"

	"github.com/LinPr/sqltui/pkg/config"
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

	// tuiapp.MysqlTui.AddPage("mysql_login", form)

	return flex
}

func renderLoginForm() *tview.Form {
	mysqlConf, err := config.ReadMySqlConfig()
	if err != nil {
		log.Println("ReadMySqlConfig error: ", err)
		return nil
	}

	form := tview.NewForm().
		AddInputField("username:", mysqlConf.UserName, 20, nil, nil).
		AddInputField("password:", mysqlConf.Password, 20, nil, nil).
		AddInputField("    host:", mysqlConf.Host, 20, nil, nil).
		AddInputField("    port:", mysqlConf.Port, 20, nil, nil).
		AddInputField("  dbname:", mysqlConf.DbName, 20, nil, nil).
		SetFieldBackgroundColor(tcell.ColorGray)
	// AddDropDown(" charset:", []string{"utf8", "ascall", "unicode"}, 0, nil)

	form.AddButton("Connect", ConnectCallback(form)).
		AddButton("Save", SaveCallback(form)).
		AddButton("Quit", QuitCallback()).
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tcell.ColorGray).
		SetButtonTextColor(tcell.ColorLightGoldenrodYellow)

	form.SetBorder(true).SetBorderColor(tcell.ColorWhite)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

		_ = dbc

		// save current config
		SaveCallback(form)()

		SetRootTreeNodeName(GetDbName())
		tuiapp.MysqlTui.ShowPage("mysql_dashboard")
	}
}

func SaveCallback(form *tview.Form) func() {
	return func() {
		mysqlConfig := &config.MysqlConfig{
			UserName: form.GetFormItem(0).(*tview.InputField).GetText(),
			Password: form.GetFormItem(1).(*tview.InputField).GetText(),
			Host:     form.GetFormItem(2).(*tview.InputField).GetText(),
			Port:     form.GetFormItem(3).(*tview.InputField).GetText(),
			DbName:   form.GetFormItem(4).(*tview.InputField).GetText(),
		}
		if err := config.WriteMysqlConfig(mysqlConfig); err != nil {
			log.Println("WriteMysqlConfig error: ", err)
		}
	}
}

func QuitCallback() func() {
	return func() {
		// app.Stop()
		tuiapp.MysqlTui.App.Stop()
	}
}
