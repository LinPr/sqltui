package redis

import (
	"fmt"
	"strconv"
	"time"

	"github.com/LinPr/sqltui/pkg/config"
	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rivo/tview"
)

func RenderLoginPage() *tview.Flex {

	// build the error text view first so that errors during form
	// construction (e.g. a corrupt config file) are shown on screen
	textView := renderLoginErrTextView()
	form := renderLoginForm()

	flex := tview.NewFlex().
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 1, false).
			AddItem(form, 15, 3, true).
			AddItem(textView, 0, 1, false), 50, 3, true).
		AddItem(tview.NewBox().SetBorder(false).SetTitle(""), 0, 2, false)

	return flex
}

var LoginErrOut *tview.TextView

func printfLoginErrOut(format string, a ...any) {
	if LoginErrOut == nil {
		return
	}
	LoginErrOut.Clear()
	erMsg := fmt.Sprintf(format, tuiapp.EscapeArgs(a)...)
	LoginErrOut.SetText(erMsg)
}

func renderLoginForm() *tview.Form {
	redisConf, err := config.ReadRedisConfig()
	if err != nil {
		// fall back to an empty config, the user can still type everything in
		redisConf = &config.RedisConfig{}
		printfLoginErrOut("[red]ReadRedisConfig error: %s", err)
	}

	form := tview.NewForm().
		AddInputField("username:", redisConf.UserName, 25, nil, nil).
		AddPasswordField("password:", redisConf.Password, 25, '*', nil).
		AddInputField("    host:", redisConf.Host, 25, nil, nil).
		AddInputField("    port:", redisConf.Port, 25, nil, nil).
		AddInputField("  rdbNum:", redisConf.RdbNum, 25, nil, nil).
		SetFieldBackgroundColor(tcell.ColorGray)
	// AddDropDown(" charset:", []string{"utf8", "ascall", "unicode"}, 0, nil)

	form.AddButton("Connect", ConnectCallback(form)).
		AddButton("Save", SaveCallback(form)).
		AddButton("Quit", QuitCallback()).
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tcell.ColorGray).
		SetButtonTextColor(tcell.ColorLightGoldenrodYellow)

	form.SetBorder(true).SetBorderColor(tcell.ColorWhite).SetTitle("[green]Redis Login")

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlS:
			SaveCallback(form)()
			return nil
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
		username := form.GetFormItem(0).(*tview.InputField).GetText()
		password := form.GetFormItem(1).(*tview.InputField).GetText()
		host := form.GetFormItem(2).(*tview.InputField).GetText()
		port := form.GetFormItem(3).(*tview.InputField).GetText()
		rdbNumStr := form.GetFormItem(4).(*tview.InputField).GetText()
		rdbNum, err := strconv.Atoi(rdbNumStr)
		if err != nil {
			printfLoginErrOut("[red]Error: %s", err.Error())
			return
		}

		rdsc, err := NewRDS(&redis.Options{
			Addr:         fmt.Sprintf("%s:%s", host, port),
			Username:     username,
			Password:     password,
			DB:           rdbNum,
			WriteTimeout: 3 * time.Second,
			ReadTimeout:  2 * time.Second,
		}) // must init database client here
		if err != nil {
			printfLoginErrOut("[red]Error: %s", err.Error())
			return
		}
		_ = rdsc

		// save current config
		SaveCallback(form)()

		// SetRootTreeNodeName(GetDbName())
		tuiapp.RedisTui.ShowPage("redis_dashboard")
	}
}

func SaveCallback(form *tview.Form) func() {
	return func() {
		redisConf := &config.RedisConfig{
			UserName: form.GetFormItem(0).(*tview.InputField).GetText(),
			Password: form.GetFormItem(1).(*tview.InputField).GetText(),
			Host:     form.GetFormItem(2).(*tview.InputField).GetText(),
			Port:     form.GetFormItem(3).(*tview.InputField).GetText(),
			RdbNum:   form.GetFormItem(4).(*tview.InputField).GetText(),
		}

		if err := config.WriteRedisConfig(redisConf); err != nil {
			printfLoginErrOut("[red]WriteRedisConfig error: %s", err)
		}
	}
}

func QuitCallback() func() {
	return func() {
		// app.Stop()
		tuiapp.RedisTui.App.Stop()
	}
}
