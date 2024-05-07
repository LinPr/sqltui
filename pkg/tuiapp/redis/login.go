package redis

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/LinPr/sqltui/pkg/config"
	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/redis/go-redis/v9"
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

	flex.SetBorder(true)

	tuiapp.RedisTui.AddPage("redis_login", form)

	return flex
}

var LoginErrOut *tview.TextView

func printfLoginErrOut(format string, a ...any) {
	LoginErrOut.Clear()
	erMsg := fmt.Sprintf(format, a...)
	LoginErrOut.SetText(erMsg)
}

func renderLoginForm() *tview.Form {
	redisConf, err := config.ReadRedisConfig()
	if err != nil {
		log.Println("ReadRedisConfig error: ", err)
		return nil
	}

	form := tview.NewForm().
		AddInputField("username:", redisConf.UserName, 20, nil, nil).
		AddInputField("password:", redisConf.Password, 20, nil, nil).
		AddInputField("    host:", redisConf.Host, 20, nil, nil).
		AddInputField("    port:", redisConf.Port, 20, nil, nil).
		AddInputField("  rdbNum:", redisConf.RdbNum, 20, nil, nil).
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
		rdbNumStr := form.GetFormItem(4).(*tview.InputField).GetText()
		rdbNum, err := strconv.Atoi(rdbNumStr)
		if err != nil {
			printfLoginErrOut("[red]" + err.Error())
			return
		}

		rdsc, err := NewRDS(&redis.Options{
			Addr:         fmt.Sprintf("%s:%s", host, port),
			Username:     username,
			Password:     password,
			DB:           rdbNum,
			WriteTimeout: 3 * time.Second,
			ReadTimeout:  2 * time.Second,
		}) // mmust init database client here
		if err != nil {
			printfLoginErrOut("[red]" + err.Error())
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
			log.Println("WriteMysqlConfig error: ", err)
		}
	}
}

func QuitCallback() func() {
	return func() {
		// app.Stop()
		tuiapp.RedisTui.App.Stop()
	}
}
