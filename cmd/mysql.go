package cmd

import (
	"github.com/spf13/cobra"

	"github.com/LinPr/sqltui/internal/ui/dbmode"
)

var mysqlCmd = &cobra.Command{
	Use:   "mysql",
	Short: "Connect to a MySQL server",
	Long: `Connect to a live MySQL server.

A connection form opens first, asking for host, port, user, password and
database. It is prefilled from the saved config (~/.config/sqltui/config.yaml)
and ctrl+s stores the values back for next time.

After connecting, the schema browser lists the server's databases and tables:
selecting a table loads its rows into a tab, and :query runs any SQL statement
against the live connection (non-query statements report rows affected).`,
	Example: `  # open the connection form, prefilled from the saved config
  sqltui mysql`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbmode.Run(dbmode.KindMysql)
	},
}

func init() {
	rootCmd.AddCommand(mysqlCmd)
}
