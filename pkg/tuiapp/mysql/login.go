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

	return flex
}

func renderLoginForm() *tview.Form {
	mysqlConf, err := config.ReadMySqlConfig()
	if err != nil {
		log.Println("ReadMySqlConfig error: ", err)
		mysqlConf = &config.MysqlConfig{}
	}

	form := tview.NewForm().
		AddInputField("username:", mysqlConf.UserName, 25, nil, nil).
		AddPasswordField("password:", mysqlConf.Password, 25, '*', nil).
		AddInputField("    host:", mysqlConf.Host, 25, nil, nil).
		AddInputField("    port:", mysqlConf.Port, 25, nil, nil).
		AddInputField("  dbname:", mysqlConf.DbName, 25, nil, nil).
		SetFieldBackgroundColor(tcell.ColorGray)

	form.AddButton("Connect", ConnectCallback(form)).
		AddButton("Save", SaveCallback(form)).
		AddButton("Quit", QuitCallback()).
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tcell.ColorGray).
		SetButtonTextColor(tcell.ColorLightGoldenrodYellow)

	form.SetBorder(true).SetBorderColor(tcell.ColorWhite).SetTitle("[green]Mysql Login")

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
		username := form.GetFormItem(0).(*tview.InputField).GetText()
		password := form.GetFormItem(1).(*tview.InputField).GetText()
		host := form.GetFormItem(2).(*tview.InputField).GetText()
		port := form.GetFormItem(3).(*tview.InputField).GetText()
		dbname := form.GetFormItem(4).(*tview.InputField).GetText()

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", username, password, host, port, dbname)

		if _, err := NewDB(dsn); err != nil { // must init database client here
			printfLoginErrOut("[red]%s", err.Error())
			return
		}
		printfLoginErrOut("")

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
			printfLoginErrOut("[red]WriteMysqlConfig error: %s", err.Error())
		}
	}
}

func QuitCallback() func() {
	return func() {
		tuiapp.MysqlTui.App.Stop()
	}
}
