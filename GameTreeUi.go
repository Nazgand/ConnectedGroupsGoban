package main

import (
	"fmt"
	"image/color"
	"maps"
	"math"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// GameTreeLayout is a fyne.Layout that sizes the game-tree scroll container
// to fill its parent and re-centers the current node on every layout pass,
// so the selected button stays near the viewport center as the split is resized.
type GameTreeLayout struct {
	Win *GoWin
}

// Layout sizes the single child (the Scroll container) to fill the given size,
// then re-centers the selected node within the scroll viewport.
func (L *GameTreeLayout) Layout(Objects []fyne.CanvasObject, Size fyne.Size) {
	for _, O := range Objects {
		O.Resize(Size)
		O.Move(fyne.NewPos(0, 0))
	}
	L.Win.CenterGameTreeOnCurrentNode()
}

// MinSize returns the max MinSize of the objects.
func (L *GameTreeLayout) MinSize(Objects []fyne.CanvasObject) fyne.Size {
	Min := fyne.NewSize(0, 0)
	for _, O := range Objects {
		M := O.MinSize()
		if M.Width > Min.Width {
			Min.Width = M.Width
		}
		if M.Height > Min.Height {
			Min.Height = M.Height
		}
	}
	return Min
}

// FixedSizeLayout is a fyne.Layout that reports a fixed MinSize and leaves
// child positions and sizes untouched (children are placed manually via
// Move/Resize). Used as the layout for the flat game-tree content container
// so the enclosing Scroll sees the true content size and enables scrolling.
type FixedSizeLayout struct {
	Size fyne.Size
}

// Layout is a no-op: children keep the positions assigned manually.
func (L *FixedSizeLayout) Layout(_ []fyne.CanvasObject, _ fyne.Size) {}

// MinSize returns the fixed content size.
func (L *FixedSizeLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return L.Size
}

// GameDisplayName returns a display label for a game in the collection.
// Uses Information["Game Name"] when nonempty, otherwise joins bracketed player names.
func GameDisplayName(Hist *GameHistory, _ int) string {
	// Try Game Name first
	if GameName, Exists := Hist.Information["Game Name"]; Exists && GameName != nil && *GameName != "" {
		return *GameName
	}

	PlayerNames := make([]string, int(Hist.Players))
	for Index := range PlayerNames {
		PlayerNames[Index] = "〘" + Hist.PlayerNames[uint8(Index+1)] + "〙"
	}
	return strings.Join(PlayerNames, " vs ")
}

// TreeNodeButton is a Button that also implements desktop.Hoverable.
// When HasCustomColor is true, the button renders with BackgroundColor
// and TextColor instead of the default theme colors.
// LayoutX/LayoutY store the button's authoritative position within the flat
// game-tree container (set during buildGameTreeUI), used for centering math
// without depending on Fyne's transient layout state.
type TreeNodeButton struct {
	widget.Button
	OnHoverIn       func()
	OnHoverOut      func()
	HasCustomColor  bool
	Selected        bool
	BackgroundColor color.NRGBA
	TextColor       color.NRGBA
	LayoutX         float32
	LayoutY         float32
}

// TreeNodeButtonRenderer draws a TreeNodeButton with optional custom colors.
type TreeNodeButtonRenderer struct {
	Button     *TreeNodeButton
	Background *canvas.Rectangle
	Border     *canvas.Rectangle
	Label      *canvas.Text
	Objects_   []fyne.CanvasObject
}

// CreateRenderer always uses the custom renderer so font size and padding
// are consistent regardless of whether a custom stone color is set.
func (Button *TreeNodeButton) CreateRenderer() fyne.WidgetRenderer {
	Background := canvas.NewRectangle(Button.BackgroundColor)
	Background.CornerRadius = 0
	Border := canvas.NewRectangle(color.NRGBA{})
	Border.FillColor = color.NRGBA{}
	Border.CornerRadius = 0
	Border.StrokeWidth = 1.39
	Label := canvas.NewText(Button.Text, Button.TextColor)
	Label.Alignment = fyne.TextAlignCenter
	Label.TextStyle.Bold = true
	Label.TextSize = 9.3
	return &TreeNodeButtonRenderer{
		Button:     Button,
		Background: Background,
		Border:     Border,
		Label:      Label,
		Objects_:   []fyne.CanvasObject{Background, Border, Label},
	}
}

// MinSize returns the minimum size for the custom-colored button.
func (R *TreeNodeButtonRenderer) MinSize() fyne.Size {
	Padding := theme.InnerPadding() * 2
	TextSize := fyne.MeasureText(R.Label.Text, R.Label.TextSize, R.Label.TextStyle)
	return fyne.NewSize(TextSize.Width+Padding, TextSize.Height+Padding)
}

// Layout positions the background, border, and label within the given size.
func (R *TreeNodeButtonRenderer) Layout(Size fyne.Size) {
	R.Background.Resize(Size)
	R.Background.Move(fyne.NewPos(0, 0))
	BorderInset := float32(1.93)
	R.Border.Resize(fyne.NewSize(Size.Width-BorderInset*2, Size.Height-BorderInset*2))
	R.Border.Move(fyne.NewPos(BorderInset, BorderInset))
	TextSize := fyne.MeasureText(R.Label.Text, R.Label.TextSize, R.Label.TextStyle)
	R.Label.Resize(fyne.NewSize(Size.Width, TextSize.Height))
	R.Label.Move(fyne.NewPos(0, (Size.Height-TextSize.Height)/2))
}

// Refresh updates the renderer's visual state from the button fields.
func (R *TreeNodeButtonRenderer) Refresh() {
	if R.Button.HasCustomColor {
		R.Background.FillColor = R.Button.BackgroundColor
		R.Label.Color = R.Button.TextColor
	} else {
		R.Background.FillColor = theme.Color(theme.ColorNameButton)
		R.Label.Color = theme.Color(theme.ColorNameForeground)
	}
	R.Background.Refresh()
	if R.Button.Selected {
		if R.Button.HasCustomColor {
			R.Border.StrokeColor = R.Button.TextColor
		} else {
			R.Border.StrokeColor = theme.Color(theme.ColorNamePrimary)
		}
	} else {
		R.Border.StrokeColor = color.NRGBA{}
	}
	R.Border.Refresh()
	R.Label.Text = R.Button.Text
	R.Label.Refresh()
}

// Objects returns the canvas objects managed by this renderer.
func (R *TreeNodeButtonRenderer) Objects() []fyne.CanvasObject { return R.Objects_ }

// Destroy is a no-op.
func (R *TreeNodeButtonRenderer) Destroy() {}

// NewTreeNodeButton constructs a TreeNodeButton with the given label text,
// tap callback, and hover-in/hover-out callbacks.
func NewTreeNodeButton(label string, onTap func(), onHoverIn func(), onHoverOut func()) *TreeNodeButton {
	Button := &TreeNodeButton{OnHoverIn: onHoverIn, OnHoverOut: onHoverOut}
	Button.Text = label
	Button.OnTapped = onTap
	Button.ExtendBaseWidget(Button)
	return Button
}

// MouseIn is called by Fyne when the mouse enters the button; it triggers OnHoverIn.
func (Button *TreeNodeButton) MouseIn(_ *desktop.MouseEvent) { Button.OnHoverIn() }

// MouseMoved is called by Fyne when the mouse moves within the button (no-op).
func (Button *TreeNodeButton) MouseMoved(_ *desktop.MouseEvent) {}

// MouseOut is called by Fyne when the mouse leaves the button; it triggers OnHoverOut.
func (Button *TreeNodeButton) MouseOut() { Button.OnHoverOut() }

// RefreshGameSelector rebuilds the game selector dropdown options and sets
// the selected option to match the current game. Sets the Selected field
// directly (not via SetSelectedIndex) to avoid re-triggering OnChanged.
func (GoWin *GoWin) RefreshGameSelector() {
	if GoWin.GameSelectorSelect == nil {
		return
	}

	// Build options from CollectionNode children (games)
	Options := []string{}
	for Index, Child := range GoWin.Coll.CollectionNode.Children {
		if Child.Board != nil && Child.Board.Hist != nil {
			DisplayName := GameDisplayName(Child.Board.Hist, Index)
			Options = append(Options, DisplayName)
		}
	}

	GoWin.GameSelectorSelect.Options = Options

	// Set selected option to current game without triggering OnChanged
	if GoWin.Coll.CurrentGameH != nil {
		for Index, Child := range GoWin.Coll.CollectionNode.Children {
			if Child.Board != nil && Child.Board.Hist == GoWin.Coll.CurrentGameH {
				if Index < len(Options) {
					GoWin.GameSelectorSelect.Selected = Options[Index]
				}
				break
			}
		}
	} else {
		GoWin.GameSelectorSelect.Selected = ""
	}

	GoWin.GameSelectorSelect.Refresh()
}

// UpdateGameTreeUI rebuilds the entire game-tree widget hierarchy from the
// current game's RootNode (or shows empty tree if no game selected), replaces
// the scroll container's content, and centers the view on the current node.
func (GoWin *GoWin) UpdateGameTreeUI() {
	GoWin.GameTreeNodeToButton = map[*GameTreeNode]*TreeNodeButton{}
	GoWin.GameTreeNodeToContainer = map[*GameTreeNode]*fyne.Container{}

	var RootNode *GameTreeNode
	if GoWin.Coll.CurrentGameH != nil {
		RootNode = GoWin.Coll.CurrentGameH.RootNode
	}

	NewGameTreeUI := GoWin.buildGameTreeUI(RootNode)
	GoWin.GameTreeContainer.Content = NewGameTreeUI
	fyne.Do(func() {
		GoWin.GameTreeContainer.Refresh()
		GoWin.CenterGameTreeOnCurrentNode()
	})
}

// CenterGameTreeOnCurrentNode scrolls the game tree so the currently-selected
// button is as close to the viewport center as possible. Uses the button's
// stored LayoutX/LayoutY (set in buildGameTreeUI) rather than querying Fyne's
// transient layout state.
func (GoWin *GoWin) CenterGameTreeOnCurrentNode() {
	Scroll := GoWin.GameTreeContainer
	Button, Ok := GoWin.CurrentNodeGui.(*TreeNodeButton)
	if !Ok || Button == nil || Scroll == nil || Scroll.Content == nil {
		return
	}
	ViewportSize := Scroll.Size()
	ContentSize := Scroll.Content.Size()
	ButtonSize := Button.Size()

	CenterX := Button.LayoutX + ButtonSize.Width/2
	CenterY := Button.LayoutY + ButtonSize.Height/2

	TargetX := CenterX - ViewportSize.Width/2
	TargetY := CenterY - ViewportSize.Height/2

	MaxX := max(ContentSize.Width-ViewportSize.Width, 0)
	MaxY := max(ContentSize.Height-ViewportSize.Height, 0)
	if TargetX < 0 {
		TargetX = 0
	} else if TargetX > MaxX {
		TargetX = MaxX
	}
	if TargetY < 0 {
		TargetY = 0
	} else if TargetY > MaxY {
		TargetY = MaxY
	}

	Scroll.ScrollToOffset(fyne.NewPos(TargetX, TargetY))
}

// UpdateGameTreeSelection performs a fast selection-only update of the game
// tree UI, toggling the Selected state on the old and new node buttons.
func (GoWin *GoWin) UpdateGameTreeSelection(OldNode, NewNode *GameTreeNode) {
	if OldButton, Ok := GoWin.GameTreeNodeToButton[OldNode]; Ok {
		OldButton.Selected = false
		if !OldButton.HasCustomColor {
			OldButton.Importance = widget.MediumImportance
		}
	}
	if NewButton, Ok := GoWin.GameTreeNodeToButton[NewNode]; Ok {
		NewButton.Selected = true
		if !NewButton.HasCustomColor {
			NewButton.Importance = widget.HighImportance
		}
		GoWin.CurrentNodeGui = NewButton
	}
	fyne.Do(func() {
		if OldButton, Ok := GoWin.GameTreeNodeToButton[OldNode]; Ok {
			OldButton.Refresh()
		}
		if NewButton, Ok := GoWin.GameTreeNodeToButton[NewNode]; Ok {
			NewButton.Refresh()
		}
		GoWin.CenterGameTreeOnCurrentNode()
	})
}

// NodeHasAnnotations returns true if the node has any annotation bits or labels set.
func NodeHasAnnotations(Gtn *GameTreeNode) bool {
	if Gtn.Board == nil {
		return false
	}
	for _, v := range Gtn.Board.Vertices {
		if v&^PlayerAtVertexMask != 0 {
			return true
		}
	}
	return len(Gtn.Labels) > 0
}

// TreeNodeAffectedCoords returns the board coordinates affected by a node,
// for the purpose of tree-hover highlighting. Annotations are ignored.
// Returns nil if the node is not in the same game as the current view.
func (GoWin *GoWin) TreeNodeAffectedCoords(Gtn *GameTreeNode) []Coord {
	if Gtn.Board == nil || Gtn.Board.Hist != GoWin.Coll.CurrentGameH {
		return nil
	}
	var coords []Coord
	switch Gtn.LastMove {
	case RootCoords:
		for C, V := range Gtn.Board.Vertices {
			if V&PlayerAtVertexMask != 0 && Gtn.Board.Hist.InBounds(C) {
				coords = append(coords, C)
			}
		}
	case EditCoords:
		if Gtn.Parent != nil && Gtn.Parent.Board != nil {
			allCoords := map[Coord]struct{}{}
			for C := range Gtn.Board.Vertices {
				allCoords[C] = struct{}{}
			}
			for C := range Gtn.Parent.Board.Vertices {
				allCoords[C] = struct{}{}
			}
			for C := range allCoords {
				if !Gtn.Board.Hist.InBounds(C) {
					continue
				}
				if Gtn.Board.Vertices[C]&PlayerAtVertexMask != Gtn.Parent.Board.Vertices[C]&PlayerAtVertexMask {
					coords = append(coords, C)
				}
			}
		}
	default:
		if Gtn.Board.Hist.InBounds(Gtn.LastMove) {
			coords = append(coords, Gtn.LastMove)
		}
	}
	return coords
}

// DrawHighlights draws 39%-transparent hot-pink squares on the board
// for the given coordinates. Shared function used by both game tree node
// hover and score table cell hover.
func (GoWin *GoWin) DrawHighlights(coords []Coord) {
	HighlightsLayer := GoWin.Layers["Highlights"]
	HighlightsLayer.RemoveAll()
	for _, C := range coords {
		Pos := GoWin.BoardCoordsToPixel(C)
		Rect := canvas.NewRectangle(Colors["Highlight"])
		Rect.StrokeWidth = 0
		Rect.Resize(fyne.NewSquareSize(GoWin.CellSize))
		Rect.Move(Pos)
		HighlightsLayer.Add(Rect)
	}
	HighlightsLayer.Refresh()
}

// GameTreeNodeLabel returns the text shown on a game-tree button: the
// board's move description plus a suffix summarising attached content.
// "[AC]" means the node carries both annotations and a comment; "[A]"
// means annotations only; "[C]" means comment only; no suffix otherwise.
func GameTreeNodeLabel(Node *GameTreeNode) string {
	Label := Node.Board.ToString(Node.LastMove)
	HasA := NodeHasAnnotations(Node)
	HasC := Node.Comment != ""
	switch {
	case HasA && HasC:
		Label += "[AC]"
	case HasA:
		Label += "[A]"
	case HasC:
		Label += "[C]"
	}
	return Label
}

// TreeNodeButtonSize returns the minimum button size needed to fit
// Label: the rendered text width/height plus a small pad on every side
// so the text never touches the selection border. Keep the pad in sync
// with TreeNodeButton's CreateRenderer (text size / bold) and Layout
// (BorderInset).
func TreeNodeButtonSize(Label string) fyne.Size {
	const Pad float32 = 8 // 4 px on every side — room for 1.93 px border inset, 1.39 px stroke, and a tiny gap
	TextSize := fyne.MeasureText(Label, 9.3, fyne.TextStyle{Bold: true})
	return fyne.NewSize(TextSize.Width+Pad, TextSize.Height+Pad)
}

// RefreshGameTreeNodeLabel updates the cached tree button's visible text
// for Node, so Comment/annotation edits reflect in the game tree without
// a full UpdateGameTreeUI rebuild. Also re-sizes the button to fit the
// new label and recenters it on its current center so the button never
// drifts away from its column. No-op when the button is unmapped (e.g.,
// after a deletion clears it).
func (GoWin *GoWin) RefreshGameTreeNodeLabel(Node *GameTreeNode) {
	if Node == nil || Node.Board == nil {
		return
	}
	Btn, Ok := GoWin.GameTreeNodeToButton[Node]
	if !Ok || Btn == nil {
		return
	}
	NewLabel := GameTreeNodeLabel(Node)
	OldSize := Btn.Size()
	CenterX := Btn.LayoutX + OldSize.Width/2
	CenterY := Btn.LayoutY + OldSize.Height/2
	Btn.SetText(NewLabel)
	NewSize := TreeNodeButtonSize(NewLabel)
	Btn.Resize(NewSize)
	Btn.LayoutX = CenterX - NewSize.Width/2
	Btn.LayoutY = CenterY - NewSize.Height/2
	Btn.Move(fyne.NewPos(Btn.LayoutX, Btn.LayoutY))
}

// NodeLayout holds the computed grid position for one game tree node.
// Col and ParentCol are stored as float32 because the post-order
// centering pass positions a parent at the midpoint between its first
// and last child columns, which is fractional whenever those columns
// differ by an odd number.
type NodeLayout struct {
	Node      *GameTreeNode
	Col       float32
	Row       int
	ParentCol float32
	ParentRow int
	HasParent bool // false for the root node of the tree
}

// buildGameTreeUI builds a flat Fyne container for the game tree rooted at
// Gtn. All buttons are direct children of a single container.NewWithoutLayout
// so Fyne never recurses deeper than one level when applying themes, regardless
// of game length.
//
// Layout: two iterative passes.
// Pass 1 (post-order): compute subtree width of each node.
// Pass 2 (pre-order): assign (col, row) centering each parent over its children.
// Connector lines (yellow then blue) are added before buttons so buttons
// render on top.
func (GoWin *GoWin) buildGameTreeUI(Gtn *GameTreeNode) fyne.CanvasObject {
	if Gtn == nil {
		return container.NewWithoutLayout()
	}

	// Row stride and horizontal layout constants are declared inside the
	// button-placement block below, since the horizontal layout uses
	// per-column widths rather than a uniform CellW.

	// --- Pass 1: compute subtree widths (iterative post-order) ---
	WidthOf := map[*GameTreeNode]int{}
	// Post-order via two-stack method.
	Stack1 := []*GameTreeNode{Gtn}
	PostOrder := []*GameTreeNode{}
	for len(Stack1) > 0 {
		N := Stack1[len(Stack1)-1]
		Stack1 = Stack1[:len(Stack1)-1]
		PostOrder = append(PostOrder, N)
		for _, Child := range N.Children {
			Stack1 = append(Stack1, Child)
		}
	}
	for I := len(PostOrder) - 1; I >= 0; I-- {
		N := PostOrder[I]
		if len(N.Children) == 0 {
			WidthOf[N] = 1
		} else {
			W := 0
			for _, Child := range N.Children {
				W += WidthOf[Child]
			}
			WidthOf[N] = W
		}
	}

	// --- Pass 2: assign (col, row) iteratively (pre-order) ---
	// Leaves get their final column directly (LeftEdge); non-leaf columns
	// are placed at the subtree midpoint here and then refined by the
	// post-order adjustment pass below.
	type PreEntry struct {
		Node      *GameTreeNode
		Row       int
		LeftEdge  int
		ParentCol float32
		ParentRow int
		HasParent bool
	}
	Layouts := []NodeLayout{}
	MaxCol := 0
	MaxRow := 0
	PreStack := []PreEntry{{Node: Gtn, Row: 0, LeftEdge: 0, HasParent: false}}
	for len(PreStack) > 0 {
		Entry := PreStack[len(PreStack)-1]
		PreStack = PreStack[:len(PreStack)-1]

		IntCol := Entry.LeftEdge + (WidthOf[Entry.Node]-1)/2
		Col := float32(IntCol)
		Layouts = append(Layouts, NodeLayout{
			Node:      Entry.Node,
			Col:       Col,
			Row:       Entry.Row,
			ParentCol: Entry.ParentCol,
			ParentRow: Entry.ParentRow,
			HasParent: Entry.HasParent,
		})
		if IntCol > MaxCol {
			MaxCol = IntCol
		}
		if Entry.Row > MaxRow {
			MaxRow = Entry.Row
		}

		// Push children in reverse so leftmost child is processed first.
		ChildLeft := Entry.LeftEdge
		ChildEntries := make([]PreEntry, len(Entry.Node.Children))
		for I, Child := range Entry.Node.Children {
			ChildEntries[I] = PreEntry{
				Node:      Child,
				Row:       Entry.Row + 1,
				LeftEdge:  ChildLeft,
				ParentCol: Col,
				ParentRow: Entry.Row,
				HasParent: true,
			}
			ChildLeft += WidthOf[Child]
		}
		for I := len(ChildEntries) - 1; I >= 0; I-- {
			PreStack = append(PreStack, ChildEntries[I])
		}
	}

	// --- Pass 3: post-order adjustment — each non-leaf parent's Col is
	// rewritten to the midpoint between its first-child and last-child
	// Cols, so the parent sits exactly halfway between the left side of
	// the leftmost child and the right side of the rightmost child. For a
	// single-child parent this reduces to the child's own Col, placing
	// the parent directly above the child. Leaves are untouched, so
	// MaxCol (taken from the pre-order pass over leaf LeftEdges) stays
	// valid. PostOrder was built as a pre-order DFS (root first), so
	// iterate it in reverse — children processed before their parent.
	NodeToLayout := map[*GameTreeNode]*NodeLayout{}
	for I := range Layouts {
		NodeToLayout[Layouts[I].Node] = &Layouts[I]
	}
	for I := len(PostOrder) - 1; I >= 0; I-- {
		N := PostOrder[I]
		if len(N.Children) == 0 {
			continue
		}
		First := NodeToLayout[N.Children[0]].Col
		Last := NodeToLayout[N.Children[len(N.Children)-1]].Col
		NodeToLayout[N].Col = (First + Last) / 2
	}
	for I := range Layouts {
		if P := Layouts[I].Node.Parent; P != nil {
			if PL, Ok := NodeToLayout[P]; Ok {
				Layouts[I].ParentCol = PL.Col
			}
		}
	}

	// --- Build buttons and connector lines ---
	Buttons := make([]*TreeNodeButton, 0, len(Layouts))
	CurrentNode := GoWin.CurrentNode()

	// Per-column widths: each integer column c gets a width equal to the
	// widest LEAF button anchored at that column. Leaves always have an
	// integer Col, so this captures every node that needs its own
	// horizontal slot. Non-leaf parents sit at fractional Cols (the
	// midpoint of their first and last child) and are NOT included in
	// ColWidth — their horizontal span is absorbed by the children's
	// column gap, which is typically ≥ their button width. (If a parent
	// label really is wider than that gap, the post-pass below shifts the
	// whole tree so it still fits within the container.) This keeps
	// adjacent sibling subtrees packed tightly: when two same-row leaves
	// share their column's max width, the space between their button
	// edges equals exactly HGap pixels, matching the vertical row gap.
	const HGap float32 = 6
	const LeftPad float32 = 2
	const RightPad float32 = 2
	const CellH float32 = 25
	NodeLabels := make(map[*GameTreeNode]string, len(Layouts))
	NodeSize := make(map[*GameTreeNode]fyne.Size, len(Layouts))
	for _, Layout := range Layouts {
		L := GameTreeNodeLabel(Layout.Node)
		NodeLabels[Layout.Node] = L
		NodeSize[Layout.Node] = TreeNodeButtonSize(L)
	}
	ColCount := MaxCol + 1
	ColWidth := make([]float32, ColCount)
	for _, Layout := range Layouts {
		if len(Layout.Node.Children) != 0 {
			continue // Skip non-leaves; see comment above.
		}
		C := int(math.Floor(float64(Layout.Col)))
		if C < 0 {
			C = 0
		}
		if C >= ColCount {
			C = ColCount - 1
		}
		if W := NodeSize[Layout.Node].Width; W > ColWidth[C] {
			ColWidth[C] = W
		}
	}
	ColCenter := make([]float32, ColCount)
	Cursor := LeftPad
	for C := range ColCount {
		ColCenter[C] = Cursor + ColWidth[C]/2
		Cursor += ColWidth[C]
		if C < ColCount-1 {
			Cursor += HGap
		}
	}
	CenterOf := func(F float32) float32 {
		if F <= 0 {
			return ColCenter[0]
		}
		if F >= float32(ColCount-1) {
			return ColCenter[ColCount-1]
		}
		Lo := int(math.Floor(float64(F)))
		Hi := Lo + 1
		T := F - float32(Lo)
		return ColCenter[Lo] + T*(ColCenter[Hi]-ColCenter[Lo])
	}
	// Parent buttons wider than their children's column span can stick
	// out past the left-/rightmost column boundary (a parent's center is
	// interpolated between child columns; its button half-width may
	// exceed that distance to the outer edge). Detect the maximum overrun
	// in either direction and shift all centers by that amount so every
	// button lands at X ≥ LeftPad and the container's TotalW accounts
	// for any right-side overrun.
	LeftOverrun := float32(0)
	RightOverrun := float32(0)
	for _, Layout := range Layouts {
		CX := CenterOf(Layout.Col)
		Left := CX - NodeSize[Layout.Node].Width/2
		Right := CX + NodeSize[Layout.Node].Width/2
		if Under := LeftPad - Left; Under > LeftOverrun {
			LeftOverrun = Under
		}
		if Over := Right - (Cursor); Over > RightOverrun {
			RightOverrun = Over
		}
	}
	if LeftOverrun > 0 {
		for C := range ColCenter {
			ColCenter[C] += LeftOverrun
		}
		Cursor += LeftOverrun
	}
	TotalW := Cursor + RightPad + RightOverrun

	for _, Layout := range Layouts {
		Node := Layout.Node
		NodeLabel := NodeLabels[Node]
		CapturedNode := Node
		NodeButton := NewTreeNodeButton(NodeLabel, func() {
			GoWin.SetMouseMode("Play")
			GoWin.OnUserNavigate()
			GoWin.SetCurrentNode(CapturedNode)
		}, func() {
			fyne.Do(func() {
				GoWin.DrawHighlights(GoWin.TreeNodeAffectedCoords(CapturedNode))
			})
		}, func() {
			fyne.Do(func() {
				GoWin.Layers["Highlights"].RemoveAll()
				GoWin.Layers["Highlights"].Refresh()
			})
		})
		if Node.PreviousStonePlacer > 0 {
			if PC, Ok := PlayerColors[Node.PreviousStonePlacer]; Ok {
				NodeButton.HasCustomColor = true
				NodeButton.BackgroundColor = PC.Base
				NodeButton.TextColor = PC.Contrast
			}
		}
		if Node == CurrentNode {
			NodeButton.Selected = true
			if !NodeButton.HasCustomColor {
				NodeButton.Importance = widget.HighImportance
			}
			GoWin.CurrentNodeGui = NodeButton
		}
		GoWin.GameTreeNodeToButton[Node] = NodeButton
		ButtonSize := NodeSize[Node]
		NodeButton.Resize(ButtonSize)
		CX := CenterOf(Layout.Col)
		X := CX - ButtonSize.Width/2
		Y := float32(Layout.Row)*CellH + (CellH-ButtonSize.Height)/2
		NodeButton.LayoutX = X
		NodeButton.LayoutY = Y
		NodeButton.Move(fyne.NewPos(X, Y))
		Buttons = append(Buttons, NodeButton)
	}

	// Helper to add one L-shaped (or straight) connector in the given color.
	// Lines connect the center of each button.
	AddConnector := func(FlatContainer *fyne.Container, Layout NodeLayout, LineColor color.NRGBA, StrokeWidth float32) {
		if !Layout.HasParent {
			return
		}
		ParentCX := CenterOf(Layout.ParentCol)
		ParentCY := float32(Layout.ParentRow)*CellH + CellH/2
		ChildCX := CenterOf(Layout.Col)
		ChildCY := float32(Layout.Row)*CellH + CellH/2
		AddLine := func(X1, Y1, X2, Y2 float32) {
			L := canvas.NewLine(LineColor)
			L.Position1 = fyne.NewPos(X1, Y1)
			L.Position2 = fyne.NewPos(X2, Y2)
			L.StrokeWidth = StrokeWidth
			FlatContainer.Add(L)
		}
		if Layout.Col == Layout.ParentCol {
			AddLine(ParentCX, ParentCY, ChildCX, ChildCY)
		} else {
			MidY := (ParentCY + ChildCY) / 2
			HalfSW := StrokeWidth / 2
			// Extend segments by HalfSW into each corner so caps meet cleanly.
			AddLine(ParentCX, ParentCY, ParentCX, MidY+HalfSW)
			if ParentCX < ChildCX {
				AddLine(ParentCX-HalfSW, MidY, ChildCX+HalfSW, MidY)
			} else {
				AddLine(ParentCX+HalfSW, MidY, ChildCX-HalfSW, MidY)
			}
			AddLine(ChildCX, MidY-HalfSW, ChildCX, ChildCY)
		}
	}

	TotalH := float32(MaxRow+1) * CellH
	FlatContainer := container.New(&FixedSizeLayout{Size: fyne.NewSize(TotalW, TotalH)})
	// 1. All yellow lines
	for _, Layout := range Layouts {
		AddConnector(FlatContainer, Layout, color.NRGBA{R: 255, G: 200, B: 0, A: 220}, 3)
	}
	// 2. Blue line only to the favorite child of each parent. When no
	// FavoriteChild is set we fall back to Children[0] so the favored path
	// (the one Page Down would follow) is always highlighted.
	for _, Layout := range Layouts {
		Parent := Layout.Node.Parent
		if Parent == nil {
			continue
		}
		Favored := Parent.FavoriteChild
		if Favored == nil && len(Parent.Children) > 0 {
			Favored = Parent.Children[0]
		}
		if Layout.Node != Favored {
			continue
		}
		AddConnector(FlatContainer, Layout, color.NRGBA{R: 80, G: 160, B: 255, A: 220}, 1.5)
	}
	// 3. All buttons
	for _, Button := range Buttons {
		FlatContainer.Add(Button)
	}

	FlatContainer.Resize(fyne.NewSize(TotalW, TotalH))
	return FlatContainer
}

// ShowError prints the error to stdout and displays it in a Fyne error dialog.
func (GoWin *GoWin) ShowError(err error) {
	fmt.Printf("Error: %v\n", err)
	dialog.ShowError(err, GoWin.Win)
}

// AddFreshBoard creates a new game using the currently active preset.
func (GoWin *GoWin) AddFreshBoard() {
	GoWin.AddFreshBoardFromPreset(GoWin.GetActivePreset())
}

// AddFreshBoardFromPreset creates a new board using the given preset values
// and stamps the current UNIX timestamp on the board's Date And Time field.
func (GoWin *GoWin) AddFreshBoardFromPreset(Preset FreshBoardPreset) {
	NewRootNode := GoWin.Coll.NewRootNode(Preset.Width, Preset.Height)
	NewRootNode.Board.Hist.Komi = Preset.Komi

	// Set player count from preset
	if Preset.Players > 0 {
		NewRootNode.Board.Hist.Players = Preset.Players
	} else {
		NewRootNode.Board.Hist.Players = 2
	}

	// Set board wrapping from preset
	NewRootNode.Board.Hist.WrapXMulY = Preset.WrapXMulY
	NewRootNode.Board.Hist.WrapYMulX = Preset.WrapYMulX
	NewRootNode.Board.Hist.WrapXShiftY = Preset.WrapXShiftY
	NewRootNode.Board.Hist.WrapYShiftX = Preset.WrapYShiftX
	NewRootNode.Board.Hist.RecomputeWrapInverses()

	// Apply liberty sharing from preset. A nil preset matrix means "no sharing".
	NewRootNode.Board.Hist.LibertySharingFixed = Preset.LibertySharingFixed
	PresetMatrix := Preset.LibertySharingMatrix
	if PresetMatrix == nil {
		PresetMatrix = NewLibertySharingMatrix(NewRootNode.Board.Hist.Players, SharingRefused)
	} else {
		PresetMatrix = PresetMatrix.Resize(NewRootNode.Board.Hist.Players)
	}
	NewRootNode.Board.NextLibertySharingMatrix = PresetMatrix

	// Copy player names from preset
	maps.Copy(NewRootNode.Board.Hist.PlayerNames, Preset.PlayerNames)

	if Preset.RuleSet != nil {
		NewRootNode.Board.Hist.RuleSet = Preset.RuleSet
		NewRootNode.RemainingStonePlacements = Preset.RuleSet.StonePlacementsPerMove
	}
	RuleSetName := NewRootNode.Board.Hist.RuleSet.Names[0]
	NewRootNode.Board.Hist.Information["Rules"] = &RuleSetName
	for Key, Val := range Preset.Information {
		V := Val
		NewRootNode.Board.Hist.Information[Key] = &V
	}
	Timestamp := fmt.Sprintf("%ds", time.Now().Unix())
	NewRootNode.Board.Hist.Information["Date And Time"] = &Timestamp
	GoWin.SetCurrentNode(NewRootNode)
	GoWin.SetMouseMode("Play")

	// Re-initialize any attached engines for the new board size.
	// Only do this for 2-player games since GTP engines only support 2 players.
	if NewRootNode.Board.Hist.Players == 2 {
		for Player := range GoWin.GtpEngines {
			if Err := GoWin.InitializeEngineForPlayer(Player); Err != nil {
				GoWin.ShowError(Err)
				GoWin.DetachEngineForPlayer(Player)
			}
		}
	}

	// Refresh game selector after adding a new game
	GoWin.RefreshGameSelector()
}

// GetActivePreset returns the active FreshBoardPreset, falling back to "Default".
func (GoWin *GoWin) GetActivePreset() FreshBoardPreset {
	if Preset, Ok := CurrentAppConfig.FreshBoardPresets[CurrentAppConfig.ActivePresetName]; Ok {
		return Preset
	}
	return CurrentAppConfig.FreshBoardPresets["Default"]
}

// CurrentNode returns the current GameTreeNode of the active game, or the
// CollectionNode if no game is selected.
func (GoWin *GoWin) CurrentNode() *GameTreeNode {
	if GoWin.Coll.CurrentGameH == nil {
		return GoWin.Coll.CollectionNode
	}
	return GoWin.Coll.CurrentGameH.CurrentNode
}

// SetCurrentNode navigates to Gtn: updates the active game and current-node
// pointers, sets the parent's FavoriteChild, resets mouse mode (switching to
// Score if the game is over), redraws the board and tree, detaches engines
// unless called by PlayMove, and refreshes the game selector.
func (GoWin *GoWin) SetCurrentNode(Gtn *GameTreeNode, KeepEngines ...bool) {
	OldNode := GoWin.CurrentNode()

	if Gtn == GoWin.Coll.CollectionNode || Gtn.Board == nil {
		GoWin.Coll.CurrentGameH = nil
	} else {
		GoWin.Coll.CurrentGameH = Gtn.Board.Hist
		Gtn.Board.Hist.CurrentNode = Gtn
	}
	// Imports can replace the collection before navigation, so compare against
	// the game represented by the menu rather than the previous node.
	if GoWin.SetVertexMenuGameH != GoWin.Coll.CurrentGameH {
		GoWin.RefreshSetVertexMenu()
	}

	// Track whether the favorite-child pointer actually changes so the
	// blue connector line (which is drawn only for favorite-child edges)
	// can be refreshed. The fast selection-only update path can't do
	// that, so fall back to a full rebuild whenever it moves.
	FavoriteChildChanged := Gtn.Parent != nil && Gtn.Parent.FavoriteChild != Gtn
	if Gtn.Parent != nil {
		Gtn.Parent.FavoriteChild = Gtn
	}
	GoWin.SetMouseMode("Play")
	GoWin.HoverCoords = ErrorCoords
	if Gtn.Board != nil && GoWin.IsGameOver() {
		GoWin.SetMouseMode("Score")
	}
	_, OldInMap := GoWin.GameTreeNodeToButton[OldNode]
	_, NewInMap := GoWin.GameTreeNodeToButton[Gtn]
	if OldInMap && NewInMap && !FavoriteChildChanged {
		GoWin.UpdateGameTreeSelection(OldNode, Gtn)
	} else {
		GoWin.UpdateGameTreeUI()
	}
	GoWin.UpdateCommentTextbox()
	GoWin.DrawBoard()
	GoWin.ResetHover()
	GoWin.FixWindowTitle()

	// Refresh game selector to stay in sync with current game
	GoWin.RefreshGameSelector()

	if !(len(KeepEngines) > 0 && KeepEngines[0]) {
		GoWin.DetachAllEngines()
	}
	GoWin.RefreshEngineMenuItems()
}

// OnUserNavigate returns players to Human before browsing the game tree.
func (GoWin *GoWin) OnUserNavigate() {
	GoWin.DetachAllEngines()
}

// RefreshPassMenuItem enables or disables the Pass menu item based on whether
// passing is currently available (Play mode, board not nil,
// and the next player is not engine-controlled).
func (GoWin *GoWin) RefreshPassMenuItem() {
	if GoWin.PassItem == nil {
		return
	}
	PassAvailable := GoWin.MouseMode == "Play" &&
		GoWin.CurrentNode().Board != nil &&
		GoWin.GtpEngines[GoWin.CurrentNode().GetNextStonePlacer()] == nil
	GoWin.PassItem.Disabled = !PassAvailable
	if GoWin.MainMenu != nil {
		GoWin.Win.SetMainMenu(GoWin.MainMenu)
	}
}

// RefreshDiplomacyMenuItem enables or disables the Game > Diplomacy menu item
// based on whether the liberty sharing matrix is currently changeable.
func (GoWin *GoWin) RefreshDiplomacyMenuItem() {
	if GoWin.DiplomacyItem == nil {
		return
	}
	Available := false
	Current := GoWin.CurrentNode()
	if Current != nil && Current.Board != nil && GoWin.Coll.CurrentGameH != nil {
		Available = !GoWin.Coll.CurrentGameH.LibertySharingFixed
	}
	GoWin.DiplomacyItem.Disabled = !Available
	if GoWin.MainMenu != nil {
		GoWin.Win.SetMainMenu(GoWin.MainMenu)
	}
}

// FixWindowTitle updates the window title to show the version, the currently
// opened file path (SGF or CGG, if any), and an indicator showing whose turn it is.
func (GoWin *GoWin) FixWindowTitle() {
	Title := "Connected Groups Goban Version " + Version
	if GoWin.OpenedFilePath != "" {
		Title += " | " + GoWin.OpenedFilePath
	}
	// Show next player indicator for any number of players
	if GoWin.CurrentNode() == nil || GoWin.CurrentNode().Board == nil {
		GoWin.Win.SetTitle(Title)
		return
	}
	NextPlayer := GoWin.CurrentNode().GetNextStonePlacer()
	if NextPlayer >= 1 && NextPlayer <= 15 {
		if NextPlayerName := GoWin.Coll.CurrentGameH.PlayerNames[NextPlayer]; NextPlayerName != "" {
			Title = fmt.Sprintf("Player %d (%s) — ", NextPlayer, NextPlayerName) + Title
		} else {
			Title = fmt.Sprintf("Player %d — ", NextPlayer) + Title
		}
	}

	// Show remaining stone placements when StonePlacementsPerMove > 1
	if GoWin.Coll.CurrentGameH.RuleSet.StonePlacementsPerMove > 1 {
		RemainingPlacements := GoWin.CurrentNode().RemainingStonePlacements
		Title = fmt.Sprintf("(%d/%d) ", RemainingPlacements, GoWin.Coll.CurrentGameH.RuleSet.StonePlacementsPerMove) + Title
	}
	GoWin.Win.SetTitle(Title)
	GoWin.RefreshPassMenuItem()
	GoWin.RefreshDiplomacyMenuItem()
}
