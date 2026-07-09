package cmd

import (
	"github.com/spf13/cobra"

	"github.com/LinPr/sqltui/internal/ui/dbmode"
)

var redisCmd = &cobra.Command{
	Use:   "redis",
	Short: "Connect to a Redis server",
	Long: `Connect to a live Redis server.

A connection form opens first, asking for host, port, user, password and
database number. It is prefilled from the saved config
(~/.config/sqltui/config.yaml) and ctrl+s stores the values back for next time.

After connecting, a key browser groups keys by type: selecting a key shows its
rendered value, and :query executes raw Redis commands (with inline argument
hints and completion), showing results as pretty JSON.`,
	Example: `  # open the connection form, prefilled from the saved config
  sqltui redis`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbmode.Run(dbmode.KindRedis)
	},
}

func init() {
	rootCmd.AddCommand(redisCmd)
}
