package mysql

import (
	"fmt"
	"github.com/rivo/tview"
)

var textViewOut *tview.TextView

// This wiil be used when the inputField capture the enter key
func PrintfTextView(format string, a ...any) {
	textViewOut.Clear()
	fmt.Fprintf(textViewOut, format, a...)
}

func PrintlnTextView(a ...any) {
	textViewOut.Clear()
	fmt.Fprintln(textViewOut, a...)
}

func RenderTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetText("[yellow]Status: null").
		SetWrap(true).
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetChangedFunc(func() {})

	// textView.SetBorder(true).SetTitle("Query Result")
	textViewOut = textView
	return textView

}
