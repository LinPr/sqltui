package sqlite

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
)

func Init() {
	tuiapp.SqliteTui.AddPage("sqlite_login", RenderLoginPage())
	tuiapp.SqliteTui.AddPage("sqlite_dashboard", RenderDashBoardPage())

	// first enter into login page
	tuiapp.SqliteTui.ShowPage("sqlite_login")
}
