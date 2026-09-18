package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// main is the application entry point. It parses command-line flags, creates
// a Fyne application, and opens one GoWin per loadable file argument (.Sgf
// or any .CGG.json* suffix from CggAllSuffixes), or a blank window if none
// are given.
func main() {
	ConfigDir = *flag.String("ConfigDir", ".",
		"The directory in which to read and write the config file")
	flag.Parse()
	filesToLoad := flag.Args()

	fyneApp := app.NewWithID("Nazgand.ConnectedGroupsGoban")

	filesLoaded := 0
	for _, fileToLoad := range filesToLoad {
		if !IsSupportedGamePath(fileToLoad) {
			fmt.Println("Ignoring file `" + fileToLoad + "`: Needs `.Sgf` or `.CGG.json*` extension")
			continue
		}
		newWindow := fyneApp.NewWindow("Connected Groups Goban Version " + Version)
		app := &GoWin{Win: newWindow, OpenedFilePath: fileToLoad}
		app.Init(false)
		filesLoaded++
	}
	if filesLoaded == 0 {
		newWindow := fyneApp.NewWindow("Connected Groups Goban Version " + Version)
		goWin := &GoWin{Win: newWindow}
		goWin.Init(false)
	}
	fyneApp.Run()
}

// IsSupportedGamePath reports whether Path ends with a supported game-file
// extension (.Sgf, or any suffix in CggAllSuffixes). Case-insensitive.
func IsSupportedGamePath(Path string) bool {
	if strings.EqualFold(filepath.Ext(Path), ".Sgf") {
		return true
	}
	Lower := strings.ToLower(Path)
	for _, Suffix := range CggAllSuffixes {
		if strings.HasSuffix(Lower, strings.ToLower(Suffix)) {
			return true
		}
	}
	return false
}

// RefreshSetVertexMenu keeps the player choices in sync with the current game.
func (GoWin *GoWin) RefreshSetVertexMenu() {
	Item := GoWin.SetVertexMenuItem
	if Item == nil {
		return
	}
	GoWin.SetVertexMenuGameH = GoWin.Coll.CurrentGameH
	Players := 0
	Item.Disabled = GoWin.Coll.CurrentGameH == nil
	if !Item.Disabled {
		Players = int(GoWin.Coll.CurrentGameH.Players)
	}
	if int(GoWin.SetVertexPlayer) > Players {
		GoWin.SetVertexPlayer = 0
	}
	Item.ChildMenu.Items = nil
	for Player := 0; Player <= Players; Player++ {
		Value := uint8(Player)
		Item.ChildMenu.Items = append(Item.ChildMenu.Items,
			fyne.NewMenuItem(VertexOptionLabel(Value), func() {
				GoWin.SetVertexPlayer = Value
				if GoWin.MouseMode == "Set Vertex" {
					GoWin.DrawBoard()
					GoWin.DrawHover(GoWin.HoverCoords)
				} else {
					GoWin.SetMouseMode("Set Vertex")
				}
			}))
	}
	if GoWin.MainMenu != nil {
		GoWin.Win.SetMainMenu(GoWin.MainMenu)
	}
}

// Init initializes the GoWin: loads config, sets up colors, creates UI
// widgets (comment box, score container, mouse-mode controls, board layers,
// game tree, menus), registers keyboard shortcuts, and shows the window.
func (GoWin *GoWin) Init(StartWithEmptyCollection bool) {
	GoWin.Win.SetOnClosed(GoWin.DetachAllEngines)
	GoWin.MouseMode = "Play"

	Err := LoadConfig()
	if Err != nil {
		fmt.Println("Unable to load config:", Err)
		CurrentAppConfig = DefaultConfig
		ValidateAndFillConfig()
	}
	SnapshotCurrentConfigBytes()
	GoWin.InitColors(GetActiveTheme())

	GoWin.Coll = NewCollection()

	GoWin.Win.Canvas().SetOnTypedKey(GoWin.HandleKeyEvent)
	GoWin.Win.Canvas().AddShortcut(CtrlN,
		func(shortcut fyne.Shortcut) { GoWin.HandleNewGame() })
	GoWin.Win.Canvas().AddShortcut(CtrlS,
		func(shortcut fyne.Shortcut) { GoWin.HandleExportCgg() })
	GoWin.Win.Canvas().AddShortcut(CtrlO,
		func(shortcut fyne.Shortcut) { GoWin.HandleImportCgg() })
	GoWin.Win.Canvas().AddShortcut(CtrlShiftO,
		func(shortcut fyne.Shortcut) { GoWin.HandleImportSgf() })
	GoWin.Win.Canvas().AddShortcut(CtrlShiftS,
		func(shortcut fyne.Shortcut) { GoWin.HandleExportSgf() })
	GoWin.Win.Canvas().AddShortcut(CtrlE,
		func(shortcut fyne.Shortcut) { GoWin.ShowEngineSettings() })
	GoWin.Win.Canvas().AddShortcut(CtrlQ,
		func(Shortcut fyne.Shortcut) { GoWin.DetachAllEngines() })
	GoWin.Win.Canvas().AddShortcut(CtrlG,
		func(shortcut fyne.Shortcut) { GoWin.ShowGoToMoveDialog() })
	GoWin.Win.Canvas().AddShortcut(CtrlD,
		func(shortcut fyne.Shortcut) { ShowDiplomacyDialog(GoWin) })

	// Create status bar label
	GoWin.StatusLabel = NewStatusBar()

	GoWin.SetVertexPlayer = 0
	GoWin.SetAnnotationMask = CircleMask

	// ScoreContainer is built dynamically by updateScoreTable; create an empty stack for now
	GoWin.ScoreContainer = container.NewStack()
	GoWin.ScoreContainer.Hide()

	// Create comment entry with placeholder
	GoWin.CommentEntry = widget.NewMultiLineEntry()
	GoWin.CommentEntry.SetPlaceHolder("Current move\ncomment")

	// Attach a listener to update the current node's comment when the textbox changes
	GoWin.CommentEntry.OnChanged = func(content string) {
		if GoWin.CurrentNode() != nil {
			GoWin.CurrentNode().Comment = content
			GoWin.RefreshGameTreeNodeLabel(GoWin.CurrentNode())
		}
	}

	// Create game selector dropdown
	GoWin.GameSelectorSelect = widget.NewSelect([]string{}, func(selected string) {
		// Find the game by display name and navigate to its CurrentNode
		for Index, Child := range GoWin.Coll.CollectionNode.Children {
			if Child.Board != nil && Child.Board.Hist != nil {
				displayName := GameDisplayName(Child.Board.Hist, Index)
				if displayName == selected {
					GoWin.OnUserNavigate()
					GoWin.SetCurrentNode(Child.Board.Hist.CurrentNode)
					break
				}
			}
		}
	})
	GoWin.GameSelectorSelect.PlaceHolder = "Select game..."

	// Create board canvas and related containers
	GoWin.PlayArea = canvas.NewRectangle(Colors["Play Area"])
	GoWin.Layers = make(map[string]*fyne.Container, len(LayerOrder))
	for _, LayerKey := range LayerOrder {
		GoWin.Layers[LayerKey] = container.NewWithoutLayout()
	}
	InputLayerWidget := NewInputLayer(GoWin)
	StackObjects := []fyne.CanvasObject{GoWin.PlayArea}
	for _, LayerKey := range LayerOrder {
		StackObjects = append(StackObjects, GoWin.Layers[LayerKey])
	}
	StackObjects = append(StackObjects, InputLayerWidget)
	GoWin.PlayContainer = container.NewStack(StackObjects...)

	// Initialize the game tree and the board
	GoWin.GameTreeContainer = container.NewScroll(nil)
	GoWin.GameTreeNodeToButton = map[*GameTreeNode]*TreeNodeButton{}
	GoWin.GameTreeNodeToContainer = map[*GameTreeNode]*fyne.Container{}
	GoWin.UpdateGameTreeUI()

	// Define the "File" menu
	FileMenu := fyne.NewMenu("File",
		&fyne.MenuItem{Label: "Save Game", Action: GoWin.HandleExportCgg, Shortcut: CtrlS},
		&fyne.MenuItem{Label: "Load Game", Action: GoWin.HandleImportCgg, Shortcut: CtrlO},
		fyne.NewMenuItem("Export board as SVG", func() {
			GoWin.HandleExportImage()
		}),
		&fyne.MenuItem{Label: "Export Sgf", Action: GoWin.HandleExportSgf, Shortcut: CtrlShiftS},
		&fyne.MenuItem{Label: "Import Sgf", Action: GoWin.HandleImportSgf, Shortcut: CtrlShiftO},
	)

	// Define the "Game" menu
	GoWin.PassItem = &fyne.MenuItem{Label: "Pass", Action: GoWin.HandlePass, Shortcut: PassShortcut}
	GoWin.DiplomacyItem = &fyne.MenuItem{Label: "Diplomacy", Action: func() { ShowDiplomacyDialog(GoWin) }, Shortcut: CtrlD}
	GameMenu := fyne.NewMenu("Game",
		&fyne.MenuItem{Label: "New Game", Action: GoWin.HandleNewGame, Shortcut: CtrlN},
		fyne.NewMenuItem("Game Information", func() {
			GoWin.ShowGameInformation()
		}),
		&fyne.MenuItem{Label: "Go To Move #", Action: GoWin.ShowGoToMoveDialog, Shortcut: CtrlG},
		fyne.NewMenuItem("Search for move by text in comment", func() {
			GoWin.ShowSearchCommentDialog()
		}),
		fyne.NewMenuItem("Search Collection", func() {
			GoWin.ShowCollectionDialog()
		}),
		GoWin.PassItem,
		GoWin.DiplomacyItem,
		&fyne.MenuItem{Label: "Delete Node", Shortcut: DeleteNodeShortcut, Action: func() {
			GoWin.OnUserNavigate()
			GoWin.DeleteCurrentNode()
		}},
	)

	// Define the "App" menu
	AppMenu := fyne.NewMenu("App",
		fyne.NewMenuItem("Theme", func() { GoWin.ShowThemeDialog() }),
		fyne.NewMenuItem("Rule Sets", func() { GoWin.ShowRuleSetDialog() }),
	)

	// Symmetry submenu: per-window view transform; lives at the top of the
	// Window menu since it's a windowing concern (each window has its own
	// SymmetryIndex and its own SymmetryItems slice).
	GoWin.SymmetryItems = make([]*fyne.MenuItem, len(SymmetryNames))
	for Index, Name := range SymmetryNames {
		SymmetryIndex := uint8(Index)
		GoWin.SymmetryItems[Index] = fyne.NewMenuItem(Name, func() {
			GoWin.SymmetryIndex = SymmetryIndex
			GoWin.RefreshSymmetryMenuItems()
			GoWin.DrawBoard()
		})
	}
	SymmetrySubMenu := fyne.NewMenuItem("Symmetry", nil)
	SymmetrySubMenu.ChildMenu = fyne.NewMenu("", GoWin.SymmetryItems...)
	GoWin.RefreshSymmetryMenuItems()

	// Define the "Mouse Mode" menu
	MouseModeMenuItems := []*fyne.MenuItem{}
	for _, MouseMode := range MouseModes {
		Item := fyne.NewMenuItem(MouseMode, nil)
		switch MouseMode {
		case "Set Vertex":
			GoWin.SetVertexMenuItem = Item
			Item.ChildMenu = fyne.NewMenu("")
		case "Toggle Annotation":
			Item.ChildMenu = fyne.NewMenu("")
			for _, Mask := range []uint8{CircleMask, SquareMask, TriangleMask, XMask} {
				Item.ChildMenu.Items = append(Item.ChildMenu.Items,
					fyne.NewMenuItem(AnnotationOptionLabel(Mask), func() {
						GoWin.SetAnnotationMask = Mask
						GoWin.SetMouseMode("Toggle Annotation")
					}))
			}
		default:
			Item.Action = func() { GoWin.SetMouseMode(MouseMode) }
		}
		MouseModeMenuItems = append(MouseModeMenuItems, Item)
	}
	GoWin.RefreshSetVertexMenu()
	MouseModeMenu := fyne.NewMenu("Mouse Mode", MouseModeMenuItems...)

	// Define the "Engine" menu
	GoWin.PlayerEngineItems = []*fyne.MenuItem{
		fyne.NewMenuItem("Player 1", nil),
		fyne.NewMenuItem("Player 2", nil),
	}
	EngineMenu := fyne.NewMenu("Engine",
		&fyne.MenuItem{Label: "Settings", Action: GoWin.ShowEngineSettings, Shortcut: CtrlE},
		GoWin.PlayerEngineItems[0],
		GoWin.PlayerEngineItems[1],
		&fyne.MenuItem{Label: "Detach All Engines", Action: GoWin.DetachAllEngines, Shortcut: CtrlQ},
	)
	GoWin.RefreshEngineMenuItems()

	WindowMenu := fyne.NewMenu("Window",
		SymmetrySubMenu,
		fyne.NewMenuItem("Duplicate Window", func() { GoWin.DuplicateWindow() }),
		fyne.NewMenuItem("Add Window", func() { GoWin.AddWindow() }),
		fyne.NewMenuItem("Close Window", func() { GoWin.CloseWindow() }),
	)

	GoWin.MainMenu = fyne.NewMainMenu(
		FileMenu,
		GameMenu,
		AppMenu,
		MouseModeMenu,
		EngineMenu,
		WindowMenu,
	)
	GoWin.Win.SetMainMenu(GoWin.MainMenu)

	// Layout for Controls
	TopControls := container.NewBorder(
		GoWin.ScoreContainer,
		GoWin.GameSelectorSelect,
		nil,
		nil,
		GoWin.CommentEntry,
	)
	GoWin.VSplit = container.NewVSplit(
		NewVSplitChildWrapper(TopControls, GoWin),
		container.New(&GameTreeLayout{Win: GoWin}, GoWin.GameTreeContainer),
	)
	RestoredVSplitOffset := CurrentAppConfig.VSplitOffset
	if RestoredVSplitOffset <= 0 {
		RestoredVSplitOffset = 0
	}
	GoWin.VSplit.SetOffset(RestoredVSplitOffset)

	// Main layout with split view
	MainContent := container.NewHSplit(
		GoWin.VSplit,
		GoWin.PlayContainer,
	)
	MainContent.SetOffset(0)
	GoWin.HSplit = MainContent
	Content := NewContentWrapper(
		container.NewBorder(nil, GoWin.StatusLabel.Container, nil, nil, MainContent),
		GoWin,
	)

	if GoWin.OpenedFilePath == "" {
		if !StartWithEmptyCollection {
			GoWin.AddFreshBoard()
		}
	} else if strings.EqualFold(filepath.Ext(GoWin.OpenedFilePath), ".Sgf") {
		GoWin.ImportSgfFile()
	} else {
		GoWin.ImportCggFile(GoWin.OpenedFilePath)
	}
	GoWin.RefreshEngineMenuItems()
	GoWin.Win.SetContent(Content)
	if CurrentAppConfig.WindowWidth > 0 && CurrentAppConfig.WindowHeight > 0 {
		GoWin.Win.Resize(fyne.NewSize(CurrentAppConfig.WindowWidth, CurrentAppConfig.WindowHeight))
	} else {
		GoWin.Win.Resize(fyne.NewSize(800, 600))
	}
	GoWin.Win.Show()

	// Start the status bar update loop
	GoWin.UpdateStatusBarLoop()
}

// UpdateStatusBar updates the status bar with current mouse mode, timing info, and hover group details
func (GoWin *GoWin) UpdateStatusBar() {
	CurrentNode := GoWin.CurrentNode()
	if CurrentNode == nil {
		return
	}

	StatusBuilder := ""
	StatusBuilder += "Mouse mode: " + GoWin.MouseMode

	// Add timing info if not root/collection node
	if CurrentNode.LastMove != RootCoords && CurrentNode.LastMove != CollectionCoords && CurrentNode.Board != nil {
		// Time between current and parent node creation
		if CurrentNode.Parent != nil && CurrentNode.UnixMilli != 0 && CurrentNode.Parent.UnixMilli != 0 {
			TimeSinceParent := float64(CurrentNode.UnixMilli-CurrentNode.Parent.UnixMilli) / 1000.0
			StatusBuilder += fmt.Sprintf(" | Last move: %.3fs", TimeSinceParent)
		}
	}

	if GoWin.MouseMode != "Score" {
		// Time since current node was created
		if CurrentNode.UnixMilli != 0 {
			CurrentTime := time.Now().UnixMilli()
			TimeSinceCreation := float64(CurrentTime-CurrentNode.UnixMilli) / 1000.0
			StatusBuilder += fmt.Sprintf(" | Since previous move: %.3fs", TimeSinceCreation)
		}

		// Add hover group info if exists and mouse is on board
		if GoWin.HoverGroup != nil && GoWin.HoverCoords != ErrorCoords {
			StatusBuilder += fmt.Sprintf(" | Hover group: %d stones, %d liberties",
				len(GoWin.HoverGroup.Vertices[GoWin.HoverGroup.Owner]), len(GoWin.HoverGroup.Vertices[0]))
		}
	}

	fyne.Do(func() {
		GoWin.StatusLabel.SetText(StatusBuilder)
	})
}

// UpdateStatusBarLoop runs UpdateStatusBar and schedules itself to run again after ~139ms.
func (GoWin *GoWin) UpdateStatusBarLoop() {
	GoWin.UpdateStatusBar()
	time.AfterFunc(139*time.Millisecond, func() {
		GoWin.UpdateStatusBarLoop()
	})
}

// DeleteCurrentNode removes the current node from the game tree and
// selects its parent. Does nothing if the current node is the collection root.
func (GoWin *GoWin) DeleteCurrentNode() {
	if GoWin.CurrentNode() == GoWin.Coll.CollectionNode {
		return
	}
	NodeToDelete := GoWin.CurrentNode()
	delete(GoWin.GameTreeNodeToButton, NodeToDelete)
	delete(GoWin.GameTreeNodeToContainer, NodeToDelete)
	Parent := NodeToDelete.Parent
	if Parent != nil {
		for Index, Child := range Parent.Children {
			if Child == NodeToDelete {
				Parent.Children = append(Parent.Children[:Index], Parent.Children[Index+1:]...)
				break
			}
		}
	}
	GoWin.SetCurrentNode(Parent)
	Parent.FavoriteChild = nil
}
