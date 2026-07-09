package ui

import (
	"sync/atomic"

	"github.com/LinPr/sqltui/internal/data"
)

// ViewMode selects what the pane body shows.
type ViewMode int

const (
	ModeTable ViewMode = iota
	ModeSheet
)

// entry is one level of a pane's frame stack.
type entry struct {
	frame *data.Frame
	crumb string
}

// Pane is one tab: a stack of frames (base + one per applied operation),
// a table view state and a sheet scroll offset.
type Pane struct {
	Title string
	id    int
	stack []entry

	Table    TableView
	Mode     ViewMode
	SheetOff int
}

// paneIDCounter hands out stable unique pane IDs (see Pane.ID).
var paneIDCounter atomic.Int64

// NewPane creates a pane with a base frame.
func NewPane(title string, f *data.Frame) *Pane {
	return &Pane{Title: title, id: int(paneIDCounter.Add(1)), stack: []entry{{frame: f}}}
}

// ID returns the pane's stable unique identity (positive; 0 is never used, so
// it can act as an "unset" marker in messages).
func (p *Pane) ID() int { return p.id }

// Current returns the frame on top of the stack (nil for an empty pane).
func (p *Pane) Current() *data.Frame {
	if len(p.stack) == 0 {
		return nil
	}
	return p.stack[len(p.stack)-1].frame
}

// Depth reports the stack depth.
func (p *Pane) Depth() int { return len(p.stack) }

// Push adds a derived frame with a crumb label and resets the view onto it.
func (p *Pane) Push(f *data.Frame, crumb string) {
	p.stack = append(p.stack, entry{frame: f, crumb: crumb})
	p.Table.Reset()
	p.Mode = ModeTable
	p.SheetOff = 0
}

// Pop removes the top frame. It reports false when the stack is already at
// its base (the caller then closes the tab instead).
func (p *Pane) Pop() bool {
	if len(p.stack) <= 1 {
		return false
	}
	p.stack = p.stack[:len(p.stack)-1]
	p.Table.ClampTo(p.Current())
	p.Mode = ModeTable
	p.SheetOff = 0
	return true
}

// Reset drops every derived frame, back to the base.
func (p *Pane) Reset() {
	if len(p.stack) > 1 {
		p.stack = p.stack[:1]
	}
	p.Table.ClampTo(p.Current())
	p.Mode = ModeTable
	p.SheetOff = 0
}

// Crumbs returns the breadcrumb chain: tab title, then one label per
// derived frame.
func (p *Pane) Crumbs() []string {
	out := []string{p.Title}
	for _, e := range p.stack[1:] {
		out = append(out, e.crumb)
	}
	return out
}
