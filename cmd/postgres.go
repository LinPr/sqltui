package cmd

import (
	"github.com/spf13/cobra"

	"github.com/LinPr/sqltui/internal/ui/dbmode"
)

var postgresCmd = &cobra.Command{
	Use:     "postgres",
	Aliases: []string{"pg", "postgresql"},
	Short:   "Connect to a PostgreSQL server",
	Long: `Connect to a live PostgreSQL server.

A connection form opens first, asking for host, port, user, password, database
and ssl mode. It is prefilled from the saved config
(~/.config/sqltui/config.yaml) and ctrl+s stores the values back for next time.

After connecting, the schema browser lists schemas and tables: selecting a
table loads its rows into a tab, and :query runs any SQL statement against the
live connection (non-query statements report rows affected).`,
	Example: `  # open the connection form, prefilled from the saved config
  sqltui postgres

  # "pg" and "postgresql" work too
  sqltui pg`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbmode.Run(dbmode.KindPostgres)
	},
}

func init() {
	rootCmd.AddCommand(postgresCmd)
}
