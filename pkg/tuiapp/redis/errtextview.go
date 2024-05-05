package redis

import (
	"fmt"
	"github.com/rivo/tview"
)

var errTextViewOut *tview.TextView

// This wiil be used when the inputField capture the enter key
func PrintfErrTextView(format string, a ...any) {
	errTextViewOut.Clear()
	fmt.Fprintf(errTextViewOut, format, a...)
}

func PrintlnErrTextView(a ...any) {
	errTextViewOut.Clear()
	fmt.Fprintln(errTextViewOut, a...)
}

func ClearErrTextView() {
	if errTextViewOut != nil {
		errTextViewOut.Clear()
	}
}

func RenderErrTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetText("[yellow]Status: null").
		SetWrap(true).
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetChangedFunc(func() {})

	// textView.SetBorder(true).SetTitle("Query Result")
	errTextViewOut = textView
	return textView

}
