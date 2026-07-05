package postgres

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
)

func Init() {
	tuiapp.PostgresTui.AddPage("postgres_login", RenderLoginPage())
	tuiapp.PostgresTui.AddPage("postgres_dashboard", RenderDashBoardPage())

	// first enter into login page
	tuiapp.PostgresTui.ShowPage("postgres_login")
}
