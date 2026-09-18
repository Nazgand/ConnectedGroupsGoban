package main

import (
	"fmt"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// HandleExportCgg opens the "Save Game" dialog where the user picks a
// compression method (Kanzi, Gzip, Zstd, or Plaintext) and a per-method
// compression level, then proceeds to ShowCggSaveDialog for the file save.
func (GoWin *GoWin) HandleExportCgg() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	CompressionMethodVar := "Kanzi"
	CompressionLevelVar := 9
	KanziBlockSizeVar := uint(1024 * 1024)

	MaxLevelForMethod := map[string]int{"Kanzi": 9, "Gzip": 9, "Zstd": ZstdMaxLevel, "Plaintext": 0}

	LevelLabel := widget.NewLabel(fmt.Sprintf("Compression level: %d", CompressionLevelVar))
	LevelSlider := widget.NewSlider(0, 9)
	LevelSlider.SetValue(9)
	LevelSlider.OnChanged = func(V float64) {
		CompressionLevelVar = int(V)
		LevelLabel.SetText(fmt.Sprintf("Compression level: %d", CompressionLevelVar))
	}

	UpdateSliderForMethod := func(Method string) {
		MaxLevel := MaxLevelForMethod[Method]
		LevelSlider.Max = float64(MaxLevel)
		CompressionLevelVar = MaxLevel
		LevelSlider.SetValue(float64(CompressionLevelVar))
		LevelLabel.SetText(fmt.Sprintf("Compression level: %d", CompressionLevelVar))
		if MaxLevel == 0 {
			LevelSlider.Hide()
			LevelLabel.Hide()
		} else {
			LevelSlider.Show()
			LevelLabel.Show()
		}
	}

	MethodSelect := widget.NewSelect([]string{"Kanzi", "Gzip", "Zstd", "Plaintext"}, func(Selected string) {
		CompressionMethodVar = Selected
		UpdateSliderForMethod(Selected)
	})
	MethodSelect.SetSelected("Kanzi")

	Content := container.NewVBox(
		MethodSelect,
		LevelLabel,
		LevelSlider,
	)

	SaveOptionsDlg := dialog.NewCustomConfirm("Save Game", "Save", "Cancel", Content, func(Confirmed bool) {
		GoWin.DialogClosed()
		if !Confirmed {
			return
		}
		GoWin.ShowCggSaveDialog(CompressionMethodVar, CompressionLevelVar, KanziBlockSizeVar)
	}, GoWin.Win)
	GoWin.DismissDialog = func() { SaveOptionsDlg.Hide() }
	GoWin.SubmitDialog = func() { SaveOptionsDlg.Confirm() }
	SaveOptionsDlg.Show()
}
