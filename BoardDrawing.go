package main

import (
	"fmt"
	"image/color"
	"log"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// MakeSolidCircleOnBoard creates a filled circle at Coord c with the given
// color and size (fraction of CellSize, 0..1). Stroke is zero.
func (GoWin *GoWin) MakeSolidCircleOnBoard(c Coord, color color.Color, size float32) *canvas.Circle {
	return GoWin.MakeCircleOnBoard(c, color, color, size, 0)
}

// MakeCircleOnBoard creates a circle at Coord c with separate fill and stroke
// colors, a size (fraction of CellSize, clamped 0..1), and a stroke width
// factor. Returns nil if c is out of bounds.
func (GoWin *GoWin) MakeCircleOnBoard(c Coord, fillColor color.Color,
	strokeColor color.Color, size float32, stroke float32) *canvas.Circle {
	var circle *canvas.Circle

	if GoWin.Coll.CurrentGameH.InBounds(c) {
		switch {
		case size < 0:
			size = 0
		case size > 1:
			size = 1
		}
		circle = canvas.NewCircle(fillColor)
		circle.StrokeColor = strokeColor
		circle.StrokeWidth = GoWin.CellSize * size * stroke / 2
		circle.Resize(fyne.NewSize(GoWin.CellSize*size, GoWin.CellSize*size))
		pos := GoWin.BoardCoordsToPixel(c)
		circle.Move(fyne.Position{
			X: pos.X + 0.5*GoWin.CellSize - circle.Size().Width/2,
			Y: pos.Y + 0.5*GoWin.CellSize - circle.Size().Height/2,
		})
	}
	return circle
}

// DrawLastMoveHighlight draws a colored ring on the intersection of the
// last move to visually indicate where the most recent stone was placed.
func (GoWin *GoWin) DrawLastMoveHighlight() {
	if GoWin.CurrentNode().Board == nil {
		return
	}
	if GoWin.Coll.CurrentGameH.InBounds(GoWin.CurrentNode().LastMove) {
		if Highlight := GoWin.MakeCircleOnBoard(GoWin.CurrentNode().LastMove, Colors["Last Move"],
			PlayerColors[GoWin.CurrentNode().PreviousStonePlacer].Base, 0.39, 0.319); Highlight != nil {
			GoWin.Layers["LastMoveHighlight"].Add(Highlight)
		}
	}
}

// DrawChildNodeDots draws a small dot at every move-type child's LastMove,
// fill = Colors["Child Node"], stroke = the base color of the player who
// made that child move. Edit, pass, and root children are skipped.
func (GoWin *GoWin) DrawChildNodeDots() {
	if !GetActiveTheme().ShowChildNodeDots {
		return
	}
	if GoWin.CurrentNode().Board == nil {
		return
	}
	H := GoWin.Coll.CurrentGameH
	for _, Child := range GoWin.CurrentNode().Children {
		LM := Child.LastMove
		if LM == EditCoords || LM == PassCoords || LM == RootCoords {
			continue
		}
		if !H.InBounds(LM) {
			continue
		}
		Stroke := PlayerColors[Child.PreviousStonePlacer].Base
		if Dot := GoWin.MakeCircleOnBoard(LM, Colors["Child Node"], Stroke, 0.4, 0.51); Dot != nil {
			GoWin.Layers["LastMoveHighlight"].Add(Dot)
		}
	}
}

// CoordColumnLabel returns the column label string for column X using the
// active coordinate format. Falls back to HikaruNoGo if the current format
// cannot represent X.
func (GoWin *GoWin) CoordColumnLabel(X uint8) string {
	switch GetCoordFormat() {
	case CoordFmtComputer:
		return fmt.Sprintf("%d", X)
	case CoordFmtHikaruNoGo:
		return fmt.Sprintf("%dの", X+1)
	case CoordFmt1stQuadrant:
		return fmt.Sprintf("(%d,", X)
	case CoordFmtSgf:
		if GoWin.Coll.CurrentGameH.Width <= 52 && GoWin.Coll.CurrentGameH.Height <= 52 {
			CoordColumnLabel, err := Uint8ToSgfChar(X)
			if err == nil {
				return CoordColumnLabel + "?"
			}
		}
	case CoordFmtGtp:
		if GoWin.Coll.CurrentGameH.Width <= 25 && GoWin.Coll.CurrentGameH.Height <= 25 {
			GtpCoord, Err := GoWin.Coll.CurrentGameH.ClientToGTPCoords(Coord{X: X, Y: 0})
			if Err == nil {
				return GtpCoord[:1]
			}
		}
	}
	return fmt.Sprintf("%dの", X+1)
}

// CoordRowLabel returns the row label string for row Y using the active
// coordinate format. Falls back to HikaruNoGo if the current format cannot
// represent Y.
func (GoWin *GoWin) CoordRowLabel(Y uint8) string {
	switch GetCoordFormat() {
	case CoordFmtComputer:
		return fmt.Sprintf(",%d", Y)
	case CoordFmtHikaruNoGo:
		return fmt.Sprintf("の%d", Y+1)
	case CoordFmt1stQuadrant:
		return fmt.Sprintf(",%d)", GoWin.Height()-1-Y)
	case CoordFmtGtp:
		if GoWin.Coll.CurrentGameH.Width <= 25 && GoWin.Coll.CurrentGameH.Height <= 25 {
			GtpCoord, Err := GoWin.Coll.CurrentGameH.ClientToGTPCoords(Coord{X: 0, Y: Y})
			if Err == nil {
				return GtpCoord[1:]
			}
		}
	case CoordFmtSgf:
		if GoWin.Coll.CurrentGameH.Width <= 52 && GoWin.Coll.CurrentGameH.Height <= 52 {
			CoordRowLabel, err := Uint8ToSgfChar(Y)
			if err == nil {
				return "?" + CoordRowLabel
			}
		}
	}
	return fmt.Sprintf("の%d", Y+1)
}

// DrawAnnotation draws an annotation at the given coordinate with the specified color and mask
// into the provided layer container.
func (GoWin *GoWin) DrawAnnotation(mask uint8, c Coord, annotationColor color.Color, layer *fyne.Container) {
	pos := GoWin.BoardCoordsToPixel(c)

	switch mask {
	case CircleMask:
		circle := canvas.NewCircle(color.Transparent)
		circle.StrokeColor = annotationColor
		circle.StrokeWidth = GoWin.CellSize * 0.05
		circle.Resize(fyne.NewSize(GoWin.CellSize*0.6, GoWin.CellSize*0.6))
		circle.Move(fyne.Position{
			X: pos.X + 0.5*GoWin.CellSize - circle.Size().Width/2,
			Y: pos.Y + 0.5*GoWin.CellSize - circle.Size().Height/2,
		})
		layer.Add(circle)

	case SquareMask:
		square := canvas.NewRectangle(color.Transparent)
		square.StrokeColor = annotationColor
		square.StrokeWidth = GoWin.CellSize * 0.05
		square.Resize(fyne.NewSize(GoWin.CellSize*0.6, GoWin.CellSize*0.6))
		square.Move(fyne.Position{
			X: pos.X + 0.5*GoWin.CellSize - square.Size().Width/2,
			Y: pos.Y + 0.5*GoWin.CellSize - square.Size().Height/2,
		})
		layer.Add(square)

	case TriangleMask:
		tSize := GoWin.CellSize * 0.39
		tXOffset := tSize * float32(math.Sin(math.Pi/3))
		tYOffset := tSize * float32(math.Cos(math.Pi/3))

		pos0 := fyne.NewPos(pos.X+0.5*GoWin.CellSize, pos.Y+0.5*GoWin.CellSize-tSize)
		pos1 := fyne.NewPos(pos.X+0.5*GoWin.CellSize-tXOffset, pos.Y+0.5*GoWin.CellSize+tYOffset)
		pos2 := fyne.NewPos(pos.X+0.5*GoWin.CellSize+tXOffset, pos.Y+0.5*GoWin.CellSize+tYOffset)

		line1 := canvas.NewLine(annotationColor)
		line1.StrokeWidth = GoWin.CellSize * 0.05
		line1.Position1 = pos0
		line1.Position2 = pos1
		line2 := canvas.NewLine(annotationColor)
		line2.StrokeWidth = GoWin.CellSize * 0.05
		line2.Position1 = pos1
		line2.Position2 = pos2
		line3 := canvas.NewLine(annotationColor)
		line3.StrokeWidth = GoWin.CellSize * 0.05
		line3.Position1 = pos2
		line3.Position2 = pos0

		layer.Add(line1)
		layer.Add(line2)
		layer.Add(line3)

	case XMask:
		size := GoWin.CellSize * 0.6
		line1 := canvas.NewLine(annotationColor)
		line1.StrokeWidth = GoWin.CellSize * 0.05
		line1.Position1 = fyne.NewPos(pos.X+0.5*GoWin.CellSize-size/2, pos.Y+0.5*GoWin.CellSize-size/2)
		line1.Position2 = fyne.NewPos(pos.X+0.5*GoWin.CellSize+size/2, pos.Y+0.5*GoWin.CellSize+size/2)
		line2 := canvas.NewLine(annotationColor)
		line2.StrokeWidth = GoWin.CellSize * 0.05
		line2.Position1 = fyne.NewPos(pos.X+0.5*GoWin.CellSize+size/2, pos.Y+0.5*GoWin.CellSize-size/2)
		line2.Position2 = fyne.NewPos(pos.X+0.5*GoWin.CellSize-size/2, pos.Y+0.5*GoWin.CellSize+size/2)

		layer.Add(line1)
		layer.Add(line2)
	}
}

// DrawBoard redraws the board layers. The Goban layer (coord labels, goban
// rect, grid lines) is only rebuilt when the PlayContainer size changes or the goban dimensions change.
// All other game-state layers (LibertyLine through Label, Territory) are
// cleared and repopulated on every call.
func (GoWin *GoWin) DrawBoard() {
	if GoWin.CurrentNode() == GoWin.Coll.CollectionNode {
		for _, Key := range LayerOrder {
			GoWin.Layers[Key].Objects = nil
			GoWin.Layers[Key].Refresh()
		}
		GoWin.LastGobanKey = GobanLayerKey{}
		return
	}
	Width, Height := GoWin.Width(), GoWin.Height()
	DisplayWidth, DisplayHeight := GoWin.DisplayDims()

	CurrentSize := GoWin.PlayContainer.Size()
	GoWin.CellSize = min(CurrentSize.Width/float32(DisplayWidth+2),
		CurrentSize.Height/float32(DisplayHeight+2))

	// Redraw Goban layer only when container size, board dimensions, or symmetry change
	GobanKey := GobanLayerKey{Size: CurrentSize, Width: Width, Height: Height, SymmetryIndex: GoWin.SymmetryIndex}
	if GobanKey != GoWin.LastGobanKey {
		GoWin.LastGobanKey = GobanKey
		GoWin.DrawGoban()
	}

	// Clear game-state layers
	GameStateLayers := []string{
		"LibertyLine", "IllegalDot", "StoneConnection", "Stone",
		"LastMoveHighlight", "Annotation", "Label", "Territory",
	}
	for _, Key := range GameStateLayers {
		GoWin.Layers[Key].Objects = nil
	}

	// Populate game-state layers
	GoWin.DrawLibertyLines()
	GoWin.DrawEmptyIllegalMoves()
	GoWin.DrawStoneConnections()
	GoWin.DrawStones()
	GoWin.DrawLastMoveHighlight()
	GoWin.DrawChildNodeDots()
	GoWin.DrawTerritoryMarkers()
	GoWin.DrawAnnotations()

	// Refresh all game-state layers
	for _, Key := range GameStateLayers {
		GoWin.Layers[Key].Refresh()
	}
}

// DrawLibertyLines draws lines from each stone to its liberties for every group.
// Draws into the LibertyLine layer. Skipped if the active theme has ShowLibertyLine disabled.
func (GoWin *GoWin) DrawLibertyLines() {
	if !GetActiveTheme().ShowLibertyLine {
		return
	}
	LibertyLayer := GoWin.Layers["LibertyLine"]
	// DrawLineBetween draws a line between cell centers of two logically
	// adjacent cells A and B, but inset by half the line width at each end
	// so multiple incoming lines (e.g. an atari cell with 3 neighbors in the
	// same group) do not overlap at the shared vertex. The inset is applied
	// in source-space cell units and routed through BoardPointToPixel so the
	// line rotates with the goban under any SymmetryIndex.
	DrawLineBetween := func(A, B Coord, LineColor color.Color, Layer *fyne.Container) {
		DX := float32(B.X) - float32(A.X)
		DY := float32(B.Y) - float32(A.Y)
		Inset := float32(GridLineThickness) / 2
		StartX := float32(A.X) + 0.5 + DX*Inset
		StartY := float32(A.Y) + 0.5 + DY*Inset
		EndX := float32(B.X) + 0.5 - DX*Inset
		EndY := float32(B.Y) + 0.5 - DY*Inset
		Line := canvas.NewLine(LineColor)
		Line.Position1 = GoWin.BoardPointToPixel(StartX, StartY)
		Line.Position2 = GoWin.BoardPointToPixel(EndX, EndY)
		Line.StrokeWidth = GoWin.CellSize * GridLineThickness
		Layer.Add(Line)
	}
	// Wrap-seam stub: draw a line in source space from the center of cell C
	// toward one of its edges (Direction: 'N', 'S', 'E', 'W'). Endpoints go
	// through BoardPointToPixel so the stub rotates with the goban.
	DrawStub := func(C Coord, Direction byte, LineColor color.Color, Layer *fyne.Container) {
		CenterX, CenterY := float32(C.X)+0.5, float32(C.Y)+0.5
		EdgeX, EdgeY := CenterX, CenterY
		var DX, DY float32
		switch Direction {
		case 'W':
			EdgeX = float32(C.X)
			DX = -1
		case 'E':
			EdgeX = float32(C.X) + 1
			DX = 1
		case 'N':
			EdgeY = float32(C.Y)
			DY = -1
		case 'S':
			EdgeY = float32(C.Y) + 1
			DY = 1
		}
		Inset := float32(GridLineThickness) / 2
		CenterX += DX * Inset
		CenterY += DY * Inset
		Line := canvas.NewLine(LineColor)
		Line.Position1 = GoWin.BoardPointToPixel(CenterX, CenterY)
		Line.Position2 = GoWin.BoardPointToPixel(EdgeX, EdgeY)
		Line.StrokeWidth = GoWin.CellSize * GridLineThickness
		Layer.Add(Line)
	}
	GHist := GoWin.Coll.CurrentGameH
	Board := GoWin.CurrentNode().Board
	for Group := range GoWin.CurrentNode().Board.Groups {
		var LineColor color.Color
		switch len(Group.Vertices[0]) {
		case 1:
			LineColor = Colors["1-Liberty Line"]
		case 2:
			LineColor = Colors["2-Liberty Line"]
		case 3:
			LineColor = Colors["3-Liberty Line"]
		case 4:
			LineColor = Colors["4-Liberty Line"]
		default:
			LineColor = Colors["≥5-Liberty Line"]
		}
		for _, Liberty := range Group.Vertices[0] {
			West := Coord{X: Liberty.X - 1, Y: Liberty.Y}
			if GoWin.Coll.CurrentGameH.InBounds(West) &&
				GoWin.CurrentNode().Board.GetGroupAtCoord(West) == Group {
				DrawLineBetween(West, Liberty, LineColor, LibertyLayer)
			}
			East := Coord{X: Liberty.X + 1, Y: Liberty.Y}
			if GoWin.Coll.CurrentGameH.InBounds(East) &&
				GoWin.CurrentNode().Board.GetGroupAtCoord(East) == Group {
				DrawLineBetween(Liberty, East, LineColor, LibertyLayer)
			}
			North := Coord{X: Liberty.X, Y: Liberty.Y - 1}
			if GoWin.Coll.CurrentGameH.InBounds(North) &&
				GoWin.CurrentNode().Board.GetGroupAtCoord(North) == Group {
				DrawLineBetween(North, Liberty, LineColor, LibertyLayer)
			}
			South := Coord{X: Liberty.X, Y: Liberty.Y + 1}
			if GoWin.Coll.CurrentGameH.InBounds(South) &&
				GoWin.CurrentNode().Board.GetGroupAtCoord(South) == Group {
				DrawLineBetween(Liberty, South, LineColor, LibertyLayer)
			}
			// Wrap-connected neighbors: draw a stub from each end toward the
			// goban edge so the visual break across the wrap seam stays legible.
			if Liberty.X == 0 && GHist.WrapXMulY != 0 {
				WrappedY := Int16ToUInt8Modulo(
					(int16(Liberty.Y)-int16(GHist.WrapXShiftY))*GHist.WrapXMulYInv,
					GHist.Height)
				Wrapped := Coord{X: GHist.Width - 1, Y: WrappedY}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'W', LineColor, LibertyLayer)
					DrawStub(Wrapped, 'E', LineColor, LibertyLayer)
				}
			}
			if Liberty.X == GHist.Width-1 && GHist.WrapXMulY != 0 {
				WrappedY := Int16ToUInt8Modulo(
					int16(Liberty.Y)*int16(GHist.WrapXMulY)+int16(GHist.WrapXShiftY),
					GHist.Height)
				Wrapped := Coord{X: 0, Y: WrappedY}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'E', LineColor, LibertyLayer)
					DrawStub(Wrapped, 'W', LineColor, LibertyLayer)
				}
			}
			if Liberty.Y == 0 && GHist.WrapYMulX != 0 {
				WrappedX := Int16ToUInt8Modulo(
					(int16(Liberty.X)-int16(GHist.WrapYShiftX))*GHist.WrapYMulXInv,
					GHist.Width)
				Wrapped := Coord{X: WrappedX, Y: GHist.Height - 1}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'N', LineColor, LibertyLayer)
					DrawStub(Wrapped, 'S', LineColor, LibertyLayer)
				}
			}
			if Liberty.Y == GHist.Height-1 && GHist.WrapYMulX != 0 {
				WrappedX := Int16ToUInt8Modulo(
					int16(Liberty.X)*int16(GHist.WrapYMulX)+int16(GHist.WrapYShiftX),
					GHist.Width)
				Wrapped := Coord{X: WrappedX, Y: 0}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'S', LineColor, LibertyLayer)
					DrawStub(Wrapped, 'N', LineColor, LibertyLayer)
				}
			}
		}
	}
}

// DrawGoban clears and rebuilds the Goban layer: coord background, coord labels,
// goban rectangle, and grid lines. Called only when needed, e.g. on resize (PlayContainer size change).
func (GoWin *GoWin) DrawGoban() {
	GobanLayer := GoWin.Layers["Goban"]
	GobanLayer.Objects = nil
	Width, Height := GoWin.Width(), GoWin.Height()
	DisplayWidth, DisplayHeight := GoWin.DisplayDims()

	// Draw coord background — anchored in display space so it covers the full
	// outer rect (including label margins) regardless of symmetry.
	Pos := GoWin.BoardUnitsToPixel(-1, -1)
	CoordBg := canvas.NewRectangle(Colors["Coord Background"])
	CoordBg.StrokeWidth = 0
	CoordBg.Resize(fyne.NewSize(GoWin.CellSize*float32(DisplayWidth+2),
		GoWin.CellSize*float32(DisplayHeight+2)))
	CoordBg.Move(Pos)
	GobanLayer.Add(CoordBg)

	// Draw coord labels
	DrawCoordLabel := func(C Coord, CoordLabel string) {
		LabelPos := GoWin.BoardCoordsToPixel(C)
		Text := canvas.NewText(CoordLabel, Colors["Coord"])
		Text.TextSize = GoWin.CellSize * 0.39
		Text.Alignment = fyne.TextAlignCenter
		Text.TextStyle = fyne.TextStyle{Bold: true}
		Text.Resize(Text.MinSize())
		Text.Move(fyne.Position{
			X: LabelPos.X + 0.5*GoWin.CellSize - Text.Size().Width/2,
			Y: LabelPos.Y + 0.5*GoWin.CellSize - Text.Size().Height/2,
		})
		GobanLayer.Add(Text)
	}
	var X, Y uint8
	for X = range Width {
		ColumnLabel := GoWin.CoordColumnLabel(X)
		for _, C := range []Coord{{X: X, Y: 0xff}, {X: X, Y: Height}} {
			DrawCoordLabel(C, ColumnLabel)
		}
	}
	for Y = range Height {
		RowLabel := GoWin.CoordRowLabel(Y)
		for _, C := range []Coord{{X: 0xff, Y: Y}, {X: Width, Y: Y}} {
			DrawCoordLabel(C, RowLabel)
		}
	}

	// Draw goban rectangle — anchored in display space so size matches the
	// rotated board's bounding box.
	GobanPos := GoWin.BoardUnitsToPixel(0, 0)
	GobanRect := canvas.NewRectangle(Colors["Goban"])
	GobanRect.StrokeWidth = 0
	GobanRect.Resize(fyne.NewSize(GoWin.CellSize*float32(DisplayWidth),
		GoWin.CellSize*float32(DisplayHeight)))
	GobanRect.Move(GobanPos)
	GobanLayer.Add(GobanRect)

	// Grid lines are drawn in source-space cell units, then rotated through
	// BoardPointToPixel. When the board wraps in a given axis, the line
	// extends past the edge intersections to the outer goban edge so the
	// wrap seam reads as a continuous board. Under non-identity symmetries
	// the rotation is applied at the point level, so wrap-Y / wrap-X stay
	// correct regardless of how axes swap.
	// Extend each grid line by half the line width past the outer
	// intersections so adjacent grid lines meet flush at the corner vertices
	// rather than leaving a gap there. Wrap axes push the extension all the
	// way to the goban edge instead, so the seam reads as continuous.
	GHist := GoWin.Coll.CurrentGameH
	HalfWidth := float32(GridLineThickness) / 2
	VerticalLineStart := -HalfWidth
	VerticalLineEnd := float32(Height-1) + HalfWidth
	if GHist != nil && GHist.WrapYMulX != 0 {
		VerticalLineStart = -0.5
		VerticalLineEnd = float32(Height-1) + 0.5
	}
	HorizontalLineStart := -HalfWidth
	HorizontalLineEnd := float32(Width-1) + HalfWidth
	if GHist != nil && GHist.WrapXMulY != 0 {
		HorizontalLineStart = -0.5
		HorizontalLineEnd = float32(Width-1) + 0.5
	}

	// Vertical (source-space) grid lines, one per column.
	for X = range Width {
		Line := canvas.NewLine(Colors["Goban Line"])
		Pos1 := GoWin.BoardPointToPixel(float32(X)+0.5, VerticalLineStart+0.5)
		Pos2 := GoWin.BoardPointToPixel(float32(X)+0.5, VerticalLineEnd+0.5)
		Line.Position1 = Pos1
		Line.Position2 = Pos2
		Line.StrokeWidth = GoWin.CellSize * GridLineThickness
		GobanLayer.Add(Line)
	}

	// Horizontal (source-space) grid lines, one per row.
	for Y = range Height {
		Line := canvas.NewLine(Colors["Goban Line"])
		Pos1 := GoWin.BoardPointToPixel(HorizontalLineStart+0.5, float32(Y)+0.5)
		Pos2 := GoWin.BoardPointToPixel(HorizontalLineEnd+0.5, float32(Y)+0.5)
		Line.Position1 = Pos1
		Line.Position2 = Pos2
		Line.StrokeWidth = GoWin.CellSize * GridLineThickness
		GobanLayer.Add(Line)
	}

	GobanLayer.Refresh()
}

// DrawStoneConnections draws group connection fills into the StoneConnection layer.
// Skipped if the active theme has ShowStoneConnection disabled.
func (GoWin *GoWin) DrawStoneConnections() {
	if !GetActiveTheme().ShowStoneConnection {
		return
	}
	ConnectionLayer := GoWin.Layers["StoneConnection"]
	// Draw 3+ stone connections to represent groups
	for Y := uint8(1); Y < GoWin.Height(); Y++ {
		for X := uint8(1); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			//DU // Stone arrangement
			//LO
			LStone := GoWin.CurrentNode().Board.GetPlayerAt(Coord{X: X - 1, Y: Y})
			OStone := GoWin.CurrentNode().Board.GetPlayerAt(C)
			UStone := GoWin.CurrentNode().Board.GetPlayerAt(Coord{X: X, Y: Y - 1})
			DStone := GoWin.CurrentNode().Board.GetPlayerAt(Coord{X: X - 1, Y: Y - 1})
			StonesToConsider := []uint8{LStone, OStone, UStone, DStone}
			playerCounts := make(map[uint8]int)
			for _, val := range StonesToConsider {
				if 0 < val && val <= GoWin.Coll.CurrentGameH.Players {
					playerCounts[val]++
				}
			}
			// Find the player with majority (>2 out of 4 stones)
			var MajorityPlayer uint8
			for player, count := range playerCounts {
				if count > 2 {
					MajorityPlayer = player
					break
				}
			}
			if MajorityPlayer == 0 {
				continue
			}
			// The square-minus-4-circles image (and its all-4 solid-square
			// degenerate case) is centered on the vertex at source point
			// (C.X, C.Y) — the corner shared by the four cells we just
			// inspected. Route that vertex through BoardPointToPixel so the
			// image rotates with the goban under any SymmetryIndex.
			VertexCenter := GoWin.BoardPointToPixel(float32(C.X), float32(C.Y))
			ImageTopLeft := fyne.NewPos(VertexCenter.X-0.5*GoWin.CellSize,
				VertexCenter.Y-0.5*GoWin.CellSize)
			if len(playerCounts) == 1 && playerCounts[MajorityPlayer] == 4 {
				Square := canvas.NewRectangle(PlayerColors[MajorityPlayer].Base)
				Square.StrokeWidth = 0
				Square.Resize(fyne.NewSquareSize(GoWin.CellSize))
				Square.Move(ImageTopLeft)
				ConnectionLayer.Add(Square)
			} else {
				PixelCellSize := int(GoWin.CellSize * GoWin.Win.Canvas().Scale())
				Image, Err := MakeSquareMinusFourCirclesImage(
					PlayerColors[MajorityPlayer].Base, PixelCellSize)
				if Err != nil {
					log.Printf("MakeSquareMinusFourCirclesImage failed with (%d, %d)", MajorityPlayer, PixelCellSize)
					continue
				}
				Image.Resize(fyne.NewSquareSize(GoWin.CellSize))
				Image.Move(ImageTopLeft)
				ConnectionLayer.Add(Image)
			}
		}
	}

	// DrawConnectionBetween draws a CS×CS rectangle centered on the midpoint
	// of the cell centers of logically-adjacent cells A and B, in the color
	// of Stone. Rotation-agnostic: reads both endpoints' display pixels so
	// the connection rect auto-orients to the rotated direction between A
	// and B.
	DrawConnectionBetween := func(A, B Coord, Stone uint8) {
		Rect := canvas.NewRectangle(PlayerColors[Stone].Base)
		Rect.StrokeWidth = 0
		PA := GoWin.BoardCoordsToPixel(A)
		PB := GoWin.BoardCoordsToPixel(B)
		Rect.Move(fyne.NewPos((PA.X+PB.X)/2, (PA.Y+PB.Y)/2))
		Rect.Resize(fyne.NewSize(GoWin.CellSize, GoWin.CellSize))
		ConnectionLayer.Add(Rect)
	}

	// Draw vertical (logical) stone connections
	for Y := uint8(1); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			A := Coord{X, Y - 1}
			B := Coord{X, Y}
			Stone1 := GoWin.CurrentNode().Board.GetPlayerAt(A)
			Stone2 := GoWin.CurrentNode().Board.GetPlayerAt(B)
			if Stone1 != 0 && Stone1 == Stone2 {
				DrawConnectionBetween(A, B, Stone1)
			}
		}
	}

	// Draw horizontal (logical) stone connections
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(1); X < GoWin.Width(); X++ {
			A := Coord{X - 1, Y}
			B := Coord{X, Y}
			Stone1 := GoWin.CurrentNode().Board.GetPlayerAt(A)
			Stone2 := GoWin.CurrentNode().Board.GetPlayerAt(B)
			if Stone1 != 0 && Stone1 == Stone2 {
				DrawConnectionBetween(A, B, Stone1)
			}
		}
	}

	// Wrap-connected stone pairs: draw a half-cell rect on each side of the
	// seam so the connection aesthetic continues across the wrap edge.
	// Source-space approach: describe the half-rect as Cell + source-space
	// Direction (N/S/E/W), then let BoardRectToPixel rotate it. The source
	// direction W becomes whatever display edge W maps to under the active
	// symmetry, so half-rects always align with the rotated goban seam.
	GHist := GoWin.Coll.CurrentGameH
	Board := GoWin.CurrentNode().Board
	AddHalfRect := func(Player uint8, Cell Coord, Direction byte) {
		Px, Py := float32(Cell.X), float32(Cell.Y)
		var Sx, Sy float32
		switch Direction {
		case 'W':
			Sx, Sy = 0.5, 1
		case 'E':
			Px, Sx, Sy = Px+0.5, 0.5, 1
		case 'N':
			Sx, Sy = 1, 0.5
		case 'S':
			Py, Sx, Sy = Py+0.5, 1, 0.5
		}
		Pos, Size := GoWin.BoardRectToPixel(Px, Py, Sx, Sy)
		Rect := canvas.NewRectangle(PlayerColors[Player].Base)
		Rect.StrokeWidth = 0
		Rect.Resize(Size)
		Rect.Move(Pos)
		ConnectionLayer.Add(Rect)
	}
	if GHist.WrapXMulY != 0 {
		for Y := uint8(0); Y < GHist.Height; Y++ {
			LeftStone := Board.GetPlayerAt(Coord{X: 0, Y: Y})
			if LeftStone == 0 {
				continue
			}
			WrappedY := Int16ToUInt8Modulo(
				(int16(Y)-int16(GHist.WrapXShiftY))*GHist.WrapXMulYInv,
				GHist.Height)
			RightCoord := Coord{X: GHist.Width - 1, Y: WrappedY}
			if Board.GetPlayerAt(RightCoord) != LeftStone {
				continue
			}
			AddHalfRect(LeftStone, Coord{X: 0, Y: Y}, 'W')
			AddHalfRect(LeftStone, RightCoord, 'E')
		}
	}
	if GHist.WrapYMulX != 0 {
		for X := uint8(0); X < GHist.Width; X++ {
			TopStone := Board.GetPlayerAt(Coord{X: X, Y: 0})
			if TopStone == 0 {
				continue
			}
			WrappedX := Int16ToUInt8Modulo(
				(int16(X)-int16(GHist.WrapYShiftX))*GHist.WrapYMulXInv,
				GHist.Width)
			BottomCoord := Coord{X: WrappedX, Y: GHist.Height - 1}
			if Board.GetPlayerAt(BottomCoord) != TopStone {
				continue
			}
			AddHalfRect(TopStone, Coord{X: X, Y: 0}, 'N')
			AddHalfRect(TopStone, BottomCoord, 'S')
		}
	}
}

// DrawStones draws all stones into the Stone layer.
func (GoWin *GoWin) DrawStones() {
	StoneLayer := GoWin.Layers["Stone"]
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			Stone := GoWin.CurrentNode().Board.GetPlayerAt(C)
			if Stone != 0 {
				Circle := canvas.NewCircle(PlayerColors[Stone].Base)
				Circle.StrokeWidth = 0
				Pos := GoWin.BoardCoordsToPixel(C)
				Circle.Resize(fyne.NewSize(GoWin.CellSize, GoWin.CellSize))
				Circle.Move(Pos)
				StoneLayer.Add(Circle)
			}
		}
	}
}

// DrawAnnotations draws (Circle / Square / Triangle / X Mark) shapes into the Annotation layer and
// LB text labels into the Label layer.
func (GoWin *GoWin) DrawAnnotations() {
	AnnotationLayer := GoWin.Layers["Annotation"]
	LabelLayer := GoWin.Layers["Label"]

	// Draw annotation bits from Vertices
	AnnotationMaskOrder := []uint8{CircleMask, SquareMask, TriangleMask, XMask}
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			VertexValue := GoWin.CurrentNode().Board.Vertices[C]
			for _, Mask := range AnnotationMaskOrder {
				if VertexValue&Mask != 0 {
					GoWin.DrawAnnotation(Mask, C, Colors["Annotation"], AnnotationLayer)
				}
			}
		}
	}

	// Draw Labels (LB) into the Label layer
	for C, LabelText := range GoWin.CurrentNode().Labels {
		Pos := GoWin.BoardCoordsToPixel(C)
		Text := canvas.NewText(LabelText, Colors["Annotation"])
		Text.TextSize = GoWin.CellSize * 0.39
		Text.Alignment = fyne.TextAlignCenter
		Text.TextStyle = fyne.TextStyle{Bold: true}
		Text.Resize(Text.MinSize())
		Text.Move(fyne.Position{
			X: Pos.X + 0.5*GoWin.CellSize - Text.Size().Width/2,
			Y: Pos.Y + 0.5*GoWin.CellSize - Text.Size().Height/2,
		})
		LabelLayer.Add(Text)
	}
}

// DrawEmptyIllegalMoves places small colored circles on every empty
// intersection where placing a stone would be illegal. In Set Vertex mode
// it checks for the selected player with EditMode; in Play mode it checks
// for the next player. Skipped in Score mode, Set Vertex delete mode, and
// when the active theme has ShowIllegalDot disabled.
func (GoWin *GoWin) DrawEmptyIllegalMoves() {
	if GoWin.MouseMode == "Score" || !GetActiveTheme().ShowIllegalDot {
		return
	}
	Board := GoWin.CurrentNode().Board
	var CheckPlayer uint8
	var IsEditMode bool
	switch GoWin.MouseMode {
	case "Set Vertex":
		if GoWin.SetVertexPlayer == 0 {
			return
		}
		CheckPlayer = GoWin.SetVertexPlayer
		IsEditMode = true
	default:
		CheckPlayer = GoWin.CurrentNode().GetNextStonePlacer()
		IsEditMode = false
	}
	Width, Height := GoWin.Width(), GoWin.Height()
	IllegalLayer := GoWin.Layers["IllegalDot"]

	for X := range Width {
		for Y := range Height {
			C := Coord{X, Y}
			if Board.GetPlayerAt(C) != 0 {
				continue
			}
			if _, PlacementErr := Board.AttemptMove(C, CheckPlayer, IsEditMode, true); PlacementErr != nil {
				Illegal := GoWin.MakeSolidCircleOnBoard(C, Colors["Illegal"], 0.51)
				IllegalLayer.Add(Illegal)
			}
		}
	}
}

// DrawTerritoryMarkers draws territory markers into the Territory layer when in Score mode.
func (GoWin *GoWin) DrawTerritoryMarkers() {
	if GoWin.MouseMode != "Score" {
		return
	}
	TerritoryLayer := GoWin.Layers["Territory"]
	for C, Owner := range GoWin.CurrentNode().TerritoryMap {
		if 0 < Owner && Owner <= GoWin.Coll.CurrentGameH.Players {
			Rect := canvas.NewRectangle(PlayerColors[Owner].Territory)
			Rect.StrokeColor = Colors["Territory Stroke"]
			Rect.StrokeWidth = GoWin.CellSize * 0.093
			SquareSize := GoWin.CellSize * 0.54
			Pos := GoWin.BoardCoordsToPixel(C)
			Pos = fyne.Position{X: Pos.X + 0.5*GoWin.CellSize - SquareSize/2, Y: Pos.Y + 0.5*GoWin.CellSize - SquareSize/2}
			Rect.Resize(fyne.NewSize(SquareSize, SquareSize))
			Rect.Move(Pos)
			TerritoryLayer.Add(Rect)
		}
	}
}

// Converts board coordinates to pixel positions for rendering.
// 0xff is used as -1 for label positions outside the board.
// RefreshSymmetryMenuItems disables the menu item for the active SymmetryIndex
// (its action would be a no-op) and enables the rest. Propagates the change
// to the Win via SetMainMenu, matching RefreshEngineMenuItems / RefreshPassMenuItem.
func (GoWin *GoWin) RefreshSymmetryMenuItems() {
	for Index, Item := range GoWin.SymmetryItems {
		Item.Disabled = uint8(Index) == GoWin.SymmetryIndex
	}
	if GoWin.MainMenu != nil {
		GoWin.Win.SetMainMenu(GoWin.MainMenu)
	}
}

// TransformBoardCoord applies the active SymmetryIndex to (X, Y) (which may
// each be -1 for the label area outside the top/left edge), returning the
// display coordinates (DX, DY) and the display bounding-box dimensions
// (DW, DH). For odd symmetry indices the display swaps Width and Height.
func (GoWin *GoWin) TransformBoardCoord(X, Y int) (DX, DY, DW, DH int) {
	W, H := int(GoWin.Width()), int(GoWin.Height())
	switch GoWin.SymmetryIndex {
	case 1:
		return H - 1 - Y, X, H, W
	case 2:
		return W - 1 - X, H - 1 - Y, W, H
	case 3:
		return Y, W - 1 - X, H, W
	case 4:
		return W - 1 - X, Y, W, H
	case 5:
		return H - 1 - Y, W - 1 - X, H, W
	case 6:
		return X, H - 1 - Y, W, H
	case 7:
		return Y, X, H, W
	}
	return X, Y, W, H
}

// InverseTransformDisplayCoord is the inverse of TransformBoardCoord: given
// display coordinates (DX, DY), return the logical board (X, Y) under the
// active SymmetryIndex.
func (GoWin *GoWin) InverseTransformDisplayCoord(DX, DY int) (X, Y int) {
	W, H := int(GoWin.Width()), int(GoWin.Height())
	switch GoWin.SymmetryIndex {
	case 1:
		return DY, H - 1 - DX
	case 2:
		return W - 1 - DX, H - 1 - DY
	case 3:
		return W - 1 - DY, DX
	case 4:
		return W - 1 - DX, DY
	case 5:
		return W - 1 - DY, H - 1 - DX
	case 6:
		return DX, H - 1 - DY
	case 7:
		return DY, DX
	}
	return DX, DY
}

// DisplayDims returns the board's outer bounding-box dimensions after the
// active SymmetryIndex is applied — Width and Height are swapped for odd
// symmetry indices (90°/270° rotations and the transpose variants).
func (GoWin *GoWin) DisplayDims() (uint8, uint8) {
	if GoWin.SymmetryIndex&1 != 0 {
		return GoWin.Height(), GoWin.Width()
	}
	return GoWin.Width(), GoWin.Height()
}

// RotateBoardPoint applies SymmetryIndex to a source-space board-unit point
// (continuous; cell units). A source point at (px, py) means "px cells east,
// py cells south of the goban's top-left intersection". The returned
// (dpx, dpy) is the same point expressed in DISPLAY-space board units after
// the active symmetry rotates/mirrors the goban. Points outside [0, W]×[0, H]
// (e.g. -1 or Width+1 for label margins) transform correctly.
func (GoWin *GoWin) RotateBoardPoint(px, py float32) (dpx, dpy float32) {
	W, H := float32(GoWin.Width()), float32(GoWin.Height())
	switch GoWin.SymmetryIndex {
	case 1:
		return H - py, px
	case 2:
		return W - px, H - py
	case 3:
		return py, W - px
	case 4:
		return W - px, py
	case 5:
		return H - py, W - px
	case 6:
		return px, H - py
	case 7:
		return py, px
	}
	return px, py
}

// BoardUnitsToPixel converts a DISPLAY-space board-unit point (dpx, dpy) to
// a pixel position. No rotation — callers who want to rotate first should
// use BoardPointToPixel or BoardRectToPixel.
func (GoWin *GoWin) BoardUnitsToPixel(dpx, dpy float32) fyne.Position {
	Size := GoWin.PlayContainer.Size()
	DW, DH := GoWin.DisplayDims()
	return fyne.NewPos(
		(Size.Width+(2*dpx-float32(DW))*GoWin.CellSize)/2,
		(Size.Height+(2*dpy-float32(DH))*GoWin.CellSize)/2,
	)
}

// BoardPointToPixel rotates a source-space board-unit point through the
// active symmetry, then converts to a pixel position.
func (GoWin *GoWin) BoardPointToPixel(px, py float32) fyne.Position {
	dpx, dpy := GoWin.RotateBoardPoint(px, py)
	return GoWin.BoardUnitsToPixel(dpx, dpy)
}

// BoardRectToPixel takes a source-space rectangle (top-left (px, py), size
// (sx, sy)) in cell units, rotates it under the active symmetry, and returns
// the DISPLAY-ALIGNED pixel top-left and size. Works for negative positions
// (label margins) and non-cell-aligned positions (e.g. connection rects that
// straddle a cell boundary).
func (GoWin *GoWin) BoardRectToPixel(px, py, sx, sy float32) (fyne.Position, fyne.Size) {
	X1, Y1 := GoWin.RotateBoardPoint(px, py)
	X2, Y2 := GoWin.RotateBoardPoint(px+sx, py+sy)
	MinX, MaxX := min(X1, X2), max(X1, X2)
	MinY, MaxY := min(Y1, Y2), max(Y1, Y2)
	return GoWin.BoardUnitsToPixel(MinX, MinY),
		fyne.NewSize((MaxX-MinX)*GoWin.CellSize, (MaxY-MinY)*GoWin.CellSize)
}

// BoardCoordsToPixel returns the display-space pixel of the top-left corner
// of cell c after the active symmetry. Sentinel 0xff maps to -1 (label
// margin outside top/left); c.X==Width or c.Y==Height are also supported
// and land in the bottom/right label margin. Delegates to BoardRectToPixel
// so non-identity symmetries yield the display-aligned top-left (not the
// rotated source top-left, which would land on the wrong corner).
func (GoWin *GoWin) BoardCoordsToPixel(c Coord) fyne.Position {
	x := float32(c.X)
	if c.X == 0xff {
		x = -1
	}
	y := float32(c.Y)
	if c.Y == 0xff {
		y = -1
	}
	Pos, _ := GoWin.BoardRectToPixel(x, y, 1, 1)
	return Pos
}
