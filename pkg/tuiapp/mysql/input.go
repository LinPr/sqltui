package mysql

import (
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// keep at most 100 histories for autocompletion
var histories = capStringSlice(readCommandHistroies(), 100)

func RenderInputFiedl() *tview.InputField {

	inputField := tview.NewInputField().
		SetLabel("Query: ").
		SetPlaceholder("Enter mysql query here...").
		SetPlaceholderStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray)).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetAutocompleteStyles(tcell.ColorBlack, tcell.StyleDefault, tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorGray)).
		SetFieldWidth(1024)

	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			// execute sql and show results in table
			query := inputField.GetText()
			if strings.TrimSpace(query) == "" {
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
				addCommandHistory(query)
			} else {
				rowAffected, err := rawCmdResult.Result.RowsAffected()
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
				PrintfTextView("[yellow]Status: Success ! \n\t Rows affected: %d, Last Insert ID: %d", rowAffected, lastInsertId)
				addCommandHistory(query)
			}
		}
	})

	inputField.SetAutocompleteFunc(func(currentText string) (entries []string) {
		if len(currentText) == 0 {
			return
		}
		for _, word := range histories {
			if strings.HasPrefix(strings.ToLower(word), strings.ToLower(currentText)) {
				entries = append(entries, word)
			}
		}
		if len(entries) <= 1 {
			entries = nil
		}
		return
	})
	inputField.SetAutocompletedFunc(func(text string, index, source int) bool {
		if source != tview.AutocompletedNavigate {
			inputField.SetText(text)
		}
		return source == tview.AutocompletedEnter || source == tview.AutocompletedClick
	})

	return inputField
}

func readCommandHistroies() []string {
	var historys []string
	rawHistory, err := os.ReadFile(os.Getenv("HOME") + "/.mysql_history")
	if err != nil {
		return historys
	}
	newHistory := strings.ReplaceAll(string(rawHistory), "\n", "")
	newHistory = strings.ReplaceAll(string(newHistory), "\\040", " ")

	historys = strings.Split(newHistory, ";")
	return distinctStringSlice(historys)
}

func distinctStringSlice(histories []string) []string {
	tmpSet := make(map[string]struct{})
	for _, v := range histories {
		tmpSet[v] = struct{}{}
	}
	histories = make([]string, 0, len(tmpSet))
	for k := range tmpSet {
		histories = append(histories, k)
	}
	return histories
}

func capStringSlice(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func addCommandHistory(command string) {
	histories = append(histories, command)
}
