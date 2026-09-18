package main

import (
	"fmt"
	"image/color"
	"maps"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ScoreColLayout stacks children vertically with ScoreCellPad gap between them.
type ScoreColLayout struct{}

func (ScoreColLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objects {
		ms := o.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		h += ms.Height
		if i > 0 {
			h += ScoreCellPad
		}
	}
	return fyne.NewSize(w, h)
}

func (ScoreColLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for i, o := range objects {
		if i > 0 {
			y += ScoreCellPad
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h
	}
}

// ScoreTableLayout places column containers and dividers side by side.
// objects: [col0, div0, col1, div1, ..., colN]
type ScoreTableLayout struct{}

func (ScoreTableLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objects {
		ms := o.MinSize()
		if i%2 == 0 { // column
			w += ms.Width
			if ms.Height > h {
				h = ms.Height
			}
		} else { // divider
			w += ScoreDividerWidth
		}
	}
	return fyne.NewSize(w, h)
}

func (ScoreTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, o := range objects {
		if i%2 == 1 { // divider
			o.Move(fyne.NewPos(x, 0))
			o.Resize(fyne.NewSize(ScoreDividerWidth, size.Height))
			x += ScoreDividerWidth
		} else { // column
			w := o.MinSize().Width
			o.Move(fyne.NewPos(x, 0))
			o.Resize(fyne.NewSize(w, size.Height))
			x += w
		}
	}
}

// ScoreCell is a widget for score table cells that supports hover events.
// Used for game state rows (1-7) to enable tooltips showing multiplier values.
// Only Live Stones, Territory, and On-board Prisoners rows also highlight
// relevant stones on hover. RuleSet rows (8+) do not use ScoreCell.
type ScoreCell struct {
	widget.BaseWidget
	Text        string
	TextColor   color.NRGBA
	OnHoverIn   func()
	OnHoverOut  func()
	Row         int   // Table row index
	Column      int   // Table column index
	Player      uint8 // Player number (for player columns)
	TooltipText string
	Win         *GoWin
}

// ScoreCellRenderer renders a ScoreCell as a simple text object.
type ScoreCellRenderer struct {
	Cell     *ScoreCell
	Text     *canvas.Text
	Objects_ []fyne.CanvasObject
}

// CreateRenderer creates the renderer for ScoreCell.
func (Cell *ScoreCell) CreateRenderer() fyne.WidgetRenderer {
	Text := canvas.NewText(Cell.Text, Cell.TextColor)
	Text.Alignment = fyne.TextAlignCenter
	Text.TextSize = ScoreTextSize
	return &ScoreCellRenderer{
		Cell:     Cell,
		Text:     Text,
		Objects_: []fyne.CanvasObject{Text},
	}
}

// MinSize returns the minimum size for the cell.
func (Rend *ScoreCellRenderer) MinSize() fyne.Size {
	return Rend.Text.MinSize()
}

// Layout positions the text within the given size.
func (Rend *ScoreCellRenderer) Layout(Size fyne.Size) {
	Rend.Text.Resize(Size)
	Rend.Text.Move(fyne.NewPos(0, 0))
}

// Refresh updates the renderer from the cell state.
func (Rend *ScoreCellRenderer) Refresh() {
	Rend.Text.Text = Rend.Cell.Text
	Rend.Text.Color = Rend.Cell.TextColor
	Rend.Text.Refresh()
}

// Objects returns the canvas objects managed by this renderer.
func (Rend *ScoreCellRenderer) Objects() []fyne.CanvasObject { return Rend.Objects_ }

// Destroy is a no-op.
func (Rend *ScoreCellRenderer) Destroy() {}

// MouseIn is called when the mouse enters the cell.
func (Cell *ScoreCell) MouseIn(_ *desktop.MouseEvent) {
	if Cell.OnHoverIn != nil {
		Cell.OnHoverIn()
	}
}

// MouseMoved is called when the mouse moves within the cell (no-op).
func (Cell *ScoreCell) MouseMoved(_ *desktop.MouseEvent) {}

// MouseOut is called when the mouse leaves the cell.
func (Cell *ScoreCell) MouseOut() {
	if Cell.OnHoverOut != nil {
		Cell.OnHoverOut()
	}
}

// FormatPoints formats a float64 point value (komi, score, etc.) with 9 digits
// of precision, then strips all trailing zeros and any trailing decimal point.
func FormatPoints(V float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.9f", V), "0"), ".")
}

// UpdateCommentTextbox synchronizes the comment entry widget with the current
// node's Comment field. Clears the textbox if the node has no comment.
func (GoWin *GoWin) UpdateCommentTextbox() {
	fyne.Do(func() {
		if GoWin.CurrentNode() != nil && GoWin.CurrentNode().Comment != "" {
			GoWin.CommentEntry.SetText(GoWin.CurrentNode().Comment)
		} else {
			GoWin.CommentEntry.SetText("") // Clears the textbox if there's no comment
		}
	})
}

// initializeTerritoryMap initializes the TerritoryMap from the current board's
// Vertices (stones only, no annotations) if nil, then flood-fills empty regions
// to assign territory ownership.
func (GoWin *GoWin) InitializeTerritoryMap() {
	if GoWin.CurrentNode().TerritoryMap == nil {
		GoWin.CurrentNode().TerritoryMap = map[Coord]uint8{}
		for Coord := range GoWin.CurrentNode().Board.Vertices {
			GoWin.CurrentNode().TerritoryMap[Coord] = GoWin.CurrentNode().Board.GetPlayerAt(Coord)
		}
	}
	GoWin.AssignTerritoryToEmptyRegions()
}

// assignTerritoryToEmptyRegions flood-fills every contiguous empty region.
// If an empty region is adjacent to stones of exactly one player, the region
// is assigned to that player. Mixed-adjacent regions remain unowned.
// Dead stones (marked as 0 in TerritoryMap) are treated as empty.
// Afterwards, score is recalculated and displayed.
func (GoWin *GoWin) AssignTerritoryToEmptyRegions() {
	Visited := map[Coord]struct{}{}

	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			_, VisitedCoord := Visited[C]
			if !VisitedCoord {
				TerritoryOwner := GoWin.CurrentNode().TerritoryMap[C]
				if 0 < TerritoryOwner && TerritoryOwner <= GoWin.Coll.CurrentGameH.Players {
					// Stone is alive, skip
				} else {
					// Empty or dead stone, start flood fill
					Stack := []Coord{C}
					RegionAdjacentStones := map[uint8]struct{}{}
					Region := []Coord{}

					for len(Stack) > 0 {
						// Get last Coords from stack
						StackLastC := Stack[len(Stack)-1]
						// Remove last Coords from stack
						Stack = Stack[:len(Stack)-1]

						_, visitedStackLastC := Visited[StackLastC]
						if visitedStackLastC {
							continue
						}
						Visited[StackLastC] = struct{}{}
						Region = append(Region, StackLastC)

						Neighbors := GoWin.Coll.CurrentGameH.Neighbors(StackLastC)
						for _, Neighbor := range Neighbors {
							NeighborOwner := GoWin.CurrentNode().TerritoryMap[Neighbor]
							if 0 < NeighborOwner && NeighborOwner <= GoWin.Coll.CurrentGameH.Players {
								RegionAdjacentStones[NeighborOwner] = struct{}{}
							} else {
								_, VisitedNeighbor := Visited[Neighbor]
								if !VisitedNeighbor {
									Stack = append(Stack, Neighbor)
								}
							}
						}
					}

					// Determine owner
					if len(RegionAdjacentStones) == 1 {
						var Owner uint8
						for k := range RegionAdjacentStones {
							Owner = k
						}
						// Assign territory
						for _, Coord := range Region {
							GoWin.CurrentNode().TerritoryMap[Coord] = Owner
						}
					}
				}
			}
		}
	}
	GoWin.CalculateAndDisplayScore()
}

// FindRuleSetByName searches ProtectedRuleSets for an entry whose Names
// slice contains the given string. Returns a pointer to the matching
// element, or nil if not found.
func FindRuleSetByName(Name string) *RuleSet {
	for Index := range ProtectedRuleSets {
		if slices.Contains(ProtectedRuleSets[Index].Names, Name) {
			return &ProtectedRuleSets[Index]
		}
	}
	return nil
}

// RegisterRuleSet ensures RS has an entry in RuleSetId, assigning the next
// available ID if it is new. Safe to call multiple times for the same pointer.
func RegisterRuleSet(RuleSet *RuleSet) {
	if RuleSet == nil {
		return
	}
	if _, Found := RuleSetId[RuleSet]; !Found {
		RuleSetId[RuleSet] = RuleSetNextId
		RuleSetNextId++
	}
}

// RuleSetDisplayName returns the display name for RS given the full set of
// rule sets being shown. If any other entry in Shown shares Names[0] with RS,
// the name is suffixed with " #<id>" to disambiguate.
func RuleSetDisplayName(RuleSet *RuleSet, Shown []*RuleSet) string {
	Name := RuleSet.Names[0]
	for _, Other := range Shown {
		if !Other.Equal(RuleSet) && Other.Names[0] == Name {
			return fmt.Sprintf("%s #%d", Name, RuleSetId[RuleSet])
		}
	}
	return Name
}

// players returns a slice of player numbers [1..Players] for the current
// game, defaulting to 2 if Players is zero.
func (GoWin *GoWin) Players() []uint8 {
	NumPlayers := GoWin.Coll.CurrentGameH.Players
	if NumPlayers == 0 {
		NumPlayers = 2
	}
	Result := make([]uint8, NumPlayers)
	for Index := range Result {
		Result[Index] = uint8(Index + 1)
	}
	return Result
}

// calculateScoreDetails tallies live stones, territory, prisoners, passes,
// stone suicides, and komi from the TerritoryMap and board state, then
// computes the total score for each player under every shown rule set.
func (GoWin *GoWin) CalculateScoreDetails() ScoreDetails {
	Players := GoWin.Players()
	LiveStones := map[uint8]int{}
	Territory := map[uint8]int{}
	OnBoardPrisoners := map[uint8]int{}

	Board := GoWin.CurrentNode().Board
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			TerritoryOwner := GoWin.CurrentNode().TerritoryMap[C]
			if TerritoryOwner != 0 {
				Territory[TerritoryOwner]++
				switch Board.GetPlayerAt(C) {
				case 0:
				case TerritoryOwner:
					LiveStones[TerritoryOwner]++
				default:
					OnBoardPrisoners[TerritoryOwner]++
				}
			}
		}
	}

	Hist := Board.Hist
	ActiveRuleSet := Hist.RuleSet

	ShownRuleSets := slices.DeleteFunc(CurrentAppConfig.ShownRuleSets(), func(RS *RuleSet) bool {
		return RS.CaptureConverts != ActiveRuleSet.CaptureConverts
	})
	if !slices.ContainsFunc(ShownRuleSets, func(RS *RuleSet) bool { return RS.Equal(ActiveRuleSet) }) {
		ShownRuleSets = append(ShownRuleSets, ActiveRuleSet)
	}

	Scores := map[*RuleSet]map[uint8]float64{}
	for _, RuleSet := range ShownRuleSets {
		Scores[RuleSet] = map[uint8]float64{}
		for _, Player := range Players {
			Score := float64(Territory[Player]) +
				RuleSet.LiveStones*float64(LiveStones[Player]) +
				RuleSet.Prisoners*float64(Board.Prisoners[Player]+OnBoardPrisoners[Player]) +
				RuleSet.Passes*float64(Board.Passes[Player]) +
				RuleSet.StoneSuicides*float64(Board.StoneSuicides[Player]) +
				RuleSet.Conversions*float64(Board.ConversionsTo[Player]-Board.ConversionsFrom[Player])
			// Add per-player komi
			if PlayerKomi, Exists := Hist.Komi[Player]; Exists {
				Score += PlayerKomi
			}
			Scores[RuleSet][Player] = Score
		}
	}

	return ScoreDetails{
		LiveStones:        LiveStones,
		Territory:         Territory,
		OffBoardPrisoners: maps.Clone(Board.Prisoners),
		OnBoardPrisoners:  OnBoardPrisoners,
		Passes:            maps.Clone(Board.Passes),
		StoneSuicides:     maps.Clone(Board.StoneSuicides),
		ConversionsTo:     maps.Clone(Board.ConversionsTo),
		ConversionsFrom:   maps.Clone(Board.ConversionsFrom),
		Komi:              Hist.Komi,
		Score:             Scores,
		ActiveRuleSet:     ActiveRuleSet,
		ShownRuleSets:     ShownRuleSets,
	}
}

// GetLiveStoneCoords returns the coordinates of live stones for the given player.
// A stone is live if it's on the board and the territory map shows it belongs to that player.
func (GoWin *GoWin) GetLiveStoneCoords(Player uint8) []Coord {
	var Coords []Coord
	Board := GoWin.CurrentNode().Board
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			TerritoryOwner := GoWin.CurrentNode().TerritoryMap[C]
			if TerritoryOwner == Player && Board.GetPlayerAt(C) == Player {
				Coords = append(Coords, C)
			}
		}
	}
	return Coords
}

// GetTerritoryCoords returns the coordinates of territory owned by the given player.
// This includes both empty intersections and live stones in that territory.
func (GoWin *GoWin) GetTerritoryCoords(Player uint8) []Coord {
	var Coords []Coord
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			if GoWin.CurrentNode().TerritoryMap[C] == Player {
				Coords = append(Coords, C)
			}
		}
	}
	return Coords
}

// GetOnBoardPrisonerCoords returns the coordinates of on-board prisoners (dead stones)
// for the given player. These are stones that belong to a different player than the
// territory owner.
func (GoWin *GoWin) GetOnBoardPrisonerCoords(Player uint8) []Coord {
	var Coords []Coord
	Board := GoWin.CurrentNode().Board
	for Y := uint8(0); Y < GoWin.Height(); Y++ {
		for X := uint8(0); X < GoWin.Width(); X++ {
			C := Coord{X, Y}
			TerritoryOwner := GoWin.CurrentNode().TerritoryMap[C]
			if TerritoryOwner == Player && Board.GetPlayerAt(C) != 0 && Board.GetPlayerAt(C) != Player {
				Coords = append(Coords, C)
			}
		}
	}
	return Coords
}

// DrawScoreTooltip draws a multi-line tooltip overlay on the Hover layer.
// Each line in Lines is rendered as a separate canvas.Text object stacked
// vertically. The tooltip is positioned at the top-left corner of the
// score container.
func (GoWin *GoWin) DrawScoreTooltip(Lines []string) {
	HoverLayer := GoWin.Layers["Hover"]
	GoWin.ClearScoreTooltip()

	TooltipPos := GoWin.ScoreContainer.Position()
	TextColor := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	const FontSize = float32(12)
	const PadX = float32(5)
	const PadY = float32(4)
	const LineSpacing = float32(2)

	// Measure all lines to find max width and total height.
	TextObjects := []*canvas.Text{}
	MaxWidth := float32(0)
	TotalHeight := float32(0)
	for _, Line := range Lines {
		TextObj := canvas.NewText(Line, TextColor)
		TextObj.TextSize = FontSize
		TextObj.Alignment = fyne.TextAlignLeading
		TextObj.Resize(TextObj.MinSize())
		TextObjects = append(TextObjects, TextObj)
		if TextObj.MinSize().Width > MaxWidth {
			MaxWidth = TextObj.MinSize().Width
		}
		if TotalHeight > 0 {
			TotalHeight += LineSpacing
		}
		TotalHeight += TextObj.MinSize().Height
	}

	// Background rectangle
	TooltipBg := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x93})
	TooltipBg.Resize(fyne.NewSize(MaxWidth+PadX*2, TotalHeight+PadY*2))
	TooltipBg.Move(TooltipPos)
	HoverLayer.Add(TooltipBg)
	GoWin.TooltipObjects = append(GoWin.TooltipObjects, TooltipBg)

	// Position each text line
	OffsetY := PadY
	for _, TextObj := range TextObjects {
		TextObj.Move(fyne.Position{
			X: TooltipPos.X + PadX,
			Y: TooltipPos.Y + OffsetY,
		})
		HoverLayer.Add(TextObj)
		GoWin.TooltipObjects = append(GoWin.TooltipObjects, TextObj)
		OffsetY += TextObj.MinSize().Height + LineSpacing
	}

	HoverLayer.Refresh()
}

// ClearScoreTooltip removes all tooltip objects from the Hover layer.
func (GoWin *GoWin) ClearScoreTooltip() {
	HoverLayer := GoWin.Layers["Hover"]
	for _, Obj := range GoWin.TooltipObjects {
		HoverLayer.Remove(Obj)
	}
	GoWin.TooltipObjects = nil
	HoverLayer.Refresh()
}

// MultiplierColor returns Green for positive, Red for negative, Yellow for zero.
func MultiplierColor(V float64) color.NRGBA {
	switch {
	case V < 0:
		return ScoreColorRed
	case V > 0:
		return ScoreColorGreen
	default:
		return ScoreColorYellow
	}
}

// ScoreRowInfo holds the data for one row of the score table.
type ScoreRowInfo struct {
	Label          string
	PlayerValues   []string // one per player
	Color          color.NRGBA
	Multiplier     float64
	MultiplierName string
	TooltipSuffix  string
	TooltipFunc    func(Player uint8) []string // custom multi-line tooltip; overrides default
	HighlightFunc  func(Player uint8) []Coord  // nil = no board highlights
	IsStatRow      bool                        // true for game-state rows with hover tooltips
}

// calculateAndDisplayScore computes scoring details and refreshes the
// score table widget.
func (GoWin *GoWin) CalculateAndDisplayScore() {
	d := GoWin.CalculateScoreDetails()
	GoWin.UpdateScoreTable(d)
}

// UpdateScoreTable rebuilds the score table grid inside ScoreContainer
// from the given ScoreDetails, coloring each row according to the active
// rule set's multipliers. Prisoner rows are hidden for CaptureConverts
// rule sets; the Conversions row is hidden for standard rule sets.
func (GoWin *GoWin) UpdateScoreTable(ScoreDetails ScoreDetails) {
	Players := GoWin.Players()
	ActiveRuleSet := ScoreDetails.ActiveRuleSet
	IsCaptureConverts := ActiveRuleSet != nil && ActiveRuleSet.CaptureConverts

	// Store ScoreDetails for hover handlers
	GoWin.CurrentScoreDetails = ScoreDetails

	NumCols := 1 + len(Players)
	IntStr := func(N int) string { return fmt.Sprintf("%d", N) }

	PlayerIntValues := func(Data map[uint8]int) []string {
		Values := make([]string, len(Players))
		for Index, Player := range Players {
			Values[Index] = IntStr(Data[Player])
		}
		return Values
	}

	// Build rows dynamically, skipping irrelevant rows
	Rows := []ScoreRowInfo{}

	// Header row
	HeaderValues := make([]string, len(Players))
	for Index, Player := range Players {
		HeaderValues[Index] = fmt.Sprintf("%d", Player)
	}
	Rows = append(Rows, ScoreRowInfo{
		Label: "Player", PlayerValues: HeaderValues,
		Color: ScoreColorLightBlue,
	})

	// No Result (Draw) row, shown when this node triggered a NonRepetitionRuleNoResult draw.
	if GoWin.CurrentNode().NoResultDraw {
		NoResultValues := make([]string, len(Players))
		for Index := range Players {
			NoResultValues[Index] = "Draw"
		}
		Rows = append(Rows, ScoreRowInfo{
			Label: "No Result", PlayerValues: NoResultValues,
			Color: ScoreColorYellow,
		})
	}

	// Live Stones
	Rows = append(Rows, ScoreRowInfo{
		Label: "Live Stones", PlayerValues: PlayerIntValues(ScoreDetails.LiveStones),
		Color: MultiplierColor(ActiveRuleSet.LiveStones), IsStatRow: true,
		Multiplier: ActiveRuleSet.LiveStones, MultiplierName: "Live Stones",
		TooltipSuffix: "per stone",
		HighlightFunc: func(Player uint8) []Coord {
			return GoWin.GetLiveStoneCoords(Player)
		},
	})

	// Territory
	Rows = append(Rows, ScoreRowInfo{
		Label: "Territory", PlayerValues: PlayerIntValues(ScoreDetails.Territory),
		Color: ScoreColorGreen, IsStatRow: true,
		Multiplier: 1.0, MultiplierName: "Territory",
		TooltipSuffix: "per vertex",
		HighlightFunc: func(Player uint8) []Coord {
			return GoWin.GetTerritoryCoords(Player)
		},
	})

	// Prisoners row (hidden for CaptureConverts)
	if !IsCaptureConverts {
		TotalPrisoners := map[uint8]int{}
		for _, Player := range Players {
			TotalPrisoners[Player] = ScoreDetails.OffBoardPrisoners[Player] + ScoreDetails.OnBoardPrisoners[Player]
		}
		Rows = append(Rows, ScoreRowInfo{
			Label: "Prisoners", PlayerValues: PlayerIntValues(TotalPrisoners),
			Color: MultiplierColor(ActiveRuleSet.Prisoners), IsStatRow: true,
			Multiplier: ActiveRuleSet.Prisoners, MultiplierName: "Prisoners",
			TooltipFunc: func(Player uint8) []string {
				return []string{
					fmt.Sprintf("Prisoners: %s per stone", FormatPoints(ActiveRuleSet.Prisoners)),
					fmt.Sprintf("On board: %d", ScoreDetails.OnBoardPrisoners[Player]),
					fmt.Sprintf("Off board: %d", ScoreDetails.OffBoardPrisoners[Player]),
				}
			},
			HighlightFunc: func(Player uint8) []Coord {
				return GoWin.GetOnBoardPrisonerCoords(Player)
			},
		})
	}

	// Passes
	Rows = append(Rows, ScoreRowInfo{
		Label: "Passes", PlayerValues: PlayerIntValues(ScoreDetails.Passes),
		Color: MultiplierColor(ActiveRuleSet.Passes), IsStatRow: true,
		Multiplier: ActiveRuleSet.Passes, MultiplierName: "Passes",
		TooltipSuffix: "per pass",
	})

	// Stone Suicides (hidden when suicide is not legal)
	if ActiveRuleSet != nil && ActiveRuleSet.SuicideIsLegal {
		Rows = append(Rows, ScoreRowInfo{
			Label: "Stone Suicides", PlayerValues: PlayerIntValues(ScoreDetails.StoneSuicides),
			Color: MultiplierColor(ActiveRuleSet.StoneSuicides), IsStatRow: true,
			Multiplier: ActiveRuleSet.StoneSuicides, MultiplierName: "Stone Suicides",
			TooltipSuffix: "per stone",
		})
	}

	// Conversions row (hidden for !CaptureConverts)
	if IsCaptureConverts {
		NetConversions := map[uint8]int{}
		for _, Player := range Players {
			NetConversions[Player] = ScoreDetails.ConversionsTo[Player] - ScoreDetails.ConversionsFrom[Player]
		}
		Rows = append(Rows, ScoreRowInfo{
			Label: "Conversions", PlayerValues: PlayerIntValues(NetConversions),
			Color: MultiplierColor(ActiveRuleSet.Conversions), IsStatRow: true,
			Multiplier: ActiveRuleSet.Conversions, MultiplierName: "Conversions",
			TooltipFunc: func(Player uint8) []string {
				return []string{
					fmt.Sprintf("Conversions: %s per stone", FormatPoints(ActiveRuleSet.Conversions)),
					fmt.Sprintf("Others' stones converted by Player: %d", ScoreDetails.ConversionsTo[Player]),
					fmt.Sprintf("Player's stones converted by Others: %d", ScoreDetails.ConversionsFrom[Player]),
				}
			},
		})
	}

	// Komi
	KomiValues := make([]string, len(Players))
	for Index, Player := range Players {
		if PlayerKomi, KomiExists := ScoreDetails.Komi[Player]; KomiExists {
			KomiValues[Index] = FormatPoints(PlayerKomi)
		} else {
			KomiValues[Index] = "0"
		}
	}
	Rows = append(Rows, ScoreRowInfo{
		Label: "Komi", PlayerValues: KomiValues,
		Color: ScoreColorGreen, IsStatRow: true,
		MultiplierName: "Komi",
	})

	// Rule set total rows
	for _, ShownRuleSet := range ScoreDetails.ShownRuleSets {
		RegisterRuleSet(ShownRuleSet)
		RuleSetValues := make([]string, len(Players))
		for Index, Player := range Players {
			RuleSetValues[Index] = FormatPoints(ScoreDetails.Score[ShownRuleSet][Player])
		}
		RowColor := ScoreColorYellow
		if ShownRuleSet.Equal(ActiveRuleSet) {
			RowColor = ScoreColorLightBlue
		}
		Rows = append(Rows, ScoreRowInfo{
			Label:        RuleSetDisplayName(ShownRuleSet, ScoreDetails.ShownRuleSets),
			PlayerValues: RuleSetValues, Color: RowColor,
		})
	}

	// Render rows into cells
	NumRows := len(Rows)
	GoWin.ScoreTexts = make([][]*canvas.Text, NumRows)
	ColObjects := make([][]fyne.CanvasObject, NumCols)
	for Col := range NumCols {
		ColObjects[Col] = make([]fyne.CanvasObject, NumRows)
	}
	for RowIndex, RowInfo := range Rows {
		GoWin.ScoreTexts[RowIndex] = make([]*canvas.Text, NumCols)
		for Col := range NumCols {
			CellText := RowInfo.Label
			if Col > 0 {
				CellText = RowInfo.PlayerValues[Col-1]
			}

			var Cell fyne.CanvasObject
			if RowInfo.IsStatRow && Col > 0 {
				Player := Players[Col-1]
				ScoreCell := &ScoreCell{
					Text:      CellText,
					TextColor: RowInfo.Color,
					Row:       RowIndex,
					Column:    Col,
					Player:    Player,
					Win:       GoWin,
				}
				ScoreCell.ExtendBaseWidget(ScoreCell)

				CapturedRowInfo := RowInfo
				ScoreCell.OnHoverIn = func() {
					fyne.Do(func() {
						if CapturedRowInfo.HighlightFunc != nil {
							GoWin.DrawHighlights(CapturedRowInfo.HighlightFunc(Player))
						}
						var TooltipLines []string
						if CapturedRowInfo.TooltipFunc != nil {
							TooltipLines = CapturedRowInfo.TooltipFunc(Player)
						} else if CapturedRowInfo.MultiplierName == "Komi" {
							TooltipLines = []string{fmt.Sprintf("%s: %s", CapturedRowInfo.MultiplierName, FormatPoints(ScoreDetails.Komi[Player]))}
						} else if CapturedRowInfo.TooltipSuffix != "" {
							TooltipLines = []string{fmt.Sprintf("%s: %s %s", CapturedRowInfo.MultiplierName, FormatPoints(CapturedRowInfo.Multiplier), CapturedRowInfo.TooltipSuffix)}
						} else {
							TooltipLines = []string{fmt.Sprintf("%s: %s", CapturedRowInfo.MultiplierName, FormatPoints(CapturedRowInfo.Multiplier))}
						}
						GoWin.DrawScoreTooltip(TooltipLines)
					})
				}

				ScoreCell.OnHoverOut = func() {
					fyne.Do(func() {
						GoWin.Layers["Highlights"].RemoveAll()
						GoWin.Layers["Highlights"].Refresh()
						GoWin.ClearScoreTooltip()
					})
				}

				Cell = container.New(layout.NewCustomPaddedLayout(ScoreCellPad, ScoreCellPad, ScoreCellPad, ScoreCellPad), ScoreCell)
				Text := canvas.NewText(CellText, RowInfo.Color)
				Text.Alignment = fyne.TextAlignCenter
				Text.TextSize = ScoreTextSize
				GoWin.ScoreTexts[RowIndex][Col] = Text
			} else {
				Text := canvas.NewText(CellText, RowInfo.Color)
				Text.Alignment = fyne.TextAlignCenter
				Text.TextSize = ScoreTextSize
				GoWin.ScoreTexts[RowIndex][Col] = Text
				Cell = container.New(layout.NewCustomPaddedLayout(ScoreCellPad, ScoreCellPad, ScoreCellPad, ScoreCellPad), Text)
			}
			ColObjects[Col][RowIndex] = Cell
		}
	}
	White := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	TableItems := make([]fyne.CanvasObject, 0, NumCols*2-1)
	for Col := range NumCols {
		TableItems = append(TableItems, container.New(ScoreColLayout{}, ColObjects[Col]...))
		if Col < NumCols-1 {
			TableItems = append(TableItems, canvas.NewRectangle(White))
		}
	}
	Background := canvas.NewRectangle(ScoreColorBlack)
	Table := container.New(ScoreTableLayout{}, TableItems...)
	GoWin.ScoreContainer.Objects = []fyne.CanvasObject{Background, Table}
	GoWin.ScoreContainer.Refresh()
	// Auto-adjust HSplit if the score table needs more horizontal room
	if GoWin.HSplit != nil && GoWin.MouseMode == "Score" {
		ScoreMinWidth := GoWin.ScoreContainer.MinSize().Width
		TotalWidth := GoWin.Win.Canvas().Size().Width
		if TotalWidth > 0 {
			CurrentLeftWidth := float32(GoWin.HSplit.Offset) * TotalWidth
			if ScoreMinWidth > CurrentLeftWidth {
				GoWin.HSplit.SetOffset(float64(ScoreMinWidth / TotalWidth))
			}
		}
	}
}

// IsGameOver reports whether the current position ends the game via consecutive passes.
// The game ends when every player has passed consecutively (in player order).
// When the active rule set has LastPlayerMustPassLast, the final pass must
// come from the highest-numbered player (player 2 in a standard game).
func (GoWin *GoWin) IsGameOver() bool {
	Node := GoWin.CurrentNode()
	if Node.Board == nil {
		return false
	}
	if Node.NoResultDraw {
		return true
	}
	NumPlayers := int(GoWin.Coll.CurrentGameH.Players)
	if NumPlayers == 0 {
		NumPlayers = 2
	}

	// Walk back NumPlayers nodes; every node must be a pass from a distinct player.
	// Passes always create exactly one node (consuming all remaining stone
	// placements), so this works correctly with StonePlacementsPerMove > 1.
	Cur := Node
	PassingPlayers := map[uint8]bool{}
	for Range := 0; Range < NumPlayers; Range++ {
		if Cur.PreviousStonePlacer == 0 {
			return false
		}
		if Cur.LastMove != PassCoords {
			return false
		}
		PassingPlayers[Cur.PreviousStonePlacer] = true
		Cur = Cur.Parent
	}
	if len(PassingPlayers) != NumPlayers {
		return false
	}

	// LastPlayerMustPassLast: the final pass must come from the last player.
	if GoWin.Coll.CurrentGameH.RuleSet.LastPlayerMustPassLast && Node.PreviousStonePlacer != uint8(NumPlayers) {
		return false
	}

	return true
}

// flips the alive/dead status of the group or empty
// region at Coord c in Score mode. Stones toggle between alive (original owner)
// and dead (0). Empty regions toggle between their current owner and unowned (0).
// Afterwards, territory is reassigned and score updated.
func (GoWin *GoWin) ToggleGroupStatus(InitialRebelCoord Coord) {
	InitialRebelPlayer := GoWin.CurrentNode().Board.GetPlayerAt(InitialRebelCoord)
	PreviousOwner := GoWin.CurrentNode().TerritoryMap[InitialRebelCoord]
	if PreviousOwner == 0 && InitialRebelPlayer == 0 {
		// Region is unowned and no stone, don't toggle
		return
	}
	RebelPlayers := map[uint8]struct{}{0: {}} // The Players rebelling against PreviousOwner
	RebelPlayers[InitialRebelPlayer] = struct{}{}
	RebelCoordsVisited := map[Coord]struct{}{}
	RebelCoordsToVisit := []Coord{InitialRebelCoord}
	PotentiallyVisit := []Coord{}
	PotentiallyVisitPlayers := map[uint8]struct{}{}
	// Decide new owner
	// Prepare for the possibility that InitialRebelCoord is dead, mark as alive
	NewOwner := InitialRebelPlayer
	if PreviousOwner == InitialRebelPlayer {
		// InitialRebelCoord is alive, mark as dead
		NewOwner = 0
	}
	for {
		// if 0 < InitialRebel && InitialRebel <= GoWin.Coll.CurrentGameH.Players {
		for len(RebelCoordsToVisit) > 0 {
			// Get last Coords from stack
			ActiveRebelCoord := RebelCoordsToVisit[len(RebelCoordsToVisit)-1]
			// Remove last Coords from stack
			RebelCoordsToVisit = RebelCoordsToVisit[:len(RebelCoordsToVisit)-1]
			// Be sure the rebel has not been visited yet
			if _, ActiveRebelVisited := RebelCoordsVisited[ActiveRebelCoord]; ActiveRebelVisited {
				continue
			}
			// Visit active rebel
			RebelCoordsVisited[ActiveRebelCoord] = struct{}{}
			ActiveRebelPlayer := GoWin.CurrentNode().Board.GetPlayerAt(ActiveRebelCoord)
			// Add Neighbors owned by PreviousOwner
			Neighbors := GoWin.Coll.CurrentGameH.Neighbors(ActiveRebelCoord)
			for _, NeighborCoord := range Neighbors {
				NeighborPlayer := GoWin.CurrentNode().Board.GetPlayerAt(NeighborCoord)
				switch GoWin.CurrentNode().TerritoryMap[NeighborCoord] {
				case 0:
				case PreviousOwner:
				default:
					continue
				}
				NowChooseToVisit := InitialRebelPlayer == 0
				if !NowChooseToVisit {
					_, NowChooseToVisit = RebelPlayers[NeighborPlayer]
				}
				if NowChooseToVisit {
					// Do not revisit
					if _, NeighborIsVisitedRebel := RebelCoordsVisited[NeighborCoord]; !NeighborIsVisitedRebel {
						RebelCoordsToVisit = append(RebelCoordsToVisit, NeighborCoord)
					}
				} else {
					PotentiallyVisit = append(PotentiallyVisit, NeighborCoord)
					PotentiallyVisitPlayers[NeighborPlayer] = struct{}{}
				}
			}
			// Change the owner
			if ActiveRebelPlayer == 0 {
				GoWin.CurrentNode().TerritoryMap[ActiveRebelCoord] = 0
			} else {
				GoWin.CurrentNode().TerritoryMap[ActiveRebelCoord] = NewOwner
			}
		}
		_, PotentiallyVisitPreviousOwner := PotentiallyVisitPlayers[PreviousOwner]
		if len(PotentiallyVisit) == 0 || PotentiallyVisitPreviousOwner {
			break
		}
		// All Coords in PotentiallyVisit rebel against PreviousOwner
		RebelCoordsToVisit = PotentiallyVisit
		PotentiallyVisit = []Coord{}
		// Conscript new rebel players
		for _, NewRebel := range RebelCoordsToVisit {
			NewRebelPlayer := GoWin.CurrentNode().Board.GetPlayerAt(NewRebel)
			RebelPlayers[NewRebelPlayer] = struct{}{}
		}
	}
	// Recalculate the territory
	GoWin.AssignTerritoryToEmptyRegions()
}
