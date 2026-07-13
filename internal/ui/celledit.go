// Package ui celledit holds the commit helpers for single-cell edits and
// row deletes. In database mode it builds and runs UPDATE/DELETE statements
// keyed on the table's primary key; in file mode it rewrites the frame
// in place through WithCell/WithoutRows and pushes the new frame.
package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinPr/sqltui/internal/data"
	"github.com/LinPr/sqltui/internal/db"
)

// PendingEdit captures an in-flight cell edit awaiting confirmation. The
// popup that collects the new value populates this and the app commits it
// via the "saveedit" command.
type PendingEdit struct {
	Frame    *data.Frame
	Row      int
	Col      int
	ColName  string
	OldValue string
	NewValue string
	Table    string
	Namespace string
}

// PendingDelete captures one or more rows awaiting deletion. The app populates
// it from the multi-select set (or the single cursor row) and commits it via
// the "deleterows" command.
type PendingDelete struct {
	Frame    *data.Frame
	Rows     []int
	Table    string
	Namespace string
}

// cellSavedMsg carries the outcome of an async UPDATE back to the app.
type cellSavedMsg struct {
	rows int64
	err  error
}

// rowsDeletedMsg carries the outcome of an async DELETE back to the app.
type rowsDeletedMsg struct {
	rows int64
	err  error
}

// quoteIdent quotes a SQL identifier for the backend's dialect: backticks for
// mysql, double quotes otherwise. Embedded quote characters are doubled.
func quoteIdent(be db.Backend, name string) string {
	if be != nil && be.Kind() == "mysql" {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// quoteCol quotes a column identifier for the active dialect.
func quoteCol(be db.Backend, name string) string {
	return quoteIdent(be, name)
}

// sqlLiteral wraps a string value in single quotes with embedded quotes
// doubled.
func sqlLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// parseTyped converts the edited string to the cell's native Go type so
// WithCell preserves the column type.
func parseTyped(val string, t data.DType) (any, error) {
	switch t {
	case data.TypeString:
		return val, nil
	case data.TypeBool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("expected bool, got %q", val)
		}
		return b, nil
	case data.TypeInt, data.TypeDate, data.TypeDatetime:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", val)
		}
		return n, nil
	case data.TypeFloat:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, fmt.Errorf("expected float, got %q", val)
		}
		return f, nil
	default:
		return val, nil
	}
}

// buildUpdate returns a single-row UPDATE keyed on the table's primary key.
// The table identifier is dialect-quoted; PK values are read from frame. It
// errors when a named PK column is absent from the frame.
func buildUpdate(be db.Backend, table string, pks []string, frame *data.Frame, row int, colName, val string) (string, error) {
	setClause := quoteCol(be, colName) + "=" + sqlLiteral(val)
	var where []string
	for _, pk := range pks {
		idx := frame.ColumnIndex(pk)
		if idx < 0 {
			return "", fmt.Errorf("primary key column not found: %s", pk)
		}
		where = append(where, quoteCol(be, pk)+"="+sqlLiteral(frame.CellString(row, idx)))
	}
	return "UPDATE " + quoteIdent(be, table) + " SET " + setClause + " WHERE " + strings.Join(where, " AND "), nil
}

// buildDelete returns a DELETE keyed on the table's primary key. Single-PK
// tables produce a "WHERE pk IN (...)" clause; multi-PK tables produce a
// "WHERE (pk0=v AND pk1=v) OR (...)" clause. It errors when there are no PKs
// (the caller checks len(pks)==0 first) or a PK column is missing.
func buildDelete(be db.Backend, table string, pks []string, frame *data.Frame, rows []int) (string, error) {
	if len(pks) == 0 {
		return "", fmt.Errorf("no primary key")
	}
	pkIdx := make([]int, len(pks))
	for i, pk := range pks {
		idx := frame.ColumnIndex(pk)
		if idx < 0 {
			return "", fmt.Errorf("primary key column not found: %s", pk)
		}
		pkIdx[i] = idx
	}
	tbl := quoteIdent(be, table)
	if len(pks) == 1 {
		vals := make([]string, 0, len(rows))
		for _, r := range rows {
			vals = append(vals, sqlLiteral(frame.CellString(r, pkIdx[0])))
		}
		return "DELETE FROM " + tbl + " WHERE " + quoteCol(be, pks[0]) + " IN (" + strings.Join(vals, ",") + ")", nil
	}
	ors := make([]string, 0, len(rows))
	for _, r := range rows {
		ands := make([]string, 0, len(pks))
		for i, pk := range pks {
			ands = append(ands, quoteCol(be, pk)+"="+sqlLiteral(frame.CellString(r, pkIdx[i])))
		}
		ors = append(ors, "("+strings.Join(ands, " AND ")+")")
	}
	return "DELETE FROM " + tbl + " WHERE " + strings.Join(ors, " OR "), nil
}

// commitCellEdit runs the edit described by pe. In database mode it issues an
// UPDATE and returns a command producing cellSavedMsg; in file mode it rewrites
// the cell locally and returns a command producing ApplyFrameMsg.
func commitCellEdit(ctx AppContext, pe PendingEdit) tea.Cmd {
	be := ctx.Backend()
	if be != nil {
		pks, err := be.PrimaryKeys(pe.Namespace, pe.Table)
		if err != nil {
			e := err
			return func() tea.Msg { return ErrorMsg{Err: e} }
		}
		if len(pks) == 0 {
			return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("no primary key, cannot edit")} }
		}
		stmt, err := buildUpdate(be, pe.Table, pks, pe.Frame, pe.Row, pe.ColName, pe.NewValue)
		if err != nil {
			e := err
			return func() tea.Msg { return ErrorMsg{Err: e} }
		}
		return func() tea.Msg {
			res, err := be.Run(stmt)
			if err != nil {
				return cellSavedMsg{err: err}
			}
			n := int64(0)
			if res.Exec != nil {
				n = res.Exec.RowsAffected
			}
			return cellSavedMsg{rows: n}
		}
	}
	// file mode
	typed, err := parseTyped(pe.NewValue, pe.Frame.Columns[pe.Col].Type)
	if err != nil {
		e := err
		return func() tea.Msg { return ErrorMsg{Err: e} }
	}
	newFrame := pe.Frame.WithCell(pe.Row, pe.Col, typed)
	paneID := ctx.ActivePaneID()
	return func() tea.Msg { return ApplyFrameMsg{Frame: newFrame, Crumb: "edit", PaneID: paneID} }
}

// commitRowDelete runs the delete described by pd. In database mode it issues a
// DELETE and returns a command producing rowsDeletedMsg; in file mode it drops
// the rows locally and returns a command producing ApplyFrameMsg.
func commitRowDelete(ctx AppContext, pd PendingDelete) tea.Cmd {
	be := ctx.Backend()
	if be != nil {
		pks, err := be.PrimaryKeys(pd.Namespace, pd.Table)
		if err != nil {
			e := err
			return func() tea.Msg { return ErrorMsg{Err: e} }
		}
		stmt, err := buildDelete(be, pd.Table, pks, pd.Frame, pd.Rows)
		if err != nil {
			e := err
			return func() tea.Msg { return ErrorMsg{Err: e} }
		}
		return func() tea.Msg {
			res, err := be.Run(stmt)
			if err != nil {
				return rowsDeletedMsg{err: err}
			}
			n := int64(0)
			if res.Exec != nil {
				n = res.Exec.RowsAffected
			}
			return rowsDeletedMsg{rows: n}
		}
	}
	// file mode
	newFrame := pd.Frame.WithoutRows(pd.Rows)
	paneID := ctx.ActivePaneID()
	return func() tea.Msg { return ApplyFrameMsg{Frame: newFrame, Crumb: "delete", PaneID: paneID} }
}
