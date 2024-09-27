package main

import (
	"fmt"
	"image/color"
	"io"
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
	version           = "1"
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
)

type Game struct {
	sizeX             int
	sizeY             int
	player            string
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
	inScoringMode     bool
	territoryMap      [][]string
	territoryLayer    *fyne.Container
	scoringStatus     *widget.Label
	commentEntry      *widget.Entry
}

type GameTreeNode struct {
	boardState       [][]string
	move             [2]int
	player           string
	children         []*GameTreeNode
	parent           *GameTreeNode
	id               string
	koX              int
	koY              int
	Comment          string
	addedBlackStones []string // New field for AB properties
	addedWhiteStones []string // New field for AW properties
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
	a := app.New()
	w := a.NewWindow("Connected Groups Goban Version " + version)
	game := &Game{
		player:  black,
		window:  w,
		nodeMap: make(map[string]*GameTreeNode),
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
	game.drawBoard()

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
					game.drawBoard()
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
		fyne.NewMenuItem("Toggle Scoring Mode", func() {
			game.toggleScoringMode()
		}),
	)

	// Create the main menu and set it to the window
	mainMenu := fyne.NewMainMenu(
		fileMenu,
		gameMenu,
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

	// Main layout with split view
	content := container.NewHSplit(
		controls,
		game.boardCanvas,
	)
	w.SetContent(content)
	w.Resize(fyne.NewSize(800, 600))
	w.Show()

	a.Run()
}

func (g *Game) updateCommentTextbox() {
	if g.currentNode != nil && g.currentNode.Comment != "" {
		g.commentEntry.SetText(g.currentNode.Comment)
	} else {
		g.commentEntry.SetText("") // Clears the textbox if there's no comment
	}
}

func (g *Game) toggleScoringMode() {
	if g.inScoringMode {
		g.exitScoringMode()
	} else {
		g.enterScoringMode()
	}
}

func (g *Game) enterScoringMode() {
	g.inScoringMode = true
	g.initializeTerritoryMap()
	g.assignTerritoryToEmptyRegions()
	g.redrawBoard()
	g.calculateAndDisplayScore()
	// Hide the hoverStone
	if g.hoverStone != nil {
		g.gridContainer.Remove(g.hoverStone)
		g.hoverStone = nil
	}
}

func (g *Game) exitScoringMode() {
	g.inScoringMode = false
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

func (g *Game) calculateScore() (int, int) {
	blackScore := 0
	whiteScore := 0
	for y := 0; y < g.sizeY; y++ {
		for x := 0; x < g.sizeX; x++ {
			owner := g.territoryMap[y][x]
			if owner == black {
				blackScore++
			} else if owner == white {
				whiteScore++
			}
		}
	}
	return blackScore, whiteScore
}

func (g *Game) calculateAndDisplayScore() {
	blackScore, whiteScore := g.calculateScore()
	g.scoringStatus.SetText(fmt.Sprintf("Black: %d, White: %d", blackScore, whiteScore))
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
	} else if len(node.addedBlackStones) > 0 || len(node.addedWhiteStones) > 0 {
		// Handle added stones
		labels := []string{}

		// Process added Black stones
		for _, coord := range node.addedBlackStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				label := fmt.Sprintf("+B:(%d,%d)", xy[0], xy[1])
				labels = append(labels, label)
			} else {
				labels = append(labels, fmt.Sprintf("+B:Invalid(%s)", coord))
			}
		}

		// Process added White stones
		for _, coord := range node.addedWhiteStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				label := fmt.Sprintf("+W:(%d,%d)", xy[0], xy[1])
				labels = append(labels, label)
			} else {
				labels = append(labels, fmt.Sprintf("+W:Invalid(%s)", coord))
			}
		}

		nodeLabel = strings.Join(labels, ", ")
	} else if node.move[0] == -1 && node.move[1] == -1 {
		nodeLabel = fmt.Sprintf("%s:Pass", switchPlayer(node.player))
	} else {
		nodeLabel = fmt.Sprintf("%s:(%d,%d)", switchPlayer(node.player), node.move[0], node.move[1])
	}

	// Create a button for the node
	nodeButton := widget.NewButton(nodeLabel, func() {
		// Add the following lines to exit scoring mode if active
		if g.inScoringMode {
			g.exitScoringMode()
		}

		g.setCurrentNode(node)
		g.redrawBoard()
		g.updateGameTreeUI()
	})

	// Highlight the current node
	if node == g.currentNode {
		nodeButton.Importance = widget.HighImportance
	}

	// Handle children nodes
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
	rootNode := &GameTreeNode{
		boardState: makeEmptyBoard(g.sizeX, g.sizeY),
		player:     black,
		id:         fmt.Sprintf("%d", g.idCounter),
		koX:        -1,
		koY:        -1,
	}
	g.rootNode = rootNode
	g.currentNode = rootNode
	g.player = rootNode.player
	g.nodeMap = make(map[string]*GameTreeNode)
	g.nodeMap[rootNode.id] = rootNode
	if g.inScoringMode {
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
	g.player = node.player
	g.updateCommentTextbox()
}

func (g *Game) drawBoard() {
	g.hoverStone = nil
	g.redrawBoard()
}

func (g *Game) redrawBoard() {
	// Clear previous grid lines and stones
	g.gridContainer.Objects = nil

	// Hide gridContainer for optimized efficiency
	g.gridContainer.Hide()

	// If territoryLayer exists, remove it
	if g.territoryLayer != nil {
		g.gridContainer.Remove(g.territoryLayer)
	}

	size := g.boardCanvas.Size()
	g.cellSize = min(size.Width/float32(g.sizeX), size.Height/float32(g.sizeY))

	// Draw grid lines
	for x := 0; x < g.sizeX; x++ {
		line := canvas.NewLine(lineColor)
		pos := g.boardCoordsToPixel(x, 0)
		line.Position1 = fyne.NewPos(pos.X+0.5*g.cellSize, pos.Y+(0.5-gridLineThickness/2)*g.cellSize)
		pos = g.boardCoordsToPixel(x, g.sizeY-1)
		line.Position2 = fyne.NewPos(pos.X+0.5*g.cellSize, pos.Y+(0.5+gridLineThickness/2)*g.cellSize)
		line.StrokeWidth = g.cellSize * gridLineThickness
		g.gridContainer.Add(line)
	}
	for y := 0; y < g.sizeY; y++ {
		line := canvas.NewLine(lineColor)
		pos := g.boardCoordsToPixel(0, y)
		line.Position1 = fyne.NewPos(pos.X+(0.5-gridLineThickness/2)*g.cellSize, pos.Y+0.5*g.cellSize)
		pos = g.boardCoordsToPixel(g.sizeX-1, y)
		line.Position2 = fyne.NewPos(pos.X+(0.5+gridLineThickness/2)*g.cellSize, pos.Y+0.5*g.cellSize)
		line.StrokeWidth = g.cellSize * gridLineThickness
		g.gridContainer.Add(line)
	}

	// Draw 4-square stone connections
	for y := 1; y < g.sizeY; y++ {
		for x := 1; x < g.sizeX; x++ {
			stone1 := g.currentNode.boardState[y][x-1]
			stone2 := g.currentNode.boardState[y][x]
			stone3 := g.currentNode.boardState[y-1][x-1]
			stone4 := g.currentNode.boardState[y-1][x]
			if stone1 != empty && stone2 != empty && stone3 != empty && stone4 != empty {
				if stone3 == stone2 && stone1 == stone4 && stone1 != stone2 { // Rule out cross cut
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

	// Draw stones
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

	// Draw territory markers if in scoring mode
	if g.inScoringMode {
		// Create new territoryLayer
		g.territoryLayer = container.NewWithoutLayout()
		// Draw territory markers
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

	g.gridContainer.Show()
	g.gridContainer.Refresh()
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

func (g *Game) pixelToBoardCoords(pos fyne.Position) (int, int, bool) {
	size := g.boardCanvas.Size()
	x := int(((pos.X*2-size.Width)/g.cellSize + float32(g.sizeX)) / 2)
	y := int(((pos.Y*2-size.Height)/g.cellSize + float32(g.sizeY)) / 2)

	if x < 0 || x >= g.sizeX || y < 0 || y >= g.sizeY {
		return 0, 0, false
	}

	return x, y, true
}

func (g *Game) boardCoordsToPixel(x, y int) fyne.Position {
	size := g.boardCanvas.Size()
	return fyne.NewPos(
		(float32(2*x-g.sizeX)*g.cellSize+size.Width)/2,
		(float32(2*y-g.sizeY)*g.cellSize+size.Height)/2,
	)
}

func (g *Game) handleMouseMove(ev *desktop.MouseEvent) {
	if g.inScoringMode {
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

	if g.currentNode.boardState[y][x] != empty || !g.isMoveLegal(x, y, g.player) {
		if g.hoverStone != nil {
			g.gridContainer.Remove(g.hoverStone)
			g.hoverStone = nil
			g.gridContainer.Refresh()
		}
		return
	}

	// Remove previous hover stone
	if g.hoverStone != nil {
		g.gridContainer.Remove(g.hoverStone)
	}

	// Add new hover stone
	circle := canvas.NewCircle(transparentBlackColor)
	if g.player == white {
		circle.FillColor = transparentWhiteColor
	}
	circle.StrokeWidth = 0
	circle.Resize(fyne.NewSize(g.cellSize, g.cellSize))
	circle.Move(g.boardCoordsToPixel(x, y))
	g.gridContainer.Add(circle)
	g.hoverStone = circle
	g.gridContainer.Refresh()
}

func (g *Game) handleMouseClick(ev *fyne.PointEvent) {
	x, y, ok := g.pixelToBoardCoords(ev.Position)
	if !ok {
		return
	}

	if g.inScoringMode {
		g.toggleGroupStatus(x, y)
		g.assignTerritoryToEmptyRegions()
		g.redrawBoard()
		g.calculateAndDisplayScore()
		return
	}

	if g.currentNode.boardState[y][x] != empty {
		return
	}

	if !g.isMoveLegal(x, y, g.player) {
		return
	}

	// Check if the current node has added stones
	if len(g.currentNode.addedBlackStones) > 0 || len(g.currentNode.addedWhiteStones) > 0 {
		// Do not allow moves immediately after adding stones
		return
	}

	// Copy the current board state
	boardCopy := copyBoard(g.currentNode.boardState)

	// Place stone
	boardCopy[y][x] = g.player

	// Capture any opponent stones, possibly updating koX and koY
	koX, koY := g.captureStones(boardCopy, x, y, g.player)

	// Create new game tree node
	g.idCounter++
	newNode := &GameTreeNode{
		boardState: boardCopy,
		move:       [2]int{x, y},
		player:     switchPlayer(g.player),
		parent:     g.currentNode,
		id:         fmt.Sprintf("%d", g.idCounter),
		koX:        koX,
		koY:        koY,
	}
	g.currentNode.children = append(g.currentNode.children, newNode)
	g.nodeMap[newNode.id] = newNode
	g.currentNode = newNode
	g.player = newNode.player

	// Refresh the game tree UI
	g.updateGameTreeUI()

	// Update the comment textbox
	g.updateCommentTextbox()

	g.redrawBoard()
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

func hasLiberty(board [][]string, x, y int, player string, sizeX, sizeY int) bool {
	visited := make(map[[2]int]bool)
	return dfs(board, x, y, player, visited, sizeX, sizeY)
}

func dfs(board [][]string, x, y int, player string, visited map[[2]int]bool, sizeX, sizeY int) bool {
	if x < 0 || x >= sizeX || y < 0 || y >= sizeY {
		return false
	}

	if visited[[2]int{x, y}] {
		return false
	}

	if board[y][x] == empty {
		return true
	}

	if board[y][x] != player {
		return false
	}

	visited[[2]int{x, y}] = true

	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range dirs {
		if dfs(board, x+d[0], y+d[1], player, visited, sizeX, sizeY) {
			return true
		}
	}
	return false
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
	sgfContent string
	index      int
	length     int
}

type SGFGameTree struct {
	sequence []*SGFNode
	subtrees []*SGFGameTree
}

type SGFNode struct {
	properties map[string][]string
}

func parseSGF(sgfContent string) ([]*SGFGameTree, error) {
	parser := &SGFParser{
		sgfContent: sgfContent,
		index:      0,
		length:     len(sgfContent),
	}
	collection, err := parser.parseCollection()
	if err != nil {
		return nil, fmt.Errorf("error parsing SGF at index %d: %v", parser.index, err)
	}
	return collection, nil
}

func (p *SGFParser) parseCollection() ([]*SGFGameTree, error) {
	var collection []*SGFGameTree
	for p.index < p.length {
		if p.sgfContent[p.index] == '(' {
			p.index++
			gameTree, err := p.parseGameTree()
			if err != nil {
				return nil, err
			}
			collection = append(collection, gameTree)
		} else {
			p.index++
		}
	}
	return collection, nil
}

func (p *SGFParser) parseGameTree() (*SGFGameTree, error) {
	sequence, err := p.parseSequence()
	if err != nil {
		return nil, err
	}
	var subtrees []*SGFGameTree
	for p.index < p.length && p.sgfContent[p.index] == '(' {
		p.index++
		subtree, err := p.parseGameTree()
		if err != nil {
			return nil, err
		}
		subtrees = append(subtrees, subtree)
	}
	if p.index < p.length && p.sgfContent[p.index] == ')' {
		p.index++
	} else {
		return nil, fmt.Errorf("expected ')' at index %d", p.index)
	}
	return &SGFGameTree{
		sequence: sequence,
		subtrees: subtrees,
	}, nil
}

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

// Add the following method to the SGFParser struct
func (p *SGFParser) skipWhitespace() {
	for p.index < p.length && (p.sgfContent[p.index] == ' ' || p.sgfContent[p.index] == '\n' || p.sgfContent[p.index] == '\r' || p.sgfContent[p.index] == '\t') {
		p.index++
	}
}

func (p *SGFParser) parseNode() (*SGFNode, error) {
	properties := make(map[string][]string)
	p.index++ // Skip the ';'

	for {
		p.skipWhitespace() // Skip any whitespace before the next property
		if p.index >= p.length {
			break
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

// Update the isUpperCaseLetter method to accept runes
func (p *SGFParser) isUpperCaseLetter(r rune) bool {
	return unicode.IsUpper(r)
}

func (p *SGFParser) parseProperty() (string, []string, error) {
	ident := ""
	for p.index < p.length {
		r, size, err := p.nextRune()
		if err != nil {
			return "", nil, err
		}
		if !unicode.IsUpper(r) {
			break
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
			break
		}
		value, err := p.parsePropValue()
		if err != nil {
			return "", nil, err
		}
		values = append(values, value)
	}
	return ident, values, nil
}

// Add this method to the SGFParser struct
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
			break
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
			g.rootNode.addedBlackStones = append(g.rootNode.addedBlackStones, coord)
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
			g.rootNode.addedWhiteStones = append(g.rootNode.addedWhiteStones, coord)
		}
	}

	// Assign comment to root node if present
	if commentProps, hasC := gameTree.sequence[0].properties["C"]; hasC && len(commentProps) > 0 {
		g.rootNode.Comment = commentProps[0]
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

func convertSGFCoordToXY(coord string) []int {
	if len(coord) != 2 {
		return nil
	}
	x, err1 := charToInt(rune(coord[0]))
	y, err2 := charToInt(rune(coord[1]))
	if err1 != nil || err2 != nil {
		return nil
	}
	if x >= 0 && x < 52 && y >= 0 && y < 52 {
		return []int{x, y}
	} else {
		return nil
	}
}

func (g *Game) processMainLine(gameTree *SGFGameTree, parentNode *GameTreeNode, lastNode **GameTreeNode) error {
	currentParent := parentNode
	sequenceStartIndex := 0
	if currentParent == g.rootNode {
		sequenceStartIndex = 1 // Skip root node as we have already processed it
	}
	for i := sequenceStartIndex; i < len(gameTree.sequence); i++ {
		nodeProperties := gameTree.sequence[i].properties
		moveData, err := extractMoveFromNode(nodeProperties)
		if err != nil {
			return err
		}
		// Copy the board state
		newBoardState := copyBoard(currentParent.boardState)
		newNode := &GameTreeNode{
			boardState:       newBoardState,
			parent:           currentParent,
			id:               fmt.Sprintf("%d", g.idCounter),
			koX:              -1,
			koY:              -1,
			addedBlackStones: moveData.addedBlackStones, // Assign added stones
			addedWhiteStones: moveData.addedWhiteStones,
		}
		g.idCounter++
		g.nodeMap[newNode.id] = newNode
		currentParent.children = append(currentParent.children, newNode)

		// Apply added black stones
		for _, coord := range moveData.addedBlackStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				newBoardState[xy[1]][xy[0]] = black
			}
		}

		// Apply added white stones
		for _, coord := range moveData.addedWhiteStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				newBoardState[xy[1]][xy[0]] = white
			}
		}

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
			newNode.player = switchPlayer(player)
		} else {
			// No move; just carry over the player
			newNode.player = switchPlayer(currentParent.player)
		}

		// Assign comment to the new node if present
		if commentProps, hasC := nodeProperties["C"]; hasC && len(commentProps) > 0 {
			newNode.Comment = commentProps[0]
		}

		currentParent = newNode
		*lastNode = newNode // Update lastNode to the current node
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
	move             *Move
	pass             bool
	addedBlackStones []string // field for AB properties
	addedWhiteStones []string // field for AW properties
}

type Move struct {
	x      int
	y      int
	player string
}

func extractMoveFromNode(nodeProperties map[string][]string) (*MoveData, error) {
	var move *Move = nil
	var player string = ""
	var addedBlackStones []string
	var addedWhiteStones []string

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
		addedBlackStones = abProps
	}

	// Handle Add White Stones
	if awProps, hasAW := nodeProperties["AW"]; hasAW {
		addedWhiteStones = awProps
	}

	return &MoveData{
		move:             move,
		pass:             move != nil && move.x == -1 && move.y == -1,
		addedBlackStones: addedBlackStones,
		addedWhiteStones: addedWhiteStones,
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

	for _, nodeProperties := range sequence {
		moveData, err := extractMoveFromNode(nodeProperties.properties)
		if err != nil {
			return err
		}
		// Copy the board state
		newBoardState := copyBoard(currentParent.boardState)
		newNode := &GameTreeNode{
			boardState:       newBoardState,
			parent:           currentParent,
			id:               fmt.Sprintf("%d", g.idCounter),
			koX:              -1,
			koY:              -1,
			addedBlackStones: moveData.addedBlackStones, // Assign added stones
			addedWhiteStones: moveData.addedWhiteStones,
		}
		g.idCounter++
		g.nodeMap[newNode.id] = newNode
		currentParent.children = append(currentParent.children, newNode)

		// Apply added black stones
		for _, coord := range moveData.addedBlackStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				newBoardState[xy[1]][xy[0]] = black
			}
		}

		// Apply added white stones
		for _, coord := range moveData.addedWhiteStones {
			xy := convertSGFCoordToXY(coord)
			if xy != nil {
				newBoardState[xy[1]][xy[0]] = white
			}
		}

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
			newNode.player = switchPlayer(player)
		} else {
			// No move; just carry over the player
			newNode.player = switchPlayer(currentParent.player)
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

func convertCoordinatesToSGF(x, y int) string {
	sgfX, _ := intToChar(x)
	sgfY, _ := intToChar(y)
	return sgfX + sgfY
}

func generateSGF(node *GameTreeNode, sizeX, sizeY int) string {
	sgf := ""
	sgf += "(" // Start of variation

	if node.parent == nil {
		// Root node properties
		sgf += ";"
		sgf += "FF[4]"                                          // File format version
		sgf += "GM[1]"                                          // Game type (1 = Go)
		sgf += "CA[UTF-8]"                                      // Unicode format
		sgf += "AP[ConnectedGroupsGobanVersion" + version + "]" // Application name

		// Adjust the board size property for rectangular boards
		if sizeX == sizeY {
			sgf += fmt.Sprintf("SZ[%d]", sizeX)
		} else {
			sgf += fmt.Sprintf("SZ[%d:%d]", sizeX, sizeY)
		}

		// Collect initial stones
		blackStones := node.addedBlackStones
		whiteStones := node.addedWhiteStones

		// Add initial stones to SGF
		if len(blackStones) > 0 {
			sgf += "AB"
			for _, coord := range blackStones {
				sgf += fmt.Sprintf("[%s]", coord)
			}
		}
		if len(whiteStones) > 0 {
			sgf += "AW"
			for _, coord := range whiteStones {
				sgf += fmt.Sprintf("[%s]", coord)
			}
		}

		// Include comment if present
		if node.Comment != "" {
			// Escape ']' and '\' characters in comments
			escapedComment := strings.ReplaceAll(node.Comment, "\\", "\\\\")
			escapedComment = strings.ReplaceAll(escapedComment, "]", "\\]")
			sgf += fmt.Sprintf("C[%s]", escapedComment)
		}
	} else {
		// Add move
		sgf += ";"
		move := node.move
		moveStr := ""
		if node.parent.player == black {
			moveStr += "B"
		} else {
			moveStr += "W"
		}
		if move[0] >= 0 && move[1] >= 0 {
			coords := convertCoordinatesToSGF(move[0], move[1])
			moveStr += fmt.Sprintf("[%s]", coords)
		} else {
			// Pass move
			moveStr += "[]"
		}

		// Include comment if present
		if node.Comment != "" {
			// Escape ']' and '\' characters in comments
			escapedComment := strings.ReplaceAll(node.Comment, "\\", "\\\\")
			escapedComment = strings.ReplaceAll(escapedComment, "]", "\\]")
			moveStr += fmt.Sprintf("C[%s]", escapedComment)
		}

		sgf += moveStr

		// Handle added stones in variations
		if len(node.addedBlackStones) > 0 {
			for _, coord := range node.addedBlackStones {
				sgf += fmt.Sprintf("AB[%s]", coord)
			}
		}
		if len(node.addedWhiteStones) > 0 {
			for _, coord := range node.addedWhiteStones {
				sgf += fmt.Sprintf("AW[%s]", coord)
			}
		}
	}

	if len(node.children) > 0 {
		if len(node.children) == 1 {
			// Continue the main line without starting a new variation
			childSGF := generateSGF(node.children[0], sizeX, sizeY)
			// Remove outer parentheses
			childSGF = childSGF[1 : len(childSGF)-1]
			sgf += childSGF
		} else {
			// Multiple variations
			for _, child := range node.children {
				sgf += generateSGF(child, sizeX, sizeY)
			}
		}
	}

	sgf += ")" // End of variation
	return sgf
}

func (g *Game) handlePass() {
	// Create new game tree node representing a pass move
	g.idCounter++
	newNode := &GameTreeNode{
		boardState: copyBoard(g.currentNode.boardState),
		move:       [2]int{-1, -1}, // Pass move represented by (-1, -1)
		player:     switchPlayer(g.player),
		parent:     g.currentNode,
		id:         fmt.Sprintf("%d", g.idCounter),
		koX:        -1,
		koY:        -1,
	}
	g.currentNode.children = append(g.currentNode.children, newNode)
	g.nodeMap[newNode.id] = newNode
	g.currentNode = newNode
	g.player = newNode.player

	// Refresh the game tree UI
	g.updateGameTreeUI()

	// Update the comment textbox
	g.updateCommentTextbox()

	g.redrawBoard()

	if g.inScoringMode {
		g.exitScoringMode()
	}
}
