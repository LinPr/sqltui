/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sqliteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sqliteCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
