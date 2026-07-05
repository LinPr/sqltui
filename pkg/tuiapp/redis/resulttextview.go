package redis

import (
	"fmt"

	"github.com/LinPr/sqltui/pkg/tuiapp"
	"github.com/rivo/tview"
)

var resultTextViewOut *tview.TextView

func PrintfResultTextView(format string, a ...any) {
	resultTextViewOut.Clear()
	fmt.Fprintf(resultTextViewOut, format, tuiapp.EscapeArgs(a)...)
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

	textView.SetBorder(true).SetTitle("[green]Result")

	resultTextViewOut = textView
	return textView
}
