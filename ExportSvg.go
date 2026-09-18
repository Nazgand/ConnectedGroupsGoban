package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// SvgRenderer implements ImageRenderer by emitting exact-primitive SVG
// into a strings.Builder. All drawing goes through BoardDrawer which
// handles symmetry-aware coordinate math; this type is only concerned
// with the primitive-to-SVG mapping.
//
// The "square minus four circles" 3-meet shape is emitted as a
// <use href="#SquareMinusFourCircles"> reference to a shared <symbol>
// defined in the <defs> block, mirroring SquareMinusFourCircles#039fef.svg.
// Font-family is "Noto Sans, sans-serif" to match Fyne's bundled default
// font. Text is vertically centered via an explicit y-offset
// (y = cy + 0.35·FontSize) rather than `dominant-baseline`, because
// QtSvg-based viewers (Gwenview, Qt apps) do not implement that attribute.
type SvgRenderer struct {
	Builder *strings.Builder
}

func (S *SvgRenderer) FillRect(X, Y, W, H float32, Fill color.NRGBA) {
	fmt.Fprintf(S.Builder,
		`  <rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n",
		SvgFloat(X), SvgFloat(Y), SvgFloat(W), SvgFloat(H), SvgColor(Fill))
}

func (S *SvgRenderer) FillCircle(CX, CY, R float32, Fill color.NRGBA) {
	fmt.Fprintf(S.Builder,
		`  <circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n",
		SvgFloat(CX), SvgFloat(CY), SvgFloat(R), SvgColor(Fill))
}

func (S *SvgRenderer) StrokeCircle(CX, CY, R, StrokeWidth float32, Fill, Stroke color.NRGBA) {
	fmt.Fprintf(S.Builder,
		`  <circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="%s"/>`+"\n",
		SvgFloat(CX), SvgFloat(CY), SvgFloat(R), SvgColor(Fill), SvgColor(Stroke), SvgFloat(StrokeWidth))
}

func (S *SvgRenderer) StrokeRect(X, Y, W, H, StrokeWidth float32, Fill, Stroke color.NRGBA) {
	fmt.Fprintf(S.Builder,
		`  <rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="%s"/>`+"\n",
		SvgFloat(X), SvgFloat(Y), SvgFloat(W), SvgFloat(H), SvgColor(Fill), SvgColor(Stroke), SvgFloat(StrokeWidth))
}

func (S *SvgRenderer) StrokeLine(X1, Y1, X2, Y2, StrokeWidth float32, Stroke color.NRGBA) {
	fmt.Fprintf(S.Builder,
		`  <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="%s"/>`+"\n",
		SvgFloat(X1), SvgFloat(Y1), SvgFloat(X2), SvgFloat(Y2), SvgColor(Stroke), SvgFloat(StrokeWidth))
}

func (S *SvgRenderer) StrokePolygon(Points []float32, StrokeWidth float32, Stroke color.NRGBA) {
	var Pts strings.Builder
	for I := 0; I+1 < len(Points); I += 2 {
		if I > 0 {
			Pts.WriteByte(' ')
		}
		Pts.WriteString(SvgFloat(Points[I]))
		Pts.WriteByte(',')
		Pts.WriteString(SvgFloat(Points[I+1]))
	}
	fmt.Fprintf(S.Builder,
		`  <polygon points="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="miter"/>`+"\n",
		Pts.String(), SvgColor(Stroke), SvgFloat(StrokeWidth))
}

func (S *SvgRenderer) DrawText(CX, CY, FontSize float32, Content string, Fill color.NRGBA) {
	if Content == "" {
		return
	}
	fmt.Fprintf(S.Builder,
		`  <text x="%s" y="%s" font-size="%s" font-family="Noto Sans, sans-serif" font-weight="bold" text-anchor="middle" fill="%s">%s</text>`+"\n",
		SvgFloat(CX), SvgFloat(CY+FontSize*0.35), SvgFloat(FontSize), SvgColor(Fill), SvgEscapeText(Content))
}

func (S *SvgRenderer) Draw3MeetShape(X, Y, Size float32, Fill color.NRGBA) {
	fmt.Fprintf(S.Builder,
		`  <use href="#SquareMinusFourCircles" x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n",
		SvgFloat(X), SvgFloat(Y), SvgFloat(Size), SvgFloat(Size), SvgColor(Fill))
}

// SvgColor renders an NRGBA as "#rrggbb" (opaque) or "#rrggbbaa".
func SvgColor(C color.NRGBA) string {
	if C.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", C.R, C.G, C.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", C.R, C.G, C.B, C.A)
}

// SvgEscapeText escapes the five XML-reserved characters for safe use
// in text content and attribute values.
func SvgEscapeText(S string) string {
	S = strings.ReplaceAll(S, "&", "&amp;")
	S = strings.ReplaceAll(S, "<", "&lt;")
	S = strings.ReplaceAll(S, ">", "&gt;")
	S = strings.ReplaceAll(S, "\"", "&quot;")
	S = strings.ReplaceAll(S, "'", "&apos;")
	return S
}

// SvgFloat formats a float32 with minimal trailing zeros.
func SvgFloat(V float32) string {
	return strconv.FormatFloat(float64(V), 'f', -1, 32)
}

// BuildBoardSvg generates the full SVG document for the current position
// at the given CellSize. Output dimensions are exactly
// CellSize * (DisplayWidth+2) × CellSize * (DisplayHeight+2) pixels.
func (GoWin *GoWin) BuildBoardSvg(CellSize uint16) string {
	CS := float32(CellSize)

	var Builder strings.Builder
	Renderer := &SvgRenderer{Builder: &Builder}
	Drawer := NewBoardDrawer(GoWin, CS, Renderer)
	TotalW, TotalH := Drawer.TotalSize()

	Builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	Builder.WriteByte('\n')
	fmt.Fprintf(&Builder,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		SvgFloat(TotalW), SvgFloat(TotalH), SvgFloat(TotalW), SvgFloat(TotalH))
	Builder.WriteByte('\n')
	Builder.WriteString(SvgDefsBlock)

	Drawer.DrawAll()

	Builder.WriteString("</svg>\n")
	return Builder.String()
}

// BuildAnimatedBoardSvg generates an animated SVG walking the favorite-
// child path from the current game's root to its last descendant. Each
// node is displayed for SecondsPerNode (must be > 0). The animation
// plays once and holds on the final frame indefinitely.
//
// Each frame is a fully self-contained, opaque snapshot — each
// <g id="FrameK"> emits the coord background, the goban layer, the
// node layers, and the move-counter overlay. Frame 0 starts visible;
// frame K≥1 starts with visibility="hidden" and is switched on by a
// SMIL <set> at begin="(K*SPN)s" with fill="freeze". To keep paint
// cost constant regardless of frame index, each frame K (except the
// last) is also switched off at begin="((K+1)*SPN)s" by a second
// <set> — otherwise every earlier, covered-up frame would still sit
// in the render tree and be repainted every animation tick, and the
// per-tick work would grow linearly with K, making later frames
// noticeably slow to appear. The hide event is emitted *after* the
// show event inside the XML so simultaneous events at the boundary
// are processed show-before-hide by document order; combined with
// the stack-and-cover opacity invariant (frame K+1 fully covers K)
// this leaves no visual gap at transitions.
//
// Progress (may be nil) is invoked once with (0, N) before the loop and
// then once after each frame with (K+1, N) where N = len(Path). Callers
// typically schedule this function on a goroutine and marshal progress
// updates via fyne.Do.
//
// Returns "" if the collection has no active game. The first and last
// nodes always have Board != nil (the root is materialized by
// NewRootNode; later nodes are materialized by MakeChild or on load).
func (GoWin *GoWin) BuildAnimatedBoardSvg(CellSize uint16, SecondsPerNode float64, Progress func(Done, Total int)) string {
	GHist := GoWin.Coll.CurrentGameH
	if GHist == nil || GHist.RootNode == nil {
		return ""
	}
	if SecondsPerNode <= 0 {
		return ""
	}
	Nodes := FavoriteChildPath(GHist.RootNode)
	N := len(Nodes)
	CS := float32(CellSize)

	var Builder strings.Builder
	Renderer := &SvgRenderer{Builder: &Builder}
	SizeDrawer := NewBoardDrawer(GoWin, CS, Renderer)
	SizeDrawer.Node = Nodes[0]
	TotalW, TotalH := SizeDrawer.TotalSize()

	Builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	Builder.WriteByte('\n')
	fmt.Fprintf(&Builder,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		SvgFloat(TotalW), SvgFloat(TotalH), SvgFloat(TotalW), SvgFloat(TotalH))
	Builder.WriteByte('\n')
	Builder.WriteString(SvgDefsBlock)

	BeginSecondsStr := func(BeginSeconds float64) string {
		return strconv.FormatFloat(BeginSeconds, 'f', -1, 64)
	}

	if Progress != nil {
		Progress(0, N)
	}
	for K, Node := range Nodes {
		if K == 0 {
			fmt.Fprintf(&Builder, `  <g id="Frame%d">`+"\n", K)
		} else {
			fmt.Fprintf(&Builder, `  <g id="Frame%d" visibility="hidden">`+"\n", K)
			fmt.Fprintf(&Builder,
				`    <set attributeName="visibility" to="visible" begin="%ss" fill="freeze"/>`+"\n",
				BeginSecondsStr(float64(K)*SecondsPerNode))
		}
		if K < N-1 {
			fmt.Fprintf(&Builder,
				`    <set attributeName="visibility" to="hidden" begin="%ss" fill="freeze"/>`+"\n",
				BeginSecondsStr(float64(K+1)*SecondsPerNode))
		}
		FrameDrawer := NewBoardDrawer(GoWin, CS, Renderer)
		FrameDrawer.Node = Node
		FrameDrawer.DrawAll()
		FrameDrawer.DrawMoveCounterOverlay(K, N-1)
		Builder.WriteString("  </g>\n")
		if Progress != nil {
			Progress(K+1, N)
		}
	}

	Builder.WriteString("</svg>\n")
	return Builder.String()
}
