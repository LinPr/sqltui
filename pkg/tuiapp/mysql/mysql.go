package mysql

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	// "github.com/rivo/tview"
)

func init() {
	tuiapp.AddPage("mysql_login", RenderLoginPage())
	tuiapp.AddPage("mysql_dashboard", RenderDashBoardPage())

	// first enter into login page
	tuiapp.ShowPage("mysql_login")
	// tuiapp.ShowPage("mysql_dashboard")
}
