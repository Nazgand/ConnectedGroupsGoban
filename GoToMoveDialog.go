package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (GoWin *GoWin) ShowGoToMoveDialog() {
	if GoWin.DialogShowing {
		return
	}
	if GoWin.Coll.CurrentGameH == nil {
		return
	}
	GoWin.DialogShowing = true

	MoveEntry := NewDialogEntry(GoWin)
	MoveEntry.SetPlaceHolder("Move number")

	FormItems := []*widget.FormItem{
		widget.NewFormItem("Move #", MoveEntry),
	}

	Dlg := dialog.NewForm("(Ctrl+G) Go To Move #", "Go", "Cancel", FormItems, func(Confirmed bool) {
		GoWin.DialogClosed()
		if !Confirmed {
			return
		}
		MoveNum, Err := strconv.Atoi(MoveEntry.Text)
		if Err != nil || MoveNum < 0 {
			GoWin.ShowError(fmt.Errorf("please enter a non-negative integer"))
			return
		}
		Hist := GoWin.Coll.CurrentGameH
		if Hist == nil {
			return
		}
		Path := FavoriteChildPath(Hist.RootNode)
		Index := MoveNum
		if Index >= len(Path) {
			Index = len(Path) - 1
		}
		GoWin.OnUserNavigate()
		GoWin.SetCurrentNode(Path[Index])
	}, GoWin.Win)

	GoWin.DismissDialog = func() { Dlg.Hide() }
	GoWin.WireSubmitOnEnter(MoveEntry)
	Dlg.Resize(Dlg.MinSize())
	Dlg.Show()
	GoWin.Win.Canvas().Focus(MoveEntry)
}
