package sqlite

import (
	"log"

	"github.com/LinPr/sqltui/pkg/tuiapp"
	// "github.com/rivo/tview"
)

func Init() {
	tuiapp.SqliteTui.AddPage("sqlite_login", RenderLoginPage())
	tuiapp.SqliteTui.AddPage("sqlite_dashboard", RenderDashBoardPage())
	log.Println("has page sqlite_login", tuiapp.SqliteTui.Pages.HasPage("sqlite_login"))
	log.Printf("%v", tuiapp.SqliteTui.Pages)
	// first enter into login page
	tuiapp.SqliteTui.ShowPage("sqlite_login")
	// tuiapp.ShowPage("sqlite_dashboard")
}
