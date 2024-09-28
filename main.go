package main

import (
	"bufio"
	"fmt"
	"image/color"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	empty             = "."
	black             = "B"
	white             = "W"
	gridLineThickness = 0.15
	version           = "2"
)

var (
	gobanColor            = color.RGBA{108, 84, 60, 255}
	lineColor             = color.RGBA{93, 74, 51, 255}
	blackColor            = color.Black
	whiteColor            = color.White
	blackScoreColor       = color.RGBA{0, 0, 255, 255}
	whiteScoreColor       = color.RGBA{0, 255, 0, 255}
	transparentWhiteColor = color.NRGBA{255, 255, 255, 128}
	transparentBlackColor = color.NRGBA{0, 0, 0, 128}
	redColor              = color.RGBA{255, 0, 0, 255}
)

type Game struct {
	sizeX             int
	sizeY             int
	boardCanvas       *fyne.Container
	gridContainer     *fyne.Container
	hoverStone        *canvas.Circle
	window            fyne.Window
	cellSize          float32
	currentNode       *GameTreeNode
	rootNode          *GameTreeNode
	nodeMap           map[string]*GameTreeNode
	idCounter         int
	gameTreeContainer *container.Scroll
	mouseMode         string
	territoryMap      [][]string
	territoryLayer    *fyne.Container
	scoringStatus     *widget.Label
	commentEntry      *widget.Entry
	komi              float64
	gtpPath           string
	gtpArgs           string
	gtpColor          string
	gtpCmd            *exec.Cmd
	gtpIn             io.WriteCloser
	gtpOut            io.ReadCloser
	gtpReader         *bufio.Reader
}

func (g *Game) newGameTreeNode() *GameTreeNode {
	g.idCounter++

	newNode := &GameTreeNode{
		boardState:       makeEmptyBoard(g.sizeX, g.sizeY),
		id:               fmt.Sprintf("%d", g.idCounter),
		koX:              -1,
		koY:              -1,
		addedBlackStones: make([][]bool, g.sizeY),
		addedWhiteStones: make([][]bool, g.sizeY),
		AE:               make([][]bool, g.sizeY),
		CR:               make([][]bool, g.sizeY),
		SQ:               make([][]bool, g.sizeY),
		TR:               make([][]bool, g.sizeY),
		MA:               make([][]bool, g.sizeY),
		LB:               make([][]string, g.sizeY),
	}

	for y := 0; y < g.sizeY; y++ {
		newNode.addedBlackStones[y] = make([]bool, g.sizeX)
		newNode.addedWhiteStones[y] = make([]bool, g.sizeX)
		newNode.AE[y] = make([]bool, g.sizeX)
		newNode.CR[y] = make([]bool, g.sizeX)
		newNode.SQ[y] = make([]bool, g.sizeX)
		newNode.TR[y] = make([]bool, g.sizeX)
		newNode.MA[y] = make([]bool, g.sizeX)
		newNode.LB[y] = make([]string, g.sizeX)
	}

	g.nodeMap[newNode.id] = newNode
	return newNode
}

type GameTreeNode struct {
	boardState       [][]string      // Current state of the board at this node
	move             [2]int          // Coordinates of the move ([x, y]); (-1, -1) represents a pass
	player           string          // Player who made the move ("B" for Black, "W" for White)
	children         []*GameTreeNode // Child nodes representing subsequent moves
	parent           *GameTreeNode   // Parent node in the game tree
	id               string          // Unique identifier for the node
	koX              int             // X-coordinate for ko rule; -1 if not applicable
	koY              int             // Y-coordinate for ko rule; -1 if not applicable
	Comment          string          // Optional comment for the move
	addedBlackStones [][]bool        // Coordinates of additional Black stones (AB properties)
	addedWhiteStones [][]bool        // Coordinates of additional White stones (AW properties)
	AE               [][]bool        // Coordinates of points made empty (AE properties)
	CR               [][]bool        // Coordinates for circle annotations
	SQ               [][]bool        // Coordinates for square annotations
	TR               [][]bool        // Coordinates for triangle annotations
	MA               [][]bool        // Coordinates for mark (X) annotations
	LB               [][]string      // Labels for specific points on the board
}

func (gtn *GameTreeNode) addBlackStone(x, y int) {
	if x >= 0 && x < len(gtn.addedBlackStones[0]) && y >= 0 && y < len(gtn.addedBlackStones) {
		gtn.addedBlackStones[y][x] = true
	}
}

func (gtn *GameTreeNode) hasAddedBlackStones() bool {
	for _, arr := range gtn.addedBlackStones {
		for _, el := range arr {
			if el {
				return true
			}
		}
	}
	return false
}

func (gtn *GameTreeNode) addWhiteStone(x, y int) {
	if x >= 0 && x < len(gtn.addedWhiteStones[0]) && y >= 0 && y < len(gtn.addedWhiteStones) {
		gtn.addedWhiteStones[y][x] = true
	}
}

func (gtn *GameTreeNode) hasAddedWhiteStones() bool {
	for _, arr := range gtn.addedWhiteStones {
		for _, el := range arr {
			if el {
				return true
			}
		}
	}
	return false
}

type ResizingContainer struct {
	widget.BaseWidget
	content     fyne.CanvasObject
	placeholder fyne.CanvasObject
	resizeTimer *time.Timer
	mutex       sync.Mutex
}

func NewResizingContainer(content fyne.CanvasObject, placeholder fyne.CanvasObject) *ResizingContainer {
	rc := &ResizingContainer{
		content:     content,
		placeholder: placeholder,
	}
	rc.ExtendBaseWidget(rc)
	rc.placeholder.Hide() // Hide the placeholder initially
	return rc
}

func (rc *ResizingContainer) CreateRenderer() fyne.WidgetRenderer {
	return &resizingContainerRenderer{
		container: rc,
	}
}

func (rc *ResizingContainer) Resize(size fyne.Size) {
	// Check if the size is different before proceeding
	if rc.Size() == size {
		return // Skip handling if the size has not changed
	}
	rc.BaseWidget.Resize(size)
	rc.handleResize()
}

func (rc *ResizingContainer) handleResize() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if rc.resizeTimer != nil {
		rc.resizeTimer.Stop()
	}

	rc.content.Hide()
	rc.placeholder.Show()
	rc.Refresh()

	rc.resizeTimer = time.AfterFunc(39*time.Millisecond, func() {
		rc.mutex.Lock()
		defer rc.mutex.Unlock()
		rc.placeholder.Hide()
		rc.content.Show()
		rc.Refresh()
	})
}

type resizingContainerRenderer struct {
	container *ResizingContainer
}

func (r *resizingContainerRenderer) Layout(size fyne.Size) {
	if r.container.content.Visible() {
		r.container.content.Resize(size)
	}
	if r.container.placeholder.Visible() {
		r.container.placeholder.Resize(size)
	}
}

func (r *resizingContainerRenderer) MinSize() fyne.Size {
	minSize := fyne.NewSize(0, 0)
	if r.container.content.Visible() {
		minSize = minSize.Max(r.container.content.MinSize())
	}
	if r.container.placeholder.Visible() {
		minSize = minSize.Max(r.container.placeholder.MinSize())
	}
	return minSize
}

func (r *resizingContainerRenderer) Refresh() {
	canvas.Refresh(r.container)
}

func (r *resizingContainerRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

func (r *resizingContainerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.container.content, r.container.placeholder}
}

func (r *resizingContainerRenderer) Destroy() {}

func main() {
	a := app.NewWithID("com.nazgand.connectedgroupsgoban")
	w := a.NewWindow("Connected Groups Goban Version " + version)
	game := &Game{
		window:    w,
		mouseMode: "play",
		nodeMap:   make(map[string]*GameTreeNode),
		komi:      7.0,
		gtpPath:   "/usr/games/gnugo",
		gtpArgs:   "--mode gtp --level 15 --large-scale --cache-size 93 --chinese-rules --komi 7",
		gtpColor:  "B",
	}

	// Create scoring status label
	game.scoringStatus = widget.NewLabel("Not in scoring mode.")

	// Create comment entry with placeholder
	game.commentEntry = widget.NewMultiLineEntry()
	game.commentEntry.SetPlaceHolder("Current move comment")

	// Attach a listener to update the current node's comment when the textbox changes
	game.commentEntry.OnChanged = func(content string) {
		if game.currentNode != nil {
			game.currentNode.Comment = content
		}
	}

	// Create board canvas and related containers
	background := canvas.NewRectangle(gobanColor)
	inputLayer := newInputLayer(game)
	game.gridContainer = container.NewWithoutLayout()

	game.boardCanvas = container.NewStack(
		background,
		game.gridContainer,
		inputLayer,
	)

	game.sizeX = 19
	game.sizeY = 19

	// Initialize the board here
	game.initializeBoard()
	game.redrawBoard()

	// Initialize the game tree
	game.gameTreeContainer = container.NewScroll(nil)
	game.updateGameTreeUI()

	// Define the "File" menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Import SGF", func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				defer reader.Close()
				sgfContent, err := io.ReadAll(reader)
				if err != nil {
					dialog.ShowError(err, game.window)
					return
				}
				err = game.importFromSGF(string(sgfContent))
				if err != nil {
					dialog.ShowError(err, game.window)
					return
				}
				game.gameTreeContainer.ScrollToBottom()
			}, game.window)
		}),
		fyne.NewMenuItem("Export SGF", func() {
			dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				defer writer.Close()
				sgfContent, err := game.exportToSGF()
				if err != nil {
					dialog.ShowError(err, game.window)
					return
				}
				_, err = writer.Write([]byte(sgfContent))
				if err != nil {
					dialog.ShowError(err, game.window)
					return
				}
			}, game.window)
		}),
	)

	gameMenu := fyne.NewMenu("Game",
		fyne.NewMenuItem("Fresh Board", func() {
			// Define the input entries outside the dialog
			widthEntry := widget.NewEntry()
			widthEntry.SetPlaceHolder("(1-52)")
			widthEntry.SetText(strconv.Itoa(game.sizeX))
			heightEntry := widget.NewEntry()
			heightEntry.SetPlaceHolder("(1-52)")
			heightEntry.SetText(strconv.Itoa(game.sizeY))

			// Create form items
			formItems := []*widget.FormItem{
				widget.NewFormItem("Width", widthEntry),
				widget.NewFormItem("Height", heightEntry),
			}

			// Create a custom dialog to input board width and height
			boardSizeDialog := dialog.NewForm(
				"Fresh Board",
				"OK",
				"Cancel",
				formItems,
				func(ok bool) {
					if !ok {
						return
					}
					widthStr := widthEntry.Text
					heightStr := heightEntry.Text
					x, errX := strconv.Atoi(widthStr)
					y, errY := strconv.Atoi(heightStr)
					if errX != nil || errY != nil || x < 1 || y < 1 || x > 52 || y > 52 {
						dialog.ShowError(fmt.Errorf("invalid board size (must be between 1 and 52)"), game.window)
						return
					}
					game.sizeX = x
					game.sizeY = y
					game.initializeBoard()
					game.redrawBoard()
					game.updateGameTreeUI() // Refresh the game tree UI
				},
				game.window,
			)

			// Optionally, handle dialog close if needed
			boardSizeDialog.SetOnClosed(func() {
				// You can perform additional actions here when the dialog is closed
			})

			// Show the dialog
			boardSizeDialog.Show()
		}),
		fyne.NewMenuItem("Pass", func() {
			game.handlePass()
		}),
		fyne.NewMenuItem("Set Komi", func() {
			game.showSetKomiDialog()
		}),
	)

	// Define the "MouseMode" menu
	mouseModeMenu := fyne.NewMenu("MouseMode",
		fyne.NewMenuItem("Play", func() { game.setMouseMode("play") }),
		fyne.NewMenuItem("Score", func() { game.setMouseMode("score") }),
		fyne.NewMenuItem("Set Label", func() { game.setMouseMode("label") }),
		fyne.NewMenuItem("Add Black", func() { game.setMouseMode("addBlack") }),
		fyne.NewMenuItem("Add White", func() { game.setMouseMode("addWhite") }),
		fyne.NewMenuItem("Add Empty", func() { game.setMouseMode("addEmpty") }),
		fyne.NewMenuItem("Toggle Circle", func() { game.setMouseMode("circle") }),
		fyne.NewMenuItem("Toggle Square", func() { game.setMouseMode("square") }),
		fyne.NewMenuItem("Toggle Triangle", func() { game.setMouseMode("triangle") }),
		fyne.NewMenuItem("Toggle X Mark", func() { game.setMouseMode("xMark") }),
	)

	// Define the "Engine" menu
	engineMenu := fyne.NewMenu("Engine",
		fyne.NewMenuItem("Settings", func() {
			game.showEngineSettings()
		}),
		fyne.NewMenuItem("Attach Engine", func() {
			game.attachEngine()
		}),
		fyne.NewMenuItem("Detach Engine", func() {
			game.detachEngine()
		}),
	)

	// Update the main menu to include the new "Engine" menu
	mainMenu := fyne.NewMainMenu(
		fileMenu,
		gameMenu,
		mouseModeMenu,
		engineMenu, // Add Engine menu here
	)
	w.SetMainMenu(mainMenu)

	// Wrap the gameTreeContainer in a ResizingContainer
	resizingLabel := widget.NewLabel("Resizing")
	gameTreeResizingContainer := NewResizingContainer(game.gameTreeContainer, resizingLabel)

	// Layout for controls
	controls := container.NewVSplit(
		container.NewVBox(
			game.scoringStatus,
			game.commentEntry,
		),
		gameTreeResizingContainer, // Use the ResizingContainer here
	)
	controls.SetOffset(0)

	// Main layout with split view
	content := container.NewHSplit(
		controls,
		game.boardCanvas,
	)
	content.SetOffset(0)
	w.SetContent(content)
	w.Resize(fyne.NewSize(800, 600))
	w.Show()

	a.Run()
}

func (g *Game) showSetKomiDialog() {
	komiEntry := widget.NewEntry()
	komiEntry.SetText(fmt.Sprintf("%.1f", g.komi))
	komiEntry.Validator = func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("invalid komi value")
		}
		return nil
	}
	formItems := []*widget.FormItem{
		widget.NewFormItem("Komi", komiEntry),
	}
	komiDialog := dialog.NewForm("Set Komi", "OK", "Cancel", formItems, func(ok bool) {
		if ok {
			komiValue, err := strconv.ParseFloat(komiEntry.Text, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("invalid komi value"), g.window)
				return
			}
			g.komi = komiValue

			// If engine is attached, send komi command
			if g.gtpCmd != nil {
				_, err := g.sendGTPCommand(fmt.Sprintf("komi %.1f", g.komi))
				if err != nil {
					dialog.ShowError(err, g.window)
				}
			}

			// Recalculate and display score if in scoring mode
			if g.mouseMode == "score" {
				g.calculateAndDisplayScore()
			}
		}
	}, g.window)
	komiDialog.Show()
}

func (g *Game) showEngineSettings() {
	// Create input entries for settings
	gtpPathEntry := widget.NewEntry()
	gtpPathEntry.SetText(g.gtpPath)
	gtpArgsEntry := widget.NewEntry()
	gtpArgsEntry.SetText(g.gtpArgs)
	gtpColorEntry := widget.NewSelect([]string{"B", "W"}, func(value string) {})
	gtpColorEntry.SetSelected(g.gtpColor)

	// Create form items
	formItems := []*widget.FormItem{
		widget.NewFormItem("GTP Path", gtpPathEntry),
		widget.NewFormItem("GTP Arguments", gtpArgsEntry),
		widget.NewFormItem("GTP Color", gtpColorEntry),
	}

	// Show settings dialog
	settingsDialog := dialog.NewForm("Engine Settings", "OK", "Cancel", formItems, func(ok bool) {
		if ok {
			g.gtpPath = gtpPathEntry.Text
			g.gtpArgs = gtpArgsEntry.Text
			g.gtpColor = gtpColorEntry.Selected
		}
	}, g.window)
	settingsDialog.Show()
}

func (g *Game) attachEngine() {
	// Start the GTP engine process
	args := strings.Fields(g.gtpArgs)
	g.gtpCmd = exec.Command(g.gtpPath, args...)

	var err error
	g.gtpIn, err = g.gtpCmd.StdinPipe()
	if err != nil {
		dialog.ShowError(err, g.window)
		return
	}

	g.gtpOut, err = g.gtpCmd.StdoutPipe()
	if err != nil {
		dialog.ShowError(err, g.window)
		return
	}

	if err := g.gtpCmd.Start(); err != nil {
		dialog.ShowError(err, g.window)
		return
	}

	g.gtpReader = bufio.NewReader(g.gtpOut)

	// Initialize the engine
	if err := g.initializeEngine(); err != nil {
		dialog.ShowError(err, g.window)
		g.detachEngine()
	} else {
		dialog.ShowInformation("Engine Attached", "Successfully attached to the engine.", g.window)
	}
}

func (g *Game) detachEngine() {
	if g.gtpCmd != nil {
		// Kill the engine process
		err := g.gtpCmd.Process.Kill()
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to kill engine process: %v", err), g.window)
		}

		// Close stdin pipe
		if g.gtpIn != nil {
			err := g.gtpIn.Close()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to close engine stdin: %v", err), g.window)
			}
			g.gtpIn = nil
		}

		// Close stdout pipe
		if g.gtpOut != nil {
			err := g.gtpOut.Close()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to close engine stdout: %v", err), g.window)
			}
			g.gtpOut = nil
		}

		// Wait for the process to exit
		err = g.gtpCmd.Wait()
		if err != nil && !strings.Contains(err.Error(), "killed") {
			dialog.ShowError(fmt.Errorf("error while waiting for engine process to exit: %v", err), g.window)
		}

		// Set engine-related variables to nil
		g.gtpCmd = nil
		g.gtpReader = nil

		dialog.ShowInformation("Engine Detached", "Successfully detached from the engine.", g.window)
	}
}

func (g *Game) initializeEngine() error {
	// Check if the required commands are supported
	supportedCommands, err := g.sendGTPCommand("list_commands")
	if err != nil {
		return err
	}

	requiredCommands := []string{"boardsize", "komi", "play", "genmove"}
	for _, cmd := range requiredCommands {
		if !strings.Contains(supportedCommands, cmd) {
			return fmt.Errorf("engine does not support required command: %s", cmd)
		}
	}

	// Set board size
	if g.sizeX == g.sizeY {
		if _, err := g.sendGTPCommand(fmt.Sprintf("boardsize %d", g.sizeX)); err != nil {
			return err
		}
	} else {
		// Check if rectangular_boardsize is supported
		if strings.Contains(supportedCommands, "rectangular_boardsize") {
			if _, err := g.sendGTPCommand(fmt.Sprintf("rectangular_boardsize %d %d", g.sizeX, g.sizeY)); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("engine does not support rectangular boards and board is not square")
		}
	}

	// Set komi
	if _, err := g.sendGTPCommand(fmt.Sprintf("komi %.1f", g.komi)); err != nil {
		return err
	}

	// Send the current board state to the engine
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			stone := g.currentNode.boardState[y][x]
			if stone != empty {
				coord := g.clientToGTPCoords(x, y)
				if _, err := g.sendGTPCommand(fmt.Sprintf("play %s %s", stone, coord)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (g *Game) sendGTPCommand(command string) (string, error) {
	if g.gtpIn == nil || g.gtpReader == nil {
		return "", fmt.Errorf("engine is not attached")
	}

	// Send command
	_, err := g.gtpIn.Write([]byte(command + "\n"))
	if err != nil {
		return "", err
	}

	// Read response
	var responseLines []string
	for {
		line, err := g.gtpReader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line[0] == '=' || line[0] == '?' {
			// Response start
			if len(line) > 1 {
				responseLines = append(responseLines, strings.TrimSpace(line[1:]))
			}
			// Read any additional output lines
			for {
				nextLine, err := g.gtpReader.ReadString('\n')
				if err != nil {
					return "", err
				}
				nextLine = strings.TrimSpace(nextLine)
				if nextLine == "" {
					break
				}
				responseLines = append(responseLines, nextLine)
			}
			if line[0] == '?' {
				return strings.Join(responseLines, "\n"), fmt.Errorf("error from engine: %s", strings.Join(responseLines, "\n"))
			}
			return strings.Join(responseLines, "\n"), nil
		}
	}
}

// GTP coordinates use letters A-H, J-T (I is skipped), and numbers from 1 upwards
func (g *Game) clientToGTPCoords(x, y int) string {
	// Convert x to letter
	letterRunes := []rune("ABCDEFGHJKLMNOPQRSTUVWXYZ")
	if x < 0 || x >= len(letterRunes) {
		return ""
	}
	letter := string(letterRunes[x])
	// GTP coordinates have origin at lower-left corner
	number := g.sizeY - y
	return fmt.Sprintf("%s%d", letter, number)
}

func (g *Game) gtpToClientCoords(coord string) (int, int, error) {
	if len(coord) < 2 {
		return 0, 0, fmt.Errorf("invalid GTP coordinate: %s", coord)
	}
	letter := coord[:1]
	numberStr := coord[1:]
	letterRunes := []rune("ABCDEFGHJKLMNOPQRSTUVWXYZ")
	x := -1
	for i, r := range letterRunes {
		if string(r) == letter {
			x = i
			break
		}
	}
	if x == -1 {
		return 0, 0, fmt.Errorf("invalid GTP coordinate letter: %s", letter)
	}
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0, 0, err
	}
	y := g.sizeY - number
	if y < 0 || y >= g.sizeY {
		return 0, 0, fmt.Errorf("invalid GTP coordinate number: %s", numberStr)
	}
	return x, y, nil
}

func (g *Game) handleEngineMove(coord string) {
	coord = strings.TrimSpace(coord)
	if coord == "resign" {
		dialog.ShowInformation("Engine Resigned", "The engine has resigned.", g.window)
		return
	}
	x, y, err := g.gtpToClientCoords(coord)
	if err != nil {
		dialog.ShowError(err, g.window)
		return
	}
	player := switchPlayer(g.currentNode.player)
	if !g.isMoveLegal(x, y, player) {
		dialog.ShowError(fmt.Errorf("engine played an illegal move"), g.window)
		return
	}
	boardCopy := copyBoard(g.currentNode.boardState)
	boardCopy[y][x] = player
	koX, koY := g.captureStones(boardCopy, x, y, player)
	newNode := g.newGameTreeNode()
	newNode.boardState = boardCopy
	newNode.move = [2]int{x, y}
	newNode.player = player
	newNode.parent = g.currentNode
	newNode.koX = koX
	newNode.koY = koY
	g.currentNode.children = append(g.currentNode.children, newNode)
	g.currentNode = newNode
	g.updateCommentTextbox()
	g.updateGameTreeUI()
	g.redrawBoard()
}

func (g *Game) updateCommentTextbox() {
	if g.currentNode != nil && g.currentNode.Comment != "" {
		g.commentEntry.SetText(g.currentNode.Comment)
	} else {
		g.commentEntry.SetText("") // Clears the textbox if there's no comment
	}
}

func (g *Game) enterScoringMode() {
	g.initializeTerritoryMap()
	g.assignTerritoryToEmptyRegions()
	g.redrawBoard()
	g.calculateAndDisplayScore()
}

func (g *Game) exitScoringMode() {
	// Remove territory markers
	if g.territoryLayer != nil {
		g.gridContainer.Remove(g.territoryLayer)
		g.territoryLayer = nil
	}
	// Re-draw the board
	g.redrawBoard()
	// Reset any scoring status
	g.scoringStatus.SetText("Not in scoring mode.")
}

func (g *Game) initializeTerritoryMap() {
	g.territoryMap = make([][]string, g.sizeY)
	for y := 0; y < g.sizeY; y++ {
		g.territoryMap[y] = make([]string, g.sizeX)
		for x := 0; x < g.sizeX; x++ {
			stone := g.currentNode.boardState[y][x]
			if stone == black || stone == white {
				g.territoryMap[y][x] = stone
			} else {
				g.territoryMap[y][x] = "?"
			}
		}
	}
}

func (g *Game) assignTerritoryToEmptyRegions() {
	visited := make([][]bool, g.sizeY)
	for y := 0; y < g.sizeY; y++ {
		visited[y] = make([]bool, g.sizeX)
	}

	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.boardState[y][x] == empty && !visited[y][x] {
				// Start flood fill for this empty region
				stack := [][2]int{{x, y}}
				adjacentStones := make(map[string]bool)
				region := [][2]int{}

				for len(stack) > 0 {
					cx, cy := stack[len(stack)-1][0], stack[len(stack)-1][1]
					stack = stack[:len(stack)-1]

					if visited[cy][cx] {
						continue
					}
					visited[cy][cx] = true
					region = append(region, [2]int{cx, cy})

					dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
					for _, d := range dirs {
						nx, ny := cx+d[0], cy+d[1]
						if nx >= 0 && nx < g.sizeX && ny >= 0 && ny < g.sizeY {
							neighborStone := g.territoryMap[ny][nx]
							if g.currentNode.boardState[ny][nx] == empty && !visited[ny][nx] {
								stack = append(stack, [2]int{nx, ny})
							} else if neighborStone == black || neighborStone == white {
								adjacentStones[neighborStone] = true
							}
						}
					}
				}

				// Determine owner
				if len(adjacentStones) == 1 {
					var owner string
					for k := range adjacentStones {
						owner = k
					}
					// Assign territory
					for _, pos := range region {
						g.territoryMap[pos[1]][pos[0]] = owner
					}
				} else {
					// Neutral territory, leave as "?"
					for _, pos := range region {
						g.territoryMap[pos[1]][pos[0]] = "?"
					}
				}
			}
		}
	}
}

func (g *Game) calculateScore() (float64, float64) {
	blackScore := 0.0
	whiteScore := 0.0
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			owner := g.territoryMap[y][x]
			if owner == black {
				blackScore += 1.0
			} else if owner == white {
				whiteScore += 1.0
			}
		}
	}

	// Add komi to white's score
	whiteScore += g.komi

	return blackScore, whiteScore
}

func (g *Game) calculateAndDisplayScore() {
	blackScore, whiteScore := g.calculateScore()
	g.scoringStatus.SetText(fmt.Sprintf("Black: %.1f, White: %.1f", blackScore, whiteScore))
}

func (g *Game) toggleGroupStatus(x, y int) {
	originalOwner := g.currentNode.boardState[y][x]
	if originalOwner != black && originalOwner != white {
		return
	}

	newOwner := switchPlayer(g.territoryMap[y][x])

	visited := make(map[[2]int]bool)
	stack := [][2]int{{x, y}}

	for len(stack) > 0 {
		cx, cy := stack[len(stack)-1][0], stack[len(stack)-1][1]
		stack = stack[:len(stack)-1]
		if visited[[2]int{cx, cy}] {
			continue
		}
		visited[[2]int{cx, cy}] = true

		stone := g.currentNode.boardState[cy][cx]

		if stone == originalOwner {
			// Toggle the ownership
			g.territoryMap[cy][cx] = newOwner
		}

		// Add neighbors to stack
		dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
		for _, d := range dirs {
			nx, ny := cx+d[0], cy+d[1]
			if nx >= 0 && nx < g.sizeX && ny >= 0 && ny < g.sizeY {
				neighborStone := g.currentNode.boardState[ny][nx]
				if !visited[[2]int{nx, ny}] && (neighborStone == originalOwner || neighborStone == empty) {
					stack = append(stack, [2]int{nx, ny})
				}
			}
		}
	}

	// Reset all empty spaces to "?" (no owner)
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.boardState[y][x] == empty {
				g.territoryMap[y][x] = "?"
			}
		}
	}

	// Recalculate the territory
	g.assignTerritoryToEmptyRegions()
}

func (g *Game) updateGameTreeUI() {
	scrollPosition := g.gameTreeContainer.Offset
	newGameTreeUI := g.buildGameTreeUI(g.rootNode)
	g.gameTreeContainer.Content = newGameTreeUI
	g.gameTreeContainer.Refresh()
	g.gameTreeContainer.Offset = scrollPosition
}

func (g *Game) buildGameTreeUI(node *GameTreeNode) fyne.CanvasObject {
	var nodeLabel string
	if node.parent == nil {
		nodeLabel = "Root"
	} else if node.hasAddedBlackStones() || node.hasAddedWhiteStones() {
		nodeLabel = fmt.Sprintf("%s:Setup", node.player)
	} else if node.move[0] == -1 && node.move[1] == -1 {
		nodeLabel = fmt.Sprintf("%s:Pass", node.player)
	} else {
		nodeLabel = fmt.Sprintf("%s:(%d,%d)", node.player, node.move[0], node.move[1])
	}

	nodeButton := widget.NewButton(nodeLabel, func() {
		if g.mouseMode == "score" {
			g.exitScoringMode()
		}

		g.setCurrentNode(node)
		g.redrawBoard()
		g.updateGameTreeUI()
	})

	if node == g.currentNode {
		nodeButton.Importance = widget.HighImportance
	}

	childUIs := []fyne.CanvasObject{}
	for _, child := range node.children {
		childUIs = append(childUIs, g.buildGameTreeUI(child))
	}
	childrenContainer := container.NewHBox(childUIs...)
	return container.NewVBox(nodeButton, childrenContainer)
}

func makeEmptyBoard(sizeX, sizeY int) [][]string {
	board := make([][]string, sizeY)
	for i := range board {
		board[i] = make([]string, sizeX)
		for j := range board[i] {
			board[i][j] = empty
		}
	}
	return board
}

func (g *Game) initializeBoard() {
	g.idCounter = 1
	rootNode := g.newGameTreeNode()
	rootNode.player = "" // No player has moved yet
	g.rootNode = rootNode
	g.currentNode = rootNode
	g.nodeMap = make(map[string]*GameTreeNode)
	g.nodeMap[rootNode.id] = rootNode
	if g.mouseMode == "score" {
		g.exitScoringMode()
	}

	g.updateCommentTextbox()
}

func copyBoard(board [][]string) [][]string {
	boardCopy := make([][]string, len(board))
	for i := range board {
		boardCopy[i] = make([]string, len(board[i]))
		copy(boardCopy[i], board[i])
	}
	return boardCopy
}

func (g *Game) setCurrentNode(node *GameTreeNode) {
	g.currentNode = node
	g.updateCommentTextbox()
}

func (g *Game) redrawBoard() {
	// Clear previous grid lines, stones, and annotations
	g.gridContainer.Objects = nil
	g.gridContainer.Hide()

	// Remove existing territory layer if present
	if g.territoryLayer != nil {
		g.gridContainer.Remove(g.territoryLayer)
	}

	// Calculate cell size based on the current board size and window dimensions
	size := g.boardCanvas.Size()
	g.cellSize = min(size.Width/float32(g.sizeX), size.Height/float32(g.sizeY))

	// Draw various components of the board
	g.drawGridLines()
	g.drawStoneConnections()
	g.drawStones()
	g.drawAnnotations()

	// Draw territory markers if in scoring mode
	if g.mouseMode == "score" {
		g.drawTerritoryMarkers()
	}

	// Show and refresh the grid container to render all added objects
	g.gridContainer.Show()
	g.gridContainer.Refresh()
}

// Draws the grid lines on the board
func (g *Game) drawGridLines() {
	// Draw vertical lines
	for x := 0; x < g.sizeX; x++ {
		line := canvas.NewLine(lineColor)
		startPos := g.boardCoordsToPixel(x, 0)
		endPos := g.boardCoordsToPixel(x, g.sizeY-1)
		line.Position1 = fyne.NewPos(startPos.X+0.5*g.cellSize, startPos.Y+(0.5-gridLineThickness/2)*g.cellSize)
		line.Position2 = fyne.NewPos(endPos.X+0.5*g.cellSize, endPos.Y+(0.5+gridLineThickness/2)*g.cellSize)
		line.StrokeWidth = g.cellSize * gridLineThickness
		g.gridContainer.Add(line)
	}

	// Draw horizontal lines
	for y := 0; y < g.sizeY; y++ {
		line := canvas.NewLine(lineColor)
		startPos := g.boardCoordsToPixel(0, y)
		endPos := g.boardCoordsToPixel(g.sizeX-1, y)
		line.Position1 = fyne.NewPos(startPos.X+(0.5-gridLineThickness/2)*g.cellSize, startPos.Y+0.5*g.cellSize)
		line.Position2 = fyne.NewPos(endPos.X+(0.5+gridLineThickness/2)*g.cellSize, endPos.Y+0.5*g.cellSize)
		line.StrokeWidth = g.cellSize * gridLineThickness
		g.gridContainer.Add(line)
	}
}

// Draws connections between stones to represent groups
func (g *Game) drawStoneConnections() {
	// Draw 4-square stone connections to represent groups
	for y := 1; y < g.sizeY; y++ {
		for x := 1; x < g.sizeX; x++ {
			stone1 := g.currentNode.boardState[y][x-1]
			stone2 := g.currentNode.boardState[y][x]
			stone3 := g.currentNode.boardState[y-1][x-1]
			stone4 := g.currentNode.boardState[y-1][x]
			if stone1 != empty && stone2 != empty && stone3 != empty && stone4 != empty {
				// Rule out cross cuts to prevent incorrect group representation
				if stone3 == stone2 && stone1 == stone4 && stone1 != stone2 {
					continue
				}
				rect := canvas.NewRectangle(blackColor)
				if (stone1 == white && stone1 == stone4) || (stone2 == white && stone2 == stone3) {
					rect.FillColor = whiteColor
				}
				rect.StrokeWidth = 0
				pos := g.boardCoordsToPixel(x, y)
				pos = fyne.Position{X: pos.X - 0.5*g.cellSize, Y: pos.Y - 0.5*g.cellSize}
				rect.Resize(fyne.NewSize(g.cellSize, g.cellSize))
				rect.Move(pos)
				g.gridContainer.Add(rect)
			}
		}
	}

	// Draw vertical stone connections
	for y := 1; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			stone1 := g.currentNode.boardState[y-1][x]
			stone2 := g.currentNode.boardState[y][x]
			if stone1 != empty && stone1 == stone2 {
				rect := canvas.NewRectangle(blackColor)
				if stone1 == white {
					rect.FillColor = whiteColor
				}
				rect.StrokeWidth = 0
				pos := g.boardCoordsToPixel(x, y)
				pos = fyne.Position{X: pos.X, Y: pos.Y - 0.5*g.cellSize}
				rect.Resize(fyne.NewSize(g.cellSize, g.cellSize))
				rect.Move(pos)
				g.gridContainer.Add(rect)
			}
		}
	}

	// Draw horizontal stone connections
	for y := 0; y < g.sizeY; y++ {
		for x := 1; x < g.sizeX; x++ {
			stone1 := g.currentNode.boardState[y][x-1]
			stone2 := g.currentNode.boardState[y][x]
			if stone1 != empty && stone1 == stone2 {
				rect := canvas.NewRectangle(blackColor)
				if stone1 == white {
					rect.FillColor = whiteColor
				}
				rect.StrokeWidth = 0
				pos := g.boardCoordsToPixel(x, y)
				pos = fyne.Position{X: pos.X - 0.5*g.cellSize, Y: pos.Y}
				rect.Resize(fyne.NewSize(g.cellSize, g.cellSize))
				rect.Move(pos)
				g.gridContainer.Add(rect)
			}
		}
	}
}

// Draws the stones on the board based on the current board state
func (g *Game) drawStones() {
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			stone := g.currentNode.boardState[y][x]
			if stone != empty {
				circle := canvas.NewCircle(blackColor)
				if stone == white {
					circle.FillColor = whiteColor
				}
				circle.StrokeWidth = 0
				pos := g.boardCoordsToPixel(x, y)
				circle.Resize(fyne.NewSize(g.cellSize, g.cellSize))
				circle.Move(pos)
				g.gridContainer.Add(circle)
			}
		}
	}
}

// Draws annotations such as circles, squares, triangles, marks, and labels
func (g *Game) drawAnnotations() {
	annotationsLayer := container.NewWithoutLayout()

	// Draw Circles (CR)
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.CR[y][x] {
				pos := g.boardCoordsToPixel(x, y)
				circle := canvas.NewCircle(color.Transparent)
				circle.StrokeColor = redColor
				circle.StrokeWidth = g.cellSize * 0.05
				circle.Resize(fyne.NewSize(g.cellSize*0.6, g.cellSize*0.6))
				circle.Move(fyne.Position{
					X: pos.X + 0.5*g.cellSize - circle.Size().Width/2,
					Y: pos.Y + 0.5*g.cellSize - circle.Size().Height/2,
				})
				annotationsLayer.Add(circle)
			}
		}
	}

	// Draw Squares (SQ)
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.SQ[y][x] {
				pos := g.boardCoordsToPixel(x, y)
				square := canvas.NewRectangle(color.Transparent)
				square.StrokeColor = redColor
				square.StrokeWidth = g.cellSize * 0.05
				square.Resize(fyne.NewSize(g.cellSize*0.6, g.cellSize*0.6))
				square.Move(fyne.Position{
					X: pos.X + 0.5*g.cellSize - square.Size().Width/2,
					Y: pos.Y + 0.5*g.cellSize - square.Size().Height/2,
				})
				annotationsLayer.Add(square)
			}
		}
	}

	// Draw Triangles (TR) using three lines
	tSize := g.cellSize * 0.39
	tXOffset := tSize * float32(math.Sin(math.Pi/3))
	tYOffset := tSize * float32(math.Cos(math.Pi/3))
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.TR[y][x] {
				pos := g.boardCoordsToPixel(x, y)
				pos0 := fyne.NewPos(pos.X+0.5*g.cellSize, pos.Y+0.5*g.cellSize-tSize)
				pos1 := fyne.NewPos(pos.X+0.5*g.cellSize-tXOffset, pos.Y+0.5*g.cellSize+tYOffset)
				pos2 := fyne.NewPos(pos.X+0.5*g.cellSize+tXOffset, pos.Y+0.5*g.cellSize+tYOffset)

				// Create triangle lines
				line1 := canvas.NewLine(redColor)
				line1.StrokeWidth = g.cellSize * 0.05
				line1.Position1 = pos0
				line1.Position2 = pos1

				line2 := canvas.NewLine(redColor)
				line2.StrokeWidth = g.cellSize * 0.05
				line2.Position1 = pos1
				line2.Position2 = pos2

				line3 := canvas.NewLine(redColor)
				line3.StrokeWidth = g.cellSize * 0.05
				line3.Position1 = pos2
				line3.Position2 = pos0

				// Add lines to annotations layer
				annotationsLayer.Add(line1)
				annotationsLayer.Add(line2)
				annotationsLayer.Add(line3)
			}
		}
	}

	// Draw Xs (MA) using two crossing lines
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.MA[y][x] {
				pos := g.boardCoordsToPixel(x, y)
				size := g.cellSize * 0.6

				// Define the two crossing lines relative to the position
				line1 := canvas.NewLine(redColor)
				line1.StrokeWidth = g.cellSize * 0.05
				line1.Position1 = fyne.NewPos(pos.X+0.5*g.cellSize-size/2, pos.Y+0.5*g.cellSize-size/2)
				line1.Position2 = fyne.NewPos(pos.X+0.5*g.cellSize+size/2, pos.Y+0.5*g.cellSize+size/2)

				line2 := canvas.NewLine(redColor)
				line2.StrokeWidth = g.cellSize * 0.05
				line2.Position1 = fyne.NewPos(pos.X+0.5*g.cellSize+size/2, pos.Y+0.5*g.cellSize-size/2)
				line2.Position2 = fyne.NewPos(pos.X+0.5*g.cellSize-size/2, pos.Y+0.5*g.cellSize+size/2)

				// Add lines to annotations layer
				annotationsLayer.Add(line1)
				annotationsLayer.Add(line2)
			}
		}
	}

	// Draw Labels (LB)
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			if g.currentNode.LB[y][x] != "" {
				pos := g.boardCoordsToPixel(x, y)
				text := canvas.NewText(g.currentNode.LB[y][x], redColor)
				text.TextSize = g.cellSize * 0.4
				text.Alignment = fyne.TextAlignCenter
				text.TextStyle = fyne.TextStyle{Bold: true}
				text.Resize(text.MinSize()) // Calculate the size needed for the text

				// Center the text on the point
				text.Move(fyne.Position{
					X: pos.X + 0.5*g.cellSize - text.Size().Width/2,
					Y: pos.Y + 0.5*g.cellSize - text.Size().Height/2,
				})
				annotationsLayer.Add(text)
			}
		}
	}

	// Add annotations layer to gridContainer
	g.gridContainer.Add(annotationsLayer)
}

// Draws territory markers when in scoring mode
func (g *Game) drawTerritoryMarkers() {
	// Create a new layer for territory markers
	g.territoryLayer = container.NewWithoutLayout()

	// Iterate over the territory map to place markers
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			owner := g.territoryMap[y][x]
			if owner == black || owner == white {
				rect := canvas.NewRectangle(transparentBlackColor)
				rect.StrokeColor = blackScoreColor
				if owner == white {
					rect.FillColor = transparentWhiteColor
					rect.StrokeColor = whiteScoreColor
				}
				rect.StrokeWidth = g.cellSize * 0.039
				squareSize := g.cellSize * 0.51
				pos := g.boardCoordsToPixel(x, y)
				pos = fyne.Position{X: pos.X + 0.5*g.cellSize - squareSize/2, Y: pos.Y + 0.5*g.cellSize - squareSize/2}
				rect.Resize(fyne.NewSize(squareSize, squareSize))
				rect.Move(pos)
				g.territoryLayer.Add(rect)
			}
		}
	}

	// Add the territoryLayer to gridContainer
	g.gridContainer.Add(g.territoryLayer)
}

type inputLayer struct {
	widget.BaseWidget
	game *Game
}

func newInputLayer(game *Game) *inputLayer {
	i := &inputLayer{game: game}
	i.ExtendBaseWidget(i)
	return i
}

func (i *inputLayer) CreateRenderer() fyne.WidgetRenderer {
	return &inputLayerRenderer{
		layer: i,
	}
}

func (i *inputLayer) Resize(size fyne.Size) {
	i.BaseWidget.Resize(size)
	i.Refresh()
	// Trigger redraw on resize to update cell dimensions and redraw grid
	i.game.redrawBoard()
}

func (i *inputLayer) Tapped(ev *fyne.PointEvent) {
	i.game.handleMouseClick(ev)
}

func (i *inputLayer) TappedSecondary(ev *fyne.PointEvent) {}

func (i *inputLayer) MouseMoved(ev *desktop.MouseEvent) {
	i.game.handleMouseMove(ev)
}

func (i *inputLayer) MouseIn(ev *desktop.MouseEvent) {}

func (i *inputLayer) MouseOut() {
	if i.game.hoverStone != nil {
		i.game.gridContainer.Remove(i.game.hoverStone)
		i.game.hoverStone = nil
		i.game.gridContainer.Refresh()
	}
}

type inputLayerRenderer struct {
	layer *inputLayer
}

func (r *inputLayerRenderer) Layout(size fyne.Size) {
	r.layer.Resize(size)
}

func (r *inputLayerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *inputLayerRenderer) Refresh() {}

func (r *inputLayerRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

func (r *inputLayerRenderer) Objects() []fyne.CanvasObject {
	return nil
}

func (r *inputLayerRenderer) Destroy() {}

// Converts pixel coordinates to board coordinates.
// Returns x, y indices and a boolean indicating validity.
func (g *Game) pixelToBoardCoords(pos fyne.Position) (int, int, bool) {
	size := g.boardCanvas.Size()
	x := int(((pos.X*2-size.Width)/g.cellSize + float32(g.sizeX)) / 2)
	y := int(((pos.Y*2-size.Height)/g.cellSize + float32(g.sizeY)) / 2)

	if x < 0 || x >= g.sizeX || y < 0 || y >= g.sizeY {
		return 93, 93, false // Coordinates out of bounds
	}

	return x, y, true
}

// Converts board coordinates to pixel positions for rendering.
func (g *Game) boardCoordsToPixel(x, y int) fyne.Position {
	size := g.boardCanvas.Size()
	return fyne.NewPos(
		(float32(2*x-g.sizeX)*g.cellSize+size.Width)/2,
		(float32(2*y-g.sizeY)*g.cellSize+size.Height)/2,
	)
}

// Handles mouse movement events to display a hover stone when applicable.
func (g *Game) handleMouseMove(ev *desktop.MouseEvent) {
	if g.mouseMode != "play" {
		if g.hoverStone != nil {
			g.gridContainer.Remove(g.hoverStone)
			g.hoverStone = nil
			g.gridContainer.Refresh()
		}
		return
	}

	x, y, ok := g.pixelToBoardCoords(ev.Position)
	if !ok {
		if g.hoverStone != nil {
			g.gridContainer.Remove(g.hoverStone)
			g.hoverStone = nil
			g.gridContainer.Refresh()
		}
		return
	}

	player := switchPlayer(g.currentNode.player)

	if g.currentNode.boardState[y][x] != empty || !g.isMoveLegal(x, y, player) {
		if g.hoverStone != nil {
			g.gridContainer.Remove(g.hoverStone)
			g.hoverStone = nil
			g.gridContainer.Refresh()
		}
		return
	}

	if g.hoverStone != nil {
		g.gridContainer.Remove(g.hoverStone)
	}

	circle := canvas.NewCircle(transparentBlackColor)
	if player == white {
		circle.FillColor = transparentWhiteColor
	}
	circle.StrokeWidth = 0
	circle.Resize(fyne.NewSize(g.cellSize, g.cellSize))
	circle.Move(g.boardCoordsToPixel(x, y))
	g.gridContainer.Add(circle)
	g.hoverStone = circle
	g.gridContainer.Refresh()
}

func (g *Game) setMouseMode(mode string) {
	if g.mouseMode == mode {
		return
	}
	if g.mouseMode == "score" && mode != "score" {
		g.exitScoringMode()
	}
	if mode == "score" && g.mouseMode != "score" {
		g.enterScoringMode()
	}
	g.mouseMode = mode
}

// Handles mouse click events to place stones or toggle group status in scoring mode.
func (g *Game) handleMouseClick(ev *fyne.PointEvent) {
	x, y, ok := g.pixelToBoardCoords(ev.Position)
	if !ok {
		return // Click outside the board
	}

	switch g.mouseMode {
	case "play":
		if g.currentNode.boardState[y][x] != empty {
			return
		}
		player := switchPlayer(g.currentNode.player)
		if !g.isMoveLegal(x, y, player) {
			return
		}
		boardCopy := copyBoard(g.currentNode.boardState)
		boardCopy[y][x] = player
		koX, koY := g.captureStones(boardCopy, x, y, player)
		newNode := g.newGameTreeNode()
		newNode.boardState = boardCopy
		newNode.move = [2]int{x, y}
		newNode.player = player
		newNode.parent = g.currentNode
		newNode.koX = koX
		newNode.koY = koY
		g.currentNode.children = append(g.currentNode.children, newNode)
		g.currentNode = newNode
		g.updateCommentTextbox()
		g.updateGameTreeUI()
		g.redrawBoard()
		if g.gtpCmd != nil && g.gtpColor != player {
			// Send move to engine
			coord := g.clientToGTPCoords(x, y)
			if _, err := g.sendGTPCommand(fmt.Sprintf("play %s %s", player, coord)); err != nil {
				dialog.ShowError(err, g.window)
				g.detachEngine()
			} else {
				// Get engine's response move
				engineMove, err := g.sendGTPCommand(fmt.Sprintf("genmove %s", switchPlayer(player)))
				if err != nil {
					dialog.ShowError(err, g.window)
					g.detachEngine()
				} else {
					g.handleEngineMove(engineMove)
				}
			}
		}
	case "score":
		g.toggleGroupStatus(x, y)
		g.assignTerritoryToEmptyRegions()
		g.redrawBoard()
		g.calculateAndDisplayScore()
	case "label":
		// Open a textbox popup to set or remove the label of the vertex
		entry := widget.NewEntry()
		if existingLabel := g.currentNode.LB[y][x]; existingLabel != "" {
			entry.SetText(existingLabel)
		}
		entry.SetPlaceHolder("Enter label (leave empty to remove)")
		entryDialog := dialog.NewForm("Set Label", "OK", "Cancel",
			[]*widget.FormItem{widget.NewFormItem("Label", entry)},
			func(ok bool) {
				if ok {
					label := entry.Text
					g.currentNode.LB[y][x] = label
					g.redrawBoard()
				}
			}, g.window)
		entryDialog.Show()
	case "addBlack":
		if g.currentNode.boardState[y][x] != black {
			g.currentNode.boardState[y][x] = black
			g.currentNode.addedBlackStones[y][x] = true
			g.currentNode.addedWhiteStones[y][x] = false
			g.currentNode.AE[y][x] = false
			g.redrawBoard()
		}
	case "addWhite":
		if g.currentNode.boardState[y][x] != white {
			g.currentNode.boardState[y][x] = white
			g.currentNode.addedWhiteStones[y][x] = true
			g.currentNode.addedBlackStones[y][x] = false
			g.currentNode.AE[y][x] = false
			g.redrawBoard()
		}
	case "addEmpty":
		if g.currentNode.boardState[y][x] != empty {
			g.currentNode.boardState[y][x] = empty
			g.currentNode.AE[y][x] = true
			g.currentNode.addedBlackStones[y][x] = false
			g.currentNode.addedWhiteStones[y][x] = false
			g.redrawBoard()
		}
	case "circle":
		// Toggle CR[y][x]
		g.currentNode.CR[y][x] = !g.currentNode.CR[y][x]
		g.redrawBoard()
	case "square":
		// Toggle SQ[y][x]
		g.currentNode.SQ[y][x] = !g.currentNode.SQ[y][x]
		g.redrawBoard()
	case "triangle":
		// Toggle TR[y][x]
		g.currentNode.TR[y][x] = !g.currentNode.TR[y][x]
		g.redrawBoard()
	case "xMark":
		// Toggle MA[y][x]
		g.currentNode.MA[y][x] = !g.currentNode.MA[y][x]
		g.redrawBoard()
	default:
		// Do nothing or handle other modes
	}
}

func (g *Game) isMoveLegal(x, y int, player string) bool {
	if x == g.currentNode.koX && y == g.currentNode.koY {
		return false
	}

	// Copy board
	boardCopy := copyBoard(g.currentNode.boardState)
	boardCopy[y][x] = player

	// Check if any opponent stones will be captured
	opponent := switchPlayer(player)
	captured := g.getCapturedStones(boardCopy, x, y, opponent)

	// If capturing any stones, move is legal
	if len(captured) > 0 {
		return true
	}

	// Check if the new stone has liberties
	if hasLiberty(boardCopy, x, y, player, g.sizeX, g.sizeY) {
		return true
	}

	// Move is suicide
	return false
}

// Determines if the stone at (x, y) has any liberties.
// Utilizes depth-first search to check for empty adjacent positions.
func hasLiberty(board [][]string, x, y int, player string, sizeX, sizeY int) bool {
	visited := make(map[[2]int]bool) // Tracks visited positions to prevent infinite loops
	return dfs(board, x, y, player, visited, sizeX, sizeY)
}

// Recursive DFS to check for liberties
func dfs(board [][]string, x, y int, player string, visited map[[2]int]bool, sizeX, sizeY int) bool {
	if x < 0 || x >= sizeX || y < 0 || y >= sizeY {
		return false // Out of bounds
	}

	if visited[[2]int{x, y}] {
		return false // Already visited
	}

	if board[y][x] == empty {
		return true // Found a liberty
	}

	if board[y][x] != player {
		return false // Encountered opponent's stone
	}

	visited[[2]int{x, y}] = true // Mark the current stone as visited

	// Explore all four adjacent directions
	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range dirs {
		if dfs(board, x+d[0], y+d[1], player, visited, sizeX, sizeY) {
			return true // Found a liberty in adjacent stones
		}
	}
	return false // No liberties found in this group
}

func (g *Game) captureStones(board [][]string, x, y int, player string) (int, int) {
	opponent := switchPlayer(player)

	// Check adjacent opponent stones for capture
	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	capturedGroupsSizes := []int{}
	capturedGroupsCoords := [][2]int{}

	for _, d := range dirs {
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || nx >= g.sizeX || ny < 0 || ny >= g.sizeY {
			continue
		}
		if board[ny][nx] == opponent && !hasLiberty(board, nx, ny, opponent, g.sizeX, g.sizeY) {
			// Get size of captured group
			groupSize := getGroupSize(board, nx, ny, opponent, g.sizeX, g.sizeY)
			capturedGroupsSizes = append(capturedGroupsSizes, groupSize)
			capturedGroupsCoords = append(capturedGroupsCoords, [2]int{nx, ny})

			// Capture group
			removeGroup(board, nx, ny, opponent, g.sizeX, g.sizeY)
		}
	}

	// Check for suicide
	if !hasLiberty(board, x, y, player, g.sizeX, g.sizeY) {
		// Remove player's own stone
		removeGroup(board, x, y, player, g.sizeX, g.sizeY)
	}

	// Implement ko logic
	koX := -1
	koY := -1
	if len(capturedGroupsSizes) == 1 { // 1 group was captured, might be ko
		capturingGroupSize := getGroupSize(board, x, y, player, g.sizeX, g.sizeY)
		capturedGroupSize := capturedGroupsSizes[0]
		if capturedGroupSize == 1 && capturingGroupSize == 1 {
			// Set ko point
			koX = capturedGroupsCoords[0][0]
			koY = capturedGroupsCoords[0][1]
		}
	}
	return koX, koY
}

func getGroupSize(board [][]string, x, y int, player string, sizeX, sizeY int) int {
	visited := make(map[[2]int]bool)
	groupDFS(board, x, y, player, visited, sizeX, sizeY)
	return len(visited)
}

func groupDFS(board [][]string, x, y int, player string, visited map[[2]int]bool, sizeX, sizeY int) {
	if x < 0 || x >= sizeX || y < 0 || y >= sizeY {
		return
	}

	if visited[[2]int{x, y}] {
		return
	}

	if board[y][x] != player {
		return
	}

	visited[[2]int{x, y}] = true

	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range dirs {
		groupDFS(board, x+d[0], y+d[1], player, visited, sizeX, sizeY)
	}
}

func removeGroup(board [][]string, x, y int, player string, sizeX, sizeY int) int {
	visited := make(map[[2]int]bool)
	removeDFS(board, x, y, player, visited, sizeX, sizeY)
	return len(visited)
}

func removeDFS(board [][]string, x, y int, player string, visited map[[2]int]bool, sizeX, sizeY int) {
	if x < 0 || x >= sizeX || y < 0 || y >= sizeY {
		return
	}

	if visited[[2]int{x, y}] {
		return
	}

	if board[y][x] != player {
		return
	}

	visited[[2]int{x, y}] = true

	board[y][x] = empty

	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range dirs {
		removeDFS(board, x+d[0], y+d[1], player, visited, sizeX, sizeY)
	}
}

func (g *Game) getCapturedStones(board [][]string, x, y int, opponent string) [][2]int {
	captured := make([][2]int, 0)
	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range dirs {
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || nx >= g.sizeX || ny < 0 || ny >= g.sizeY {
			continue
		}
		if board[ny][nx] == opponent && !hasLiberty(board, nx, ny, opponent, g.sizeX, g.sizeY) {
			captured = append(captured, [2]int{nx, ny})
		}
	}
	return captured
}

// Switches the current player.
// Returns "W" if the current player is "B", and vice versa.
func switchPlayer(player string) string {
	if player == black {
		return white
	}
	return black
}

func (g *Game) importFromSGF(sgfContent string) error {
	collection, err := parseSGF(sgfContent)
	if err != nil {
		return err
	}
	if len(collection) == 0 {
		return fmt.Errorf("no valid SGF game trees found")
	}
	gameTree := collection[0]
	return g.initializeGameFromSGFTree(gameTree)
}

func (g *Game) exportToSGF() (string, error) {
	sgfContent := generateSGF(g.rootNode, g.sizeX, g.sizeY)
	return sgfContent, nil
}

type SGFParser struct {
	sgfContent string // The SGF content to parse
	index      int    // Current parsing index
	length     int    // Length of the SGF content
}

type SGFGameTree struct {
	sequence []*SGFNode     // Sequence of nodes representing moves and properties
	subtrees []*SGFGameTree // Subtrees representing variations
}

type SGFNode struct {
	properties map[string][]string // Properties of the node (e.g., move, annotations)
}

func parseSGF(sgfContent string) ([]*SGFGameTree, error) {
	parser := &SGFParser{
		sgfContent: sgfContent,
		index:      0,
		length:     len(sgfContent),
	}

	// Validate the SGF content for balanced parentheses
	if err := parser.validateSGF(); err != nil {
		return nil, err
	}

	// Parse the SGF content into a collection of game trees
	collection, err := parser.parseCollection()
	if err != nil {
		return nil, fmt.Errorf("error parsing SGF at index %d: %v", parser.index, err)
	}
	return collection, nil
}

// Validates that all parentheses in the SGF content are balanced
func (p *SGFParser) validateSGF() error {
	openParens := 0
	for i, char := range p.sgfContent {
		if char == '(' {
			openParens++
		} else if char == ')' {
			openParens--
			if openParens < 0 {
				return fmt.Errorf("unmatched ')' at index %d", i)
			}
		}
	}
	if openParens != 0 {
		return fmt.Errorf("unmatched '(' in SGF content")
	}
	fmt.Println("SGF validated at the level of parenthesis.")
	return nil
}

// Parses the entire collection of game trees in the SGF content
func (p *SGFParser) parseCollection() ([]*SGFGameTree, error) {
	var collection []*SGFGameTree
	for p.index < p.length {
		p.skipWhitespace() // Skip any whitespace before checking for '('
		if p.index < p.length && p.sgfContent[p.index] == '(' {
			p.index++ // Consume '('
			gameTree, err := p.parseGameTree()
			if err != nil {
				return nil, err
			}
			collection = append(collection, gameTree)
		} else {
			p.index++ // Move to the next character
		}
	}
	return collection, nil
}

// Parses a single game tree, including its sequence and any subtrees (variations)
func (p *SGFParser) parseGameTree() (*SGFGameTree, error) {
	sequence, err := p.parseSequence()
	if err != nil {
		return nil, err
	}
	var subtrees []*SGFGameTree

	for {
		p.skipWhitespace() // Skip any whitespace before checking for '('
		if p.index < p.length && p.sgfContent[p.index] == '(' {
			p.index++ // Consume '(' before parsing subtree
			subtree, err := p.parseGameTree()
			if err != nil {
				return nil, err
			}
			subtrees = append(subtrees, subtree)
		} else {
			break // No more subtrees
		}
	}

	p.skipWhitespace() // Skip any whitespace before expecting ')'

	if p.index < p.length {
		if p.sgfContent[p.index] == ')' {
			p.index++ // Consume ')'
		} else {
			return nil, fmt.Errorf("expected ')' at index %d, found '%c'", p.index, p.sgfContent[p.index])
		}
	} else {
		return nil, fmt.Errorf("expected ')' at end of SGF, but reached end of content")
	}
	return &SGFGameTree{
		sequence: sequence,
		subtrees: subtrees,
	}, nil
}

// Parses a sequence of nodes within a game tree
func (p *SGFParser) parseSequence() ([]*SGFNode, error) {
	var nodes []*SGFNode
	for p.index < p.length && p.sgfContent[p.index] == ';' {
		node, err := p.parseNode()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// Skips whitespace characters in the SGF content
func (p *SGFParser) skipWhitespace() {
	for p.index < p.length && (p.sgfContent[p.index] == ' ' || p.sgfContent[p.index] == '\n' || p.sgfContent[p.index] == '\r' || p.sgfContent[p.index] == '\t') {
		p.index++
	}
}

// Parses a single node, extracting its properties
func (p *SGFParser) parseNode() (*SGFNode, error) {
	properties := make(map[string][]string)
	p.index++ // Skip the ';'

	for {
		p.skipWhitespace() // Skip any whitespace before the next property
		if p.index >= p.length {
			break // End of content
		}
		r, _, err := p.nextRune()
		if err != nil {
			return nil, err
		}
		if !p.isUpperCaseLetter(r) {
			break // No more properties in this node
		}
		ident, values, err := p.parseProperty()
		if err != nil {
			return nil, err
		}
		properties[ident] = values
	}

	return &SGFNode{
		properties: properties,
	}, nil
}

// Checks if a rune is an uppercase letter
func (p *SGFParser) isUpperCaseLetter(r rune) bool {
	return unicode.IsUpper(r)
}

// Parses a property, returning its identifier and associated values
func (p *SGFParser) parseProperty() (string, []string, error) {
	ident := ""
	for p.index < p.length {
		r, size, err := p.nextRune()
		if err != nil {
			return "", nil, err
		}
		if !unicode.IsUpper(r) {
			break // End of property identifier
		}
		ident += string(r)
		p.index += size
	}
	var values []string
	for p.index < p.length {
		r, _, err := p.nextRune()
		if err != nil {
			return "", nil, err
		}
		if r != '[' {
			break // No more values for this property
		}
		value, err := p.parsePropValue()
		if err != nil {
			return "", nil, err
		}
		values = append(values, value)
	}
	return ident, values, nil
}

// Retrieves the next rune from the SGF content without advancing the index
func (p *SGFParser) nextRune() (rune, int, error) {
	if p.index >= p.length {
		return 0, 0, io.EOF
	}
	r, size := utf8.DecodeRuneInString(p.sgfContent[p.index:])
	if r == utf8.RuneError && size == 1 {
		return 0, 0, fmt.Errorf("invalid UTF-8 encoding at index %d", p.index)
	}
	return r, size, nil
}

// Parses the value of a property, handling escaped characters
func (p *SGFParser) parsePropValue() (string, error) {
	p.index++ // Skip '['
	var runes []rune
	for p.index < p.length {
		r, size, err := p.nextRune()
		if err != nil {
			return "", err
		}
		if r == ']' {
			p.index += size // Skip ']'
			break           // End of property value
		}
		if r == '\\' {
			p.index += size // Skip '\\'
			if p.index >= p.length {
				return "", fmt.Errorf("unexpected end of content after '\\'")
			}
			escapedRune, escSize, err := p.nextRune()
			if err != nil {
				return "", err
			}
			runes = append(runes, escapedRune)
			p.index += escSize // Skip escaped character
		} else {
			runes = append(runes, r)
			p.index += size // Move to next rune
		}
	}
	return string(runes), nil
}

func (g *Game) initializeGameFromSGFTree(gameTree *SGFGameTree) error {
	if len(gameTree.sequence) == 0 {
		return fmt.Errorf("SGF game tree has no nodes")
	}
	rootNodeProperties := gameTree.sequence[0].properties
	// Adjust the board size based on SZ property
	sizeProp, hasSZ := rootNodeProperties["SZ"]
	if hasSZ && len(sizeProp) > 0 {
		size := sizeProp[0]
		sizes := strings.Split(size, ":")
		if len(sizes) == 2 {
			xSize, err1 := strconv.Atoi(sizes[0])
			ySize, err2 := strconv.Atoi(sizes[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("invalid SZ property: %s", size)
			}
			g.sizeX = xSize
			g.sizeY = ySize
		} else {
			sizeInt, err := strconv.Atoi(size)
			if err != nil {
				return fmt.Errorf("invalid SZ property: %s", size)
			}
			g.sizeX = sizeInt
			g.sizeY = sizeInt
		}
	} else {
		g.sizeX = 19
		g.sizeY = 19
	}
	if g.sizeX > 52 || g.sizeY > 52 {
		return fmt.Errorf("board size exceeds maximum allowed size of 52")
	}

	// Initialize the board
	g.initializeBoard()

	// Handle initial stones (AB and AW properties)
	initialBoard := g.rootNode.boardState

	abProp, hasAB := rootNodeProperties["AB"]
	if hasAB {
		for _, coord := range abProp {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid AB coordinate '%s' skipped.\n", coord)
				continue
			}
			initialBoard[xy[1]][xy[0]] = black
			g.rootNode.addBlackStone(xy[0], xy[1])
		}
	}
	awProp, hasAW := rootNodeProperties["AW"]
	if hasAW {
		for _, coord := range awProp {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid AW coordinate '%s' skipped.\n", coord)
				continue
			}
			initialBoard[xy[1]][xy[0]] = white
			g.rootNode.addWhiteStone(xy[0], xy[1])
		}
	}

	// Assign comment to root node if present
	if commentProps, hasC := gameTree.sequence[0].properties["C"]; hasC && len(commentProps) > 0 {
		g.rootNode.Comment = commentProps[0]
	}

	// Process additional properties (LB, CR, SQ, TR, MA) for the root node
	// Create a copy of properties to exclude AB, AW, C, SZ, etc.
	additionalProps := make(map[string][]string)
	for key, values := range rootNodeProperties {
		if key != "AB" && key != "AW" && key != "C" && key != "SZ" && key != "GM" && key != "FF" && key != "CA" && key != "AP" && key != "DT" && key != "GN" && key != "PC" && key != "PB" && key != "PW" && key != "BR" && key != "WR" && key != "ST" && key != "TM" && key != "OT" && key != "RE" && key != "KM" && key != "RU" {
			additionalProps[key] = values
		}
	}

	if len(additionalProps) > 0 {
		moveData, err := extractMoveFromNode(additionalProps)
		if err != nil {
			return err
		}
		// Append annotation properties to root node
		for _, coord := range moveData.CR {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid CR coordinate '%s' skipped.\n", coord)
				continue
			}
			g.rootNode.CR[xy[1]][xy[0]] = true
		}
		for _, coord := range moveData.SQ {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid CR coordinate '%s' skipped.\n", coord)
				continue
			}
			g.rootNode.SQ[xy[1]][xy[0]] = true
		}
		for _, coord := range moveData.TR {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid CR coordinate '%s' skipped.\n", coord)
				continue
			}
			g.rootNode.TR[xy[1]][xy[0]] = true
		}
		for _, coord := range moveData.MA {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid CR coordinate '%s' skipped.\n", coord)
				continue
			}
			g.rootNode.MA[xy[1]][xy[0]] = true
		}
		for coord, label := range moveData.LB {
			xy := convertSGFCoordToXY(coord)
			if xy == nil {
				fmt.Printf("Warning: Invalid LB coordinate '%s' skipped.\n", coord)
				continue
			}
			g.rootNode.LB[xy[1]][xy[0]] = label
		}
	}

	// Set the current node to the root node to ensure the comment is displayed
	g.setCurrentNode(g.rootNode)

	// Update the comment textbox to reflect the root node's comment
	g.updateCommentTextbox()

	// Start processing the main line
	var lastNode *GameTreeNode
	err := g.processMainLine(gameTree, g.rootNode, &lastNode)
	if err != nil {
		return err
	}

	// Set the current node to the last node
	if lastNode != nil {
		g.setCurrentNode(lastNode)
	}

	// Redraw the board
	g.redrawBoard()
	// Refresh the game tree UI
	g.updateGameTreeUI()

	return nil
}

func charToInt(c rune) (int, error) {
	if c >= 'a' && c <= 'z' {
		return int(c - 'a'), nil
	} else if c >= 'A' && c <= 'Z' {
		return int(c - 'A' + 26), nil
	} else {
		return 93, fmt.Errorf("invalid coordinate character: %c", c)
	}
}

// Converts SGF coordinates (e.g., "pd") to board x, y indices.
// Returns a slice with [x, y] or nil if invalid.
func convertSGFCoordToXY(coord string) []int {
	if len(coord) != 2 {
		return nil // Invalid coordinate length
	}
	x, err1 := charToInt(rune(coord[0]))
	y, err2 := charToInt(rune(coord[1]))
	if err1 != nil || err2 != nil {
		return nil // Invalid characters in coordinate
	}
	if x >= 0 && x < 52 && y >= 0 && y < 52 {
		return []int{x, y} // Valid coordinate
	}
	return nil // Coordinate out of range
}

func (g *Game) processMainLine(gameTree *SGFGameTree, parentNode *GameTreeNode, lastNode **GameTreeNode) error {
	currentParent := parentNode
	sequenceStartIndex := 0
	if currentParent == g.rootNode {
		sequenceStartIndex = 1
	}
	for i := sequenceStartIndex; i < len(gameTree.sequence); i++ {
		nodeProperties := gameTree.sequence[i].properties
		moveData, err := extractMoveFromNode(nodeProperties)
		if err != nil {
			return err
		}
		newBoardState := copyBoard(currentParent.boardState)
		newNode := g.newGameTreeNode()
		newNode.boardState = newBoardState
		newNode.parent = currentParent
		if moveData.move != nil {
			newNode.player = moveData.move.player
		} else {
			newNode.player = currentParent.player
		}
		currentParent.children = append(currentParent.children, newNode)

		// Handle move
		if moveData.move != nil {
			x, y := moveData.move.x, moveData.move.y
			player := moveData.move.player
			if x >= 0 && y >= 0 {
				// Place the stone
				newBoardState[y][x] = player
				// Capture stones and handle ko
				koX, koY := g.captureStones(newBoardState, x, y, player)
				newNode.koX = koX
				newNode.koY = koY
				newNode.move = [2]int{x, y}
			} else {
				// Pass move
				newNode.move = [2]int{-1, -1}
			}
		} else {
			// No move
			newNode.move = [2]int{93, 93}
		}

		// Apply added black stones
		for _, coord := range moveData.addedBlackStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				newBoardState[xy[1]][xy[0]] = black
				newNode.addBlackStone(xy[0], xy[1])
			}
		}

		// Apply added white stones
		for _, coord := range moveData.addedWhiteStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				newBoardState[xy[1]][xy[0]] = white
			}
		}

		// Append added empty points
		if len(moveData.addedEmptyPoints) > 0 {
			for _, coord := range moveData.addedEmptyPoints {
				xy := convertSGFCoordToXY(coord)
				if xy != nil {
					newBoardState[xy[1]][xy[0]] = empty
					newNode.AE[xy[1]][xy[0]] = true
				}
			}
		}

		// Assign comment to the new node if present
		if commentProps, hasC := nodeProperties["C"]; hasC && len(commentProps) > 0 {
			newNode.Comment = commentProps[0]
		}

		currentParent = newNode
		*lastNode = newNode
	}

	if len(gameTree.subtrees) > 0 {
		err := g.processMainLine(gameTree.subtrees[0], currentParent, lastNode)
		if err != nil {
			return err
		}
		for i := 1; i < len(gameTree.subtrees); i++ {
			err := g.processSubtree(gameTree.subtrees[i], currentParent)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type MoveData struct {
	move             *Move             // The actual move made by a player
	pass             bool              // Indicates if the move is a pass
	addedBlackStones []string          // Coordinates for additional Black stones (AB)
	addedWhiteStones []string          // Coordinates for additional White stones (AW)
	addedEmptyPoints []string          // Coordinates for making points empty (AE)
	CR               []string          // Circle annotations
	SQ               []string          // Square annotations
	TR               []string          // Triangle annotations
	MA               []string          // Mark (X) annotations
	LB               map[string]string // Labels for specific points
}

type Move struct {
	x      int    // X-coordinate of the move
	y      int    // Y-coordinate of the move
	player string // Player who made the move ("B" or "W")
}

func extractMoveFromNode(nodeProperties map[string][]string) (*MoveData, error) {
	if nodeProperties == nil {
		return nil, fmt.Errorf("node properties are nil")
	}
	var move *Move = nil
	var player string = ""
	var addedBlackStones []string
	var addedWhiteStones []string
	var addedEmptyPoints []string
	var CR []string
	var SQ []string
	var TR []string
	var MA []string
	LB := make(map[string]string)

	// Handle Black moves
	if bProp, hasB := nodeProperties["B"]; hasB {
		player = black
		coord := ""
		if len(bProp) > 0 {
			coord = bProp[0]
		}
		move = createMoveFromCoord(coord, player)
	}

	// Handle White moves
	if wProp, hasW := nodeProperties["W"]; hasW {
		player = white
		coord := ""
		if len(wProp) > 0 {
			coord = wProp[0]
		}
		move = createMoveFromCoord(coord, player)
	}

	// Handle Add Black Stones
	if abProps, hasAB := nodeProperties["AB"]; hasAB {
		addedBlackStones = append(addedBlackStones, abProps...)
	}

	// Handle Add White Stones
	if awProps, hasAW := nodeProperties["AW"]; hasAW {
		addedWhiteStones = append(addedWhiteStones, awProps...)
	}

	// Handle Add Empty Points (AE)
	if aeProps, hasAE := nodeProperties["AE"]; hasAE {
		addedEmptyPoints = append(addedEmptyPoints, aeProps...)
	}

	// Handle CR (Circle) properties
	if crProps, hasCR := nodeProperties["CR"]; hasCR {
		CR = append(CR, crProps...)
	}

	// Handle SQ (Square) properties
	if sqProps, hasSQ := nodeProperties["SQ"]; hasSQ {
		SQ = append(SQ, sqProps...)
	}

	// Handle TR (Triangle) properties
	if trProps, hasTR := nodeProperties["TR"]; hasTR {
		TR = append(TR, trProps...)
	}

	// Handle MA (Mark) properties
	if maProps, hasMA := nodeProperties["MA"]; hasMA {
		MA = append(MA, maProps...)
	}

	// Handle LB (Label) properties
	if lbProps, hasLB := nodeProperties["LB"]; hasLB {
		for _, lb := range lbProps {
			parts := strings.SplitN(lb, ":", 2)
			if len(parts) == 2 {
				point, label := parts[0], parts[1]
				LB[point] = label
			}
		}
	}

	return &MoveData{
		move:             move,
		pass:             move != nil && move.x == -1 && move.y == -1,
		addedBlackStones: addedBlackStones,
		addedWhiteStones: addedWhiteStones,
		addedEmptyPoints: addedEmptyPoints,
		CR:               CR,
		SQ:               SQ,
		TR:               TR,
		MA:               MA,
		LB:               LB,
	}, nil
}

func createMoveFromCoord(coord string, player string) *Move {
	if coord == "" {
		// Pass move
		return &Move{x: -1, y: -1, player: player}
	}
	xy := convertSGFCoordToXY(coord)
	if xy != nil {
		return &Move{x: xy[0], y: xy[1], player: player}
	}
	return nil
}

func (g *Game) processSubtree(gameTree *SGFGameTree, parentNode *GameTreeNode) error {
	currentParent := parentNode
	if currentParent == nil {
		return fmt.Errorf("current parent node is nil")
	}
	// Process the sequence of moves in the variation
	err := g.processSequence(gameTree.sequence, currentParent)
	if err != nil {
		return err
	}

	// Recursively process any sub-variations
	for _, subtree := range gameTree.subtrees {
		err := g.processSubtree(subtree, currentParent)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Game) processSequence(sequence []*SGFNode, parentNode *GameTreeNode) error {
	currentParent := parentNode
	if parentNode == nil || sequence == nil {
		return fmt.Errorf("invalid sequence or parent node, cannot process")
	}

	for _, nodeProperties := range sequence {
		moveData, err := extractMoveFromNode(nodeProperties.properties)
		if err != nil {
			return err
		}

		newBoardState := copyBoard(currentParent.boardState)

		newNode := g.newGameTreeNode()
		newNode.boardState = newBoardState
		newNode.parent = currentParent
		if moveData.move != nil {
			newNode.player = moveData.move.player
		} else {
			newNode.player = currentParent.player
		}
		currentParent.children = append(currentParent.children, newNode)

		// Handle move
		if moveData.move != nil {
			x, y := moveData.move.x, moveData.move.y
			player := moveData.move.player
			if x >= 0 && y >= 0 {
				// Place the stone
				newBoardState[y][x] = player
				// Capture stones and handle ko
				koX, koY := g.captureStones(newBoardState, x, y, player)
				newNode.koX = koX
				newNode.koY = koY
				newNode.move = [2]int{x, y}
			} else {
				// Pass move
				newNode.move = [2]int{-1, -1}
			}
		} else {
			// No move
			newNode.move = [2]int{93, 93} // Arbitrary invalid coordinates
		}

		// Append added black stones
		if len(moveData.addedBlackStones) > 0 {
			for _, coord := range moveData.addedBlackStones {
				xy := convertSGFCoordToXY(coord)
				if xy != nil {
					newBoardState[xy[1]][xy[0]] = black
					newNode.addBlackStone(xy[0], xy[1])
				}
			}
		}

		// Append added white stones
		if len(moveData.addedWhiteStones) > 0 {
			for _, coord := range moveData.addedWhiteStones {
				xy := convertSGFCoordToXY(coord)
				if xy != nil {
					newBoardState[xy[1]][xy[0]] = white
					newNode.addWhiteStone(xy[0], xy[1])
				}
			}
		}

		// Append added empty points
		if len(moveData.addedEmptyPoints) > 0 {
			for _, coord := range moveData.addedEmptyPoints {
				xy := convertSGFCoordToXY(coord)
				if xy != nil {
					newBoardState[xy[1]][xy[0]] = empty
					newNode.AE[xy[1]][xy[0]] = true
				}
			}
		}

		// Append annotation properties
		for _, coord := range moveData.CR {
			xy := convertSGFCoordToXY(coord)
			if xy[0] >= 0 && xy[1] >= 0 && xy[0] < g.sizeX && xy[1] < g.sizeY {
				newNode.CR[xy[1]][xy[0]] = true
			}
		}
		for _, coord := range moveData.SQ {
			xy := convertSGFCoordToXY(coord)
			if xy[0] >= 0 && xy[1] >= 0 && xy[0] < g.sizeX && xy[1] < g.sizeY {
				newNode.SQ[xy[1]][xy[0]] = true
			}
		}
		for _, coord := range moveData.TR {
			xy := convertSGFCoordToXY(coord)
			if xy[0] >= 0 && xy[1] >= 0 && xy[0] < g.sizeX && xy[1] < g.sizeY {
				newNode.TR[xy[1]][xy[0]] = true
			}
		}
		for _, coord := range moveData.MA {
			xy := convertSGFCoordToXY(coord)
			if xy[0] >= 0 && xy[1] >= 0 && xy[0] < g.sizeX && xy[1] < g.sizeY {
				newNode.MA[xy[1]][xy[0]] = true
			}
		}
		for coord, label := range moveData.LB {
			xy := convertSGFCoordToXY(coord)
			if xy[0] >= 0 && xy[1] >= 0 && xy[0] < g.sizeX && xy[1] < g.sizeY {
				newNode.LB[xy[1]][xy[0]] = label
			}
		}

		// Assign comment to the new node if present
		if commentProps, hasC := nodeProperties.properties["C"]; hasC && len(commentProps) > 0 {
			newNode.Comment = commentProps[0]
		}

		currentParent = newNode
	}
	return nil
}

func intToChar(n int) (string, error) {
	if n >= 0 && n <= 25 {
		// 'a' to 'z' for indices 0 to 25
		return string(rune('a' + n)), nil
	} else if n >= 26 && n <= 51 {
		// 'A' to 'Z' for indices 26 to 51
		return string(rune('A' + n - 26)), nil
	} else {
		return "", fmt.Errorf("coordinate out of range for SGF (max 52x52 board size)")
	}
}

// Converts board x, y indices to SGF coordinates.
func convertCoordinatesToSGF(x, y int) string {
	sgfX, _ := intToChar(x)
	sgfY, _ := intToChar(y)
	return sgfX + sgfY
}

// Helper function to format SGF properties for a node
func formatNodeProperties(node *GameTreeNode, isRoot bool, sizeX, sizeY int) string {
	sgf := ";"

	if isRoot {
		sgf += "FF[4]"                                          // File format version
		sgf += "GM[1]"                                          // Game type (1 = Go)
		sgf += "CA[UTF-8]"                                      // Unicode format
		sgf += "AP[ConnectedGroupsGobanVersion" + version + "]" // Application name
		if sizeX == sizeY {
			sgf += fmt.Sprintf("SZ[%d]", sizeX)
		} else {
			sgf += fmt.Sprintf("SZ[%d:%d]", sizeX, sizeY)
		}
	}

	if !isRoot && node.move[0] >= 0 && node.move[0] < sizeX && node.move[1] >= 0 && node.move[1] < sizeY {
		if node.player == black {
			sgf += "B"
		} else if node.player == white {
			sgf += "W"
		}
		coords := convertCoordinatesToSGF(node.move[0], node.move[1])
		sgf += fmt.Sprintf("[%s]", coords)
	}

	if node.Comment != "" {
		escapedComment := strings.ReplaceAll(node.Comment, "\\", "\\\\")
		escapedComment = strings.ReplaceAll(escapedComment, "]", "\\]")
		sgf += fmt.Sprintf("C[%s]", escapedComment)
	}

	sgf += formatAnnotations(node)
	sgf += formatAddedStones(node)

	return sgf
}

// Formats annotations (CR, SQ, TR, MA) for a node
func formatAnnotations(node *GameTreeNode) string {
	annotations := ""

	circleText := "CR"
	for y, arr := range node.CR {
		for x, el := range arr {
			if el {
				circleText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if circleText != "CR" {
		annotations += circleText
	}

	squareText := "SQ"
	for y, arr := range node.SQ {
		for x, el := range arr {
			if el {
				squareText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if squareText != "SQ" {
		annotations += squareText
	}

	triangleText := "TR"
	for y, arr := range node.TR {
		for x, el := range arr {
			if el {
				triangleText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if triangleText != "TR" {
		annotations += triangleText
	}

	xMarkText := "MA"
	for y, arr := range node.MA {
		for x, el := range arr {
			if el {
				xMarkText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if xMarkText != "MA" {
		annotations += xMarkText
	}

	labelsText := "LB"
	for y, arr := range node.LB {
		for x, el := range arr {
			if el != "" {
				labelsText += "[" + convertCoordinatesToSGF(x, y) + ":" + el + "]"
			}
		}
	}
	if labelsText != "LB" {
		annotations += labelsText
	}

	return annotations
}

// Formats added black and white stones for a node
func formatAddedStones(node *GameTreeNode) string {
	addedStones := ""

	blackStonesText := "AB"
	for y, arr := range node.addedBlackStones {
		for x, el := range arr {
			if el {
				blackStonesText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if blackStonesText != "AB" {
		addedStones += blackStonesText
	}

	whiteStonesText := "AW"
	for y, arr := range node.addedWhiteStones {
		for x, el := range arr {
			if el {
				whiteStonesText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if whiteStonesText != "AW" {
		addedStones += whiteStonesText
	}

	// Add this code to handle AE properties
	emptyPointsText := "AE"
	for y, arr := range node.AE {
		for x, el := range arr {
			if el {
				emptyPointsText += "[" + convertCoordinatesToSGF(x, y) + "]"
			}
		}
	}
	if emptyPointsText != "AE" {
		addedStones += emptyPointsText
	}

	return addedStones
}

func generateSGF(node *GameTreeNode, sizeX, sizeY int) string {
	sgf := "(" // Start of variation

	// Add the properties for the current node
	sgf += formatNodeProperties(node, node.parent == nil, sizeX, sizeY)

	// Recursively generate SGF for child nodes (variations)
	if len(node.children) > 0 {
		if len(node.children) == 1 {
			// Continue the main line without starting a new variation
			childSGF := generateSGF(node.children[0], sizeX, sizeY)
			childSGF = childSGF[1 : len(childSGF)-1] // Remove outer parentheses to nest within the current variation
			sgf += childSGF
		} else {
			// Multiple variations; each variation is enclosed in parentheses
			for _, child := range node.children {
				sgf += generateSGF(child, sizeX, sizeY)
			}
		}
	}

	sgf += ")" // End of variation
	return sgf
}

func (g *Game) handlePass() {
	newNode := g.newGameTreeNode()
	newNode.boardState = copyBoard(g.currentNode.boardState)
	newNode.player = switchPlayer(g.currentNode.player) // Set the player who passed
	newNode.move = [2]int{-1, -1}
	newNode.parent = g.currentNode
	g.currentNode.children = append(g.currentNode.children, newNode)
	g.currentNode = newNode
	g.updateGameTreeUI()
	g.updateCommentTextbox()
	g.redrawBoard()
	if g.mouseMode == "score" {
		g.exitScoringMode()
	}
	if g.gtpCmd != nil {
		player := newNode.player
		if _, err := g.sendGTPCommand(fmt.Sprintf("play %s pass", player)); err != nil {
			dialog.ShowError(err, g.window)
			g.detachEngine()
		} else {
			// Get engine's response move
			engineMove, err := g.sendGTPCommand(fmt.Sprintf("genmove %s", switchPlayer(player)))
			if err != nil {
				dialog.ShowError(err, g.window)
				g.detachEngine()
			} else {
				if engineMove == "pass" {
					dialog.ShowInformation("Game Over", "Both players passed.", g.window)
				} else {
					g.handleEngineMove(engineMove)
				}
			}
		}
	}
}
