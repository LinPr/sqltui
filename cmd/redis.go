package cmd

import (
	tuiredis "github.com/LinPr/sqltui/pkg/tuiapp/redis"
	"github.com/spf13/cobra"
)

// redisCmd represents the redis command
var redisCmd = &cobra.Command{
	Use:   "redis",
	Short: "start a redis tui",
	Long:  "start a redis tui",
	Run: func(cmd *cobra.Command, args []string) {
		tuiredis.NewRedisTui()
	},
}

func init() {
	rootCmd.AddCommand(redisCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// redisCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// redisCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
