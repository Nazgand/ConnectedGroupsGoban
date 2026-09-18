package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowGameInformation opens a dialog showing all GameHistory fields for the
// current game. Width, Height, Players, RuleSet, and wrap fields are
// read-only. PlayerNames, Komi, and Information are editable.
func (GoWin *GoWin) ShowGameInformation() {
	if GoWin.DialogShowing {
		return
	}
	Hist := GoWin.Coll.CurrentGameH
	if Hist == nil {
		GoWin.ShowError(fmt.Errorf("no game selected"))
		return
	}
	GoWin.DialogShowing = true

	// Working copies of editable maps so dialog edits never mutate live data until OK.
	WorkingPlayerNames := make(map[uint8]string)
	for PlayerNum, Name := range Hist.PlayerNames {
		WorkingPlayerNames[PlayerNum] = Name
	}
	WorkingKomi := make(map[uint8]float64)
	for PlayerNum, K := range Hist.Komi {
		WorkingKomi[PlayerNum] = K
	}
	WorkingInformation := make(map[string]*string)
	for Key, Val := range Hist.Information {
		if Val != nil {
			Copy := *Val
			WorkingInformation[Key] = &Copy
		}
	}

	// Read-only labels.
	ReadOnlyLabel := func(Caption, Value string) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewLabel(Caption), nil, widget.NewLabel(Value))
	}
	RuleSetName := ""
	if Hist.RuleSet != nil && len(Hist.RuleSet.Names) > 0 {
		RuleSetName = Hist.RuleSet.Names[0]
	}
	ReadOnlySection := container.NewGridWithColumns(4,
		ReadOnlyLabel("Width:", strconv.FormatUint(uint64(Hist.Width), 10)),
		ReadOnlyLabel("Height:", strconv.FormatUint(uint64(Hist.Height), 10)),
		ReadOnlyLabel("Players:", strconv.FormatUint(uint64(Hist.Players), 10)),
		ReadOnlyLabel("Rule Set:", RuleSetName),
		ReadOnlyLabel("WrapXMulY:", strconv.FormatInt(int64(Hist.WrapXMulY), 10)),
		ReadOnlyLabel("WrapYMulX:", strconv.FormatInt(int64(Hist.WrapYMulX), 10)),
		ReadOnlyLabel("WrapXShiftY:", strconv.FormatUint(uint64(Hist.WrapXShiftY), 10)),
		ReadOnlyLabel("WrapYShiftX:", strconv.FormatUint(uint64(Hist.WrapYShiftX), 10)),
	)

	// Per-player name and komi editor (mirrors HandleNewGame pattern).
	PlayerOptions := make([]string, Hist.Players)
	for PlayerIndex := uint8(0); PlayerIndex < Hist.Players; PlayerIndex++ {
		PlayerNum := PlayerIndex + 1
		if Name, Exists := WorkingPlayerNames[PlayerNum]; Exists && Name != "" {
			PlayerOptions[PlayerIndex] = fmt.Sprintf("Player %d (%s)", PlayerNum, Name)
		} else {
			PlayerOptions[PlayerIndex] = fmt.Sprintf("Player %d", PlayerNum)
		}
	}
	PlayerSelect := widget.NewSelect(PlayerOptions, nil)
	if len(PlayerOptions) > 0 {
		PlayerSelect.SetSelectedIndex(0)
	}

	IndividualNameEntry := NewDialogEntry(GoWin)
	IndividualNameEntry.SetPlaceHolder("Selected player's name")
	IndividualKomiEntry := NewDialogEntry(GoWin)
	IndividualKomiEntry.SetPlaceHolder("Selected player's komi")
	IndividualKomiEntry.Validator = func(S string) error {
		_, Err := strconv.ParseFloat(S, 64)
		if Err != nil {
			return fmt.Errorf("komi value did not parse")
		}
		return nil
	}

	SelectedPlayerNum := func() (uint8, bool) {
		SelectedText := PlayerSelect.Selected
		if SelectedText == "" {
			return 0, false
		}
		var PlayerNum uint8
		if _, Err := fmt.Sscanf(SelectedText, "Player %d", &PlayerNum); Err != nil {
			return 0, false
		}
		return PlayerNum, true
	}

	UpdateIndividualFields := func() {
		PlayerNum, Ok := SelectedPlayerNum()
		if !Ok {
			return
		}
		if Name, Exists := WorkingPlayerNames[PlayerNum]; Exists {
			IndividualNameEntry.SetText(Name)
		} else {
			IndividualNameEntry.SetText("")
		}
		if Komi, Exists := WorkingKomi[PlayerNum]; Exists {
			IndividualKomiEntry.SetText(FormatPoints(Komi))
		} else {
			IndividualKomiEntry.SetText("0")
		}
	}

	RefreshPlayerDropdown := func() {
		for PlayerIndex := uint8(0); PlayerIndex < Hist.Players; PlayerIndex++ {
			PlayerNum := PlayerIndex + 1
			if Name, Exists := WorkingPlayerNames[PlayerNum]; Exists && Name != "" {
				PlayerOptions[PlayerIndex] = fmt.Sprintf("Player %d (%s)", PlayerNum, Name)
			} else {
				PlayerOptions[PlayerIndex] = fmt.Sprintf("Player %d", PlayerNum)
			}
		}
		PlayerSelect.Options = PlayerOptions
		PlayerSelect.Refresh()
	}

	IndividualNameEntry.OnChanged = func(_ string) {
		PlayerNum, Ok := SelectedPlayerNum()
		if !Ok {
			return
		}
		WorkingPlayerNames[PlayerNum] = IndividualNameEntry.Text
		RefreshPlayerDropdown()
	}

	IndividualKomiEntry.OnChanged = func(_ string) {
		PlayerNum, Ok := SelectedPlayerNum()
		if !Ok {
			return
		}
		if Komi, Err := strconv.ParseFloat(IndividualKomiEntry.Text, 64); Err == nil {
			WorkingKomi[PlayerNum] = Komi
		}
	}

	PlayerSelect.OnChanged = func(_ string) {
		UpdateIndividualFields()
	}

	UpdateIndividualFields()

	// Information entries: display in InformationRowOrder, then any extra keys
	// present in the game data. "Rules" is displayed read-only above.
	InformationDescriptions := make([]string, 0, len(InformationRowOrder))
	InformationDescriptions = append(InformationDescriptions, InformationRowOrder...)
	InDescSet := make(map[string]bool, len(InformationDescriptions))
	for _, Desc := range InformationDescriptions {
		InDescSet[Desc] = true
	}
	const RulesDesc = "Rules"
	for Key := range Hist.Information {
		if !InDescSet[Key] && Key != RulesDesc {
			InformationDescriptions = append(InformationDescriptions, Key)
		}
	}

	InformationEntries := make(map[string]*DialogEntry, len(InformationDescriptions))
	InformationRows := make([]fyne.CanvasObject, 0, len(InformationDescriptions))
	for _, Desc := range InformationDescriptions {
		Entry := NewDialogEntry(GoWin)
		Entry.SetPlaceHolder(Desc)
		if Val, Exists := WorkingInformation[Desc]; Exists && Val != nil {
			Entry.SetText(*Val)
		}
		Desc := Desc // capture for closure
		Entry.OnChanged = func(NewText string) {
			if NewText == "" {
				delete(WorkingInformation, Desc)
			} else {
				Copy := NewText
				WorkingInformation[Desc] = &Copy
			}
		}
		InformationEntries[Desc] = Entry
		InformationRows = append(InformationRows,
			container.NewBorder(nil, nil, widget.NewLabel(Desc+":"), nil, Entry))
	}

	DialogContent := container.NewVScroll(container.NewVBox(
		append([]fyne.CanvasObject{
			widget.NewLabel("— Read-Only —"),
			ReadOnlySection,
			widget.NewSeparator(),
			widget.NewLabel("— Players —"),
			container.NewBorder(nil, nil, widget.NewLabel("Player:"), nil, PlayerSelect),
			container.NewBorder(nil, nil, widget.NewLabel("Name:"), nil, IndividualNameEntry),
			container.NewBorder(nil, nil, widget.NewLabel("Komi:"), nil, IndividualKomiEntry),
			widget.NewSeparator(),
			widget.NewLabel("— Information —"),
		}, InformationRows...)...,
	))

	Dlg := dialog.NewCustomConfirm("Game Information", "OK", "Cancel",
		DialogContent,
		func(Ok bool) {
			GoWin.DialogClosed()
			if !Ok {
				return
			}
			// Validate komi entry for currently selected player.
			if IndividualKomiEntry.Validator != nil {
				if Err := IndividualKomiEntry.Validator(IndividualKomiEntry.Text); Err != nil {
					GoWin.ShowError(Err)
					return
				}
			}
			// Apply working copies back to live GameHistory.
			Hist.PlayerNames = WorkingPlayerNames
			Hist.Komi = WorkingKomi
			Hist.Information = WorkingInformation
			// Refresh UI elements that depend on player names / game name.
			GoWin.FixWindowTitle()
			GoWin.RefreshGameSelector()
		},
		GoWin.Win,
	)
	GoWin.DismissDialog = func() { Dlg.Hide() }
	GoWin.SubmitDialog = func() { Dlg.Confirm() }
	GoWin.ResizeDialog = func() { Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	InfoEntries := make([]*DialogEntry, 0, len(InformationEntries))
	for _, E := range InformationEntries {
		InfoEntries = append(InfoEntries, E)
	}
	GoWin.WireSubmitOnEnter(append([]*DialogEntry{IndividualNameEntry, IndividualKomiEntry}, InfoEntries...)...)
	Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	Dlg.Show()
	GoWin.Win.Canvas().Focus(IndividualNameEntry)
}
