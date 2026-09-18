package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// DialogEntry is a widget.Entry that forwards the Escape key to the
// owning window's DismissDialog handler. Fyne's default Entry consumes
// every TypedKey event (including Escape) while focused, which prevents
// the canvas-level key handler in GameHandlers.go from ever seeing
// Escape. Use DialogEntry for every single- or multi-line text entry
// hosted inside our own dialogs so users can dismiss them without first
// clicking outside the field.
type DialogEntry struct {
	widget.Entry
	GoWin *GoWin
}

// NewDialogEntry returns a single-line DialogEntry bound to GoWin.
func NewDialogEntry(GoWin *GoWin) *DialogEntry {
	E := &DialogEntry{GoWin: GoWin}
	E.ExtendBaseWidget(E)
	return E
}

// NewMultiLineDialogEntry returns a multi-line DialogEntry bound to GoWin.
func NewMultiLineDialogEntry(GoWin *GoWin) *DialogEntry {
	E := &DialogEntry{GoWin: GoWin}
	E.MultiLine = true
	E.ExtendBaseWidget(E)
	return E
}

// TypedKey intercepts Escape and routes it to GoWin.DismissDialog,
// otherwise delegating to the embedded widget.Entry.
func (E *DialogEntry) TypedKey(Ev *fyne.KeyEvent) {
	if Ev.Name == fyne.KeyEscape && E.GoWin != nil && E.GoWin.DismissDialog != nil {
		E.GoWin.DismissDialog()
		return
	}
	E.Entry.TypedKey(Ev)
}
