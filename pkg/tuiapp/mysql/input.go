package mysql

import (
	"log"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// latest 100 histories
var histories = readCommandHistroies()[:100]

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
			fields, result, err := DbClinet.ExecuteRawQuery(inputField.GetText())
			if err != nil {
				PrintfTextView("[red]Error: %s", err)
				ClearTableRecords()
				return
			}
			FillTableWithQueryResult(fields, result)
			PrintfTextView("[yellow]Status: Success !")
			addCommandHistory(inputField.GetText())

		case tcell.KeyEscape:
			log.Println("KeyEscape pressed")
			// TODO:
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
	// inputField.SetBorder(true)

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

func addCommandHistory(command string) {
	histories = append(histories, command)
}
