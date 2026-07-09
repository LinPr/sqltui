package ui

// Binding ties one or more key strings (tea.KeyPressMsg.String() forms) to a
// semantic action. The lists below are the single source of truth for both
// dispatch and the help overlay.
type Binding struct {
	Keys   []string
	Action string
	Help   string
}

// Semantic action names.
const (
	ActUp        = "up"
	ActDown      = "down"
	ActLeft      = "left"
	ActRight     = "right"
	ActTop       = "top"
	ActBottom    = "bottom"
	ActHalfUp    = "half-up"
	ActHalfDown  = "half-down"
	ActPageUp    = "page-up"
	ActPageDown  = "page-down"
	ActFirstCol  = "first-col"
	ActLastCol   = "last-col"
	ActSheet     = "sheet"
	ActExpand    = "expand"
	ActInfo      = "info"
	ActRandom    = "random"
	ActGoto      = "goto"
	ActFuzzy     = "fuzzy-search"
	ActExact     = "exact-search"
	ActPalette   = "palette"
	ActTabSwitch = "tab-switch"
	ActPrevTab   = "prev-tab"
	ActNextTab   = "next-tab"
	ActPop       = "pop"
	ActQuit      = "quit"
	ActHelp      = "help"
	ActCopy      = "copy"
	ActCopyCell  = "copy-cell"
	ActCopyRow   = "copy-row"
	ActSheetUp   = "sheet-up"
	ActSheetDown = "sheet-down"
	ActBack      = "back"
	ActEscBack   = "esc-back"
)

// GlobalBindings apply in every non-overlay mode.
var GlobalBindings = []Binding{
	{Keys: []string{"Q", "shift+q"}, Action: ActQuit, Help: "quit"},
	{Keys: []string{"f1", "?"}, Action: ActHelp, Help: "help overlay"},
	{Keys: []string{":"}, Action: ActPalette, Help: "command palette"},
	{Keys: []string{"t"}, Action: ActTabSwitch, Help: "tab switcher"},
	{Keys: []string{"H", "shift+h", "shift+left"}, Action: ActPrevTab, Help: "previous tab"},
	{Keys: []string{"L", "shift+l", "shift+right"}, Action: ActNextTab, Help: "next tab"},
}

// TableBindings apply in table mode.
var TableBindings = []Binding{
	{Keys: []string{"up", "k"}, Action: ActUp, Help: "row up"},
	{Keys: []string{"down", "j"}, Action: ActDown, Help: "row down"},
	{Keys: []string{"left", "h"}, Action: ActLeft, Help: "previous column"},
	{Keys: []string{"right", "l"}, Action: ActRight, Help: "next column"},
	{Keys: []string{"g", "home"}, Action: ActTop, Help: "first row"},
	{Keys: []string{"G", "shift+g", "end"}, Action: ActBottom, Help: "last row"},
	{Keys: []string{"ctrl+u"}, Action: ActHalfUp, Help: "half page up"},
	{Keys: []string{"ctrl+d"}, Action: ActHalfDown, Help: "half page down"},
	{Keys: []string{"pgup", "ctrl+b"}, Action: ActPageUp, Help: "page up"},
	{Keys: []string{"pgdown", "ctrl+f"}, Action: ActPageDown, Help: "page down"},
	{Keys: []string{"_"}, Action: ActFirstCol, Help: "first column"},
	{Keys: []string{"$"}, Action: ActLastCol, Help: "last column"},
	{Keys: []string{"enter"}, Action: ActSheet, Help: "open row sheet"},
	{Keys: []string{"w"}, Action: ActExpand, Help: "toggle fit/wide columns"},
	{Keys: []string{"i"}, Action: ActInfo, Help: "table info"},
	{Keys: []string{"R", "shift+r"}, Action: ActRandom, Help: "random row"},
	{Keys: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, Action: ActGoto, Help: "go to row"},
	{Keys: []string{"y"}, Action: ActCopyCell, Help: "copy current cell"},
	{Keys: []string{"Y", "shift+y"}, Action: ActCopyRow, Help: "copy row (tab-separated)"},
	{Keys: []string{"/"}, Action: ActFuzzy, Help: "fuzzy search"},
	{Keys: []string{"s"}, Action: ActExact, Help: "exact search"},
	{Keys: []string{"q"}, Action: ActPop, Help: "pop frame / close tab"},
	{Keys: []string{"esc"}, Action: ActEscBack, Help: "back / previous level"},
}

// SheetBindings apply in sheet mode.
var SheetBindings = []Binding{
	{Keys: []string{"shift+down", "J", "shift+j"}, Action: ActSheetDown, Help: "scroll down"},
	{Keys: []string{"shift+up", "K", "shift+k"}, Action: ActSheetUp, Help: "scroll up"},
	{Keys: []string{"down", "j"}, Action: ActSheetDown, Help: "scroll down"},
	{Keys: []string{"up", "k"}, Action: ActSheetUp, Help: "scroll up"},
	{Keys: []string{"c"}, Action: ActCopy, Help: "copy row"},
	{Keys: []string{"q", "esc"}, Action: ActBack, Help: "back to table"},
}

// actionFor resolves a key string against binding lists in order; the first
// hit wins. Empty string means unbound.
func actionFor(key string, lists ...[]Binding) string {
	for _, list := range lists {
		for _, b := range list {
			for _, k := range b.Keys {
				if k == key {
					return b.Action
				}
			}
		}
	}
	return ""
}
