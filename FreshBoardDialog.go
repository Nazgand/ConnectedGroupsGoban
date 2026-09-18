package main

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// HandleNewGame opens the "Fresh Board" dialog, allowing the user to choose
// board dimensions, komi, rule set, player names, and presets. On confirmation
// a new game is added to the collection.
func (GoWin *GoWin) HandleNewGame() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	StoredPreset := GoWin.GetActivePreset()
	// Deep-copy maps so dialog edits never mutate the stored preset.
	ActivePreset := FreshBoardPreset{
		Width:                StoredPreset.Width,
		Height:               StoredPreset.Height,
		Players:              StoredPreset.Players,
		RuleSet:              StoredPreset.RuleSet,
		Komi:                 make(map[uint8]float64),
		PlayerNames:          make(map[uint8]string),
		Information:          make(map[string]string),
		WrapXMulY:            StoredPreset.WrapXMulY,
		WrapYMulX:            StoredPreset.WrapYMulX,
		WrapXShiftY:          StoredPreset.WrapXShiftY,
		WrapYShiftX:          StoredPreset.WrapYShiftX,
		LibertySharingFixed:  StoredPreset.LibertySharingFixed,
		LibertySharingMatrix: StoredPreset.LibertySharingMatrix.Clone(),
	}
	maps.Copy(ActivePreset.Komi, StoredPreset.Komi)
	maps.Copy(ActivePreset.PlayerNames, StoredPreset.PlayerNames)
	maps.Copy(ActivePreset.Information, StoredPreset.Information)

	WidthEntry := NewDialogEntry(GoWin)
	WidthEntry.SetPlaceHolder("1≤Width≤255")
	WidthEntry.SetText(strconv.FormatUint(uint64(ActivePreset.Width), 10))
	WidthEntry.Validator = func(S string) error {
		PWidth, Err := strconv.ParseUint(S, 10, 8)
		Width := uint8(PWidth)
		if Err != nil {
			return fmt.Errorf("width value did not parse")
		} else if Width == 0 || Width == 0xff {
			return fmt.Errorf("width value out of bounds")
		}
		return nil
	}

	HeightEntry := NewDialogEntry(GoWin)
	HeightEntry.SetPlaceHolder("1≤Height≤255")
	HeightEntry.SetText(strconv.FormatUint(uint64(ActivePreset.Height), 10))
	HeightEntry.Validator = func(S string) error {
		PHeight, Err := strconv.ParseUint(S, 10, 8)
		Height := uint8(PHeight)
		if Err != nil {
			return fmt.Errorf("height value did not parse")
		} else if Height == 0 || Height == 0xff {
			return fmt.Errorf("height value out of bounds")
		}
		return nil
	}

	PlayersEntry := NewDialogEntry(GoWin)
	PlayersEntry.SetPlaceHolder("Players")
	PlayersEntry.SetText(strconv.FormatUint(uint64(ActivePreset.Players), 10))
	if ActivePreset.Players == 0 {
		PlayersEntry.SetText("2")
	}
	PlayersEntry.Validator = func(S string) error {
		PPlayers, Err := strconv.ParseUint(S, 10, 8)
		Players := uint8(PPlayers)
		if Err != nil {
			return fmt.Errorf("players value did not parse")
		} else if Players == 0 || Players > 15 {
			return fmt.Errorf("players value out of bounds")
		}
		return nil
	}

	ValidateWrapMul := func(S string, DimEntry *DialogEntry, DimLabel string) error {
		Value, Err := strconv.ParseInt(S, 10, 8)
		if Err != nil {
			return fmt.Errorf("value did not parse")
		}
		ParsedDim, DimErr := strconv.ParseUint(DimEntry.Text, 10, 8)
		if DimErr != nil {
			return fmt.Errorf("%s is invalid", DimLabel)
		}
		if !IsValidWrapMul(int8(Value), uint8(ParsedDim)) {
			return fmt.Errorf("must be 0, or have |value| ≤ %s (%d) and gcd(|value|, %s) == 1",
				DimLabel, ParsedDim, DimLabel)
		}
		return nil
	}

	WrapXMulYEntry := NewDialogEntry(GoWin)
	WrapXMulYEntry.SetPlaceHolder("0, or coprime to height with |v| ≤ height")
	WrapXMulYEntry.SetText(strconv.FormatInt(int64(ActivePreset.WrapXMulY), 10))
	WrapXMulYEntry.Validator = func(S string) error {
		return ValidateWrapMul(S, HeightEntry, "height")
	}

	WrapYMulXEntry := NewDialogEntry(GoWin)
	WrapYMulXEntry.SetPlaceHolder("0, or coprime to width with |v| ≤ width")
	WrapYMulXEntry.SetText(strconv.FormatInt(int64(ActivePreset.WrapYMulX), 10))
	WrapYMulXEntry.Validator = func(S string) error {
		return ValidateWrapMul(S, WidthEntry, "width")
	}

	WrapXShiftYEntry := NewDialogEntry(GoWin)
	WrapXShiftYEntry.SetPlaceHolder("< board height")
	WrapXShiftYEntry.SetText(strconv.FormatUint(uint64(ActivePreset.WrapXShiftY), 10))
	WrapXShiftYEntry.Validator = func(S string) error {
		Value, Err := strconv.ParseUint(S, 10, 8)
		if Err != nil {
			return fmt.Errorf("value did not parse")
		}
		ParsedHeight, HeightErr := strconv.ParseUint(HeightEntry.Text, 10, 8)
		if HeightErr != nil {
			return fmt.Errorf("height is invalid")
		}
		if Value >= ParsedHeight {
			return fmt.Errorf("must be less than board height (%d)", ParsedHeight)
		}
		return nil
	}

	WrapYShiftXEntry := NewDialogEntry(GoWin)
	WrapYShiftXEntry.SetPlaceHolder("< board width")
	WrapYShiftXEntry.SetText(strconv.FormatUint(uint64(ActivePreset.WrapYShiftX), 10))
	WrapYShiftXEntry.Validator = func(S string) error {
		Value, Err := strconv.ParseUint(S, 10, 8)
		if Err != nil {
			return fmt.Errorf("value did not parse")
		}
		ParsedWidth, WidthErr := strconv.ParseUint(WidthEntry.Text, 10, 8)
		if WidthErr != nil {
			return fmt.Errorf("width is invalid")
		}
		if Value >= ParsedWidth {
			return fmt.Errorf("must be less than board width (%d)", ParsedWidth)
		}
		return nil
	}

	AllRuleSets := make([]*RuleSet, len(ProtectedRuleSets))
	for Index := range ProtectedRuleSets {
		AllRuleSets[Index] = &ProtectedRuleSets[Index]
		RegisterRuleSet(AllRuleSets[Index])
	}
	for _, CustomRuleSet := range CurrentAppConfig.AllCustomRuleSets() {
		RegisterRuleSet(CustomRuleSet)
		AllRuleSets = append(AllRuleSets, CustomRuleSet)
	}
	DisplayedNameToRuleSet := map[string]*RuleSet{}
	AllRuleSetNames := make([]string, 0, len(AllRuleSets))
	for _, RuleSet := range AllRuleSets {
		DisplayedName := RuleSetDisplayName(RuleSet, AllRuleSets)
		AllRuleSetNames = append(AllRuleSetNames, DisplayedName)
		DisplayedNameToRuleSet[DisplayedName] = RuleSet
	}
	SelectedRuleSet := ActivePreset.RuleSet
	if SelectedRuleSet == nil {
		SelectedRuleSet = &ProtectedRuleSets[0]
	}
	RegisterRuleSet(SelectedRuleSet)
	RuleSetSelect := widget.NewSelect(AllRuleSetNames, func(DisplayedName string) {
		if FoundRuleSet, Found := DisplayedNameToRuleSet[DisplayedName]; Found {
			SelectedRuleSet = FoundRuleSet
		}
	})
	RuleSetSelect.SetSelected(RuleSetDisplayName(SelectedRuleSet, AllRuleSets))

	// Player selector and individual player editor
	PlayerSelect := widget.NewSelect([]string{"Player 1", "Player 2"}, nil)
	PlayerSelect.SetSelected("Player 1")

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

	// Update individual player fields when selection changes
	UpdateIndividualFields := func() {
		SelectedText := PlayerSelect.Selected
		if SelectedText == "" {
			return
		}
		var PlayerNum uint8
		// Parse player number from "Player X" or "Player X (Name)" format
		if _, Err := fmt.Sscanf(SelectedText, "Player %d", &PlayerNum); Err != nil {
			return
		}

		if Name, Exists := ActivePreset.PlayerNames[PlayerNum]; Exists {
			IndividualNameEntry.SetText(Name)
		} else {
			IndividualNameEntry.SetText("")
		}

		if Komi, Exists := ActivePreset.Komi[PlayerNum]; Exists {
			IndividualKomiEntry.SetText(FormatPoints(Komi))
		} else {
			IndividualKomiEntry.SetText("0")
		}
	}

	PlayerSelect.OnChanged = func(_ string) {
		UpdateIndividualFields()
	}

	// Update player options when player count changes
	UpdatePlayerOptions := func() {
		if PlayerCount, Err := strconv.ParseUint(PlayersEntry.Text, 10, 8); Err == nil {
			// Preserve current selection
			CurrentSelection := PlayerSelect.SelectedIndex()
			var CurrentPlayerNum uint8 = 1
			if CurrentSelection >= 0 && CurrentSelection < len(PlayerSelect.Options) {
				// Extract player number from current selection
				if _, err := fmt.Sscanf(PlayerSelect.Options[CurrentSelection], "Player %d", &CurrentPlayerNum); err != nil {
					CurrentPlayerNum = 1 // Default to player 1 if parsing fails
				}
			}

			Options := make([]string, PlayerCount)
			for I := uint64(1); I <= PlayerCount; I++ {
				PlayerNum := uint8(I)
				if PlayerName, Exists := ActivePreset.PlayerNames[PlayerNum]; Exists && PlayerName != "" {
					Options[I-1] = fmt.Sprintf("Player %d (%s)", PlayerNum, PlayerName)
				} else {
					Options[I-1] = fmt.Sprintf("Player %d", PlayerNum)
				}
			}
			PlayerSelect.Options = Options

			// Restore selection by player number
			for I, Option := range Options {
				var PlayerNum uint8
				if _, Err := fmt.Sscanf(Option, "Player %d", &PlayerNum); Err == nil && PlayerNum == CurrentPlayerNum {
					PlayerSelect.SetSelectedIndex(I)
					break
				}
			}

			UpdateIndividualFields()
		}
	}

	// Helper to parse player number from current selection
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

	// Ensure ActivePreset maps are initialized
	EnsurePresetMaps := func() {
		if ActivePreset.Information == nil {
			ActivePreset.Information = make(map[string]string)
		}
		if ActivePreset.Komi == nil {
			ActivePreset.Komi = make(map[uint8]float64)
		}
		if ActivePreset.PlayerNames == nil {
			ActivePreset.PlayerNames = make(map[uint8]string)
		}
	}

	// Name changed: update ActivePreset and refresh dropdown labels
	IndividualNameEntry.OnChanged = func(_ string) {
		PlayerNum, Ok := SelectedPlayerNum()
		if !Ok {
			return
		}
		EnsurePresetMaps()
		ActivePreset.PlayerNames[PlayerNum] = IndividualNameEntry.Text
		UpdatePlayerOptions()
	}

	// Komi changed: update ActivePreset only (no dropdown refresh needed)
	IndividualKomiEntry.OnChanged = func(_ string) {
		PlayerNum, Ok := SelectedPlayerNum()
		if !Ok {
			return
		}
		EnsurePresetMaps()
		if Komi, Err := strconv.ParseFloat(IndividualKomiEntry.Text, 64); Err == nil {
			ActivePreset.Komi[PlayerNum] = Komi
		}
	}

	PlayersEntry.OnChanged = func(_ string) {
		UpdatePlayerOptions()
	}

	// Saved player name quick-select with save/delete functionality.
	// Always create this so the user can save the first name.
	BuildSavedNamesOptions := func() []string {
		Options := []string{"[Save Name]", "[Delete Name]"}
		Options = append(Options, CurrentAppConfig.PlayerNames...)
		return Options
	}

	SavedNamesSelect := widget.NewSelect(BuildSavedNamesOptions(), func(selected string) {
	})
	SavedNamesSelect.PlaceHolder = "Saved names…"
	SavedNamesSelect.OnChanged = func(Selected string) {
		if Selected == "[Save Name]" {
			CurrentName := IndividualNameEntry.Text
			if CurrentName != "" && !slices.Contains(CurrentAppConfig.PlayerNames, CurrentName) {
				CurrentAppConfig.PlayerNames = append(CurrentAppConfig.PlayerNames, CurrentName)
				sort.Strings(CurrentAppConfig.PlayerNames)
				SavedNamesSelect.Options = BuildSavedNamesOptions()
				if Err := GoWin.SaveConfig(); Err != nil {
					GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
				}
			}
			SavedNamesSelect.ClearSelected()
		} else if Selected == "[Delete Name]" {
			CurrentName := IndividualNameEntry.Text
			if Idx := slices.Index(CurrentAppConfig.PlayerNames, CurrentName); Idx >= 0 {
				CurrentAppConfig.PlayerNames = slices.Delete(CurrentAppConfig.PlayerNames, Idx, Idx+1)
				SavedNamesSelect.Options = BuildSavedNamesOptions()
				if Err := GoWin.SaveConfig(); Err != nil {
					GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
				}
			}
			SavedNamesSelect.ClearSelected()
		} else if Selected != "" {
			IndividualNameEntry.SetText(Selected)
			SavedNamesSelect.ClearSelected()
		}
	}

	// Preset selector — sorted, "Default" always present and protected.
	PresetNames := SortedPresetNames()
	PresetSelect := widget.NewSelect(PresetNames, nil)
	PresetSelect.SetSelected(CurrentAppConfig.ActivePresetName)
	FillFromPreset := func(Name string) {
		if Preset, Ok := CurrentAppConfig.FreshBoardPresets[Name]; Ok {
			WidthEntry.SetText(strconv.FormatUint(uint64(Preset.Width), 10))
			HeightEntry.SetText(strconv.FormatUint(uint64(Preset.Height), 10))
			if Preset.RuleSet != nil {
				SelectedRuleSet = Preset.RuleSet
				RuleSetSelect.SetSelected(Preset.RuleSet.Names[0])
			}
			if Preset.Players > 0 {
				PlayersEntry.SetText(strconv.FormatUint(uint64(Preset.Players), 10))
			} else {
				PlayersEntry.SetText("2")
			}
			WrapXMulYEntry.SetText(strconv.FormatInt(int64(Preset.WrapXMulY), 10))
			WrapYMulXEntry.SetText(strconv.FormatInt(int64(Preset.WrapYMulX), 10))
			WrapXShiftYEntry.SetText(strconv.FormatUint(uint64(Preset.WrapXShiftY), 10))
			WrapYShiftXEntry.SetText(strconv.FormatUint(uint64(Preset.WrapYShiftX), 10))
			// Deep-copy the preset's maps into ActivePreset
			ActivePreset.Komi = make(map[uint8]float64)
			maps.Copy(ActivePreset.Komi, Preset.Komi)
			ActivePreset.PlayerNames = make(map[uint8]string)
			maps.Copy(ActivePreset.PlayerNames, Preset.PlayerNames)
			ActivePreset.Information = make(map[string]string)
			maps.Copy(ActivePreset.Information, Preset.Information)
			ActivePreset.LibertySharingFixed = Preset.LibertySharingFixed
			ActivePreset.LibertySharingMatrix = Preset.LibertySharingMatrix.Clone()
			UpdatePlayerOptions()
			UpdateIndividualFields()
		}
	}
	PresetSelect.OnChanged = FillFromPreset

	// CurrentPresetFields returns a FreshBoardPreset from the current dialog values.
	CurrentPresetFields := func() FreshBoardPreset {
		Width, _ := strconv.ParseUint(WidthEntry.Text, 10, 8)
		Height, _ := strconv.ParseUint(HeightEntry.Text, 10, 8)
		Players, _ := strconv.ParseUint(PlayersEntry.Text, 10, 8)

		// Use the current ActivePreset komi values since they're updated in real-time
		KomiMap := make(map[uint8]float64)
		for I := uint8(1); I <= uint8(Players); I++ {
			if Komi, Exists := ActivePreset.Komi[I]; Exists {
				KomiMap[I] = Komi
			} else {
				KomiMap[I] = 0 // Default komi
			}
		}

		WrapXMulY, _ := strconv.ParseInt(WrapXMulYEntry.Text, 10, 8)
		WrapYMulX, _ := strconv.ParseInt(WrapYMulXEntry.Text, 10, 8)
		WrapXShiftY, _ := strconv.ParseUint(WrapXShiftYEntry.Text, 10, 8)
		WrapYShiftX, _ := strconv.ParseUint(WrapYShiftXEntry.Text, 10, 8)

		// Resize the in-progress sharing matrix to match the current player count
		// so saving/applying the preset always uses a correctly-sized matrix.
		Matrix := ActivePreset.LibertySharingMatrix.Resize(uint8(Players))
		ActivePreset.LibertySharingMatrix = Matrix

		return FreshBoardPreset{
			Width:                uint8(Width),
			Height:               uint8(Height),
			Players:              uint8(Players),
			Komi:                 KomiMap,
			PlayerNames:          ActivePreset.PlayerNames,
			RuleSet:              SelectedRuleSet,
			Information:          ActivePreset.Information,
			WrapXMulY:            int8(WrapXMulY),
			WrapYMulX:            int8(WrapYMulX),
			WrapXShiftY:          uint8(WrapXShiftY),
			WrapYShiftX:          uint8(WrapYShiftX),
			LibertySharingFixed:  ActivePreset.LibertySharingFixed,
			LibertySharingMatrix: Matrix.Clone(),
		}
	}

	SavePresetFields := func(Name string) {
		CurrentAppConfig.FreshBoardPresets[Name] = CurrentPresetFields()
		CurrentAppConfig.ActivePresetName = Name
		if Err := GoWin.SaveConfig(); Err != nil {
			GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
		}
		PresetSelect.Options = SortedPresetNames()
		PresetSelect.SetSelected(Name)
		PresetSelect.Refresh()
	}

	// [Save Preset] — asks for a name; confirms before overwriting an existing preset.
	SavePresetButton := widget.NewButton("Save", func() {
		NameEntry := NewDialogEntry(GoWin)
		NameEntry.SetPlaceHolder("Preset name (not \"Default\")")
		NameEntry.Validator = func(S string) error {
			if S == "" {
				return fmt.Errorf("preset name must not be blank")
			}
			if S == "Default" {
				return fmt.Errorf("\"Default\" preset cannot be overwritten")
			}
			return nil
		}
		dialog.NewForm("Save Preset", "Save", "Cancel",
			[]*widget.FormItem{widget.NewFormItem("Name", NameEntry)},
			func(Ok bool) {
				if !Ok || NameEntry.Text == "" || NameEntry.Text == "Default" {
					return
				}
				Name := NameEntry.Text
				if _, Exists := CurrentAppConfig.FreshBoardPresets[Name]; Exists {
					dialog.ShowConfirm("Overwrite Preset",
						fmt.Sprintf("Overwrite preset %q?", Name),
						func(Confirmed bool) {
							if Confirmed {
								SavePresetFields(Name)
							}
						}, GoWin.Win)
				} else {
					SavePresetFields(Name)
				}
			}, GoWin.Win).Show()
		GoWin.Win.Canvas().Focus(NameEntry)
	})

	// [Update Preset] — saves current fields into the selected preset without asking for a name.
	// Disabled when "Default" is selected.
	UpdatePresetButton := widget.NewButton("Update", func() {
		Name := PresetSelect.Selected
		if Name == "" || Name == "Default" {
			return
		}
		SavePresetFields(Name)
	})

	// [Delete Preset] — confirms, then removes the selected preset.
	// Disabled when "Default" is selected.
	DeletePresetButton := widget.NewButton("Delete", func() {
		Name := PresetSelect.Selected
		if Name == "" || Name == "Default" {
			return
		}
		dialog.ShowConfirm("Delete Preset",
			fmt.Sprintf("Delete preset %q?", Name),
			func(Confirmed bool) {
				if !Confirmed {
					return
				}
				delete(CurrentAppConfig.FreshBoardPresets, Name)
				CurrentAppConfig.ActivePresetName = "Default"
				if Err := GoWin.SaveConfig(); Err != nil {
					GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
				}
				PresetSelect.Options = SortedPresetNames()
				PresetSelect.SetSelected("Default")
				PresetSelect.Refresh()
			}, GoWin.Win)
	})

	// Enable/disable Update and Delete based on whether "Default" is selected.
	RefreshPresetButtons := func(Name string) {
		if Name == "Default" || Name == "" {
			UpdatePresetButton.Disable()
			DeletePresetButton.Disable()
		} else {
			UpdatePresetButton.Enable()
			DeletePresetButton.Enable()
		}
	}
	OrigOnChanged := PresetSelect.OnChanged
	PresetSelect.OnChanged = func(Name string) {
		OrigOnChanged(Name)
		RefreshPresetButtons(Name)
	}
	RefreshPresetButtons(PresetSelect.Selected)

	PresetButtons := container.NewHBox(SavePresetButton, UpdatePresetButton, DeletePresetButton)

	PresetRow := container.NewBorder(nil, nil, widget.NewLabel("Preset"), PresetButtons, PresetSelect)
	// Two-column layout where each cell uses Border so the label takes only
	// its natural width and the entry/select expands to fill the rest.
	LabeledCell := func(LabelText string, Field fyne.CanvasObject) *fyne.Container {
		return container.NewBorder(nil, nil, widget.NewLabel(LabelText), nil, Field)
	}
	WidthHeightRow := container.NewGridWithColumns(2,
		LabeledCell("Width", WidthEntry),
		LabeledCell("Height", HeightEntry))
	PlayersRuleSetRow := container.NewGridWithColumns(2,
		LabeledCell("Players", PlayersEntry),
		LabeledCell("Rule Set", RuleSetSelect))
	WrapMulRow := container.NewGridWithColumns(2,
		LabeledCell("WrapXMulY", WrapXMulYEntry),
		LabeledCell("WrapYMulX", WrapYMulXEntry))
	WrapShiftRow := container.NewGridWithColumns(2,
		LabeledCell("WrapXShiftY", WrapXShiftYEntry),
		LabeledCell("WrapYShiftX", WrapYShiftXEntry))
	IndividualPlayerRow := container.NewBorder(nil, nil,
		widget.NewLabel("Individual Player"), nil, PlayerSelect)
	PlayerNameRow := container.NewBorder(nil, nil,
		widget.NewLabel("Player Name"), SavedNamesSelect, IndividualNameEntry)
	PlayerKomiRow := container.NewBorder(nil, nil,
		widget.NewLabel("Player Komi"), nil, IndividualKomiEntry)

	LibertySharingFixedCheck := widget.NewCheck(
		"Liberty sharing matrix is fixed for the entire game",
		func(Checked bool) { ActivePreset.LibertySharingFixed = Checked })
	LibertySharingFixedCheck.SetChecked(ActivePreset.LibertySharingFixed)

	EditLibertySharingMatrixButton := widget.NewButton("Edit liberty sharing matrix…", func() {
		Players, _ := strconv.ParseUint(PlayersEntry.Text, 10, 8)
		PlayerCount := uint8(Players)
		if PlayerCount == 0 {
			PlayerCount = 2
		}
		ActivePreset.LibertySharingMatrix = ActivePreset.LibertySharingMatrix.Resize(PlayerCount)
		ShowLibertySharingMatrixEditor(GoWin, ActivePreset.LibertySharingMatrix, PlayerCount, ActivePreset.PlayerNames)
	})

	LibertySharingRow := container.NewBorder(nil, nil, nil, EditLibertySharingMatrixButton, LibertySharingFixedCheck)

	DeleteOtherGamesCheck := widget.NewCheck("Delete other games in collection", func(_ bool) {})
	DeleteOtherGamesCheck.SetChecked(false)

	DialogContent := container.NewVBox(
		PresetRow,
		WidthHeightRow,
		PlayersRuleSetRow,
		WrapMulRow,
		WrapShiftRow,
		widget.NewSeparator(),
		LibertySharingRow,
		widget.NewSeparator(),
		DeleteOtherGamesCheck,
		widget.NewSeparator(),
		IndividualPlayerRow,
		PlayerNameRow,
		PlayerKomiRow,
	)

	// Collect all validated entries for manual checking on confirm.
	ValidatedEntries := []*DialogEntry{
		WidthEntry, HeightEntry, PlayersEntry,
		WrapXMulYEntry, WrapYMulXEntry, WrapXShiftYEntry, WrapYShiftXEntry,
		IndividualKomiEntry,
	}

	BoardSizeDialog := dialog.NewCustomConfirm(
		"Fresh Board",
		"OK",
		"Cancel",
		DialogContent,
		func(Ok bool) {
			GoWin.DialogClosed()
			if !Ok {
				return
			}

			// Validate all entries before proceeding.
			for _, Entry := range ValidatedEntries {
				if Entry.Validator != nil {
					if Err := Entry.Validator(Entry.Text); Err != nil {
						GoWin.ShowError(Err)
						return
					}
				}
			}

			// Record which preset was selected, but do NOT write dialog
			// fields back — that is only done by Save/Update Preset buttons.
			PresetName := PresetSelect.Selected
			if PresetName == "" {
				PresetName = "Default"
			}
			CurrentAppConfig.ActivePresetName = PresetName
			if Err := GoWin.SaveConfig(); Err != nil {
				GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
			}

			// If requested, replace the entire collection before adding the new board.
			if DeleteOtherGamesCheck.Checked {
				GoWin.Coll = NewCollection()
				GoWin.GameTreeNodeToButton = map[*GameTreeNode]*TreeNodeButton{}
				GoWin.GameTreeNodeToContainer = map[*GameTreeNode]*fyne.Container{}
			}

			// Create the board directly from dialog values, not from the stored preset,
			// so the preset is never silently altered.
			GoWin.AddFreshBoardFromPreset(CurrentPresetFields())

			// Append all player names from the preset
			for _, Name := range ActivePreset.PlayerNames {
				if Name != "" {
					AppendPlayerName(Name)
				}
			}

			if Err := GoWin.SaveConfig(); Err != nil {
				GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
			}

			GoWin.DrawBoard()
			GoWin.UpdateGameTreeUI()
		},
		GoWin.Win,
	)

	// Initialize player options
	UpdatePlayerOptions()

	GoWin.DismissDialog = func() { BoardSizeDialog.Hide() }
	GoWin.SubmitDialog = func() { BoardSizeDialog.Confirm() }
	GoWin.ResizeDialog = func() { BoardSizeDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	GoWin.WireSubmitOnEnter(append(ValidatedEntries, IndividualNameEntry)...)
	BoardSizeDialog.Show()
	BoardSizeDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	GoWin.Win.Canvas().Focus(WidthEntry)
}

// SortedPresetNames returns preset names in sorted order with "Default" first.
func SortedPresetNames() []string {
	Names := make([]string, 0, len(CurrentAppConfig.FreshBoardPresets))
	for Name := range CurrentAppConfig.FreshBoardPresets {
		if Name != "Default" {
			Names = append(Names, Name)
		}
	}
	sort.Strings(Names)
	return append([]string{"Default"}, Names...)
}

// AppendPlayerName adds Name to the saved player names list if not already present.
func AppendPlayerName(Name string) {
	if Name == "" {
		return
	}
	if slices.Contains(CurrentAppConfig.PlayerNames, Name) {
		return
	}
	CurrentAppConfig.PlayerNames = append(CurrentAppConfig.PlayerNames, Name)
}
