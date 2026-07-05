package mysql

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var RootTreeNode *tview.TreeNode

// SetRootTreeNodeName is called after a successful login. It renames the
// root node and (re)loads the database list underneath it.
func SetRootTreeNodeName(dbName string) {
	if dbName == "" {
		dbName = "mysql"
	}
	RootTreeNode.SetText(tview.Escape(dbName))
	RootTreeNode.ClearChildren()
	loadDatabases(GetDB(), RootTreeNode)
	RootTreeNode.SetReference("root")
	RootTreeNode.SetExpanded(true)
}

func RenderTreeView() *tview.TreeView {

	RootTreeNode = tview.NewTreeNode("mysql").
		SetColor(tcell.ColorOlive)

	tree := tview.NewTreeView().
		SetRoot(RootTreeNode).
		SetCurrentNode(RootTreeNode)

	tree.SetBorder(true).SetTitle("[green]Mysql Databases")

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			// Selecting the root node before it has been populated
			loadDatabases(GetDB(), node)
			node.SetReference("root")
			return
		}

		if len(node.GetChildren()) > 0 {
			node.SetExpanded(!node.IsExpanded())
		}
	})
	return tree
}

// loadDatabases fills targetNode with one child node per database.
// Selecting a database node lazily loads its tables.
func loadDatabases(dbClinet *DB, targetNode *tview.TreeNode) {
	if dbClinet == nil {
		PrintfTextView("[red]Error: not connected to a mysql server yet")
		return
	}
	databases, err := dbClinet.ShowDatabases()
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		return
	}
	for _, database := range databases {
		dbNode := tview.NewTreeNode(tview.Escape(database)).
			SetReference(database).
			SetColor(tcell.ColorGreen).
			SetSelectable(true)

		dbNode.SetSelectedFunc(func() {
			// Lazily load tables the first time the database is selected.
			// Expanding/collapsing already loaded nodes is handled by the
			// tree-level selected callback.
			if len(dbNode.GetChildren()) == 0 {
				loadTables(dbClinet, database, dbNode)
			}
		})
		targetNode.AddChild(dbNode)
	}
}

// loadTables fills targetNode with one child node per table of the given
// database. Selecting a table node shows its records in the result table.
func loadTables(dbClinet *DB, database string, targetNode *tview.TreeNode) {
	tables, err := dbClinet.ShowDatabaseTables(database)
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		return
	}
	if len(tables) == 0 {
		PrintfTextView("[yellow]Status: no tables in database %s", database)
		return
	}
	for _, table := range tables {
		fullName := database + "." + table
		node := tview.NewTreeNode(tview.Escape(table)).
			SetReference(fullName).
			SetSelectable(true)

		node.SetSelectedFunc(func() {
			showTableRecords(database, table)
		})
		targetNode.AddChild(node)
	}
}

// showTableRecords executes "select * from <db>.<table>" and shows the rows
// (with a header row of column names) in the result table.
func showTableRecords(database, table string) {
	query := fmt.Sprintf("select * from %s.%s limit %d",
		quoteIdent(database), quoteIdent(table), FetchLimit)
	rawCmdResult, err := DbClinet.RawSqlCommand(query)
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}
	FillTableWithQueryResult(rawCmdResult.Fields, rawCmdResult.Records)
	PrintfTextView("[yellow]Status: Success ! %d row(s) fetched (limit %d)",
		len(rawCmdResult.Records), FetchLimit)
	addCommandHistory(query)
}
