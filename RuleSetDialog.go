package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// RuleSetDialogEntry tracks one entry in the rule set dialog list.
type RuleSetDialogEntry struct {
	RuleSet  *RuleSet
	Category string // "Built-in", "Custom", or "Game"
	// For custom rule sets: index into ShownCustomRuleSets or HiddenCustomRuleSets
	SliceIndex int
	IsShown    bool // Meaningful for "Built-in" and "Custom"; Game entries are not config-backed
}

// ProtectedRuleSetIsShown reports whether a built-in rule set is included in
// the score table configuration.
func ProtectedRuleSetIsShown(RuleSet *RuleSet) bool {
	if RuleSet == nil || len(RuleSet.Names) == 0 {
		return false
	}
	for _, Name := range CurrentAppConfig.ShownProtectedRuleSetNames {
		if Name == RuleSet.Names[0] {
			return true
		}
	}
	return false
}

// SetProtectedRuleSetShown adds or removes a built-in rule set's canonical
// name from the score table configuration.
func SetProtectedRuleSetShown(RuleSet *RuleSet, Shown bool) {
	if RuleSet == nil || len(RuleSet.Names) == 0 {
		return
	}
	Name := RuleSet.Names[0]
	ExistingIndex := -1
	for Index, ExistingName := range CurrentAppConfig.ShownProtectedRuleSetNames {
		if ExistingName == Name {
			ExistingIndex = Index
			break
		}
	}
	if Shown {
		if ExistingIndex < 0 {
			CurrentAppConfig.ShownProtectedRuleSetNames = append(CurrentAppConfig.ShownProtectedRuleSetNames, Name)
		}
		return
	}
	if ExistingIndex >= 0 {
		CurrentAppConfig.ShownProtectedRuleSetNames = append(
			CurrentAppConfig.ShownProtectedRuleSetNames[:ExistingIndex],
			CurrentAppConfig.ShownProtectedRuleSetNames[ExistingIndex+1:]...)
	}
}

// BuildRuleSetDialogEntries builds the unified list of rule sets for the dialog.
func (GoWin *GoWin) BuildRuleSetDialogEntries() []RuleSetDialogEntry {
	Entries := []RuleSetDialogEntry{}

	// Protected rule sets
	for Index := range ProtectedRuleSets {
		RegisterRuleSet(&ProtectedRuleSets[Index])
		Entries = append(Entries, RuleSetDialogEntry{
			RuleSet:  &ProtectedRuleSets[Index],
			Category: "Built-in",
			IsShown:  ProtectedRuleSetIsShown(&ProtectedRuleSets[Index]),
		})
	}

	// Custom shown rule sets
	for Index := range CurrentAppConfig.ShownCustomRuleSets {
		RegisterRuleSet(&CurrentAppConfig.ShownCustomRuleSets[Index])
		Entries = append(Entries, RuleSetDialogEntry{
			RuleSet:    &CurrentAppConfig.ShownCustomRuleSets[Index],
			Category:   "Custom",
			SliceIndex: Index,
			IsShown:    true,
		})
	}

	// Custom hidden rule sets
	for Index := range CurrentAppConfig.HiddenCustomRuleSets {
		RegisterRuleSet(&CurrentAppConfig.HiddenCustomRuleSets[Index])
		Entries = append(Entries, RuleSetDialogEntry{
			RuleSet:    &CurrentAppConfig.HiddenCustomRuleSets[Index],
			Category:   "Custom",
			SliceIndex: Index,
			IsShown:    false,
		})
	}

	// Collection game rule sets not already in the built-in/custom lists. These
	// preserve imported savegame rule sets from other people's configs until
	// the user chooses to save them as Custom.
	IsAlreadyListed := func(RuleSet *RuleSet) bool {
		for _, Entry := range Entries {
			if Entry.RuleSet.Equal(RuleSet) {
				return true
			}
		}
		return false
	}
	for _, Child := range GoWin.Coll.CollectionNode.Children {
		if Child.Board != nil && Child.Board.Hist != nil && Child.Board.Hist.RuleSet != nil {
			GameRuleSet := Child.Board.Hist.RuleSet
			if !IsAlreadyListed(GameRuleSet) {
				RegisterRuleSet(GameRuleSet)
				Entries = append(Entries, RuleSetDialogEntry{
					RuleSet:  GameRuleSet,
					Category: "Game",
				})
			}
		}
	}

	return Entries
}

// RuleSetCopyNameForCustom returns a protected-name-safe primary name for a
// custom copy of Source.
func RuleSetCopyNameForCustom(Source *RuleSet, Suffix string) string {
	BaseName := "Rule Set"
	if Source != nil && len(Source.Names) > 0 && strings.TrimSpace(Source.Names[0]) != "" {
		BaseName = Source.Names[0]
	}
	Name := BaseName + Suffix
	for RuleSetNameConflictsWithProtected([]string{Name}) {
		Name += " Copy"
	}
	return Name
}

// PrepareRuleSetCopyForCustom deep-copies Source and makes its names valid for
// a custom rule set by giving the primary name a safe suffix and dropping any
// protected-name aliases.
func PrepareRuleSetCopyForCustom(Source *RuleSet, Suffix string) RuleSet {
	Copy := DeepCopyRuleSet(*Source)
	if len(Copy.Names) == 0 {
		Copy.Names = []string{"Rule Set"}
	}
	Copy.Names[0] = RuleSetCopyNameForCustom(Source, Suffix)
	Names := []string{Copy.Names[0]}
	for _, Alias := range Copy.Names[1:] {
		Alias = strings.TrimSpace(Alias)
		if Alias == "" || RuleSetNameConflictsWithProtected([]string{Alias}) {
			continue
		}
		Duplicate := false
		for _, Name := range Names {
			if Name == Alias {
				Duplicate = true
				break
			}
		}
		if !Duplicate {
			Names = append(Names, Alias)
		}
	}
	Copy.Names = Names
	return Copy
}

// RuleSetEntryDisplayName returns the display name for a dialog entry.
// When another entry in Entries has the same Names[0] (regardless of
// category), a " #<RuleSetId>" suffix is appended to disambiguate.
func RuleSetEntryDisplayName(Entry RuleSetDialogEntry, Entries []RuleSetDialogEntry) string {
	Name := Entry.RuleSet.Names[0]
	for _, Other := range Entries {
		if Other.RuleSet != Entry.RuleSet && Other.RuleSet.Names[0] == Name {
			return fmt.Sprintf("[%s] %s #%d", Entry.Category, Name, RuleSetId[Entry.RuleSet])
		}
	}
	return fmt.Sprintf("[%s] %s", Entry.Category, Name)
}

// DeduplicateCustomRuleSets enforces the invariant that
// ShownCustomRuleSets ∪ HiddenCustomRuleSets contains no two entries where
// A.Equal(B). Duplicates are merged: presets pointing at the duplicate are
// re-aimed at the canonical survivor, then the duplicate is removed from its
// slice. When a pair spans the Shown/Hidden boundary, the Shown entry is
// preserved (user intent: it is in the score table).
func DeduplicateCustomRuleSets() {
	// Pass 1: collapse duplicates within ShownCustomRuleSets, keeping earliest.
	DedupeWithin := func(Slice *[]RuleSet) {
		Index := 0
		for Index < len(*Slice) {
			Canonical := &(*Slice)[Index]
			Inner := Index + 1
			for Inner < len(*Slice) {
				Candidate := &(*Slice)[Inner]
				if Canonical.Equal(Candidate) {
					UpdatePresetsByValue(Candidate, Canonical)
					*Slice = append((*Slice)[:Inner], (*Slice)[Inner+1:]...)
					// After the splice, the tail shifted; re-resolve Canonical
					// pointer because backing-array index shifted left for elements
					// after Inner — but Canonical is at Index which is before Inner,
					// so its address is unchanged. Continue without incrementing Inner.
				} else {
					Inner++
				}
			}
			Index++
		}
	}
	DedupeWithin(&CurrentAppConfig.ShownCustomRuleSets)
	DedupeWithin(&CurrentAppConfig.HiddenCustomRuleSets)

	// Pass 2: collapse duplicates across the boundary. Shown wins.
	OuterIndex := 0
	for OuterIndex < len(CurrentAppConfig.ShownCustomRuleSets) {
		Canonical := &CurrentAppConfig.ShownCustomRuleSets[OuterIndex]
		InnerIndex := 0
		for InnerIndex < len(CurrentAppConfig.HiddenCustomRuleSets) {
			Candidate := &CurrentAppConfig.HiddenCustomRuleSets[InnerIndex]
			if Canonical.Equal(Candidate) {
				UpdatePresetsByValue(Candidate, Canonical)
				CurrentAppConfig.HiddenCustomRuleSets = append(
					CurrentAppConfig.HiddenCustomRuleSets[:InnerIndex],
					CurrentAppConfig.HiddenCustomRuleSets[InnerIndex+1:]...)
			} else {
				InnerIndex++
			}
		}
		OuterIndex++
	}

	RelinkPresetsToCustomSlices()
}

// RelinkPresetsToCustomSlices walks every FreshBoardPreset and, when its
// RuleSet matches (by value) a current custom slice element, re-points the
// preset at that slice element's address. This must be called after any
// append/splice on ShownCustomRuleSets or HiddenCustomRuleSets, because those
// operations may reallocate the backing array and invalidate previously-held
// *RuleSet pointers.
func RelinkPresetsToCustomSlices() {
	for PresetName, Preset := range CurrentAppConfig.FreshBoardPresets {
		if Preset.RuleSet == nil {
			continue
		}
		if FindRuleSetByName(Preset.RuleSet.Names[0]) != nil {
			continue
		}
		Relinked := false
		for Index := range CurrentAppConfig.ShownCustomRuleSets {
			if Preset.RuleSet.Equal(&CurrentAppConfig.ShownCustomRuleSets[Index]) {
				Preset.RuleSet = &CurrentAppConfig.ShownCustomRuleSets[Index]
				CurrentAppConfig.FreshBoardPresets[PresetName] = Preset
				Relinked = true
				break
			}
		}
		if Relinked {
			continue
		}
		for Index := range CurrentAppConfig.HiddenCustomRuleSets {
			if Preset.RuleSet.Equal(&CurrentAppConfig.HiddenCustomRuleSets[Index]) {
				Preset.RuleSet = &CurrentAppConfig.HiddenCustomRuleSets[Index]
				CurrentAppConfig.FreshBoardPresets[PresetName] = Preset
				break
			}
		}
	}
}

// FindCustomRuleSetMatching returns a pointer to the existing custom rule set
// (Shown or Hidden) that is content-equal to Candidate, or nil if none exists.
// Used to enforce the dedupe invariant at New/Duplicate/Modify time.
func FindCustomRuleSetMatching(Candidate *RuleSet) *RuleSet {
	for Index := range CurrentAppConfig.ShownCustomRuleSets {
		if Candidate.Equal(&CurrentAppConfig.ShownCustomRuleSets[Index]) {
			return &CurrentAppConfig.ShownCustomRuleSets[Index]
		}
	}
	for Index := range CurrentAppConfig.HiddenCustomRuleSets {
		if Candidate.Equal(&CurrentAppConfig.HiddenCustomRuleSets[Index]) {
			return &CurrentAppConfig.HiddenCustomRuleSets[Index]
		}
	}
	return nil
}

// ShowRuleSetDialog opens the Rule Set Configuration dialog.
func (GoWin *GoWin) ShowRuleSetDialog() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	var Entries []RuleSetDialogEntry
	var DisplayNames []string
	var EntryByDisplayName map[string]RuleSetDialogEntry

	RuleSetSelect := widget.NewSelect(nil, nil)
	NewButton := widget.NewButton("New", nil)
	DuplicateButton := widget.NewButton("Duplicate", nil)
	SaveAsCustomButton := widget.NewButton("Save as Custom", nil)
	ModifyButton := widget.NewButton("Modify", nil)
	DeleteButton := widget.NewButton("Delete", nil)
	ShowInScoreTableCheck := widget.NewCheck("Show in score table", nil)

	// RefreshEntries rebuilds the entry list and optionally selects the entry
	// whose RuleSet pointer matches SelectRS. Selecting by pointer (rather than
	// by display name) survives name changes and same-name disambiguation.
	RefreshEntries := func(SelectRS *RuleSet) {
		Entries = GoWin.BuildRuleSetDialogEntries()
		DisplayNames = make([]string, len(Entries))
		EntryByDisplayName = map[string]RuleSetDialogEntry{}
		SelectedIndex := -1
		for Index, Entry := range Entries {
			DisplayName := RuleSetEntryDisplayName(Entry, Entries)
			DisplayNames[Index] = DisplayName
			EntryByDisplayName[DisplayName] = Entry
			if SelectRS != nil && Entry.RuleSet == SelectRS {
				SelectedIndex = Index
			}
		}
		RuleSetSelect.Options = DisplayNames
		RuleSetSelect.Refresh()
		if SelectedIndex >= 0 {
			RuleSetSelect.SetSelected(DisplayNames[SelectedIndex])
		} else if len(DisplayNames) > 0 {
			RuleSetSelect.SetSelected(DisplayNames[0])
		}
	}

	UpdateButtons := func(DisplayName string) {
		Entry, Found := EntryByDisplayName[DisplayName]
		if !Found {
			DuplicateButton.Disable()
			SaveAsCustomButton.Disable()
			ModifyButton.Disable()
			DeleteButton.Disable()
			ShowInScoreTableCheck.Disable()
			return
		}
		DuplicateButton.Enable()
		SaveAsCustomButton.Disable()
		if Entry.Category == "Custom" {
			ModifyButton.Enable()
			DeleteButton.Enable()
			ShowInScoreTableCheck.Enable()
			ShowInScoreTableCheck.SetChecked(Entry.IsShown)
		} else if Entry.Category == "Built-in" {
			ModifyButton.Disable()
			DeleteButton.Disable()
			ShowInScoreTableCheck.Enable()
			ShowInScoreTableCheck.SetChecked(Entry.IsShown)
		} else {
			SaveAsCustomButton.Enable()
			ModifyButton.Disable()
			DeleteButton.Disable()
			ShowInScoreTableCheck.Disable()
			ShowInScoreTableCheck.SetChecked(false)
		}
	}

	RuleSetSelect.OnChanged = func(DisplayName string) {
		UpdateButtons(DisplayName)
	}

	// SaveAndRefresh persists the config, rebuilds the entries list, and
	// selects the entry with RuleSet pointer == SelectRS. After any mutation
	// of the custom slices, the dedupe invariant must be re-established and
	// preset pointers must be relinked because append may have reallocated
	// the backing array.
	SaveAndRefresh := func(SelectRS *RuleSet) {
		DeduplicateCustomRuleSets()
		RelinkPresetsToCustomSlices()
		if Err := GoWin.SaveConfig(); Err != nil {
			GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
		}
		// After dedupe, SelectRS may no longer be in either slice. If so, fall
		// back to first entry.
		if SelectRS != nil {
			Exists := false
			if len(SelectRS.Names) > 0 && FindRuleSetByName(SelectRS.Names[0]) == SelectRS {
				Exists = true
			} else {
				for Index := range CurrentAppConfig.ShownCustomRuleSets {
					if &CurrentAppConfig.ShownCustomRuleSets[Index] == SelectRS {
						Exists = true
						break
					}
				}
			}
			if !Exists {
				for Index := range CurrentAppConfig.HiddenCustomRuleSets {
					if &CurrentAppConfig.HiddenCustomRuleSets[Index] == SelectRS {
						Exists = true
						break
					}
				}
			}
			if !Exists {
				SelectRS = nil
			}
		}
		RefreshEntries(SelectRS)
		if GoWin.Coll.CurrentGameH != nil {
			GoWin.CalculateAndDisplayScore()
		}
	}

	NewButton.OnTapped = func() {
		NewRuleSet := RuleSet{
			Names:                  []string{"New Rule Set"},
			StonePlacementsPerMove: 1,
			NonRepetitionRules:     NonRepetitionRuleBasicKo,
		}
		// If an existing custom rule set has identical content, select it
		// instead of appending a duplicate.
		if Existing := FindCustomRuleSetMatching(&NewRuleSet); Existing != nil {
			SaveAndRefresh(Existing)
			return
		}
		CurrentAppConfig.ShownCustomRuleSets = append(CurrentAppConfig.ShownCustomRuleSets, NewRuleSet)
		Inserted := &CurrentAppConfig.ShownCustomRuleSets[len(CurrentAppConfig.ShownCustomRuleSets)-1]
		SaveAndRefresh(Inserted)
	}

	DuplicateButton.OnTapped = func() {
		Entry, Found := EntryByDisplayName[RuleSetSelect.Selected]
		if !Found {
			return
		}
		Copy := PrepareRuleSetCopyForCustom(Entry.RuleSet, " (Copy)")
		// If another custom rule set already has identical content, select it.
		if Existing := FindCustomRuleSetMatching(&Copy); Existing != nil {
			SaveAndRefresh(Existing)
			return
		}
		CurrentAppConfig.ShownCustomRuleSets = append(CurrentAppConfig.ShownCustomRuleSets, Copy)
		Inserted := &CurrentAppConfig.ShownCustomRuleSets[len(CurrentAppConfig.ShownCustomRuleSets)-1]
		SaveAndRefresh(Inserted)
	}

	SaveAsCustomButton.OnTapped = func() {
		Entry, Found := EntryByDisplayName[RuleSetSelect.Selected]
		if !Found || Entry.Category != "Game" {
			return
		}
		Copy := PrepareRuleSetCopyForCustom(Entry.RuleSet, "")
		if Existing := FindCustomRuleSetMatching(&Copy); Existing != nil {
			SaveAndRefresh(Existing)
			return
		}
		CurrentAppConfig.ShownCustomRuleSets = append(CurrentAppConfig.ShownCustomRuleSets, Copy)
		Inserted := &CurrentAppConfig.ShownCustomRuleSets[len(CurrentAppConfig.ShownCustomRuleSets)-1]
		SaveAndRefresh(Inserted)
	}

	ModifyButton.OnTapped = func() {
		Entry, Found := EntryByDisplayName[RuleSetSelect.Selected]
		if !Found || Entry.Category != "Custom" {
			return
		}
		GoWin.ShowModifyRuleSetDialog(Entry.SliceIndex, Entry.IsShown, func() {
			// Resolve the pointer to the just-edited slice element. The slice
			// address is stable across the write-back at Modify save (no
			// append, just element overwrite).
			var NewRS *RuleSet
			if Entry.IsShown {
				NewRS = &CurrentAppConfig.ShownCustomRuleSets[Entry.SliceIndex]
			} else {
				NewRS = &CurrentAppConfig.HiddenCustomRuleSets[Entry.SliceIndex]
			}
			// If the edit made NewRS content-equal to another existing entry,
			// merge: re-aim presets and drop NewRS. SaveAndRefresh runs
			// DeduplicateCustomRuleSets which does the merge automatically.
			SaveAndRefresh(NewRS)
		})
	}

	DeleteButton.OnTapped = func() {
		Entry, Found := EntryByDisplayName[RuleSetSelect.Selected]
		if !Found || Entry.Category != "Custom" {
			return
		}
		DependentPresets := PresetsUsingRuleSet(Entry.RuleSet)
		Message := fmt.Sprintf("Delete %q?", Entry.RuleSet.Names[0])
		if len(DependentPresets) > 0 {
			Message += "\n\nThe following presets use this rule set and will also be deleted:\n  • " +
				strings.Join(DependentPresets, "\n  • ")
		}
		dialog.ShowConfirm("Delete Rule Set", Message,
			func(Confirmed bool) {
				if !Confirmed {
					return
				}
				for _, Name := range DependentPresets {
					delete(CurrentAppConfig.FreshBoardPresets, Name)
					if CurrentAppConfig.ActivePresetName == Name {
						CurrentAppConfig.ActivePresetName = "Default"
					}
				}
				if Entry.IsShown {
					CurrentAppConfig.ShownCustomRuleSets = append(
						CurrentAppConfig.ShownCustomRuleSets[:Entry.SliceIndex],
						CurrentAppConfig.ShownCustomRuleSets[Entry.SliceIndex+1:]...)
				} else {
					CurrentAppConfig.HiddenCustomRuleSets = append(
						CurrentAppConfig.HiddenCustomRuleSets[:Entry.SliceIndex],
						CurrentAppConfig.HiddenCustomRuleSets[Entry.SliceIndex+1:]...)
				}
				SaveAndRefresh(nil)
			}, GoWin.Win)
	}

	ShowInScoreTableCheck.OnChanged = func(Checked bool) {
		Entry, Found := EntryByDisplayName[RuleSetSelect.Selected]
		if !Found {
			return
		}
		if Entry.Category == "Built-in" {
			if Entry.IsShown == Checked {
				return
			}
			SetProtectedRuleSetShown(Entry.RuleSet, Checked)
			SaveAndRefresh(Entry.RuleSet)
			return
		}
		if Entry.Category != "Custom" {
			return
		}
		RuleSetValue := *Entry.RuleSet
		var Inserted *RuleSet
		if Entry.IsShown && !Checked {
			CurrentAppConfig.ShownCustomRuleSets = append(
				CurrentAppConfig.ShownCustomRuleSets[:Entry.SliceIndex],
				CurrentAppConfig.ShownCustomRuleSets[Entry.SliceIndex+1:]...)
			CurrentAppConfig.HiddenCustomRuleSets = append(
				CurrentAppConfig.HiddenCustomRuleSets, RuleSetValue)
			Inserted = &CurrentAppConfig.HiddenCustomRuleSets[len(CurrentAppConfig.HiddenCustomRuleSets)-1]
		} else if !Entry.IsShown && Checked {
			CurrentAppConfig.HiddenCustomRuleSets = append(
				CurrentAppConfig.HiddenCustomRuleSets[:Entry.SliceIndex],
				CurrentAppConfig.HiddenCustomRuleSets[Entry.SliceIndex+1:]...)
			CurrentAppConfig.ShownCustomRuleSets = append(
				CurrentAppConfig.ShownCustomRuleSets, RuleSetValue)
			Inserted = &CurrentAppConfig.ShownCustomRuleSets[len(CurrentAppConfig.ShownCustomRuleSets)-1]
		} else {
			return
		}
		// Any preset whose pointer pointed at the old slice element must be
		// re-aimed at the new element before dedupe runs. Use the pre-move
		// pointer value captured in Entry.RuleSet.
		UpdatePresetsRuleSet(Entry.RuleSet, Inserted)
		SaveAndRefresh(Inserted)
	}

	RefreshEntries(nil)
	if len(Entries) > 0 {
		UpdateButtons(RuleSetSelect.Selected)
	}

	Content := container.NewVBox(
		RuleSetSelect,
		container.NewHBox(NewButton, DuplicateButton, SaveAsCustomButton, ModifyButton, DeleteButton),
		ShowInScoreTableCheck,
	)

	Dialog := dialog.NewCustom("Rule Set Configuration", "Close", Content, GoWin.Win)
	Dialog.SetOnClosed(func() { GoWin.DialogClosed() })
	GoWin.DismissDialog = func() { Dialog.Hide() }
	Dialog.Show()
}

// ShowModifyRuleSetDialog opens a sub-dialog to edit a custom rule set.
// SliceIndex is the index into ShownCustomRuleSets (if IsShown) or
// HiddenCustomRuleSets (if !IsShown). OnDone is called after a successful save.
func (GoWin *GoWin) ShowModifyRuleSetDialog(SliceIndex int, IsShown bool, OnDone func()) {
	var Original *RuleSet
	if IsShown {
		Original = &CurrentAppConfig.ShownCustomRuleSets[SliceIndex]
	} else {
		Original = &CurrentAppConfig.HiddenCustomRuleSets[SliceIndex]
	}
	Edited := DeepCopyRuleSet(*Original)

	NameEntry := NewDialogEntry(GoWin)
	NameEntry.SetText(Edited.Names[0])

	AliasEntry := NewDialogEntry(GoWin)
	if len(Edited.Names) > 1 {
		AliasEntry.SetText(strings.Join(Edited.Names[1:], ", "))
	}
	AliasEntry.SetPlaceHolder("Comma-separated aliases")

	ParseFloat := func(Text string) (float64, error) {
		return strconv.ParseFloat(strings.TrimSpace(Text), 64)
	}

	FloatEntry := func(Value float64) *DialogEntry {
		Entry := NewDialogEntry(GoWin)
		Entry.SetText(FormatPoints(Value))
		return Entry
	}

	LiveStonesEntry := FloatEntry(Edited.LiveStones)
	PrisonersEntry := FloatEntry(Edited.Prisoners)
	PassesEntry := FloatEntry(Edited.Passes)
	StoneSuicidesEntry := FloatEntry(Edited.StoneSuicides)
	ConversionsEntry := FloatEntry(Edited.Conversions)

	StonePlacementsEntry := NewDialogEntry(GoWin)
	StonePlacementsEntry.SetText(strconv.Itoa(int(Edited.StonePlacementsPerMove)))

	LastPlayerMustPassLastCheck := widget.NewCheck("Last Player Must Pass Last", nil)
	LastPlayerMustPassLastCheck.SetChecked(Edited.LastPlayerMustPassLast)

	SuicideIsLegalCheck := widget.NewCheck("Suicide Is Legal", nil)
	SuicideIsLegalCheck.SetChecked(Edited.SuicideIsLegal)

	CaptureConvertsCheck := widget.NewCheck("Capture Converts", nil)
	CaptureConvertsCheck.SetChecked(Edited.CaptureConverts)

	// NonRepetitionRules checkboxes. The first four entries form a strict
	// hierarchy (BasicKo ⊂ NaturalSituational ⊂ Situational ⊂ Positional):
	// checking a stronger flag also checks all weaker flags, and unchecking
	// a weaker flag also unchecks all stronger flags, so the persisted
	// bitmask stays self-consistent. The runtime only evaluates the strongest
	// enabled rule. NoResult is independent of the chain.
	RepetitionChecks := make([]*widget.Check, len(NonRepetitionRuleFlags))
	ChainLen := min(4, len(NonRepetitionRuleFlags))
	for Index, FlagInfo := range NonRepetitionRuleFlags {
		CapturedIndex := Index
		RepetitionChecks[Index] = widget.NewCheck(FlagInfo.Name, func(Checked bool) {
			if CapturedIndex >= ChainLen {
				return
			}
			if Checked {
				for Weaker := range CapturedIndex {
					if !RepetitionChecks[Weaker].Checked {
						RepetitionChecks[Weaker].SetChecked(true)
					}
				}
			} else {
				for Stronger := CapturedIndex + 1; Stronger < ChainLen; Stronger++ {
					if RepetitionChecks[Stronger].Checked {
						RepetitionChecks[Stronger].SetChecked(false)
					}
				}
			}
		})
		RepetitionChecks[Index].SetChecked(Edited.NonRepetitionRules&FlagInfo.Flag != 0)
	}

	RepetitionContainer := container.NewVBox()
	for _, Check := range RepetitionChecks {
		RepetitionContainer.Add(Check)
	}

	FormContent := container.NewVBox(
		widget.NewLabel("Rule-set Name"),
		NameEntry,
		widget.NewLabel("Aliases"),
		AliasEntry,
		widget.NewSeparator(),
		widget.NewLabel("Score Multipliers"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Live Stones"), LiveStonesEntry,
			widget.NewLabel("Prisoners"), PrisonersEntry,
			widget.NewLabel("Passes"), PassesEntry,
			widget.NewLabel("Stone Suicides"), StoneSuicidesEntry,
			widget.NewLabel("Conversions"), ConversionsEntry,
		),
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			widget.NewLabel("Stone Placements Per Move"), StonePlacementsEntry,
		),
		widget.NewSeparator(),
		widget.NewLabel("Flags"),
		LastPlayerMustPassLastCheck,
		SuicideIsLegalCheck,
		CaptureConvertsCheck,
		widget.NewSeparator(),
		widget.NewLabel("Non-Repetition Rules"),
		RepetitionContainer,
	)

	ScrollContent := container.NewVScroll(FormContent)
	ScrollContent.SetMinSize(fyne.NewSize(400, 400))

	Dialog := dialog.NewCustomConfirm("Modify Rule Set", "OK", "Cancel", ScrollContent,
		func(Confirmed bool) {
			if !Confirmed {
				return
			}
			// Validate and apply
			TrimmedName := strings.TrimSpace(NameEntry.Text)
			if TrimmedName == "" {
				GoWin.ShowError(fmt.Errorf("name must not be empty"))
				return
			}
			Edited.Names = []string{TrimmedName}
			AliasText := strings.TrimSpace(AliasEntry.Text)
			if AliasText != "" {
				for _, Alias := range strings.Split(AliasText, ",") {
					TrimmedAlias := strings.TrimSpace(Alias)
					if TrimmedAlias != "" {
						Edited.Names = append(Edited.Names, TrimmedAlias)
					}
				}
			}
			if RuleSetNameConflictsWithProtected(Edited.Names) {
				GoWin.ShowError(fmt.Errorf("name or alias conflicts with a built-in rule set"))
				return
			}

			ParseAndSet := func(Entry *DialogEntry, Target *float64, FieldName string) error {
				Value, Err := ParseFloat(Entry.Text)
				if Err != nil {
					return fmt.Errorf("invalid %s: %v", FieldName, Err)
				}
				*Target = Value
				return nil
			}

			if Err := ParseAndSet(LiveStonesEntry, &Edited.LiveStones, "Live Stones"); Err != nil {
				GoWin.ShowError(Err)
				return
			}
			if Err := ParseAndSet(PrisonersEntry, &Edited.Prisoners, "Prisoners"); Err != nil {
				GoWin.ShowError(Err)
				return
			}
			if Err := ParseAndSet(PassesEntry, &Edited.Passes, "Passes"); Err != nil {
				GoWin.ShowError(Err)
				return
			}
			if Err := ParseAndSet(StoneSuicidesEntry, &Edited.StoneSuicides, "Stone Suicides"); Err != nil {
				GoWin.ShowError(Err)
				return
			}
			if Err := ParseAndSet(ConversionsEntry, &Edited.Conversions, "Conversions"); Err != nil {
				GoWin.ShowError(Err)
				return
			}

			StonePlacements, Err := strconv.ParseUint(strings.TrimSpace(StonePlacementsEntry.Text), 10, 8)
			if Err != nil || StonePlacements < 1 {
				GoWin.ShowError(fmt.Errorf("Stone Placements Per Move must be 1-255"))
				return
			}
			Edited.StonePlacementsPerMove = uint8(StonePlacements)

			Edited.LastPlayerMustPassLast = LastPlayerMustPassLastCheck.Checked
			Edited.SuicideIsLegal = SuicideIsLegalCheck.Checked
			Edited.CaptureConverts = CaptureConvertsCheck.Checked

			Edited.NonRepetitionRules = 0
			for Index, FlagInfo := range NonRepetitionRuleFlags {
				if RepetitionChecks[Index].Checked {
					Edited.NonRepetitionRules |= FlagInfo.Flag
				}
			}

			// Write back to config slice
			if IsShown {
				CurrentAppConfig.ShownCustomRuleSets[SliceIndex] = Edited
			} else {
				CurrentAppConfig.HiddenCustomRuleSets[SliceIndex] = Edited
			}
			OnDone()
		}, GoWin.Win)
	ResizeToWindow := func() {
		Dialog.Resize(fyne.NewSize(1, GoWin.Win.Canvas().Size().Height))
	}
	GoWin.ResizeDialog = ResizeToWindow
	Dialog.SetOnClosed(func() {
		GoWin.ResizeDialog = nil
	})
	Dialog.Show()
	ResizeToWindow()
}

// PresetsUsingRuleSet returns the names of all FreshBoardPresets whose
// RuleSet matches the given one by pointer identity. Pointer identity is
// sufficient because the dedupe invariant guarantees distinct custom rule set
// pointers are not content-equal.
func PresetsUsingRuleSet(RS *RuleSet) []string {
	var Names []string
	for Name, Preset := range CurrentAppConfig.FreshBoardPresets {
		if Name == "Default" {
			continue
		}
		if Preset.RuleSet == RS {
			Names = append(Names, Name)
		}
	}
	return Names
}

// UpdatePresetsRuleSet replaces the RuleSet pointer in every FreshBoardPreset
// that matched OldRS by pointer identity with NewRS. Pointer identity is
// sufficient because the dedupe invariant guarantees distinct custom rule set
// pointers are not content-equal to each other.
func UpdatePresetsRuleSet(OldRS *RuleSet, NewRS *RuleSet) {
	for Name, Preset := range CurrentAppConfig.FreshBoardPresets {
		if Preset.RuleSet == OldRS {
			Preset.RuleSet = NewRS
			CurrentAppConfig.FreshBoardPresets[Name] = Preset
		}
	}
}

// UpdatePresetsByValue replaces the RuleSet pointer in every FreshBoardPreset
// that equals OldRS by value with NewRS. Used during dedupe when distinct
// pointers are about to be collapsed into a single canonical pointer.
func UpdatePresetsByValue(OldRS *RuleSet, NewRS *RuleSet) {
	for Name, Preset := range CurrentAppConfig.FreshBoardPresets {
		if Preset.RuleSet == OldRS || (Preset.RuleSet != nil && Preset.RuleSet.Equal(OldRS)) {
			Preset.RuleSet = NewRS
			CurrentAppConfig.FreshBoardPresets[Name] = Preset
		}
	}
}

// RuleSetNameConflictsWithProtected returns true if any name in Names matches
// a name or alias of a ProtectedRuleSet.
func RuleSetNameConflictsWithProtected(Names []string) bool {
	for _, Name := range Names {
		if FindRuleSetByName(Name) != nil {
			return true
		}
	}
	return false
}

// DeepCopyRuleSet returns a deep copy of the given RuleSet.
func DeepCopyRuleSet(RS RuleSet) RuleSet {
	Copy := RS
	Copy.Names = make([]string, len(RS.Names))
	copy(Copy.Names, RS.Names)
	return Copy
}
