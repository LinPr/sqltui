package cmd

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	tuisqlite "github.com/LinPr/sqltui/pkg/tuiapp/sqlite"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

// sqliteCmd represents the sqlite command
var sqliteCmd = &cobra.Command{
	Use:   "sqlite",
	Short: "start a sqlite tui",
	Long:  "start a sqlite tui",
	Run: func(cmd *cobra.Command, args []string) {
		tuisqlite.Init()

		layout := tview.NewFlex().
			AddItem(tuiapp.SqliteTui.Pages, 0, 1, true)

		if err := tuiapp.SqliteTui.App.SetRoot(layout, true).
			EnableMouse(true).
			Run(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(sqliteCmd)
}
