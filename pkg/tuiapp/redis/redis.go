package redis

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	// "github.com/rivo/tview"
)

func Init() {
	tuiapp.RedisTui.AddPage("redis_login", RenderLoginPage())
	tuiapp.RedisTui.AddPage("redis_dashboard", RenderDashBoardPage())

	// first enter into login page
	tuiapp.RedisTui.ShowPage("redis_login")
	// tuiapp.ShowPage("mysql_dashboard")
}
