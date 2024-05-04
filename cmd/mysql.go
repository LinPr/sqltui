/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	_ "github.com/LinPr/sqltui/pkg/tuiapp/mysql"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

// mysqlCmd represents the mysql command
var mysqlCmd = &cobra.Command{
	Use:   "mysql",
	Short: "start a mysql tui",
	Long:  "start a mysql tui",
	Run: func(cmd *cobra.Command, args []string) {

		layout := tview.NewFlex().
			AddItem(tuiapp.Pages, 0, 1, true)
		tuiapp.TuiApp.MysqlApp = tview.NewApplication()
		if err := tuiapp.TuiApp.MysqlApp.SetRoot(layout, true).
			EnableMouse(true).
			Run(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(mysqlCmd)

	// mysqlCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
