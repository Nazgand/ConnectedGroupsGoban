package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// vertexOptionLabel returns a display string for the Set Vertex player
// selector: "Empty (0)" for empty, or "Player N" for player N.
func VertexOptionLabel(p uint8) string {
	if p == 0 {
		return "Empty (0)"
	}
	return fmt.Sprintf("Player %d", p)
}

// annotationOptionLabel returns the human-readable name for an annotation
// bitmask (Circle, Square, Triangle, X Mark).
func AnnotationOptionLabel(mask uint8) string {
	switch mask {
	case CircleMask:
		return "Circle"
	case SquareMask:
		return "Square"
	case TriangleMask:
		return "Triangle"
	case XMask:
		return "X Mark"
	default:
		return "Unknown"
	}
}

// Converts pixel coordinates to Coord
func (GoWin *GoWin) PixelToBoardCoords(pos fyne.Position) Coord {
	if !GoWin.PixelOnBoard(pos) {
		return ErrorCoords
	}
	Size := GoWin.PlayContainer.Size()
	DW, DH := GoWin.DisplayDims()
	DX := int(((pos.X*2-Size.Width)/GoWin.CellSize + float32(DW)) / 2)
	DY := int(((pos.Y*2-Size.Height)/GoWin.CellSize + float32(DH)) / 2)
	X, Y := GoWin.InverseTransformDisplayCoord(DX, DY)
	return Coord{uint8(X), uint8(Y)}
}

// PixelOnBoard returns true when pos falls inside the display-aligned goban
// bounding box. Uses DisplayDims directly so the test is exact for every
// symmetry, without caring where the goban's logical (0,0) lands in display.
func (GoWin *GoWin) PixelOnBoard(pos fyne.Position) bool {
	TopLeft := GoWin.BoardUnitsToPixel(0, 0)
	DW, DH := GoWin.DisplayDims()
	Right := TopLeft.X + float32(DW)*GoWin.CellSize
	Bottom := TopLeft.Y + float32(DH)*GoWin.CellSize
	return pos.X >= TopLeft.X && pos.X <= Right && pos.Y >= TopLeft.Y && pos.Y <= Bottom
}

// ResetHover clears the Hover layer and resets HoverCoords so the next
// mouse-move triggers a fresh redraw.
func (GoWin *GoWin) ResetHover() {
	GoWin.Layers["Hover"].RemoveAll()
	GoWin.Layers["Hover"].Refresh()
	GoWin.HoverCoords = ErrorCoords
}

// DrawHoverCircle draws a translucent stone preview at HoverCoords when in
// Play or Set Vertex mode. Skipped if the placement would be illegal or if
// the intersection already holds the same player.
func (GoWin *GoWin) DrawHoverCircle() {
	var CircleColor color.Color
	switch GoWin.MouseMode {
	case "Set Vertex":
		P := GoWin.SetVertexPlayer
		if P == 0 {
			if GoWin.CurrentNode().Board.GetPlayerAt(GoWin.HoverCoords) == 0 {
				return
			}
			CircleColor = Colors["Delete"]
		} else {
			if GoWin.CurrentNode().Board.GetPlayerAt(GoWin.HoverCoords) == P {
				return
			}
			if _, Err := GoWin.CurrentNode().Board.AttemptMove(GoWin.HoverCoords, P, true, true); Err != nil {
				return
			}
			CircleColor = PlayerColors[P].Hover
		}
	case "Play":
		NextPlayer := GoWin.CurrentNode().GetNextStonePlacer()
		_, PlacementErr := GoWin.CurrentNode().Board.AttemptMove(GoWin.HoverCoords, NextPlayer, false, true)
		if GoWin.GtpEngines[NextPlayer] != nil || PlacementErr != nil {
			return
		}
		CircleColor = PlayerColors[NextPlayer].Hover
	default:
		return
	}
	HoverCircle := GoWin.MakeSolidCircleOnBoard(GoWin.HoverCoords, CircleColor, 0.931547)
	GoWin.Layers["Hover"].Add(HoverCircle)
}

// DrawHoverNeighbors draws small solid circles on the orthogonal neighbors
// of HoverCoords using the "Neighbor" theme color.
func (GoWin *GoWin) DrawHoverNeighbors() {
	if GoWin.CurrentNode().Board == nil || GoWin.Coll.CurrentGameH == nil {
		return
	}
	HoverLayer := GoWin.Layers["Hover"]
	for _, NeighborCoord := range GoWin.Coll.CurrentGameH.Neighbors(GoWin.HoverCoords) {
		NeighborCircle := GoWin.MakeSolidCircleOnBoard(NeighborCoord, Colors["Neighbor"], 0.093)
		HoverLayer.Add(NeighborCircle)
	}
}

// DrawHoverCoords highlights the column and row coordinate labels around
// the board edges for the current HoverCoords. In Toggle Annotation mode
// it also draws an annotation preview on the intersection.
func (GoWin *GoWin) DrawHoverCoords() {
	Width, Height := GoWin.Width(), GoWin.Height()
	HoverLayer := GoWin.Layers["Hover"]
	DrawCoordLabel := func(C Coord, CoordLabel string) {
		Pos := GoWin.BoardCoordsToPixel(C)
		Rect := canvas.NewRectangle(Colors["Coord Hover Background"])
		Rect.StrokeWidth = 0
		Rect.Resize(fyne.NewSquareSize(GoWin.CellSize))
		Rect.Move(Pos)
		HoverLayer.Add(Rect)
		Text := canvas.NewText(CoordLabel, Colors["Coord Hover"])
		Text.TextSize = GoWin.CellSize * 0.39
		Text.Alignment = fyne.TextAlignCenter
		Text.TextStyle = fyne.TextStyle{Bold: true}
		Text.Resize(Text.MinSize())
		Text.Move(fyne.Position{
			X: Pos.X + 0.5*GoWin.CellSize - Text.Size().Width/2,
			Y: Pos.Y + 0.5*GoWin.CellSize - Text.Size().Height/2,
		})
		HoverLayer.Add(Text)
	}
	CoordColumnLabel := GoWin.CoordColumnLabel(GoWin.HoverCoords.X)
	ColCs := []Coord{{X: GoWin.HoverCoords.X, Y: 0xff},
		{X: GoWin.HoverCoords.X, Y: Height}}
	for _, C := range ColCs {
		DrawCoordLabel(C, CoordColumnLabel)
	}
	RowCs := []Coord{{X: 0xff, Y: GoWin.HoverCoords.Y},
		{X: Width, Y: GoWin.HoverCoords.Y}}
	CoordRowLabel := GoWin.CoordRowLabel(GoWin.HoverCoords.Y)
	for _, C := range RowCs {
		DrawCoordLabel(C, CoordRowLabel)
	}

	// Draw annotation preview if in annotation toggle mode
	if GoWin.MouseMode == "Toggle Annotation" {
		HasAnnotation := GoWin.CurrentNode().Board.HasAnnotation(GoWin.HoverCoords, GoWin.SetAnnotationMask)
		var PreviewColor color.NRGBA
		if HasAnnotation {
			PreviewColor = ContrastColor(Colors["Annotation"])
		} else {
			PreviewColor = Colors["Annotation"]
			PreviewColor.A = 0x93
		}
		GoWin.DrawAnnotation(GoWin.SetAnnotationMask, GoWin.HoverCoords, PreviewColor, HoverLayer)
	}
}

// DrawHoverLiberties draws small circles on the liberties of the group
// under HoverCoords. The liberty layer is cached and only rebuilt when
// the hovered group changes. Skipped in Score mode and when the active
// theme has ShowHoverLiberties disabled.
func (GoWin *GoWin) DrawHoverLiberties() {
	if !GetActiveTheme().ShowHoverLiberties {
		return
	}
	HoverLayer := GoWin.Layers["Hover"]
	HoverLayer.Remove(GoWin.LibertyLayer)
	if GoWin.CurrentNode().Board == nil {
		return
	}
	if GoWin.MouseMode != "Score" {
		NewHoverGroup := GoWin.CurrentNode().Board.GetGroupAtCoord(GoWin.HoverCoords)

		if GoWin.LibertyLayer == nil || GoWin.HoverGroup != NewHoverGroup {
			GoWin.HoverGroup = NewHoverGroup
			GoWin.LibertyLayer = container.NewWithoutLayout()
			GoWin.UpdateStatusBar()

			if GoWin.HoverGroup != nil {
				for _, C := range GoWin.HoverGroup.Vertices[0] {
					var FillColor color.Color
					switch len(GoWin.HoverGroup.Vertices[0]) {
					case 1:
						FillColor = Colors["1-Liberty Line"]
					case 2:
						FillColor = Colors["2-Liberty Line"]
					case 3:
						FillColor = Colors["3-Liberty Line"]
					case 4:
						FillColor = Colors["4-Liberty Line"]
					default:
						FillColor = Colors["≥5-Liberty Line"]
					}
					Liberty := GoWin.MakeCircleOnBoard(C, Colors["≥5-Liberty Line"], FillColor, 0.39, 0.39)
					GoWin.LibertyLayer.Add(Liberty)
				}
			}
		}

		HoverLayer.Add(GoWin.LibertyLayer)
		HoverLayer.Refresh()
	}
}

// HandleMouseMove converts a desktop mouse-move event into a board Coord
// and delegates to DrawHover.
func (GoWin *GoWin) HandleMouseMove(ev *desktop.MouseEvent) {
	C := GoWin.PixelToBoardCoords(ev.Position)
	GoWin.DrawHover(C)
}

// DrawHover updates the hover visualization for Coord C: resets the
// previous hover, then redraws the hover circle, liberty markers, and
// coordinate highlights. No-op if C equals the current HoverCoords.
func (GoWin *GoWin) DrawHover(C Coord) {
	if GoWin.HoverCoords == C {
		return
	}
	GoWin.ResetHover()
	if GoWin.CellSize <= 0 || GoWin.Coll.CurrentGameH == nil || !GoWin.Coll.CurrentGameH.InBounds(C) {
		return
	}
	GoWin.HoverCoords = C

	GoWin.DrawHoverCircle()
	GoWin.DrawHoverLiberties()
	GoWin.DrawHoverNeighbors()
	GoWin.DrawHoverCoords()

	GoWin.Layers["Hover"].Refresh()
}

// SetMouseMode transitions the UI to the given MouseMode ("Play", "Score",
// "Set Vertex", "Toggle Annotation", "Set Label"). It shows/hides the
// appropriate widgets, initializes mode-specific state, and redraws the board.
func (GoWin *GoWin) SetMouseMode(MouseMode string) {
	if GoWin.MouseMode == MouseMode {
		return
	}
	GoWin.MouseMode = MouseMode
	if MouseMode != "Play" {
		GoWin.ResetHover()
	}
	if MouseMode != "Score" {
		GoWin.ScoreContainer.Hide()
	}
	switch MouseMode {
	case "Score":
		if GoWin.CurrentNode().Board == nil {
			GoWin.MouseMode = "Play"
			return
		}
		GoWin.ScoreContainer.Show()
		GoWin.VSplit.Refresh()
		GoWin.InitializeTerritoryMap()
	case "Set Vertex":
		if GoWin.Coll.CurrentGameH == nil {
			GoWin.MouseMode = "Play"
			return
		}
		if GoWin.SetVertexPlayer > GoWin.Coll.CurrentGameH.Players {
			GoWin.SetVertexPlayer = 0
		}
	}
	// Re-draw the board
	GoWin.DrawBoard()
	GoWin.UpdateStatusBar()
	GoWin.RefreshPassMenuItem()
}

// Handles mouse click events to place stones or toggle group status in scoring mode.
func (GoWin *GoWin) HandleMouseClick(ev *fyne.PointEvent) {
	if !GoWin.PixelOnBoard(ev.Position) {
		return
	}
	C := GoWin.PixelToBoardCoords(ev.Position)
	if GoWin.CurrentNode().Board == nil {
		return
	}

	switch GoWin.MouseMode {
	case "Play":
		NextPlayer := GoWin.CurrentNode().GetNextStonePlacer()
		if GoWin.GtpEngines[NextPlayer] != nil {
			return // Engine controls this player; user may not play for it.
		}
		existingStone := GoWin.CurrentNode().Board.GetPlayerAt(C)
		if 0 < existingStone && existingStone <= GoWin.Coll.CurrentGameH.Players {
			// Stone already exists, do nothing
		} else {
			GoWin.PlayMove(C, NextPlayer, 0)
			GoWin.TriggerEngineIfNeeded()
		}
	case "Score":
		GoWin.ToggleGroupStatus(C)
	case "Set Label":
		if GoWin.DialogShowing {
			return
		} else {
			GoWin.DialogShowing = true
		}
		// Open a textbox popup to set or remove the label of the vertex
		entry := NewDialogEntry(GoWin)
		existingLabel, ok := GoWin.CurrentNode().Labels[C]
		if ok {
			entry.SetText(existingLabel)
		}
		entry.SetPlaceHolder("Enter label (leave empty to remove)")
		entryDialog := dialog.NewForm("Set Label", "OK", "Cancel",
			[]*widget.FormItem{widget.NewFormItem("Label", entry)},
			func(ok bool) {
				GoWin.DialogClosed()
				if ok {
					switch entry.Text {
					case "":
						delete(GoWin.CurrentNode().Labels, C)
					default:
						GoWin.CurrentNode().Labels[C] = entry.Text
					}
					GoWin.RefreshGameTreeNodeLabel(GoWin.CurrentNode())
					GoWin.DrawBoard()
				}
			}, GoWin.Win)
		GoWin.DismissDialog = func() { entryDialog.Hide() }
		entry.OnSubmitted = func(s string) { entryDialog.Submit() }
		entryDialog.Show()
		GoWin.Win.Canvas().Focus(entry)
	case "Set Vertex":
		Gtn := GoWin.CurrentNode()
		// Stone-edit properties: may create a new node
		switch Gtn.LastMove {
		case RootCoords:
			if len(Gtn.Children) != 0 {
				Gtn = Gtn.MakeSiblingCopyBoard(true)
				Gtn.LastMove = RootCoords
			}
		case EditCoords:
			if len(Gtn.Children) != 0 {
				Gtn = Gtn.MakeSiblingCopyBoard(true)
				Gtn.LastMove = EditCoords
			}
		default:
			Gtn = Gtn.MakeChild(true, true)
			Gtn.LastMove = EditCoords
		}
		if GoWin.SetVertexPlayer == 0 {
			// Deleting a stone — just clear the vertex
			Err := Gtn.Board.SetPlayerAt(C, 0)
			if Err == nil {
				Gtn.Board.CalculateAllGroups()
				Gtn.Board.IsPlacementLegalCache = nil
			}
		} else {
			// Placing a stone — use AttemptMove with EditMode to resolve captures
			BoardWithMove, Err := Gtn.Board.AttemptMove(C, GoWin.SetVertexPlayer, true, false)
			if Err == nil {
				Gtn.Board = BoardWithMove
			}
		}
		if Gtn != GoWin.CurrentNode() {
			GoWin.UpdateGameTreeUI()
			OldNode := GoWin.CurrentNode()
			Gtn.SelectNode()
			GoWin.UpdateGameTreeSelection(OldNode, Gtn)
		}
	case "Toggle Annotation":
		GoWin.CurrentNode().Board.ToggleAnnotation(C, GoWin.SetAnnotationMask)
		GoWin.RefreshGameTreeNodeLabel(GoWin.CurrentNode())
	}
	GoWin.DrawBoard()
	GoWin.HoverCoords = ErrorCoords // force liberty redraw at same coord
	GoWin.DrawHover(C)
}
