package redis

import (
	"fmt"

	"github.com/rivo/tview"
)

var resultTextViewOut *tview.TextView

func PrintfResultTextView(format string, a ...any) {
	resultTextViewOut.Clear()
	fmt.Fprintf(resultTextViewOut, format, a...)
}

func PrintlnResultTextView(a ...any) {
	resultTextViewOut.Clear()
	fmt.Fprintln(resultTextViewOut, a...)
}

func ClearResultTextView() {
	resultTextViewOut.Clear()
}

func RenderResultTextView() *tview.TextView {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			// app.Draw()
		})

	textView.SetBorder(true).SetTitle("Result")

	resultTextViewOut = textView
	return textView
}
