package cmd

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	tuimysql "github.com/LinPr/sqltui/pkg/tuiapp/mysql"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var mysqlCmd = &cobra.Command{
	Use:   "mysql",
	Short: "start a mysql tui",
	Long:  "start a mysql tui",
	Run: func(cmd *cobra.Command, args []string) {
		tuimysql.Init()

		layout := tview.NewFlex().
			AddItem(tuiapp.MysqlTui.Pages, 0, 1, true)

		if err := tuiapp.MysqlTui.App.SetRoot(layout, true).
			EnableMouse(true).
			Run(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(mysqlCmd)
}
