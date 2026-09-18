package main

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
)

// NewGoWin creates a new application window with an optional starting
// Collection deep-copied from SourceColl. When SourceColl is nil, the new
// window starts with a fresh empty Collection (one default board).
func NewGoWin(SourceColl *Collection) {
	FyneApp := fyne.CurrentApp()
	if FyneApp == nil {
		return
	}
	NewWindow := FyneApp.NewWindow("Connected Groups Goban Version " + Version)
	Win := &GoWin{Win: NewWindow}
	Win.Init(true)
	if SourceColl != nil {
		Encoded := EncodeCggFile(SourceColl)
		JsonBytes, Err := json.Marshal(Encoded)
		if Err != nil {
			Win.ShowError(fmt.Errorf("duplicate collection: encode: %v", Err))
			return
		}
		if ImportErr := Win.ImportCggBytes(JsonBytes, ""); ImportErr != nil {
			Win.ShowError(fmt.Errorf("duplicate collection: %v", ImportErr))
		}
	}
}

// DuplicateWindow opens a new window containing a deep copy of the current
// Collection. The duplicate has no OpenedFilePath and must be saved separately.
func (GoWin *GoWin) DuplicateWindow() {
	NewGoWin(GoWin.Coll)
}

// AddWindow opens a new window with an empty Collection.
func (GoWin *GoWin) AddWindow() {
	NewGoWin(nil)
}

// CloseWindow closes this GoWin's window. The Fyne application exits
// automatically when the last window is closed.
func (GoWin *GoWin) CloseWindow() {
	GoWin.Win.Close()
}
