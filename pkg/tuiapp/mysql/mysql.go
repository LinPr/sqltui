package mysql

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	// "github.com/rivo/tview"
)

func Init() {
	tuiapp.MysqlTui.AddPage("mysql_login", RenderLoginPage())
	tuiapp.MysqlTui.AddPage("mysql_dashboard", RenderDashBoardPage())

	// first enter into login page
	tuiapp.MysqlTui.ShowPage("mysql_login")
	// tuiapp.ShowPage("mysql_dashboard")
}
