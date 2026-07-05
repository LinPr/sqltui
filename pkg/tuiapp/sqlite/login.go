package sqlite

import (
	"fmt"

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
			AddItem(form, 9, 3, true).
			AddItem(textView, 0, 1, false), 50, 3, true).
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false)

	flex.SetBorder(true)

	return flex
}

var LoginErrOut *tview.TextView

func printfLoginErrOut(format string, a ...any) {
	LoginErrOut.Clear()
	erMsg := fmt.Sprintf(format, a...)
	LoginErrOut.SetText(erMsg)
}

func renderLoginForm() *tview.Form {
	filePath := ""
	if sqliteConf, err := config.ReadSqliteConfig(); err == nil && sqliteConf != nil {
		filePath = sqliteConf.FilePath
	}

	form := tview.NewForm().
		AddInputField("filepath:", filePath, 35, nil, nil).
		SetFieldBackgroundColor(tcell.ColorGray)

	form.AddButton("Connect", ConnectCallback(form)).
		AddButton("Save", SaveCallback(form)).
		AddButton("Quit", QuitCallback()).
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tcell.ColorGray).
		SetButtonTextColor(tcell.ColorLightGoldenrodYellow)

	form.SetBorder(true).SetBorderColor(tcell.ColorWhite).SetTitle("[green]Sqlite Login")

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

func renderLoginErrTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true)

	LoginErrOut = textView
	return textView
}

func ConnectCallback(form *tview.Form) func() {
	return func() {
		filePath := form.GetFormItem(0).(*tview.InputField).GetText()

		// must init database client here
		if _, err := NewDB(filePath); err != nil {
			printfLoginErrOut("[red]%s", err.Error())
			return
		}

		// save current config
		SaveCallback(form)()

		RefreshTree()
		tuiapp.SqliteTui.ShowPage("sqlite_dashboard")
	}
}

func SaveCallback(form *tview.Form) func() {
	return func() {
		sqliteConf := &config.SqliteConfig{
			FilePath: form.GetFormItem(0).(*tview.InputField).GetText(),
		}

		if err := config.WriteSqliteConfig(sqliteConf); err != nil {
			printfLoginErrOut("[red]WriteSqliteConfig error: %s", err)
		}
	}
}

func QuitCallback() func() {
	return func() {
		tuiapp.SqliteTui.App.Stop()
	}
}
