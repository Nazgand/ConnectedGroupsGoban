package main

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// ImageRenderer abstracts the primitive drawing calls issued by the
// board traversal, so a single walk over the current position can emit
// either an exact-primitive SVG or a rasterized PNG.
//
// All positions and dimensions are in output pixel units. Colors are
// NRGBA (A == 0xff for opaque). Stroke widths are absolute pixel widths
// centered on the drawn path, matching SVG's stroke rendering model.
type ImageRenderer interface {
	FillRect(X, Y, W, H float32, Fill color.NRGBA)
	FillCircle(CX, CY, R float32, Fill color.NRGBA)
	StrokeCircle(CX, CY, R, StrokeWidth float32, Fill, Stroke color.NRGBA)
	StrokeRect(X, Y, W, H, StrokeWidth float32, Fill, Stroke color.NRGBA)
	StrokeLine(X1, Y1, X2, Y2, StrokeWidth float32, Stroke color.NRGBA)
	StrokePolygon(Points []float32, StrokeWidth float32, Stroke color.NRGBA)
	DrawText(CX, CY, FontSize float32, Content string, Fill color.NRGBA)
	Draw3MeetShape(X, Y, Size float32, Fill color.NRGBA)
}

// BoardDrawer walks a specific game-tree node's board state and emits
// each visual layer through the ImageRenderer. The coordinate helpers
// mirror BoardDrawing.go's symmetry-aware routines but target a
// PlayContainer-independent coordinate system: the output spans exactly
// CellSize * (DisplayWidth+2) by CellSize * (DisplayHeight+2) pixels
// with one-cell margin on every side for coord labels.
//
// Node defaults to Win.CurrentNode() but can be overridden so animated
// exports can render each node on the favorite-child path without
// mutating window state.
type BoardDrawer struct {
	Win           *GoWin
	Node          *GameTreeNode
	CellSize      float32
	DisplayWidth  uint8
	DisplayHeight uint8
	R             ImageRenderer
}

// NewBoardDrawer builds a drawer for Win at the given pixel CellSize,
// dispatching primitive calls to R. Node defaults to Win.CurrentNode().
func NewBoardDrawer(Win *GoWin, CellSize float32, R ImageRenderer) *BoardDrawer {
	DW, DH := Win.DisplayDims()
	return &BoardDrawer{Win: Win, Node: Win.CurrentNode(), CellSize: CellSize, DisplayWidth: DW, DisplayHeight: DH, R: R}
}

// TotalSize returns the output image's pixel dimensions.
func (B *BoardDrawer) TotalSize() (W, H float32) {
	return B.CellSize * float32(B.DisplayWidth+2), B.CellSize * float32(B.DisplayHeight+2)
}

// DisplayPoint maps a display-space board-unit point (where (0, 0) is the
// goban top-left and (-1, -1) is the label-margin top-left) to output
// pixel coordinates.
func (B *BoardDrawer) DisplayPoint(DPX, DPY float32) (float32, float32) {
	return (DPX + 1) * B.CellSize, (DPY + 1) * B.CellSize
}

// Point rotates a source-space board-unit point through the active
// SymmetryIndex via RotateBoardPoint and returns output pixel
// coordinates.
func (B *BoardDrawer) Point(PX, PY float32) (float32, float32) {
	DPX, DPY := B.Win.RotateBoardPoint(PX, PY)
	return B.DisplayPoint(DPX, DPY)
}

// Rect rotates a source-space rectangle's two corners and returns the
// display-aligned pixel top-left and size — mirroring BoardRectToPixel.
func (B *BoardDrawer) Rect(PX, PY, SX, SY float32) (X, Y, W, H float32) {
	X1, Y1 := B.Win.RotateBoardPoint(PX, PY)
	X2, Y2 := B.Win.RotateBoardPoint(PX+SX, PY+SY)
	MinX, MaxX := min(X1, X2), max(X1, X2)
	MinY, MaxY := min(Y1, Y2), max(Y1, Y2)
	TLx, TLy := B.DisplayPoint(MinX, MinY)
	return TLx, TLy, (MaxX - MinX) * B.CellSize, (MaxY - MinY) * B.CellSize
}

// CellTopLeft returns the output pixel top-left of Cell C, honoring the
// 0xff sentinel (label margin) via -1 substitution.
func (B *BoardDrawer) CellTopLeft(C Coord) (float32, float32) {
	X := float32(C.X)
	if C.X == 0xff {
		X = -1
	}
	Y := float32(C.Y)
	if C.Y == 0xff {
		Y = -1
	}
	x, y, _, _ := B.Rect(X, Y, 1, 1)
	return x, y
}

// CellCenter returns the output pixel center of Cell C.
func (B *BoardDrawer) CellCenter(C Coord) (float32, float32) {
	PX := float32(C.X)
	if C.X == 0xff {
		PX = -1
	}
	PY := float32(C.Y)
	if C.Y == 0xff {
		PY = -1
	}
	return B.Point(PX+0.5, PY+0.5)
}

// DrawAll draws every visible layer in bottom-to-top order mirroring
// LayerOrder / DrawBoard. Convenience wrapper that emits the static
// goban layer followed by the per-node layers.
func (B *BoardDrawer) DrawAll() {
	B.DrawGobanLayer()
	B.DrawNodeLayers()
}

// DrawGobanLayer emits the frame-invariant goban background: coord-bg
// rect, goban rect, grid lines, and coord labels. Safe to emit once
// and reuse across all frames of an animated export.
func (B *BoardDrawer) DrawGobanLayer() {
	B.drawGoban()
}

// DrawNodeLayers emits every layer that varies with the active node:
// liberty lines, illegal dots, stone connections, stones, last-move
// ring, annotations, labels, and territory markers for any node that
// has a populated TerritoryMap. Theme flags gate optional layers.
// Territory is gated per-node rather than by Win.MouseMode so that
// animated exports correctly render the territory map baked into each
// scored node (the final frame of an animated game commonly carries
// one) regardless of the window's current mouse mode.
func (B *BoardDrawer) DrawNodeLayers() {
	T := GetActiveTheme()
	if T.ShowLibertyLine {
		B.drawLibertyLines()
	}
	if T.ShowIllegalDot && B.Win.MouseMode != "Score" {
		B.drawIllegalDots()
	}
	if T.ShowStoneConnection {
		B.drawStoneConnections()
	}
	B.drawStones()
	B.drawLastMoveHighlight()
	if T.ShowChildNodeDots {
		B.drawChildNodeDots()
	}
	B.drawAnnotations()
	B.drawLabels()
	if len(B.Node.TerritoryMap) > 0 {
		B.drawTerritory()
	}
}

// DrawMoveCounterOverlay emits a two-line "Current\n/Last" playback
// counter in the empty upper-left corner cell. Anchored via
// DisplayPoint so the overlay stays at the *screen* upper-left
// regardless of SymmetryIndex (it is playback meta, not board content).
func (B *BoardDrawer) DrawMoveCounterOverlay(Current, Last int) {
	CS := B.CellSize
	CX, CY := B.DisplayPoint(-0.5, -0.5)
	FontSize := CS * 0.28
	Fill := Colors["Coord"]
	B.R.DrawText(CX, CY-CS*0.22, FontSize, strconv.Itoa(Current), Fill)
	B.R.DrawText(CX, CY+CS*0.22, FontSize, "/"+strconv.Itoa(Last), Fill)
}

// drawGoban emits the coord-background rect, goban rect, grid lines, and
// coord labels. Mirrors DrawGoban in BoardDrawing.go including its
// wrap-axis extension rules.
func (B *BoardDrawer) drawGoban() {
	Win := B.Win
	Width, Height := Win.Width(), Win.Height()
	DW, DH := B.DisplayWidth, B.DisplayHeight
	CS := B.CellSize

	BgX, BgY, BgW, BgH := B.Rect(-1, -1, float32(DW)+2, float32(DH)+2)
	B.R.FillRect(BgX, BgY, BgW, BgH, Colors["Coord Background"])

	GX, GY, GW, GH := B.Rect(0, 0, float32(DW), float32(DH))
	B.R.FillRect(GX, GY, GW, GH, Colors["Goban"])

	GHist := Win.Coll.CurrentGameH
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
	GridColor := Colors["Goban Line"]
	StrokeWidth := CS * GridLineThickness
	for X := range Width {
		X1, Y1 := B.Point(float32(X)+0.5, VerticalLineStart+0.5)
		X2, Y2 := B.Point(float32(X)+0.5, VerticalLineEnd+0.5)
		B.R.StrokeLine(X1, Y1, X2, Y2, StrokeWidth, GridColor)
	}
	for Y := range Height {
		X1, Y1 := B.Point(HorizontalLineStart+0.5, float32(Y)+0.5)
		X2, Y2 := B.Point(HorizontalLineEnd+0.5, float32(Y)+0.5)
		B.R.StrokeLine(X1, Y1, X2, Y2, StrokeWidth, GridColor)
	}

	FontSize := CS * 0.39
	TextColor := Colors["Coord"]
	for X := range Width {
		Txt := Win.CoordColumnLabel(X)
		for _, C := range []Coord{{X: X, Y: 0xff}, {X: X, Y: Height}} {
			CX, CY := B.CellCenter(C)
			B.R.DrawText(CX, CY, FontSize, Txt, TextColor)
		}
	}
	for Y := range Height {
		Txt := Win.CoordRowLabel(Y)
		for _, C := range []Coord{{X: 0xff, Y: Y}, {X: Width, Y: Y}} {
			CX, CY := B.CellCenter(C)
			B.R.DrawText(CX, CY, FontSize, Txt, TextColor)
		}
	}
}

// drawStones emits one filled circle per placed stone.
func (B *BoardDrawer) drawStones() {
	Win := B.Win
	Board := B.Node.Board
	CS := B.CellSize
	for Y := range Win.Height() {
		for X := range Win.Width() {
			C := Coord{X, Y}
			Player := Board.GetPlayerAt(C)
			if Player == 0 {
				continue
			}
			CX, CY := B.CellCenter(C)
			B.R.FillCircle(CX, CY, CS/2, PlayerColors[Player].Base)
		}
	}
}

// drawStoneConnections emits:
//   - 3+ stone vertex shapes via R.Draw3MeetShape (or a plain rect for the
//     degenerate all-4 case);
//   - pair connection rects between logically adjacent same-color stones;
//   - wrap half-rects on each side of the seam.
//
// Mirrors DrawStoneConnections.
func (B *BoardDrawer) drawStoneConnections() {
	Win := B.Win
	Board := B.Node.Board
	GHist := Win.Coll.CurrentGameH
	CS := B.CellSize

	for Y := uint8(1); Y < Win.Height(); Y++ {
		for X := uint8(1); X < Win.Width(); X++ {
			C := Coord{X, Y}
			LStone := Board.GetPlayerAt(Coord{X: X - 1, Y: Y})
			OStone := Board.GetPlayerAt(C)
			UStone := Board.GetPlayerAt(Coord{X: X, Y: Y - 1})
			DStone := Board.GetPlayerAt(Coord{X: X - 1, Y: Y - 1})
			Counts := map[uint8]int{}
			for _, V := range []uint8{LStone, OStone, UStone, DStone} {
				if 0 < V && V <= GHist.Players {
					Counts[V]++
				}
			}
			var Majority uint8
			for Player, Count := range Counts {
				if Count > 2 {
					Majority = Player
					break
				}
			}
			if Majority == 0 {
				continue
			}
			VX, VY := B.Point(float32(C.X), float32(C.Y))
			TLx, TLy := VX-0.5*CS, VY-0.5*CS
			Fill := PlayerColors[Majority].Base
			if len(Counts) == 1 && Counts[Majority] == 4 {
				B.R.FillRect(TLx, TLy, CS, CS, Fill)
			} else {
				B.R.Draw3MeetShape(TLx, TLy, CS, Fill)
			}
		}
	}

	DrawConnection := func(A, B2 Coord, Stone uint8) {
		AX, AY := B.CellTopLeft(A)
		BX, BY := B.CellTopLeft(B2)
		B.R.FillRect((AX+BX)/2, (AY+BY)/2, CS, CS, PlayerColors[Stone].Base)
	}
	for Y := uint8(1); Y < Win.Height(); Y++ {
		for X := range Win.Width() {
			A := Coord{X, Y - 1}
			Bc := Coord{X, Y}
			SA := Board.GetPlayerAt(A)
			SB := Board.GetPlayerAt(Bc)
			if SA != 0 && SA == SB {
				DrawConnection(A, Bc, SA)
			}
		}
	}
	for Y := range Win.Height() {
		for X := uint8(1); X < Win.Width(); X++ {
			A := Coord{X - 1, Y}
			Bc := Coord{X, Y}
			SA := Board.GetPlayerAt(A)
			SB := Board.GetPlayerAt(Bc)
			if SA != 0 && SA == SB {
				DrawConnection(A, Bc, SA)
			}
		}
	}

	AddHalfRect := func(Player uint8, Cell Coord, Direction byte) {
		PX, PY := float32(Cell.X), float32(Cell.Y)
		var SX, SY float32
		switch Direction {
		case 'W':
			SX, SY = 0.5, 1
		case 'E':
			PX, SX, SY = PX+0.5, 0.5, 1
		case 'N':
			SX, SY = 1, 0.5
		case 'S':
			PY, SX, SY = PY+0.5, 1, 0.5
		}
		RX, RY, RW, RH := B.Rect(PX, PY, SX, SY)
		B.R.FillRect(RX, RY, RW, RH, PlayerColors[Player].Base)
	}
	if GHist.WrapXMulY != 0 {
		for Y := range GHist.Height {
			Left := Board.GetPlayerAt(Coord{X: 0, Y: Y})
			if Left == 0 {
				continue
			}
			WrappedY := Int16ToUInt8Modulo(
				(int16(Y)-int16(GHist.WrapXShiftY))*GHist.WrapXMulYInv,
				GHist.Height)
			Right := Coord{X: GHist.Width - 1, Y: WrappedY}
			if Board.GetPlayerAt(Right) != Left {
				continue
			}
			AddHalfRect(Left, Coord{X: 0, Y: Y}, 'W')
			AddHalfRect(Left, Right, 'E')
		}
	}
	if GHist.WrapYMulX != 0 {
		for X := range GHist.Width {
			Top := Board.GetPlayerAt(Coord{X: X, Y: 0})
			if Top == 0 {
				continue
			}
			WrappedX := Int16ToUInt8Modulo(
				(int16(X)-int16(GHist.WrapYShiftX))*GHist.WrapYMulXInv,
				GHist.Width)
			Bottom := Coord{X: WrappedX, Y: GHist.Height - 1}
			if Board.GetPlayerAt(Bottom) != Top {
				continue
			}
			AddHalfRect(Top, Coord{X: X, Y: 0}, 'N')
			AddHalfRect(Top, Bottom, 'S')
		}
	}
}

// drawLibertyLines emits liberty-to-stone segments (with inset to avoid
// stacking at shared vertices) and wrap-seam stubs. Mirrors
// DrawLibertyLines.
func (B *BoardDrawer) drawLibertyLines() {
	Win := B.Win
	Board := B.Node.Board
	GHist := Win.Coll.CurrentGameH
	CS := B.CellSize
	StrokeWidth := CS * GridLineThickness

	ColorForCount := func(N int) color.NRGBA {
		switch N {
		case 1:
			return Colors["1-Liberty Line"]
		case 2:
			return Colors["2-Liberty Line"]
		case 3:
			return Colors["3-Liberty Line"]
		case 4:
			return Colors["4-Liberty Line"]
		default:
			return Colors["≥5-Liberty Line"]
		}
	}

	DrawLineBetween := func(A, B2 Coord, Stroke color.NRGBA) {
		DX := float32(B2.X) - float32(A.X)
		DY := float32(B2.Y) - float32(A.Y)
		Inset := float32(GridLineThickness) / 2
		StartX := float32(A.X) + 0.5 + DX*Inset
		StartY := float32(A.Y) + 0.5 + DY*Inset
		EndX := float32(B2.X) + 0.5 - DX*Inset
		EndY := float32(B2.Y) + 0.5 - DY*Inset
		X1, Y1 := B.Point(StartX, StartY)
		X2, Y2 := B.Point(EndX, EndY)
		B.R.StrokeLine(X1, Y1, X2, Y2, StrokeWidth, Stroke)
	}
	DrawStub := func(C Coord, Direction byte, Stroke color.NRGBA) {
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
		X1, Y1 := B.Point(CenterX, CenterY)
		X2, Y2 := B.Point(EdgeX, EdgeY)
		B.R.StrokeLine(X1, Y1, X2, Y2, StrokeWidth, Stroke)
	}

	for Group := range Board.Groups {
		Stroke := ColorForCount(len(Group.Vertices[0]))
		for _, Liberty := range Group.Vertices[0] {
			West := Coord{X: Liberty.X - 1, Y: Liberty.Y}
			if GHist.InBounds(West) && Board.GetGroupAtCoord(West) == Group {
				DrawLineBetween(West, Liberty, Stroke)
			}
			East := Coord{X: Liberty.X + 1, Y: Liberty.Y}
			if GHist.InBounds(East) && Board.GetGroupAtCoord(East) == Group {
				DrawLineBetween(Liberty, East, Stroke)
			}
			North := Coord{X: Liberty.X, Y: Liberty.Y - 1}
			if GHist.InBounds(North) && Board.GetGroupAtCoord(North) == Group {
				DrawLineBetween(North, Liberty, Stroke)
			}
			South := Coord{X: Liberty.X, Y: Liberty.Y + 1}
			if GHist.InBounds(South) && Board.GetGroupAtCoord(South) == Group {
				DrawLineBetween(Liberty, South, Stroke)
			}
			if Liberty.X == 0 && GHist.WrapXMulY != 0 {
				WrappedY := Int16ToUInt8Modulo(
					(int16(Liberty.Y)-int16(GHist.WrapXShiftY))*GHist.WrapXMulYInv,
					GHist.Height)
				Wrapped := Coord{X: GHist.Width - 1, Y: WrappedY}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'W', Stroke)
					DrawStub(Wrapped, 'E', Stroke)
				}
			}
			if Liberty.X == GHist.Width-1 && GHist.WrapXMulY != 0 {
				WrappedY := Int16ToUInt8Modulo(
					int16(Liberty.Y)*int16(GHist.WrapXMulY)+int16(GHist.WrapXShiftY),
					GHist.Height)
				Wrapped := Coord{X: 0, Y: WrappedY}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'E', Stroke)
					DrawStub(Wrapped, 'W', Stroke)
				}
			}
			if Liberty.Y == 0 && GHist.WrapYMulX != 0 {
				WrappedX := Int16ToUInt8Modulo(
					(int16(Liberty.X)-int16(GHist.WrapYShiftX))*GHist.WrapYMulXInv,
					GHist.Width)
				Wrapped := Coord{X: WrappedX, Y: GHist.Height - 1}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'N', Stroke)
					DrawStub(Wrapped, 'S', Stroke)
				}
			}
			if Liberty.Y == GHist.Height-1 && GHist.WrapYMulX != 0 {
				WrappedX := Int16ToUInt8Modulo(
					int16(Liberty.X)*int16(GHist.WrapYMulX)+int16(GHist.WrapYShiftX),
					GHist.Width)
				Wrapped := Coord{X: WrappedX, Y: 0}
				if Board.GetGroupAtCoord(Wrapped) == Group {
					DrawStub(Liberty, 'S', Stroke)
					DrawStub(Wrapped, 'N', Stroke)
				}
			}
		}
	}
}

// drawIllegalDots emits a small filled circle on each empty cell where
// placing a stone would be illegal. Mirrors DrawEmptyIllegalMoves.
func (B *BoardDrawer) drawIllegalDots() {
	Win := B.Win
	Board := B.Node.Board
	CS := B.CellSize
	var CheckPlayer uint8
	var IsEditMode bool
	switch Win.MouseMode {
	case "Set Vertex":
		if Win.SetVertexPlayer == 0 {
			return
		}
		CheckPlayer = Win.SetVertexPlayer
		IsEditMode = true
	default:
		CheckPlayer = B.Node.GetNextStonePlacer()
		IsEditMode = false
	}
	R := CS * 0.51 / 2
	Fill := Colors["Illegal"]
	for X := range Win.Width() {
		for Y := range Win.Height() {
			C := Coord{X, Y}
			if Board.GetPlayerAt(C) != 0 {
				continue
			}
			if _, Err := Board.AttemptMove(C, CheckPlayer, IsEditMode, true); Err != nil {
				CX, CY := B.CellCenter(C)
				B.R.FillCircle(CX, CY, R, Fill)
			}
		}
	}
}

// drawLastMoveHighlight emits the last-move highlight ring.
func (B *BoardDrawer) drawLastMoveHighlight() {
	Node := B.Node
	LastMove := Node.LastMove
	if !B.Win.Coll.CurrentGameH.InBounds(LastMove) {
		return
	}
	CS := B.CellSize
	CX, CY := B.CellCenter(LastMove)
	B.R.StrokeCircle(CX, CY, CS*0.39/2, CS*0.39*0.319/2,
		Colors["Last Move"], PlayerColors[Node.PreviousStonePlacer].Base)
}

// drawChildNodeDots emits a small dot at each move-type child's LastMove.
// Fill = "Child Node" theme color, stroke = the base color of the child's
// PreviousStonePlacer. Edit, pass, and root children are skipped. Caller
// gates this on Theme.ShowChildNodeDots.
func (B *BoardDrawer) drawChildNodeDots() {
	H := B.Win.Coll.CurrentGameH
	CS := B.CellSize
	Fill := Colors["Child Node"]
	for _, Child := range B.Node.Children {
		LM := Child.LastMove
		if LM == EditCoords || LM == PassCoords || LM == RootCoords {
			continue
		}
		if !H.InBounds(LM) {
			continue
		}
		CX, CY := B.CellCenter(LM)
		B.R.StrokeCircle(CX, CY, CS*0.2/2, CS*0.2*0.319/2,
			Fill, PlayerColors[Child.PreviousStonePlacer].Base)
	}
}

// drawAnnotations emits the Circle/Square/Triangle/X-Mark shapes. Mask
// bits come from Board.Vertices. Shapes are drawn in display-space
// orientation — cell centers rotate with SymmetryIndex, but the glyphs
// themselves stay upright, matching the on-screen behavior.
func (B *BoardDrawer) drawAnnotations() {
	Win := B.Win
	Board := B.Node.Board
	CS := B.CellSize
	AnnotationColor := Colors["Annotation"]
	StrokeWidth := CS * 0.05
	Transparent := color.NRGBA{}

	Order := []uint8{CircleMask, SquareMask, TriangleMask, XMask}
	for Y := range Win.Height() {
		for X := range Win.Width() {
			C := Coord{X, Y}
			V := Board.Vertices[C]
			CX, CY := B.CellCenter(C)
			for _, Mask := range Order {
				if V&Mask == 0 {
					continue
				}
				switch Mask {
				case CircleMask:
					B.R.StrokeCircle(CX, CY, CS*0.3, StrokeWidth, Transparent, AnnotationColor)
				case SquareMask:
					Half := CS * 0.3
					B.R.StrokeRect(CX-Half, CY-Half, CS*0.6, CS*0.6, StrokeWidth,
						Transparent, AnnotationColor)
				case TriangleMask:
					TSize := CS * 0.39
					TXOffset := TSize * float32(math.Sin(math.Pi/3))
					TYOffset := TSize * float32(math.Cos(math.Pi/3))
					B.R.StrokePolygon([]float32{
						CX, CY - TSize,
						CX - TXOffset, CY + TYOffset,
						CX + TXOffset, CY + TYOffset,
					}, StrokeWidth, AnnotationColor)
				case XMask:
					Half := CS * 0.3
					B.R.StrokeLine(CX-Half, CY-Half, CX+Half, CY+Half, StrokeWidth, AnnotationColor)
					B.R.StrokeLine(CX+Half, CY-Half, CX-Half, CY+Half, StrokeWidth, AnnotationColor)
				}
			}
		}
	}
}

// drawLabels emits text labels from GameTreeNode.Labels.
func (B *BoardDrawer) drawLabels() {
	CS := B.CellSize
	Fill := Colors["Annotation"]
	for C, Text := range B.Node.Labels {
		CX, CY := B.CellCenter(C)
		B.R.DrawText(CX, CY, CS*0.39, Text, Fill)
	}
}

// drawTerritory emits score-mode territory markers.
func (B *BoardDrawer) drawTerritory() {
	Node := B.Node
	GHist := B.Win.Coll.CurrentGameH
	CS := B.CellSize
	Size := CS * 0.54
	StrokeWidth := CS * 0.093
	Stroke := Colors["Territory Stroke"]
	for C, Owner := range Node.TerritoryMap {
		if !(0 < Owner && Owner <= GHist.Players) {
			continue
		}
		CX, CY := B.CellCenter(C)
		Fill := PlayerColors[Owner].Territory
		B.R.StrokeRect(CX-Size/2, CY-Size/2, Size, Size, StrokeWidth, Fill, Stroke)
	}
}

// ShowSvgSaveDialog opens a file-save dialog and writes the SVG
// produced by BuildBoardSvg (static) or BuildAnimatedBoardSvg (when
// SecondsPerNode > 0) on confirm. The animated path rendering runs on
// a goroutine with a modal progress dialog; BoardDrawer is not
// goroutine-safe against concurrent window updates, but generation
// typically completes in well under a second so live locking is not
// warranted. Does not touch GoWin.OpenedFilePath.
func (GoWin *GoWin) ShowSvgSaveDialog(CellSize uint16, SecondsPerNode float64) {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	Animated := SecondsPerNode > 0
	ProposedExt := ".svg"
	if Animated {
		ProposedExt = ".animated.svg"
	}

	FileDialog := dialog.NewFileSave(func(Writer fyne.URIWriteCloser, Err error) {
		if Err != nil || Writer == nil {
			GoWin.DialogClosed()
			return
		}
		if !Animated {
			GoWin.DialogClosed()
			defer Writer.Close()
			Svg := GoWin.BuildBoardSvg(CellSize)
			if _, WriteErr := Writer.Write([]byte(Svg)); WriteErr != nil {
				GoWin.ShowError(WriteErr)
			}
			return
		}

		GoWin.DialogClosed()
		GoWin.RunAnimatedSvgExport(Writer, CellSize, SecondsPerNode)
	}, GoWin.Win)

	FileDialog.SetFileName(GoWin.ProposeImageFilename(ProposedExt, !Animated))
	FileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".svg"}))
	GoWin.ResizeDialog = func() { FileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	FileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	FileDialog.Show()
}

// RunAnimatedSvgExport renders BuildAnimatedBoardSvg on a goroutine
// while showing a modal progress dialog, then writes the bytes to
// Writer on the Fyne goroutine. Closes Writer when finished.
func (GoWin *GoWin) RunAnimatedSvgExport(Writer fyne.URIWriteCloser, CellSize uint16, SecondsPerNode float64) {
	GoWin.DialogShowing = true

	Bar := widget.NewProgressBar()
	Bar.Min = 0
	Bar.Max = 1
	Bar.SetValue(0)
	StatusLabel := widget.NewLabel("Rendering frame 0 / ?")
	Content := container.NewVBox(StatusLabel, Bar)
	ProgressDlg := dialog.NewCustomWithoutButtons("Exporting animated SVG", Content, GoWin.Win)
	ProgressDlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	GoWin.ResizeDialog = func() { ProgressDlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	ProgressDlg.Show()

	go func() {
		Progress := func(Done, Total int) {
			fyne.Do(func() {
				if Total > 0 {
					Bar.SetValue(float64(Done) / float64(Total))
				}
				StatusLabel.SetText(fmt.Sprintf("Rendering frame %d / %d", Done, Total))
			})
		}
		Svg := GoWin.BuildAnimatedBoardSvg(CellSize, SecondsPerNode, Progress)
		fyne.Do(func() {
			defer Writer.Close()
			ProgressDlg.Hide()
			GoWin.DialogClosed()
			if Svg == "" {
				GoWin.ShowError(fmt.Errorf("no active game to export"))
				return
			}
			if _, WriteErr := Writer.Write([]byte(Svg)); WriteErr != nil {
				GoWin.ShowError(WriteErr)
			}
		})
	}()
}

// ProposeImageFilename strips any recognized game-file suffix from
// GoWin.OpenedFilePath and returns a proposed export filename. Falls
// back to "Board" when no file is open. Seconds come from
// CurrentNode.UnixMilli/1000 if non-zero, else time.Now().Unix().
//
// When IncludeMoveNumber is true (static single-frame export) the
// filename is "<base>_move<N>_<seconds><Ext>" with N counted as the
// depth of CurrentNode below the game's RootNode (so the root node is
// move 0). When false (animated export — the file shows every move in
// the game) the move number is omitted: "<base>_<seconds><Ext>".
func (GoWin *GoWin) ProposeImageFilename(Ext string, IncludeMoveNumber bool) string {
	Base := "Board"
	if GoWin.OpenedFilePath != "" {
		Name := filepath.Base(GoWin.OpenedFilePath)
		Lower := strings.ToLower(Name)
		Trimmed := false
		for _, Suffix := range CggAllSuffixes {
			LowerSuffix := strings.ToLower(Suffix)
			if strings.HasSuffix(Lower, LowerSuffix) {
				Name = Name[:len(Name)-len(Suffix)]
				Trimmed = true
				break
			}
		}
		if !Trimmed && strings.EqualFold(filepath.Ext(Name), ".Sgf") {
			Name = Name[:len(Name)-len(filepath.Ext(Name))]
		}
		if Name != "" {
			Base = Name
		}
	}

	Seconds := int64(0)
	if Current := GoWin.CurrentNode(); Current != nil && Current.UnixMilli != 0 {
		Seconds = Current.UnixMilli / 1000
	}
	if Seconds == 0 {
		Seconds = time.Now().Unix()
	}

	if !IncludeMoveNumber {
		return fmt.Sprintf("%s_%d%s", Base, Seconds, Ext)
	}

	MoveNumber := 0
	if Hist := GoWin.Coll.CurrentGameH; Hist != nil {
		Node := GoWin.CurrentNode()
		for Node != nil && Node != Hist.RootNode {
			Node = Node.Parent
			MoveNumber++
		}
	}

	return fmt.Sprintf("%s_move%d_%d%s", Base, MoveNumber, Seconds, Ext)
}
