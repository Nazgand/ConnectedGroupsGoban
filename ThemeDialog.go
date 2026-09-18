package main

import (
	"fmt"
	"image/color"
	"maps"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// InitColors populates PlayerColors and Colors from the active theme,
// falling back to DefaultConfig.Themes["Default"] values for any missing keys.
func (GoWin *GoWin) InitColors(Theme Theme) {
	DefaultThemeValue := DefaultConfig.Themes["Default"]
	for Player, DefaultHexColor := range DefaultThemeValue.PlayerHexColors {
		HexColor, Ok := Theme.PlayerHexColors[Player]
		if !Ok {
			HexColor = DefaultHexColor
		}
		PlayerColors[Player] = CreatePlayerColor(HexColor)
	}
	for Key, DefaultHexColor := range DefaultThemeValue.HexColors {
		HexColor, Ok := Theme.HexColors[Key]
		if !Ok {
			HexColor = DefaultHexColor
		}
		var Err error
		Colors[Key], Err = HexColorToNRGBA(HexColor)
		if Err != nil {
			fmt.Printf("Config color error(%s, %s): %v\n", Key, HexColor, Err)
			fmt.Println("Using default theme color.")
			Colors[Key], Err = HexColorToNRGBA(DefaultHexColor)
			if Err != nil {
				fmt.Println("Error in default theme color.")
			}
		}
		if Colors[Key].A == 0 {
			fmt.Printf("Color %s has alpha 0, using default theme color.\n", Key)
			Colors[Key], _ = HexColorToNRGBA(DefaultHexColor)
		}
	}
}

// GetActiveTheme returns the currently active theme
func GetActiveTheme() Theme {
	return ThemeByName(CurrentAppConfig.ActiveTheme)
}

// AllThemeNames returns "Default" followed by sorted user theme names.
func AllThemeNames() []string {
	Names := []string{"Default"}
	User := make([]string, 0, len(CurrentAppConfig.Themes))
	for K := range CurrentAppConfig.Themes {
		if K != "Default" {
			User = append(User, K)
		}
	}
	sort.Strings(User)
	return append(Names, User...)
}

// themeByName returns the Theme for the given name.
func ThemeByName(name string) Theme {
	Theme, Exists := CurrentAppConfig.Themes[name]
	if Exists {
		return Theme
	}
	return CurrentAppConfig.Themes["Default"]
}

// ApplyThemeVisual applies theme colors/coord to the live board without
// saving the config. Also writes Theme into `CurrentAppConfig.Themes[ActiveTheme]`
// in memory so display-flag changes (ShowLibertyLine/ShowStoneConnection/
// ShowIllegalDot/ShowHoverLiberties/ShowChildNodeDots) take effect for
// drawing code that reads `GetActiveTheme()` directly. The ShowModifyThemeDialog
// close-without-save path calls this with the original theme to revert.
func (GoWin *GoWin) ApplyThemeVisual(Theme Theme) {
	CurrentAppConfig.Themes[CurrentAppConfig.ActiveTheme] = Theme
	SetCoordFormat(Theme.CoordFmt)
	GoWin.InitColors(Theme)
	if GoWin.PlayArea != nil {
		GoWin.PlayArea.FillColor = Colors["Play Area"]
		GoWin.PlayArea.Refresh()
	}
	GoWin.DrawGoban()
	GoWin.DrawBoard()
}

// applyTheme applies a theme, marks it active in the config, and saves.
func (GoWin *GoWin) ApplyTheme(name string, t Theme) {
	CurrentAppConfig.ActiveTheme = name
	GoWin.ApplyThemeVisual(t)
	if Err := GoWin.SaveConfig(); Err != nil {
		GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
	}
}

// deepCopyTheme returns an independent copy of a Theme.
func DeepCopyTheme(Theme0 Theme) Theme {
	Theme1 := Theme0
	Theme1.HexColors = make(map[string]string, len(Theme0.HexColors))
	maps.Copy(Theme1.HexColors, Theme0.HexColors)
	Theme1.PlayerHexColors = make(map[uint8]string, len(Theme0.PlayerHexColors))
	maps.Copy(Theme1.PlayerHexColors, Theme0.PlayerHexColors)
	return Theme1
}

// ShowThemeDialog opens the Theme Configuration dialog.
func (GoWin *GoWin) ShowThemeDialog() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	if CurrentAppConfig.Themes == nil {
		CurrentAppConfig.Themes = map[string]Theme{}
	}

	active := CurrentAppConfig.ActiveTheme
	if active == "" {
		active = "Default"
	}

	themeSelect := widget.NewSelect(AllThemeNames(), nil)
	themeSelect.SetSelected(active)

	duplicateBtn := widget.NewButton("Duplicate", nil)
	modifyBtn := widget.NewButton("Modify", nil)
	deleteBtn := widget.NewButton("Delete", nil)

	refreshSelect := func(selectName string) {
		themeSelect.Options = AllThemeNames()
		themeSelect.Refresh()
		themeSelect.SetSelected(selectName)
	}

	updateButtons := func(name string) {
		if name == "Default" {
			modifyBtn.Disable()
			deleteBtn.Disable()
		} else {
			modifyBtn.Enable()
			deleteBtn.Enable()
		}
	}
	updateButtons(active)

	themeSelect.OnChanged = func(name string) {
		updateButtons(name)
		GoWin.ApplyTheme(name, ThemeByName(name))
	}

	duplicateBtn.OnTapped = func() {
		srcName := themeSelect.Selected
		nameEntry := NewDialogEntry(GoWin)
		nameEntry.SetPlaceHolder("New theme name")
		d := dialog.NewForm("Duplicate Theme", "OK", "Cancel",
			[]*widget.FormItem{widget.NewFormItem("Name", nameEntry)},
			func(ok bool) {
				if !ok {
					return
				}
				newName := nameEntry.Text
				if newName == "" || newName == "Default" {
					GoWin.ShowError(fmt.Errorf("invalid theme name \"%q\"", newName))
					return
				}
				if _, exists := CurrentAppConfig.Themes[newName]; exists {
					GoWin.ShowError(fmt.Errorf("theme %q already exists", newName))
					return
				}
				CurrentAppConfig.Themes[newName] = DeepCopyTheme(ThemeByName(srcName))
				if Err := GoWin.SaveConfig(); Err != nil {
					GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
				}
				refreshSelect(newName)
				updateButtons(newName)
			}, GoWin.Win)
		d.Show()
		GoWin.Win.Canvas().Focus(nameEntry)
	}

	modifyBtn.OnTapped = func() {
		GoWin.ShowModifyThemeDialog(themeSelect.Selected, func(newName string) {
			refreshSelect(newName)
			updateButtons(newName)
		})
	}

	deleteBtn.OnTapped = func() {
		name := themeSelect.Selected
		delete(CurrentAppConfig.Themes, name)
		if CurrentAppConfig.ActiveTheme == name {
			CurrentAppConfig.ActiveTheme = "Default"
		}
		if Err := GoWin.SaveConfig(); Err != nil {
			GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
		}
		refreshSelect(CurrentAppConfig.ActiveTheme)
		updateButtons(CurrentAppConfig.ActiveTheme)
	}

	content := container.NewVBox(
		themeSelect,
		container.NewHBox(duplicateBtn, modifyBtn, deleteBtn),
	)

	d := dialog.NewCustom("Theme Configuration", "Close", content, GoWin.Win)
	d.SetOnClosed(func() { GoWin.DialogClosed() })
	GoWin.DismissDialog = func() { d.Hide() }
	d.Show()
}

// showModifyThemeDialog opens a sub-dialog to rename and edit a user theme.
// onDone is called with the (possibly new) name after a successful save.
func (GoWin *GoWin) ShowModifyThemeDialog(ThemeName string, onDone func(string)) {
	OriginalTheme := ThemeByName(ThemeName)
	Theme := DeepCopyTheme(OriginalTheme)

	NameEntry := NewDialogEntry(GoWin)
	NameEntry.SetText(ThemeName)

	CoordSelect := widget.NewSelect(CoordFmt, func(SelectedCoordFmt string) {
		Theme.CoordFmt = SelectedCoordFmt
		GoWin.ApplyThemeVisual(Theme)
	})
	CoordSelect.SetSelected(Theme.CoordFmt)

	MakeColorRow := func(Label string, GetHex func() string, SetHex func(string)) fyne.CanvasObject {
		Initial, _ := HexColorToNRGBA(GetHex())
		PreviewImage, PreviewCanvas := NewColorPreviewImage(Initial, 32, 32)
		PreviewCanvas.SetMinSize(fyne.NewSize(32, 32))
		Button := widget.NewButton(GetHex(), nil)
		Button.OnTapped = func() {
			Color, _ := HexColorToNRGBA(GetHex())
			GoWin.ShowCustomColorPicker(Label, Color, func(Picked color.NRGBA) {
				Hex := RGBAtoHexColor(Picked)
				SetHex(Hex)
				Button.SetText(Hex)
				RenderColorPreview(PreviewImage, Picked)
				PreviewCanvas.Refresh()
				GoWin.ApplyThemeVisual(Theme)
			})
		}
		return container.NewHBox(PreviewCanvas, Button)
	}

	// Add display option checkboxes
	MakeCheckboxRow := func(Label string, Checked bool, OnChange func(bool)) fyne.CanvasObject {
		CheckBox := widget.NewCheck(Label, OnChange)
		CheckBox.SetChecked(Checked)
		return CheckBox
	}

	FormItems := []*widget.FormItem{
		widget.NewFormItem("Theme Name", NameEntry),
		widget.NewFormItem("Coordinate Format", CoordSelect),
		widget.NewFormItem("Show Liberty Lines",
			MakeCheckboxRow("", Theme.ShowLibertyLine, func(Checked bool) {
				Theme.ShowLibertyLine = Checked
				GoWin.ApplyThemeVisual(Theme)
			})),
		widget.NewFormItem("Show Stone Connections",
			MakeCheckboxRow("", Theme.ShowStoneConnection, func(Checked bool) {
				Theme.ShowStoneConnection = Checked
				GoWin.ApplyThemeVisual(Theme)
			})),
		widget.NewFormItem("Show Illegal Dots",
			MakeCheckboxRow("", Theme.ShowIllegalDot, func(Checked bool) {
				Theme.ShowIllegalDot = Checked
				GoWin.ApplyThemeVisual(Theme)
			})),
		widget.NewFormItem("Show Hover Liberties",
			MakeCheckboxRow("", Theme.ShowHoverLiberties, func(Checked bool) {
				Theme.ShowHoverLiberties = Checked
				GoWin.ApplyThemeVisual(Theme)
			})),
		widget.NewFormItem("Show Child Node Dots",
			MakeCheckboxRow("", Theme.ShowChildNodeDots, func(Checked bool) {
				Theme.ShowChildNodeDots = Checked
				GoWin.ApplyThemeVisual(Theme)
			})),
	}

	// Add HexColor entries in stable order (DefaultConfig key order)
	HexKeyOrder := []string{
		"Goban", "Goban Line", "Play Area",
		"Coord", "Coord Background", "Coord Hover", "Coord Hover Background",
		"1-Liberty Line", "2-Liberty Line", "3-Liberty Line", "4-Liberty Line", "≥5-Liberty Line",
		"Illegal", "Delete", "Last Move", "Child Node", "Territory Stroke", "Annotation", "Neighbor",
	}
	DefaultThemeValue := DefaultConfig.Themes["Default"]
	for _, Key := range HexKeyOrder {
		if _, Exists := DefaultThemeValue.HexColors[Key]; !Exists {
			continue
		}
		if _, Exists := Theme.HexColors[Key]; !Exists {
			Theme.HexColors[Key] = DefaultThemeValue.HexColors[Key]
		}
		FormItems = append(FormItems, widget.NewFormItem(Key,
			MakeColorRow(Key,
				func() string { return Theme.HexColors[Key] },
				func(HexColor string) { Theme.HexColors[Key] = HexColor })))
	}
	for Player := uint8(1); Player <= 0x0f; Player++ {
		PlayerText := fmt.Sprintf("Player %d", Player)
		FormItems = append(FormItems, widget.NewFormItem(PlayerText, MakeColorRow(PlayerText,
			func() string { return Theme.PlayerHexColors[Player] },
			func(HexColor string) { Theme.PlayerHexColors[Player] = HexColor })))
	}

	FormWidget := widget.NewForm(FormItems...)
	Scroll := container.NewVScroll(FormWidget)

	Saved := false
	var Dialog dialog.Dialog
	OkButton := widget.NewButton("OK", func() {
		NewName := NameEntry.Text
		if NewName == "" || NewName == "Default" {
			GoWin.ShowError(fmt.Errorf("invalid theme name %q", NewName))
			return
		}
		if NewName != ThemeName {
			if _, exists := CurrentAppConfig.Themes[NewName]; exists {
				GoWin.ShowError(fmt.Errorf("theme %q already exists", NewName))
				return
			}
			delete(CurrentAppConfig.Themes, ThemeName)
			if CurrentAppConfig.ActiveTheme == ThemeName {
				CurrentAppConfig.ActiveTheme = NewName
			}
		}
		CurrentAppConfig.Themes[NewName] = Theme
		Saved = true
		if Err := GoWin.SaveConfig(); Err != nil {
			GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
		}
		onDone(NewName)
		Dialog.Hide()
	})

	content := container.NewBorder(nil, OkButton, nil, nil, Scroll)
	Dialog = dialog.NewCustom("Modify Theme", "Cancel", content, GoWin.Win)
	ResizeToWindow := func() {
		Dialog.Resize(fyne.NewSize(1, GoWin.Win.Canvas().Size().Height))
	}
	GoWin.ResizeDialog = ResizeToWindow
	Dialog.SetOnClosed(func() {
		GoWin.ResizeDialog = nil
		if !Saved {
			GoWin.ApplyThemeVisual(OriginalTheme)
		}
	})
	Dialog.Show()
	ResizeToWindow()
}

// showCustomColorPicker opens a custom RGBA color picker dialog.
func (GoWin *GoWin) ShowCustomColorPicker(Title string, Initial color.NRGBA, OnPick func(color.NRGBA)) {
	current := Initial
	updating := false

	prevImg, prevCanvas := NewColorPreviewImage(current, 160, 60)
	prevCanvas.SetMinSize(fyne.NewSize(160, 60))

	HexEntry := NewDialogEntry(GoWin)
	HexEntry.SetText(RGBAtoHexColor(Initial))

	sliderR := widget.NewSlider(0, 255)
	sliderG := widget.NewSlider(0, 255)
	sliderB := widget.NewSlider(0, 255)
	sliderA := widget.NewSlider(0, 255)
	sliderR.Value = float64(Initial.R)
	sliderG.Value = float64(Initial.G)
	sliderB.Value = float64(Initial.B)
	sliderA.Value = float64(Initial.A)

	labelR := widget.NewLabel(fmt.Sprintf("%d", Initial.R))
	labelG := widget.NewLabel(fmt.Sprintf("%d", Initial.G))
	labelB := widget.NewLabel(fmt.Sprintf("%d", Initial.B))
	labelA := widget.NewLabel(fmt.Sprintf("%d", Initial.A))

	refreshPreview := func() {
		RenderColorPreview(prevImg, current)
		prevCanvas.Refresh()
	}

	sliderR.OnChanged = func(v float64) {
		if updating {
			return
		}
		updating = true
		current.R = uint8(v)
		labelR.SetText(fmt.Sprintf("%d", current.R))
		HexEntry.SetText(RGBAtoHexColor(current))
		refreshPreview()
		updating = false
	}
	sliderG.OnChanged = func(v float64) {
		if updating {
			return
		}
		updating = true
		current.G = uint8(v)
		labelG.SetText(fmt.Sprintf("%d", current.G))
		HexEntry.SetText(RGBAtoHexColor(current))
		refreshPreview()
		updating = false
	}
	sliderB.OnChanged = func(v float64) {
		if updating {
			return
		}
		updating = true
		current.B = uint8(v)
		labelB.SetText(fmt.Sprintf("%d", current.B))
		HexEntry.SetText(RGBAtoHexColor(current))
		refreshPreview()
		updating = false
	}
	sliderA.OnChanged = func(v float64) {
		if updating {
			return
		}
		updating = true
		current.A = uint8(v)
		labelA.SetText(fmt.Sprintf("%d", current.A))
		HexEntry.SetText(RGBAtoHexColor(current))
		refreshPreview()
		updating = false
	}

	HexEntry.OnChanged = func(s string) {
		if updating {
			return
		}
		c, err := HexColorToNRGBA(s)
		if err != nil {
			return
		}
		updating = true
		current = c
		sliderR.Value = float64(c.R)
		sliderG.Value = float64(c.G)
		sliderB.Value = float64(c.B)
		sliderA.Value = float64(c.A)
		sliderR.Refresh()
		sliderG.Refresh()
		sliderB.Refresh()
		sliderA.Refresh()
		labelR.SetText(fmt.Sprintf("%d", c.R))
		labelG.SetText(fmt.Sprintf("%d", c.G))
		labelB.SetText(fmt.Sprintf("%d", c.B))
		labelA.SetText(fmt.Sprintf("%d", c.A))
		refreshPreview()
		updating = false
	}

	makeSliderRow := func(lbl string, slider *widget.Slider, numLabel *widget.Label) *fyne.Container {
		return container.NewBorder(nil, nil, widget.NewLabel(lbl), numLabel, slider)
	}

	var d dialog.Dialog
	okBtn := widget.NewButton("OK", func() {
		OnPick(current)
		d.Hide()
	})

	pickerContent := container.NewVBox(
		prevCanvas,
		HexEntry,
		makeSliderRow("R", sliderR, labelR),
		makeSliderRow("G", sliderG, labelG),
		makeSliderRow("B", sliderB, labelB),
		makeSliderRow("A", sliderA, labelA),
		okBtn,
	)

	d = dialog.NewCustom(Title, "Cancel", pickerContent, GoWin.Win)
	d.Show()
}
