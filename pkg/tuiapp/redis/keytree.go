package redis

import (
	"log"

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
	"Bitmap",
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
			// Selecting the root node d
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

func loadRedisTypes(targetNode *tview.TreeNode) {
	for _, keyType := range RedisKeyTypes {
		node := tview.NewTreeNode("[orange]" + keyType).
			SetReference(false).
			SetExpanded(false).
			SetSelectable(true)

		node.SetSelectedFunc(func() {

			if node.GetReference() == false {
				node.SetReference(node.GetText())
				keys, err := RdsClinet.Scan(0, "*", 0, keyType)
				if err != nil {
					log.Println(err)
				}
				log.Printf("redis keys: %+v", keys)

				for _, key := range keys {
					loadRedisKey(node, key)
				}
			}

			log.Printf("node children: %+v", node.GetChildren())
			if len(node.GetChildren()) > 0 {
				log.Printf("node isExpanded: %+v", node.IsExpanded())
				node.SetExpanded(!node.IsExpanded())
			}
		})

		targetNode.AddChild(node)

	}
}

func loadRedisKey(targetNode *tview.TreeNode, key string) {
	node := tview.NewTreeNode(key).
		SetText(key).
		SetReference(nil).
		SetSelectable(true)

	node.SetSelectedFunc(func() {
		// fetch key value
		value, err := RdsClinet.GetValue(key)
		if err != nil {
			log.Println(err)
		}

		PrintfResultTextView("[yellow]%s", value)
	})

	targetNode.AddChild(node)
}
