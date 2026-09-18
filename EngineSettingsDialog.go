package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// ShowEngineSettings opens the engine-settings dialog where the user can
// add, rename, update, or delete GTP engine definitions and toggle GTP logging.
func (GoWin *GoWin) ShowEngineSettings() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	// Working copy of engine configs.
	Engines := map[string]EngineConfig{}
	for N, EC := range CurrentAppConfig.Engines {
		Engines[N] = EC
	}
	SelectedName := ""

	// Editor fields.
	NameEntry := NewDialogEntry(GoWin)
	NameEntry.SetPlaceHolder("Engine name")
	PathEntry := NewDialogEntry(GoWin)
	PathEntry.SetPlaceHolder("Path to executable")
	ArgsEntry := NewDialogEntry(GoWin)
	ArgsEntry.SetPlaceHolder("Arguments")

	// Sorted engine names helper.
	SortedEngineNames := func() []string {
		Names := make([]string, 0, len(Engines))
		for N := range Engines {
			Names = append(Names, N)
		}
		sort.Strings(Names)
		return Names
	}

	EngineSelect := widget.NewSelect(SortedEngineNames(), nil)
	EngineSelect.PlaceHolder = "Select engine…"

	UpdateBtn := widget.NewButton("Update Engine", nil)
	DeleteBtn := widget.NewButton("Delete Engine", nil)

	RefreshEditorState := func() {
		HasSelection := SelectedName != ""
		if HasSelection {
			UpdateBtn.Enable()
			DeleteBtn.Enable()
			NameEntry.Enable()
			PathEntry.Enable()
			ArgsEntry.Enable()
		} else {
			UpdateBtn.Disable()
			DeleteBtn.Disable()
		}
	}

	SelectEngine := func(Name string) {
		SelectedName = Name
		if EC, Ok := Engines[Name]; Ok {
			NameEntry.SetText(Name)
			PathEntry.SetText(EC.GtpPath)
			ArgsEntry.SetText(EC.GtpArgs)
		} else {
			NameEntry.SetText("")
			PathEntry.SetText("")
			ArgsEntry.SetText("")
		}
		RefreshEditorState()
	}

	EngineSelect.OnChanged = SelectEngine

	// Select first engine by default.
	if Names := SortedEngineNames(); len(Names) > 0 {
		EngineSelect.SetSelected(Names[0])
		SelectEngine(Names[0])
	} else {
		RefreshEditorState()
	}

	RefreshEngineSelect := func() {
		Names := SortedEngineNames()
		EngineSelect.Options = Names
		if SelectedName != "" {
			EngineSelect.SetSelected(SelectedName)
		} else if len(Names) > 0 {
			EngineSelect.SetSelected(Names[0])
			SelectEngine(Names[0])
		} else {
			EngineSelect.ClearSelected()
			SelectEngine("")
		}
		EngineSelect.Refresh()
	}

	// [Save Engine] — adds a new engine; errors if name already exists.
	SaveBtn := widget.NewButton("Save Engine", func() {
		Name := NameEntry.Text
		if Name == "" {
			GoWin.ShowError(fmt.Errorf("engine name must not be blank"))
			return
		}
		if Name == "Human" {
			GoWin.ShowError(fmt.Errorf("engine name %q is reserved", Name))
			return
		}
		if _, Exists := Engines[Name]; Exists {
			GoWin.ShowError(fmt.Errorf("engine %q already exists; use Update Engine to overwrite", Name))
			return
		}
		Engines[Name] = EngineConfig{GtpPath: PathEntry.Text, GtpArgs: ArgsEntry.Text}
		SelectedName = Name
		RefreshEngineSelect()
	})

	// [Update Engine] — saves edits; handles rename.
	UpdateBtn.OnTapped = func() {
		NewName := NameEntry.Text
		if NewName == "" {
			GoWin.ShowError(fmt.Errorf("engine name must not be blank"))
			return
		}
		if NewName == "Human" {
			GoWin.ShowError(fmt.Errorf("engine name %q is reserved", NewName))
			return
		}
		if NewName != SelectedName {
			if _, Exists := Engines[NewName]; Exists {
				GoWin.ShowError(fmt.Errorf("engine %q already exists", NewName))
				return
			}
			delete(Engines, SelectedName)
		}
		Engines[NewName] = EngineConfig{GtpPath: PathEntry.Text, GtpArgs: ArgsEntry.Text}
		SelectedName = NewName
		RefreshEngineSelect()
	}

	// [Delete Engine] — confirms then removes.
	DeleteBtn.OnTapped = func() {
		Name := SelectedName
		dialog.ShowConfirm("Delete Engine",
			fmt.Sprintf("Delete engine %q?", Name),
			func(Confirmed bool) {
				if !Confirmed {
					return
				}
				delete(Engines, Name)
				SelectedName = ""
				RefreshEngineSelect()
			}, GoWin.Win)
	}

	BrowseBtn := widget.NewButton("Browse", func() {
		FileDialog := dialog.NewFileOpen(func(Reader fyne.URIReadCloser, Err error) {
			if Err != nil || Reader == nil {
				return
			}
			defer Reader.Close()
			PathEntry.SetText(Reader.URI().Path())
		}, GoWin.Win)
		if PathEntry.Text != "" {
			Dir := filepath.Dir(PathEntry.Text)
			URI := storage.NewFileURI(Dir)
			if ListableURI, Err := storage.ListerForURI(URI); Err == nil {
				FileDialog.SetLocation(ListableURI)
			}
		}
		FileDialog.Show()
	})

	GtpLoggingCheck := widget.NewCheck("Log GTP commands and responses to EngineName.Player.GtpLog files", nil)
	GtpLoggingCheck.SetChecked(CurrentAppConfig.GtpLogging)

	Content := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Engine:"), nil, EngineSelect),
		container.NewHBox(SaveBtn, UpdateBtn, DeleteBtn),
		container.NewBorder(nil, nil, widget.NewLabel("Name:"), nil, NameEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Path:"), BrowseBtn, PathEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Args:"), nil, ArgsEntry),
		widget.NewSeparator(),
		GtpLoggingCheck,
	)

	Dlg := dialog.NewCustomConfirm("Engine Settings", "OK", "Cancel",
		Content,
		func(Ok bool) {
			GoWin.DialogClosed()
			if !Ok {
				return
			}
			// Belt-and-suspenders validation.
			Seen := map[string]bool{}
			for N := range Engines {
				if N == "" {
					GoWin.ShowError(fmt.Errorf("engine names must not be blank"))
					return
				}
				if N == "Human" {
					GoWin.ShowError(fmt.Errorf("engine name %q is reserved", N))
					return
				}
				if Seen[N] {
					GoWin.ShowError(fmt.Errorf("duplicate engine name %q", N))
					return
				}
				Seen[N] = true
			}

			for Player, Eng := range GoWin.GtpEngines {
				if Config, Exists := Engines[Eng.Name]; !Exists || Config != CurrentAppConfig.Engines[Eng.Name] {
					GoWin.DetachEngineForPlayer(Player)
				}
			}
			CurrentAppConfig.Engines = Engines
			GoWin.RefreshEngineMenuItems()

			CurrentAppConfig.GtpLogging = GtpLoggingCheck.Checked

			if Err := GoWin.SaveConfig(); Err != nil {
				GoWin.ShowError(fmt.Errorf("failed to save config: %v", Err))
			}
		},
		GoWin.Win)
	GoWin.DismissDialog = func() { Dlg.Hide() }
	GoWin.SubmitDialog = func() { Dlg.Confirm() }
	GoWin.WireSubmitOnEnter(NameEntry, PathEntry, ArgsEntry)
	Dlg.Resize(fyne.NewSize(GoWin.Win.Canvas().Size().Width*0.6, Dlg.MinSize().Height))
	Dlg.Show()
}
