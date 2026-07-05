/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	tuipostgres "github.com/LinPr/sqltui/pkg/tuiapp/postgres"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

// postgresCmd represents the postgres command
var postgresCmd = &cobra.Command{
	Use:     "postgres",
	Aliases: []string{"pg", "postgresql"},
	Short:   "start a postgresql tui",
	Long:    "start a postgresql tui",
	Run: func(cmd *cobra.Command, args []string) {
		tuipostgres.Init()

		layout := tview.NewFlex().
			AddItem(tuiapp.PostgresTui.Pages, 0, 1, true)

		if err := tuiapp.PostgresTui.App.SetRoot(layout, true).
			EnableMouse(true).
			Run(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(postgresCmd)
}
