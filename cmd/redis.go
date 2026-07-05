package cmd

import (
	"github.com/LinPr/sqltui/pkg/tuiapp"
	tuiredis "github.com/LinPr/sqltui/pkg/tuiapp/redis"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

// redisCmd represents the redis command
var redisCmd = &cobra.Command{
	Use:   "redis",
	Short: "start a redis tui",
	Long:  "start a redis tui",
	Run: func(cmd *cobra.Command, args []string) {
		tuiredis.Init()

		layout := tview.NewFlex().
			AddItem(tuiapp.RedisTui.Pages, 0, 1, true)

		if err := tuiapp.RedisTui.App.SetRoot(layout, true).
			EnableMouse(true).
			Run(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(redisCmd)
}
