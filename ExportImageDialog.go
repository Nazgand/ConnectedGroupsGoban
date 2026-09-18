package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// HandleExportImage is the Game-menu entry point. Prompts for mode
// (single-frame vs animated), a uint16 CellSize (min 5), and — when
// animated — seconds per node (default 1.5, must be > 0), then opens
// the file-save dialog.
func (GoWin *GoWin) HandleExportImage() {
	if GoWin.DialogShowing {
		return
	}
	if GoWin.CurrentNode() == nil || GoWin.CurrentNode().Board == nil {
		GoWin.ShowError(fmt.Errorf("no active game to export"))
		return
	}
	GoWin.DialogShowing = true

	ModeSelect := widget.NewRadioGroup(
		[]string{ExportModeSingleFrame, ExportModeAnimated}, nil)
	ModeSelect.SetSelected(ExportModeSingleFrame)

	CellSizeEntry := NewDialogEntry(GoWin)
	CellSizeEntry.SetText("93")
	CellSizeEntry.SetPlaceHolder("5..65535")

	SecondsEntry := NewDialogEntry(GoWin)
	SecondsEntry.SetText("0.15")
	SecondsEntry.SetPlaceHolder("e.g. 1.5")
	SecondsFormItem := widget.NewFormItem("Seconds per node", SecondsEntry)

	SecondsEntry.Disable()
	ModeSelect.OnChanged = func(Selected string) {
		if Selected == ExportModeAnimated {
			SecondsEntry.Enable()
		} else {
			SecondsEntry.Disable()
		}
	}

	FormItems := []*widget.FormItem{
		widget.NewFormItem("Mode", ModeSelect),
		widget.NewFormItem("CellSize (px)", CellSizeEntry),
		SecondsFormItem,
	}

	Dlg := dialog.NewForm("Export board as SVG", "Export", "Cancel", FormItems,
		func(Confirmed bool) {
			GoWin.DialogClosed()
			if !Confirmed {
				return
			}
			Parsed, Err := strconv.ParseUint(strings.TrimSpace(CellSizeEntry.Text), 10, 16)
			if Err != nil {
				GoWin.ShowError(fmt.Errorf("CellSize must be a non-negative integer ≤ 65535"))
				return
			}
			if Parsed < 5 {
				GoWin.ShowError(fmt.Errorf("CellSize must be at least 5"))
				return
			}
			SecondsPerNode := 0.0
			if ModeSelect.Selected == ExportModeAnimated {
				SecondsPerNode, Err = strconv.ParseFloat(strings.TrimSpace(SecondsEntry.Text), 64)
				if Err != nil || !(SecondsPerNode > 0) || math.IsInf(SecondsPerNode, 0) {
					GoWin.ShowError(fmt.Errorf("Seconds per node must be a positive number"))
					return
				}
			}
			GoWin.ShowSvgSaveDialog(uint16(Parsed), SecondsPerNode)
		}, GoWin.Win)

	GoWin.DismissDialog = func() { Dlg.Hide() }
	Dlg.Resize(Dlg.MinSize())
	Dlg.Show()
	GoWin.Win.Canvas().Focus(CellSizeEntry)
}
