package redis

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var RootTreeNode *tview.TreeNode

var RedisKeyTypes = []string{
	"String",
	"List",
	"Set",
	"Zset",
	"Hash",
	"Stream",
}

func RenderKeyTreeView() *tview.TreeView {

	RootTreeNode = tview.NewTreeNode("Key Types").
		SetReference(false).
		SetColor(tcell.ColorOlive)

	tree := tview.NewTreeView().
		SetRoot(RootTreeNode).
		SetCurrentNode(RootTreeNode)

	tree.SetBorder(true).SetTitle("[green]Redis Databases")

	RootTreeNode.SetSelectedFunc(func() {
		if RootTreeNode.GetReference() == false {
			// Selecting the root node loads the key type nodes
			loadRedisTypes(RootTreeNode)
			RootTreeNode.SetReference(RootTreeNode.GetText())
			return
		}

		if len(RootTreeNode.GetChildren()) > 0 {
			RootTreeNode.SetExpanded(!RootTreeNode.IsExpanded())
		}
	})

	return tree
}

// RefreshKeyTree re-scans the redis keys so that keys created or deleted by
// commands show up in the tree. Type nodes are reloaded lazily on selection.
func RefreshKeyTree() {
	if RootTreeNode == nil || RootTreeNode.GetReference() == false {
		// tree has not been loaded yet, nothing to refresh
		return
	}
	RootTreeNode.ClearChildren()
	loadRedisTypes(RootTreeNode)
	RootTreeNode.SetExpanded(true)
}

func loadRedisTypes(targetNode *tview.TreeNode) {
	for _, keyType := range RedisKeyTypes {
		node := tview.NewTreeNode("[orange]" + keyType).
			SetReference(false).
			SetExpanded(false).
			SetSelectable(true)

		node.SetSelectedFunc(func() {

			if node.GetReference() == false {
				keys, err := RdsClinet.Scan(0, "*", 100, keyType)
				if err != nil {
					PrintfErrTextView("[red]Error: %s", err)
					return
				}
				node.SetReference(node.GetText())

				for _, key := range keys {
					loadRedisKey(node, key)
				}
				if len(keys) >= ScanLimit {
					PrintfErrTextView("[yellow]Status: showing the first %d %s keys only", ScanLimit, keyType)
				}
			}

			if len(node.GetChildren()) > 0 {
				node.SetExpanded(!node.IsExpanded())
			}
		})

		targetNode.AddChild(node)

	}
}

func loadRedisKey(targetNode *tview.TreeNode, key string) {
	node := tview.NewTreeNode(tview.Escape(key)).
		SetReference(nil).
		SetSelectable(true)

	node.SetSelectedFunc(func() {
		// fetch key value (TYPE is inspected inside GetValue)
		value, err := RdsClinet.GetValue(key)
		if err != nil {
			PrintfErrTextView("[red]Error: %s", err)
			return
		}

		ClearErrTextView()
		PrintfResultTextView("[yellow]%s", value)
	})

	targetNode.AddChild(node)
}
