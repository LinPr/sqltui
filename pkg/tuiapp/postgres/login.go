package postgres

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
			AddItem(form, 17, 3, true).
			AddItem(textView, 0, 1, false), 50, 3, true).
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false)

	return flex
}

func renderLoginForm() *tview.Form {
	postgresConf, err := config.ReadPostgresConfig()
	if err != nil {
		log.Println("ReadPostgresConfig error: ", err)
		postgresConf = &config.PostgresConfig{}
	}

	form := tview.NewForm().
		AddInputField("username:", postgresConf.UserName, 25, nil, nil).
		AddPasswordField("password:", postgresConf.Password, 25, '*', nil).
		AddInputField("    host:", postgresConf.Host, 25, nil, nil).
		AddInputField("    port:", postgresConf.Port, 25, nil, nil).
		AddInputField("  dbname:", postgresConf.DbName, 25, nil, nil).
		AddInputField(" sslmode:", postgresConf.SslMode, 25, nil, nil).
		SetFieldBackgroundColor(tcell.ColorGray)

	form.AddButton("Connect", ConnectCallback(form)).
		AddButton("Save", SaveCallback(form)).
		AddButton("Quit", QuitCallback()).
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tcell.ColorGray).
		SetButtonTextColor(tcell.ColorLightGoldenrodYellow)

	form.SetBorder(true).
		SetBorderColor(tcell.ColorWhite).
		SetTitle("[green]Postgres Login")

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
	errMsg := fmt.Sprintf(format, a...)
	LoginErrOut.SetText(errMsg)
}

func renderLoginErrTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true)

	LoginErrOut = textView
	return textView
}

func formPostgresConfig(form *tview.Form) *config.PostgresConfig {
	return &config.PostgresConfig{
		UserName: form.GetFormItem(0).(*tview.InputField).GetText(),
		Password: form.GetFormItem(1).(*tview.InputField).GetText(),
		Host:     form.GetFormItem(2).(*tview.InputField).GetText(),
		Port:     form.GetFormItem(3).(*tview.InputField).GetText(),
		DbName:   form.GetFormItem(4).(*tview.InputField).GetText(),
		SslMode:  form.GetFormItem(5).(*tview.InputField).GetText(),
	}
}

func ConnectCallback(form *tview.Form) func() {
	return func() {
		conf := formPostgresConfig(form)

		dsn := BuildDsn(conf.UserName, conf.Password, conf.Host, conf.Port, conf.DbName, conf.SslMode)

		if _, err := NewDB(dsn, conf.DbName); err != nil {
			printfLoginErrOut("[red]Error: %s", err.Error())
			return
		}

		// save current config on successful connection
		SaveCallback(form)()

		SetRootTreeNodeName(GetDbName())
		if err := ReloadSchemas(); err != nil {
			printfLoginErrOut("[red]Error: %s", err.Error())
			return
		}

		printfLoginErrOut("")
		tuiapp.PostgresTui.ShowPage("postgres_dashboard")
	}
}

func SaveCallback(form *tview.Form) func() {
	return func() {
		if err := config.WritePostgresConfig(formPostgresConfig(form)); err != nil {
			log.Println("WritePostgresConfig error: ", err)
		}
	}
}

func QuitCallback() func() {
	return func() {
		tuiapp.PostgresTui.App.Stop()
	}
}
