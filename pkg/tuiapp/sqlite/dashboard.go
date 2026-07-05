package sqlite

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	RootTreeNode *tview.TreeNode
	TableRecords *tview.Table
	QueryArea    *tview.TextArea

	textViewOut *tview.TextView
)

func RenderDashBoardPage() *tview.Flex {
	treeView := renderTreeView()
	queryWidget := renderQueryWidget()
	table := renderTable()
	helpBar := tuiapp.RenderHelpBar("Tab/Shift+Tab: switch focus | Ctrl+R: run query | Esc: back to login | Ctrl+Q: quit")

	tuiapp.SqliteTui.AddWidget(treeView)
	tuiapp.SqliteTui.AddWidget(QueryArea)
	tuiapp.SqliteTui.AddWidget(table)

	mainArea := tview.NewFlex().
		AddItem(treeView, 30, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryWidget, 0, 1, false).
			AddItem(table, 0, 3, false), 0, 2, false)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainArea, 0, 1, true).
		AddItem(helpBar, 1, 0, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			tuiapp.SqliteTui.SetNextFocus()
			return nil
		case tcell.KeyBacktab:
			tuiapp.SqliteTui.SetPreviousFocus()
			return nil
		case tcell.KeyCtrlR:
			runQuery()
			return nil
		case tcell.KeyEscape:
			tuiapp.SqliteTui.ShowPage("sqlite_login")
			return nil
		case tcell.KeyCtrlQ:
			tuiapp.SqliteTui.App.Stop()
			return nil
		}
		return event // this event should be returned and not to return nil
	})

	return flex
}

func renderTreeView() *tview.TreeView {
	RootTreeNode = tview.NewTreeNode("sqlite").
		SetColor(tcell.ColorOlive)

	tree := tview.NewTreeView().
		SetRoot(RootTreeNode).
		SetCurrentNode(RootTreeNode)

	tree.SetBorder(true).SetTitle("[green]Sqlite Tables")

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if node == RootTreeNode {
			// reload the table list from the database file
			RefreshTree()
			return
		}

		if len(node.GetChildren()) > 0 {
			node.SetExpanded(!node.IsExpanded())
		}
	})
	return tree
}

// RefreshTree reloads the table list of the currently connected database file.
func RefreshTree() {
	if DbClinet == nil {
		PrintfTextView("[red]Error: not connected to a sqlite database")
		return
	}

	RootTreeNode.SetText(tview.Escape(filepath.Base(GetDbFile())))
	RootTreeNode.ClearChildren()

	tables, err := DbClinet.ListTables()
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		return
	}

	for _, table := range tables {
		tableName := table
		node := tview.NewTreeNode(tview.Escape(tableName)).
			SetReference(tableName).
			SetSelectable(true)

		node.SetSelectedFunc(func() {
			// show table records in the result table
			executeAndShow(fmt.Sprintf("select * from %s limit %d",
				quoteIdentifier(tableName), FetchLimit))
		})
		RootTreeNode.AddChild(node)
	}
	RootTreeNode.SetExpanded(true)
}

func renderQueryWidget() *tview.Flex {
	QueryArea = tview.NewTextArea().
		SetPlaceholder("Enter sqlite query here, press Ctrl+R to run...").
		SetPlaceholderStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray))

	textView := renderTextView()

	queryWidget := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(QueryArea, 0, 2, false).
		AddItem(textView, 0, 1, false)
	queryWidget.SetBorder(true).SetTitle("[green]Query (Ctrl+R: run)")

	return queryWidget
}

func runQuery() {
	query := strings.TrimSpace(QueryArea.GetText())
	if query == "" {
		PrintfTextView("[red]Error: empty query")
		return
	}
	executeAndShow(query)
}

func executeAndShow(query string) {
	if DbClinet == nil {
		PrintfTextView("[red]Error: not connected to a sqlite database")
		return
	}

	rawCmdResult, err := DbClinet.RawSqlCommand(query)
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}

	if rawCmdResult.IsDQL {
		FillTableWithQueryResult(rawCmdResult.Fields, rawCmdResult.Records)
		PrintfTextView("[yellow]Status: Success !")
		return
	}

	rowsAffected, err := rawCmdResult.Result.RowsAffected()
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}
	lastInsertId, err := rawCmdResult.Result.LastInsertId()
	if err != nil {
		PrintfTextView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}
	PrintfTextView("[yellow]Status: Success ! \n\t Rows affected: %d, Last Insert ID: %d", rowsAffected, lastInsertId)
}

func renderTable() *tview.Table {
	table := tview.NewTable().
		SetBorders(true).
		SetSeparator('|').
		SetFixed(1, 0).
		SetSelectable(true, false).
		Select(0, 0)

	table.SetBorder(true).SetTitle("[green]Result Table")

	TableRecords = table
	return TableRecords
}

func ClearTableRecords() {
	TableRecords.Clear()
}

func FillTableWithQueryResult(fields []string, records [][]string) {
	TableRecords.Clear()
	// 1. fill the first line with field names (fixed, non-selectable header)
	startRow := 0
	if len(fields) > 0 {
		for j, field := range fields {
			cell := tview.NewTableCell(tview.Escape(field)).
				SetTextColor(tcell.ColorYellow).
				SetAttributes(tcell.AttrBold).
				SetAlign(tview.AlignCenter).
				SetSelectable(false)
			TableRecords.SetCell(startRow, j, cell)
		}
		startRow += 1
	}

	// 2. fill records
	for i, row := range records {
		for j, cell := range row {
			setCell(startRow+i, j, cell, tcell.ColorWhite)
		}
	}

	TableRecords.ScrollToBeginning()
	if len(records) > 0 {
		TableRecords.Select(startRow, 0)
	}
}

func setCell(row int, column int, text string, color tcell.Color) {
	cell := tview.NewTableCell(tview.Escape(text)).
		SetTextColor(color).
		SetAlign(tview.AlignCenter)
	TableRecords.SetCell(row, column, cell)
}

func renderTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetText("[yellow]Status: null").
		SetWrap(true).
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)

	textViewOut = textView
	return textView
}

// PrintfTextView prints status / error messages to the dashboard text view.
func PrintfTextView(format string, a ...any) {
	textViewOut.Clear()
	fmt.Fprintf(textViewOut, format, tuiapp.EscapeArgs(a)...)
}
