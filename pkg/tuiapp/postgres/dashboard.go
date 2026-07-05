package postgres

import (
	"fmt"
	"strings"

	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	RootTreeNode *tview.TreeNode
	QueryArea    *tview.TextArea
	TableRecords *tview.Table
	messageOut   *tview.TextView
)

// tree node references
type schemaRef struct {
	schema string
}

type tableRef struct {
	schema string
	table  string
}

func RenderDashBoardPage() tview.Primitive {
	treeView := renderTreeView()
	queryWidget := renderQueryWidget()
	table := renderTable()
	helpBar := renderHelpBar()

	tuiapp.PostgresTui.AddWidget(treeView)
	tuiapp.PostgresTui.AddWidget(QueryArea)
	tuiapp.PostgresTui.AddWidget(table)

	mainFlex := tview.NewFlex().
		AddItem(treeView, 30, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryWidget, 0, 1, false).
			AddItem(table, 0, 3, false), 0, 2, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainFlex, 0, 1, true).
		AddItem(helpBar, 1, 0, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			tuiapp.PostgresTui.SetNextFocus()
			return nil
		case tcell.KeyBacktab:
			tuiapp.PostgresTui.SetPreviousFocus()
			return nil
		case tcell.KeyCtrlR:
			runQuery()
			return nil
		case tcell.KeyEscape:
			tuiapp.PostgresTui.ShowPage("postgres_login")
			return nil
		case tcell.KeyCtrlQ:
			tuiapp.PostgresTui.App.Stop()
			return nil
		}
		return event
	})

	return flex
}

func renderHelpBar() *tview.TextView {
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("[yellow]Tab/Shift+Tab: switch focus | Ctrl+R: run query | Esc: back to login | Ctrl+Q: quit")
	return helpBar
}

func renderQueryWidget() *tview.Flex {
	queryArea := tview.NewTextArea().
		SetPlaceholder("Enter postgres query here, press Ctrl+R to run...")
	QueryArea = queryArea

	textView := renderMessageTextView()

	queryWidget := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(queryArea, 0, 2, false).
		AddItem(textView, 0, 1, false)
	queryWidget.SetBorder(true).SetTitle("[green]Query (Ctrl+R: run)")

	return queryWidget
}

func renderMessageTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetText("[yellow]Status: null").
		SetWrap(true).
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)

	messageOut = textView
	return textView
}

func PrintfMessageView(format string, a ...any) {
	messageOut.Clear()
	fmt.Fprintf(messageOut, format, a...)
}

func runQuery() {
	query := strings.TrimSpace(QueryArea.GetText())
	if query == "" {
		PrintfMessageView("[red]Error: empty query")
		return
	}

	db := GetDB()
	if db == nil {
		PrintfMessageView("[red]Error: not connected to postgres, please login first")
		return
	}

	rawCmdResult, err := db.RawSqlCommand(query)
	if err != nil {
		PrintfMessageView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}

	if rawCmdResult.IsDQL {
		FillTableWithQueryResult(rawCmdResult.Fields, rawCmdResult.Records)
		PrintfMessageView("[yellow]Status: Success ! %d row(s) returned", len(rawCmdResult.Records))
		return
	}

	rowsAffected, err := rawCmdResult.Result.RowsAffected()
	if err != nil {
		PrintfMessageView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}
	// NOTE: LastInsertId is not supported by the pgx stdlib driver
	PrintfMessageView("[yellow]Status: Success ! Rows affected: %d", rowsAffected)
}

func SetRootTreeNodeName(dbName string) {
	if dbName == "" {
		dbName = "postgres"
	}
	RootTreeNode.SetText(dbName)
}

func renderTreeView() *tview.TreeView {
	RootTreeNode = tview.NewTreeNode("postgres").
		SetColor(tcell.ColorOlive)

	tree := tview.NewTreeView().
		SetRoot(RootTreeNode).
		SetCurrentNode(RootTreeNode)

	tree.SetBorder(true).SetTitle("[green]Postgres Schemas")

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			// root node: reload the schema list
			if err := ReloadSchemas(); err != nil {
				PrintfMessageView("[red]Error: %s", err)
			}
			return
		}

		switch ref := reference.(type) {
		case schemaRef:
			if len(node.GetChildren()) == 0 {
				loadTables(node, ref.schema)
			} else {
				node.SetExpanded(!node.IsExpanded())
			}
		case tableRef:
			loadTableRecords(ref.schema, ref.table)
		}
	})

	return tree
}

// ReloadSchemas refreshes the schema nodes under the root tree node.
func ReloadSchemas() error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("not connected to postgres")
	}

	schemas, err := db.ListSchemas()
	if err != nil {
		return err
	}

	RootTreeNode.ClearChildren()
	for _, schema := range schemas {
		node := tview.NewTreeNode(schema).
			SetReference(schemaRef{schema: schema}).
			SetSelectable(true).
			SetColor(tcell.ColorGreen)
		RootTreeNode.AddChild(node)
	}
	RootTreeNode.SetExpanded(true)
	return nil
}

func loadTables(schemaNode *tview.TreeNode, schema string) {
	db := GetDB()
	if db == nil {
		PrintfMessageView("[red]Error: not connected to postgres, please login first")
		return
	}

	tables, err := db.ListTables(schema)
	if err != nil {
		PrintfMessageView("[red]Error: %s", err)
		return
	}
	if len(tables) == 0 {
		PrintfMessageView("[yellow]Status: no tables in schema %q", schema)
		return
	}

	for _, table := range tables {
		node := tview.NewTreeNode(table).
			SetReference(tableRef{schema: schema, table: table}).
			SetSelectable(true)
		schemaNode.AddChild(node)
	}
	schemaNode.SetExpanded(true)
}

func loadTableRecords(schema, table string) {
	db := GetDB()
	if db == nil {
		PrintfMessageView("[red]Error: not connected to postgres, please login first")
		return
	}

	fields, records, err := db.FetchTableRecords(schema, table)
	if err != nil {
		PrintfMessageView("[red]Error: %s", err)
		ClearTableRecords()
		return
	}

	FillTableWithQueryResult(fields, records)
	PrintfMessageView("[yellow]Status: Success ! %d row(s) fetched from %s.%s (limit %d)",
		len(records), schema, table, FetchLimit)
}

func renderTable() *tview.Table {
	table := tview.NewTable().
		SetBorders(true).
		SetSeparator('|').
		SetFixed(1, 0).
		Select(0, 0)

	table.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			table.SetSelectable(true, true)
		}
	})

	table.SetSelectedFunc(func(row int, column int) {
		table.GetCell(row, column).SetTextColor(tcell.ColorRed)
		table.SetSelectable(false, false)
	})

	table.SetBorder(true).SetTitle("[green]Result Table")

	TableRecords = table
	return TableRecords
}

func ClearTableRecords() {
	TableRecords.Clear()
}

func FillTableWithQueryResult(fields []string, result [][]string) {
	TableRecords.Clear()

	// 1. fill the first line with field names
	startRow := 0
	if fields != nil {
		for j, field := range fields {
			setCell(startRow, j, field, tcell.ColorYellow)
		}
		startRow += 1
	}

	// 2. fill result records
	for i, row := range result {
		for j, cell := range row {
			setCell(startRow+i, j, cell, tcell.ColorWhite)
		}
	}
}

func setCell(row int, column int, text string, color tcell.Color) {
	cell := tview.NewTableCell(text).
		SetTextColor(color).
		SetAlign(tview.AlignCenter)
	TableRecords.SetCell(row, column, cell)
}
