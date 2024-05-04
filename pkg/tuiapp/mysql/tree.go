package mysql

import (
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var RootTreeNode *tview.TreeNode

func SetRootTreeNodeName(dbName string) {
	RootTreeNode.SetText(dbName)
}

func RenderTreeView() *tview.TreeView {

	RootTreeNode = tview.NewTreeNode("show_tables").
		SetColor(tcell.ColorOlive)

	tree := tview.NewTreeView().
		SetRoot(RootTreeNode).
		SetCurrentNode(RootTreeNode)

	tree.SetBorder(true).SetTitle("[green]Mysql Databases")

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			// Selecting the root node d
			loadTables(GetDB(), node)
			node.SetReference("database")
			return
		}

		if len(node.GetChildren()) > 0 {
			node.SetExpanded(!node.IsExpanded())
		}

	})
	return tree
}

func loadTables(dbClinet *DB, targetNode *tview.TreeNode) {
	tables, err := dbClinet.ShowCurrentDatabaseTables()
	if err != nil {
		panic(err)
	}
	for _, table := range tables {
		node := tview.NewTreeNode(table).
			SetText(table).
			SetReference(table).
			SetSelectable(true)

		node.SetSelectedFunc(func() {
			log.Println("fetch database records !!!!")
			log.Println(node.GetText())
			// execute sql and show results in table
			query := "select * from " + node.GetText()
			fields, result, err := DbClinet.ExecuteRawQuery(query)
			if err != nil {
				PrintfTextView("[red]Error: %s", err)
				ClearTableRecords()
				return
			}
			FillTableWithQueryResult(fields, result)
			PrintfTextView("[yellow]Status: Success !")
			addCommandHistory(query)
		})
		targetNode.AddChild(node)
	}
}
