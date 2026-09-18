package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	kanziio "github.com/flanglet/kanzi-go/v2/io"
	"github.com/klauspost/compress/zstd"
)

type CggFile struct {
	SaveGameFormat string    `json:"SaveGameFormat"`
	CurrentGame    int       `json:"CurrentGame"`
	Games          []CggGame `json:"Games"`
}

type CggGame struct {
	Width               uint8              `json:"Width"`
	Height              uint8              `json:"Height"`
	Players             uint8              `json:"Players"`
	Komi                map[string]float64 `json:"Komi,omitempty"`
	PlayerNames         map[string]string  `json:"PlayerNames,omitempty"`
	Information         map[string]string  `json:"Information,omitempty"`
	WrapXMulY           int8               `json:"WrapXMulY,omitempty"`
	WrapYMulX           int8               `json:"WrapYMulX,omitempty"`
	WrapXShiftY         uint8              `json:"WrapXShiftY,omitempty"`
	WrapYShiftX         uint8              `json:"WrapYShiftX,omitempty"`
	LibertySharingFixed bool               `json:"LibertySharingFixed,omitempty"`
	RuleSet             CggRuleSet         `json:"RuleSet"`
	RootNode            CggNode            `json:"RootNode"`
}

type CggRuleSet struct {
	Names                  []string `json:"Names"`
	LiveStones             float64  `json:"LiveStones,omitempty"`
	Passes                 float64  `json:"Passes,omitempty"`
	StoneSuicides          float64  `json:"StoneSuicides,omitempty"`
	Conversions            float64  `json:"Conversions,omitempty"`
	Prisoners              float64  `json:"Prisoners,omitempty"`
	LastPlayerMustPassLast bool     `json:"LastPlayerMustPassLast,omitempty"`
	SuicideIsLegal         bool     `json:"SuicideIsLegal,omitempty"`
	CaptureConverts        bool     `json:"CaptureConverts,omitempty"`
	NonRepetitionRules     []string `json:"NonRepetitionRules,omitempty"`
	StonePlacementsPerMove uint8    `json:"StonePlacementsPerMove"`
}

type CggNode struct {
	Kind                     string           `json:"Kind"`
	CurrentNode              bool             `json:"CurrentNode,omitempty"`
	LastMove                 *[2]uint8        `json:"LastMove,omitempty"`
	PreviousStonePlacer      uint8            `json:"PreviousStonePlacer,omitempty"`
	NextStonePlacer          uint8            `json:"NextStonePlacer,omitempty"`
	RemainingStonePlacements *uint8           `json:"RemainingStonePlacements,omitempty"`
	UnixMilli                int64            `json:"UnixMilli,omitempty"`
	Comment                  string           `json:"Comment,omitempty"`
	Labels                   []CggCoordString `json:"Labels,omitempty"`
	Players                  []CggCoordUint8  `json:"Players,omitempty"`
	Annotations              []CggCoordUint8  `json:"Annotations,omitempty"`
	PlayersDiff              []CggCoordUint8  `json:"PlayersDiff,omitempty"`
	AnnotationsDiff          []CggCoordUint8  `json:"AnnotationsDiff,omitempty"`
	LibertyMatrix            []CggMatrixCell  `json:"LibertyMatrix,omitempty"`
	LibertyMatrixDiff        []CggMatrixCell  `json:"LibertyMatrixDiff,omitempty"`
	FavoriteChildIndex       int              `json:"FavoriteChildIndex,omitempty"`
	TerritoryMap             []CggCoordUint8  `json:"TerritoryMap,omitempty"`
	Children                 []CggNode        `json:"Children,omitempty"`
}

type CggCoordUint8 struct {
	Coord [2]uint8
	Value uint8
}

func (Entry CggCoordUint8) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]any{Entry.Coord, Entry.Value})
}

func (Entry *CggCoordUint8) UnmarshalJSON(Data []byte) error {
	var Raw [2]json.RawMessage
	if Err := json.Unmarshal(Data, &Raw); Err != nil {
		return Err
	}
	if Err := json.Unmarshal(Raw[0], &Entry.Coord); Err != nil {
		return Err
	}
	return json.Unmarshal(Raw[1], &Entry.Value)
}

type CggCoordString struct {
	Coord [2]uint8
	Value string
}

func (Entry CggCoordString) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]any{Entry.Coord, Entry.Value})
}

func (Entry *CggCoordString) UnmarshalJSON(Data []byte) error {
	var Raw [2]json.RawMessage
	if Err := json.Unmarshal(Data, &Raw); Err != nil {
		return Err
	}
	if Err := json.Unmarshal(Raw[0], &Entry.Coord); Err != nil {
		return Err
	}
	return json.Unmarshal(Raw[1], &Entry.Value)
}

type CggMatrixCell struct {
	Row uint8  `json:"Row"`
	Col uint8  `json:"Col"`
	V   string `json:"V"`
}

func EncodeRuleSet(RS *RuleSet) CggRuleSet {
	Result := CggRuleSet{
		Names:                  RS.Names,
		LiveStones:             RS.LiveStones,
		Passes:                 RS.Passes,
		StoneSuicides:          RS.StoneSuicides,
		Conversions:            RS.Conversions,
		Prisoners:              RS.Prisoners,
		LastPlayerMustPassLast: RS.LastPlayerMustPassLast,
		SuicideIsLegal:         RS.SuicideIsLegal,
		CaptureConverts:        RS.CaptureConverts,
		StonePlacementsPerMove: RS.StonePlacementsPerMove,
	}
	Rules := []string{}
	Strongest := StrongestRepetitionRule(RS.NonRepetitionRules)
	if Strongest != 0 {
		Rules = append(Rules, NonRepetitionRuleFlagToName[Strongest])
	}
	if RS.NonRepetitionRules&NonRepetitionRuleNoResult != 0 {
		Rules = append(Rules, "NoResult")
	}
	Result.NonRepetitionRules = Rules
	return Result
}

func DecodeRuleSet(CRS CggRuleSet) *RuleSet {
	RS := &RuleSet{
		Names:                  CRS.Names,
		LiveStones:             CRS.LiveStones,
		Passes:                 CRS.Passes,
		StoneSuicides:          CRS.StoneSuicides,
		Conversions:            CRS.Conversions,
		Prisoners:              CRS.Prisoners,
		LastPlayerMustPassLast: CRS.LastPlayerMustPassLast,
		SuicideIsLegal:         CRS.SuicideIsLegal,
		CaptureConverts:        CRS.CaptureConverts,
		StonePlacementsPerMove: CRS.StonePlacementsPerMove,
	}
	var Flags uint
	for _, Name := range CRS.NonRepetitionRules {
		if Flag, Found := NonRepetitionRuleNameToFlag[Name]; Found {
			Flags |= Flag
		}
	}
	// Auto-fill weaker scope bits for consistency
	if Flags&NonRepetitionRulePositionalSuperKo != 0 {
		Flags |= NonRepetitionRuleSituationalSuperKo
	}
	if Flags&NonRepetitionRuleSituationalSuperKo != 0 {
		Flags |= NonRepetitionRuleNaturalSituationalSuperKo
	}
	if Flags&NonRepetitionRuleNaturalSituationalSuperKo != 0 {
		Flags |= NonRepetitionRuleBasicKo
	}
	RS.NonRepetitionRules = Flags
	return RS
}

func SortedCoords(Vertices map[Coord]uint8) []Coord {
	Result := make([]Coord, 0, len(Vertices))
	for C := range Vertices {
		Result = append(Result, C)
	}
	sort.Slice(Result, func(I, J int) bool {
		if Result[I].Y != Result[J].Y {
			return Result[I].Y < Result[J].Y
		}
		return Result[I].X < Result[J].X
	})
	return Result
}

func SortedLabelCoords(Labels map[Coord]string) []Coord {
	Result := make([]Coord, 0, len(Labels))
	for C := range Labels {
		Result = append(Result, C)
	}
	sort.Slice(Result, func(I, J int) bool {
		if Result[I].Y != Result[J].Y {
			return Result[I].Y < Result[J].Y
		}
		return Result[I].X < Result[J].X
	})
	return Result
}

func EncodeLibertyMatrixFull(Matrix *LibertySharingMatrix) []CggMatrixCell {
	if Matrix == nil {
		return nil
	}
	Result := []CggMatrixCell{}
	for Row := 1; Row < len(*Matrix); Row++ {
		for Col := 1; Col < len((*Matrix)[Row]); Col++ {
			if Row == Col {
				continue
			}
			Value := (*Matrix)[Row][Col]
			ShortName := SharingStateShortNames[Value]
			Result = append(Result, CggMatrixCell{
				Row: uint8(Row), Col: uint8(Col), V: ShortName,
			})
		}
	}
	return Result
}

func EncodeLibertyMatrixDiff(Parent, Current *LibertySharingMatrix) []CggMatrixCell {
	if Parent.Equal(Current) {
		return nil
	}
	Result := []CggMatrixCell{}
	MaxLen := len(*Current)
	if Parent != nil && len(*Parent) > MaxLen {
		MaxLen = len(*Parent)
	}
	for Row := 1; Row < MaxLen; Row++ {
		RowLen := 0
		if Current != nil && Row < len(*Current) {
			RowLen = len((*Current)[Row])
		}
		ParentRowLen := 0
		if Parent != nil && Row < len(*Parent) {
			ParentRowLen = len((*Parent)[Row])
		}
		MaxCol := max(RowLen, ParentRowLen)
		for Col := 1; Col < MaxCol; Col++ {
			if Row == Col {
				continue
			}
			ParentVal := uint8(SharingRefused)
			if Parent != nil && Row < len(*Parent) && Col < len((*Parent)[Row]) {
				ParentVal = (*Parent)[Row][Col]
			}
			CurVal := uint8(SharingRefused)
			if Current != nil && Row < len(*Current) && Col < len((*Current)[Row]) {
				CurVal = (*Current)[Row][Col]
			}
			if ParentVal != CurVal {
				Result = append(Result, CggMatrixCell{
					Row: uint8(Row), Col: uint8(Col), V: SharingStateShortNames[CurVal],
				})
			}
		}
	}
	if len(Result) == 0 {
		return nil
	}
	return Result
}

func DecodeLibertyMatrixFull(Players uint8, Cells []CggMatrixCell) *LibertySharingMatrix {
	Matrix := NewLibertySharingMatrix(Players, SharingRefused)
	for _, Cell := range Cells {
		Row := int(Cell.Row)
		Col := int(Cell.Col)
		if Row < len(*Matrix) && Col < len((*Matrix)[Row]) && Row != Col {
			State, Found := SharingStateByShortName[Cell.V]
			if !Found {
				State = SharingRefused
			}
			(*Matrix)[Row][Col] = State
		}
	}
	return Matrix
}

func ApplyLibertyMatrixDiff(Base *LibertySharingMatrix, Diff []CggMatrixCell) *LibertySharingMatrix {
	Result := Base.Clone()
	for _, Cell := range Diff {
		Row := int(Cell.Row)
		Col := int(Cell.Col)
		if Row < len(*Result) && Col < len((*Result)[Row]) && Row != Col {
			State, Found := SharingStateByShortName[Cell.V]
			if !Found {
				State = SharingRefused
			}
			(*Result)[Row][Col] = State
		}
	}
	return Result
}

func EncodeLabels(Labels map[Coord]string) []CggCoordString {
	if len(Labels) == 0 {
		return nil
	}
	Result := make([]CggCoordString, 0, len(Labels))
	for _, C := range SortedLabelCoords(Labels) {
		Result = append(Result, CggCoordString{
			Coord: [2]uint8{C.X, C.Y}, Value: Labels[C],
		})
	}
	return Result
}

func DecodeLabels(Entries []CggCoordString) map[Coord]string {
	if len(Entries) == 0 {
		return map[Coord]string{}
	}
	Result := make(map[Coord]string, len(Entries))
	for _, Entry := range Entries {
		Result[Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}] = Entry.Value
	}
	return Result
}

func EncodeTerritoryMap(TerritoryMap map[Coord]uint8) []CggCoordUint8 {
	if len(TerritoryMap) == 0 {
		return nil
	}
	Sorted := make([]Coord, 0, len(TerritoryMap))
	for C := range TerritoryMap {
		Sorted = append(Sorted, C)
	}
	sort.Slice(Sorted, func(I, J int) bool {
		if Sorted[I].Y != Sorted[J].Y {
			return Sorted[I].Y < Sorted[J].Y
		}
		return Sorted[I].X < Sorted[J].X
	})
	Result := make([]CggCoordUint8, 0, len(TerritoryMap))
	for _, C := range Sorted {
		Result = append(Result, CggCoordUint8{
			Coord: [2]uint8{C.X, C.Y}, Value: TerritoryMap[C],
		})
	}
	return Result
}

func DecodeTerritoryMap(Entries []CggCoordUint8) map[Coord]uint8 {
	if len(Entries) == 0 {
		return nil
	}
	Result := make(map[Coord]uint8, len(Entries))
	for _, Entry := range Entries {
		Result[Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}] = Entry.Value
	}
	return Result
}

func EncodeNode(Node *GameTreeNode, Parent *GameTreeNode, CurrentNode *GameTreeNode) CggNode {
	Result := CggNode{}
	if Node == CurrentNode {
		Result.CurrentNode = true
	}
	Result.UnixMilli = Node.UnixMilli
	Result.Comment = Node.Comment
	Result.Labels = EncodeLabels(Node.Labels)
	Result.TerritoryMap = EncodeTerritoryMap(Node.TerritoryMap)

	if Parent == nil {
		// Root node
		Result.Kind = "Root"
		Result.NextStonePlacer = Node.NextStonePlacer
		if Node.Board.Hist.RuleSet != nil &&
			Node.RemainingStonePlacements != Node.Board.Hist.RuleSet.StonePlacementsPerMove {
			Rsp := Node.RemainingStonePlacements
			Result.RemainingStonePlacements = &Rsp
		}
		// Emit full players and annotations from Board.Vertices
		PlayerEntries := []CggCoordUint8{}
		AnnotationEntries := []CggCoordUint8{}
		for _, C := range SortedCoords(Node.Board.Vertices) {
			V := Node.Board.Vertices[C]
			Player := V & PlayerAtVertexMask
			Annotation := V &^ PlayerAtVertexMask
			if Player != 0 {
				PlayerEntries = append(PlayerEntries, CggCoordUint8{
					Coord: [2]uint8{C.X, C.Y}, Value: Player,
				})
			}
			if Annotation != 0 {
				AnnotationEntries = append(AnnotationEntries, CggCoordUint8{
					Coord: [2]uint8{C.X, C.Y}, Value: Annotation,
				})
			}
		}
		if len(PlayerEntries) > 0 {
			Result.Players = PlayerEntries
		}
		if len(AnnotationEntries) > 0 {
			Result.Annotations = AnnotationEntries
		}
		Result.LibertyMatrix = EncodeLibertyMatrixFull(Node.Board.NextLibertySharingMatrix)
	} else if Node.LastMove == EditCoords {
		// Edit node
		Result.Kind = "Edit"
		// PlayersDiff: player nibble changes vs parent
		PlayersDiff := []CggCoordUint8{}
		AllCoords := map[Coord]bool{}
		for C := range Node.Board.Vertices {
			AllCoords[C] = true
		}
		for C := range Parent.Board.Vertices {
			AllCoords[C] = true
		}
		SortedAll := make([]Coord, 0, len(AllCoords))
		for C := range AllCoords {
			SortedAll = append(SortedAll, C)
		}
		sort.Slice(SortedAll, func(I, J int) bool {
			if SortedAll[I].Y != SortedAll[J].Y {
				return SortedAll[I].Y < SortedAll[J].Y
			}
			return SortedAll[I].X < SortedAll[J].X
		})
		for _, C := range SortedAll {
			ParentPlayer := Parent.Board.Vertices[C] & PlayerAtVertexMask
			ChildPlayer := Node.Board.Vertices[C] & PlayerAtVertexMask
			if ParentPlayer != ChildPlayer {
				PlayersDiff = append(PlayersDiff, CggCoordUint8{
					Coord: [2]uint8{C.X, C.Y}, Value: ChildPlayer,
				})
			}
		}
		if len(PlayersDiff) > 0 {
			Result.PlayersDiff = PlayersDiff
		}
		// AnnotationsDiff: annotation nibble changes vs parent
		AnnotationsDiff := []CggCoordUint8{}
		for _, C := range SortedAll {
			ParentAnnotation := Parent.Board.Vertices[C] &^ PlayerAtVertexMask
			ChildAnnotation := Node.Board.Vertices[C] &^ PlayerAtVertexMask
			if ParentAnnotation != ChildAnnotation {
				AnnotationsDiff = append(AnnotationsDiff, CggCoordUint8{
					Coord: [2]uint8{C.X, C.Y}, Value: ChildAnnotation,
				})
			}
		}
		if len(AnnotationsDiff) > 0 {
			Result.AnnotationsDiff = AnnotationsDiff
		}
		Result.LibertyMatrixDiff = EncodeLibertyMatrixDiff(
			Parent.Board.NextLibertySharingMatrix,
			Node.Board.NextLibertySharingMatrix)
	} else {
		// Move or Pass node
		Result.Kind = "Move"
		LastMove := [2]uint8{Node.LastMove.X, Node.LastMove.Y}
		Result.LastMove = &LastMove
		Result.PreviousStonePlacer = Node.PreviousStonePlacer
		// Omit NextStonePlacer when it matches the default cycle
		DefaultNext := NextPlayer(Node.PreviousStonePlacer, Node.Board.Hist.Players)
		if Node.NextStonePlacer != 0 && Node.NextStonePlacer != DefaultNext {
			Result.NextStonePlacer = Node.NextStonePlacer
		}
		// RemainingStonePlacements: omit when default
		DefaultRsp := uint8(1)
		if Node.Board != nil && Node.Board.Hist != nil && Node.Board.Hist.RuleSet != nil {
			DefaultRsp = Node.Board.Hist.RuleSet.StonePlacementsPerMove
		}
		if Node.RemainingStonePlacements != DefaultRsp {
			Rsp := Node.RemainingStonePlacements
			Result.RemainingStonePlacements = &Rsp
		}
		// Move nodes strip parent annotations so emit full annotation set
		AnnotationEntries := []CggCoordUint8{}
		for _, C := range SortedCoords(Node.Board.Vertices) {
			Annotation := Node.Board.Vertices[C] &^ PlayerAtVertexMask
			if Annotation != 0 {
				AnnotationEntries = append(AnnotationEntries, CggCoordUint8{
					Coord: [2]uint8{C.X, C.Y}, Value: Annotation,
				})
			}
		}
		if len(AnnotationEntries) > 0 {
			Result.Annotations = AnnotationEntries
		}
		Result.LibertyMatrixDiff = EncodeLibertyMatrixDiff(
			Parent.Board.NextLibertySharingMatrix,
			Node.Board.NextLibertySharingMatrix)
	}

	// FavoriteChildIndex
	if Node.FavoriteChild != nil && len(Node.Children) > 1 {
		for I, Child := range Node.Children {
			if Child == Node.FavoriteChild {
				Result.FavoriteChildIndex = I
				break
			}
		}
	}

	// Children
	if len(Node.Children) > 0 {
		Result.Children = make([]CggNode, len(Node.Children))
		for I, Child := range Node.Children {
			Result.Children[I] = EncodeNode(Child, Node, CurrentNode)
		}
	}

	return Result
}

func EncodeGame(Hist *GameHistory, CurrentGameH *GameHistory) CggGame {
	Game := CggGame{
		Width:               Hist.Width,
		Height:              Hist.Height,
		Players:             Hist.Players,
		WrapXMulY:           Hist.WrapXMulY,
		WrapYMulX:           Hist.WrapYMulX,
		WrapXShiftY:         Hist.WrapXShiftY,
		WrapYShiftX:         Hist.WrapYShiftX,
		LibertySharingFixed: Hist.LibertySharingFixed,
	}
	if Hist.RuleSet != nil {
		Game.RuleSet = EncodeRuleSet(Hist.RuleSet)
	}
	// Komi: omit entries with value 0
	if Hist.Komi != nil {
		KomiMap := map[string]float64{}
		for Player, Value := range Hist.Komi {
			if Value != 0 {
				KomiMap[strconv.Itoa(int(Player))] = Value
			}
		}
		if len(KomiMap) > 0 {
			Game.Komi = KomiMap
		}
	}
	// PlayerNames
	if len(Hist.PlayerNames) > 0 {
		Names := map[string]string{}
		for Player, Name := range Hist.PlayerNames {
			if Name != "" {
				Names[strconv.Itoa(int(Player))] = Name
			}
		}
		if len(Names) > 0 {
			Game.PlayerNames = Names
		}
	}
	// Information: strip "Komi" key if present
	if len(Hist.Information) > 0 {
		Info := map[string]string{}
		for Key, ValPtr := range Hist.Information {
			if Key == "Komi" || ValPtr == nil {
				continue
			}
			Info[Key] = *ValPtr
		}
		if len(Info) > 0 {
			Game.Information = Info
		}
	}
	Game.RootNode = EncodeNode(Hist.RootNode, nil, Hist.CurrentNode)
	return Game
}

func EncodeCggFile(Coll *Collection) CggFile {
	File := CggFile{
		SaveGameFormat: CggSaveGameFormat,
		CurrentGame:    -1,
	}
	for Index, Child := range Coll.CollectionNode.Children {
		if Child.Board != nil && Child.Board.Hist != nil {
			File.Games = append(File.Games, EncodeGame(Child.Board.Hist, Coll.CurrentGameH))
			if Coll.CurrentGameH == Child.Board.Hist {
				File.CurrentGame = Index
			}
		}
	}
	return File
}

func DecodeGame(CGame CggGame, Coll *Collection) (*GameHistory, error) {
	Hist := &GameHistory{
		Collection:          Coll,
		Width:               CGame.Width,
		Height:              CGame.Height,
		Players:             CGame.Players,
		Komi:                map[uint8]float64{},
		PlayerNames:         map[uint8]string{},
		Information:         map[string]*string{},
		WrapXMulY:           CGame.WrapXMulY,
		WrapYMulX:           CGame.WrapYMulX,
		WrapXShiftY:         CGame.WrapXShiftY,
		WrapYShiftX:         CGame.WrapYShiftX,
		LibertySharingFixed: CGame.LibertySharingFixed,
		RuleSet:             DecodeRuleSet(CGame.RuleSet),
	}
	Hist.RecomputeWrapInverses()
	RegisterRuleSet(Hist.RuleSet)
	for PlayerStr, Value := range CGame.Komi {
		Player, Err := strconv.ParseUint(PlayerStr, 10, 8)
		if Err != nil {
			return nil, fmt.Errorf("invalid komi player key %q", PlayerStr)
		}
		Hist.Komi[uint8(Player)] = Value
	}
	for PlayerStr, Name := range CGame.PlayerNames {
		Player, Err := strconv.ParseUint(PlayerStr, 10, 8)
		if Err != nil {
			return nil, fmt.Errorf("invalid player name key %q", PlayerStr)
		}
		Hist.PlayerNames[uint8(Player)] = Name
	}
	for Key, Value := range CGame.Information {
		if Key == "Komi" {
			continue
		}
		V := Value
		Hist.Information[Key] = &V
	}

	// Build root node
	RootNode := Coll.MakeChild(Coll.CollectionNode)
	RootNode.Board = Hist.NewBoardState()
	RootNode.LastMove = RootCoords
	Hist.RootNode = RootNode
	Hist.CurrentNode = RootNode

	// Apply root node data
	var CurrentNodePtr *GameTreeNode
	Err := DecodeNode(&CGame.RootNode, RootNode, nil, Hist, &CurrentNodePtr, []int{})
	if Err != nil {
		return nil, Err
	}
	if CurrentNodePtr != nil {
		Hist.CurrentNode = CurrentNodePtr
	}
	return Hist, nil
}

func DecodeNode(CNode *CggNode, Gtn *GameTreeNode, Parent *GameTreeNode, Hist *GameHistory, CurrentNodePtr **GameTreeNode, Path []int) error {
	Gtn.UnixMilli = CNode.UnixMilli
	Gtn.Comment = CNode.Comment
	Gtn.Labels = DecodeLabels(CNode.Labels)
	Gtn.TerritoryMap = DecodeTerritoryMap(CNode.TerritoryMap)

	if CNode.CurrentNode {
		*CurrentNodePtr = Gtn
	}

	switch CNode.Kind {
	case "Root":
		Gtn.NextStonePlacer = CNode.NextStonePlacer
		if CNode.RemainingStonePlacements != nil {
			Gtn.RemainingStonePlacements = *CNode.RemainingStonePlacements
		} else if Hist.RuleSet != nil {
			Gtn.RemainingStonePlacements = Hist.RuleSet.StonePlacementsPerMove
		}
		// Apply Players and Annotations to Board.Vertices
		for _, Entry := range CNode.Players {
			C := Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}
			Gtn.Board.SetPlayerAt(C, Entry.Value)
		}
		for _, Entry := range CNode.Annotations {
			C := Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}
			Gtn.Board.Vertices[C] = (Gtn.Board.Vertices[C] & PlayerAtVertexMask) | (Entry.Value &^ PlayerAtVertexMask)
		}
		// Apply LibertyMatrix
		if CNode.LibertyMatrix != nil {
			Gtn.Board.NextLibertySharingMatrix = DecodeLibertyMatrixFull(Hist.Players, CNode.LibertyMatrix)
		}
		Gtn.Board.CalculateAllGroups()

	case "Move":
		if CNode.LastMove == nil {
			return fmt.Errorf("move node at path %v missing LastMove", Path)
		}
		LastMove := Coord{X: CNode.LastMove[0], Y: CNode.LastMove[1]}
		Player := CNode.PreviousStonePlacer
		if Player == 0 {
			return fmt.Errorf("move node at path %v missing PreviousStonePlacer", Path)
		}
		if Parent == nil || Parent.Board == nil {
			return fmt.Errorf("move node at path %v has no parent board", Path)
		}
		// If liberty matrix diff, apply to parent board before AttemptMove
		ParentMatrix := Parent.Board.NextLibertySharingMatrix
		if len(CNode.LibertyMatrixDiff) > 0 {
			NewMatrix := ApplyLibertyMatrixDiff(ParentMatrix, CNode.LibertyMatrixDiff)
			Parent.Board.NextLibertySharingMatrix = NewMatrix
			defer func() { Parent.Board.NextLibertySharingMatrix = ParentMatrix }()
		}
		// Temporarily set Hist.CurrentNode to parent for repetition checking
		SavedCurrentNode := Hist.CurrentNode
		Hist.CurrentNode = Parent
		NewBoard, Err := Parent.Board.AttemptMove(LastMove, Player, false, false)
		Hist.CurrentNode = SavedCurrentNode
		if Err != nil {
			return fmt.Errorf("move node at path %v rejected: %v", Path, Err)
		}
		Gtn.Board = NewBoard
		Gtn.LastMove = LastMove
		Gtn.PreviousStonePlacer = Player
		Gtn.NextStonePlacer = CNode.NextStonePlacer
		if CNode.RemainingStonePlacements != nil {
			Gtn.RemainingStonePlacements = *CNode.RemainingStonePlacements
		}
		// If liberty matrix changed, set child's matrix (AttemptMove inherited parent's)
		if len(CNode.LibertyMatrixDiff) > 0 {
			Gtn.Board.NextLibertySharingMatrix = ApplyLibertyMatrixDiff(ParentMatrix, CNode.LibertyMatrixDiff)
		}
		// Apply full annotations (move nodes strip inherited annotations)
		for _, Entry := range CNode.Annotations {
			C := Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}
			Gtn.Board.Vertices[C] = (Gtn.Board.Vertices[C] & PlayerAtVertexMask) | (Entry.Value &^ PlayerAtVertexMask)
		}
		// NoResultDraw is set by AttemptMove via Board.NoResultTriggered
		Gtn.NoResultDraw = NewBoard.NoResultTriggered

	case "Edit":
		if Parent == nil || Parent.Board == nil {
			return fmt.Errorf("edit node at path %v has no parent board", Path)
		}
		Gtn.Board = Parent.Board.Copy(true)
		Gtn.Board.IsPlacementLegalCache = nil
		Gtn.LastMove = EditCoords
		Gtn.PreviousStonePlacer = Parent.PreviousStonePlacer
		// Apply PlayersDiff
		for _, Entry := range CNode.PlayersDiff {
			C := Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}
			Gtn.Board.SetPlayerAt(C, Entry.Value)
		}
		// Apply AnnotationsDiff
		for _, Entry := range CNode.AnnotationsDiff {
			C := Coord{X: Entry.Coord[0], Y: Entry.Coord[1]}
			Gtn.Board.Vertices[C] = (Gtn.Board.Vertices[C] & PlayerAtVertexMask) | (Entry.Value &^ PlayerAtVertexMask)
		}
		// Apply LibertyMatrixDiff
		if len(CNode.LibertyMatrixDiff) > 0 {
			Gtn.Board.NextLibertySharingMatrix = ApplyLibertyMatrixDiff(
				Parent.Board.NextLibertySharingMatrix, CNode.LibertyMatrixDiff)
		}
		Gtn.Board.CalculateAllGroups()

	default:
		return fmt.Errorf("unknown node kind %q at path %v", CNode.Kind, Path)
	}

	// Children
	for I := range CNode.Children {
		ChildGtn := Hist.Collection.MakeChild(Gtn)
		ChildPath := append(slices.Clone(Path), I)
		Err := DecodeNode(&CNode.Children[I], ChildGtn, Gtn, Hist, CurrentNodePtr, ChildPath)
		if Err != nil {
			return Err
		}
	}

	// FavoriteChildIndex
	if CNode.FavoriteChildIndex > 0 && CNode.FavoriteChildIndex < len(Gtn.Children) {
		Gtn.FavoriteChild = Gtn.Children[CNode.FavoriteChildIndex]
	} else if len(Gtn.Children) > 0 {
		Gtn.FavoriteChild = Gtn.Children[0]
	}

	return nil
}

type NopWriteCloser struct {
	Buffer bytes.Buffer
}

func (N *NopWriteCloser) Write(P []byte) (int, error) { return N.Buffer.Write(P) }
func (N *NopWriteCloser) Close() error                { return nil }


// CggSuffixFileFilter implements storage.FileFilter by matching full suffixes
// (e.g. ".CGG.json") rather than just the last-dot extension.
type CggSuffixFileFilter struct {
	Suffixes []string
}

func (Filter *CggSuffixFileFilter) Matches(Uri fyne.URI) bool {
	Name := Uri.Name()
	for _, Suffix := range Filter.Suffixes {
		if strings.HasSuffix(strings.ToLower(Name), strings.ToLower(Suffix)) {
			return true
		}
	}
	return false
}

func (GoWin *GoWin) ShowCggSaveDialog(Method string, CompressionLevel int, KanziBlockSize uint) {
	Extension := CggMethodToExtension[Method]
	DefaultFilename := fmt.Sprintf("%ds%s", time.Now().Unix(), Extension)
	FileDialog := dialog.NewFileSave(func(Writer fyne.URIWriteCloser, Err error) {
		GoWin.DialogClosed()
		if Err != nil || Writer == nil {
			return
		}
		ChosenPath := Writer.URI().Path()
		Writer.Close()

		if !strings.HasSuffix(ChosenPath, Extension) {
			ChosenPath += Extension
		}

		CggData := EncodeCggFile(GoWin.Coll)
		JsonBytes, Err := json.MarshalIndent(CggData, "", "\t")
		if Err != nil {
			GoWin.ShowError(Err)
			return
		}

		var OutputBytes []byte
		switch Method {
		case "Plaintext":
			OutputBytes = JsonBytes
		case "Gzip":
			var Buf bytes.Buffer
			GzipWriter, _ := gzip.NewWriterLevel(&Buf, CompressionLevel)
			_, Err = GzipWriter.Write(JsonBytes)
			if Err == nil {
				Err = GzipWriter.Close()
			}
			OutputBytes = Buf.Bytes()
		case "Zstd":
			var Buf bytes.Buffer
			ZstdWriter, ZstdErr := zstd.NewWriter(&Buf, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(CompressionLevel)))
			if ZstdErr != nil {
				GoWin.ShowError(fmt.Errorf("zstd writer: %v", ZstdErr))
				return
			}
			_, Err = ZstdWriter.Write(JsonBytes)
			if Err == nil {
				Err = ZstdWriter.Close()
			}
			OutputBytes = Buf.Bytes()
		case "Kanzi":
			KanziFile, KanziErr := os.Create(ChosenPath)
			if KanziErr != nil {
				GoWin.ShowError(KanziErr)
				return
			}
			Jobs := max(uint(runtime.NumCPU()), 1)
			KanziWriter, KErr := kanziio.NewWriter(KanziFile, "BWT+RANK+ZRLT", "ANS0", KanziBlockSize, Jobs, 32, int64(len(JsonBytes)), false)
			if KErr != nil {
				KanziFile.Close()
				GoWin.ShowError(fmt.Errorf("kanzi writer: %v", KErr))
				return
			}
			_, Err = KanziWriter.Write(JsonBytes)
			if Err == nil {
				Err = KanziWriter.Close()
			}
			KanziFile.Close()
			if Err != nil {
				GoWin.ShowError(Err)
			} else {
				GoWin.OpenedFilePath = ChosenPath
				GoWin.FixWindowTitle()
			}
			return
		}
		if Err == nil {
			Err = os.WriteFile(ChosenPath, OutputBytes, 0644)
		}

		if Err != nil {
			GoWin.ShowError(Err)
		} else {
			GoWin.OpenedFilePath = ChosenPath
			GoWin.FixWindowTitle()
		}
	}, GoWin.Win)
	FileDialog.SetFileName(DefaultFilename)
	FileDialog.SetFilter(&CggSuffixFileFilter{Suffixes: []string{Extension}})
	GoWin.ResizeDialog = func() { FileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	FileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	FileDialog.Show()
}

// DecompressCgg picks the decompression algorithm from Path's extension and
// returns the JSON bytes. If no known compression extension matches, RawBytes
// is returned unchanged.
func DecompressCgg(RawBytes []byte, Path string) ([]byte, error) {
	switch {
	case strings.HasSuffix(Path, ".gzip"):
		Reader, Err := gzip.NewReader(bytes.NewReader(RawBytes))
		if Err != nil {
			return nil, fmt.Errorf("gzip decompression: %v", Err)
		}
		defer Reader.Close()
		Data, ReadErr := io.ReadAll(Reader)
		if ReadErr != nil {
			return nil, fmt.Errorf("gzip decompression: %v", ReadErr)
		}
		return Data, nil
	case strings.HasSuffix(Path, ".kanzi"):
		ReadCloser := io.NopCloser(bytes.NewReader(RawBytes))
		Jobs := max(uint(runtime.NumCPU()), 1)
		Reader, Err := kanziio.NewReader(ReadCloser, Jobs)
		if Err != nil {
			return nil, fmt.Errorf("kanzi reader: %v", Err)
		}
		defer Reader.Close()
		Data, ReadErr := io.ReadAll(Reader)
		if ReadErr != nil {
			return nil, fmt.Errorf("kanzi decompression: %v", ReadErr)
		}
		return Data, nil
	case strings.HasSuffix(Path, ".zstd"):
		Reader, Err := zstd.NewReader(bytes.NewReader(RawBytes))
		if Err != nil {
			return nil, fmt.Errorf("zstd reader: %v", Err)
		}
		defer Reader.Close()
		Data, ReadErr := io.ReadAll(Reader)
		if ReadErr != nil {
			return nil, fmt.Errorf("zstd decompression: %v", ReadErr)
		}
		return Data, nil
	default:
		return RawBytes, nil
	}
}

// ImportCggBytes decodes RawBytes as a CGG file (optionally compressed per
// Path's extension), installs the decoded Collection on GoWin, refreshes the
// UI, and sets GoWin.OpenedFilePath = Path on success. Returns a fatal error
// that prevented any loading; per-game errors are surfaced via ShowError.
func (GoWin *GoWin) ImportCggBytes(RawBytes []byte, Path string) error {
	JsonBytes, Err := DecompressCgg(RawBytes, Path)
	if Err != nil {
		return Err
	}
	var CggData CggFile
	if Err := json.Unmarshal(JsonBytes, &CggData); Err != nil {
		return fmt.Errorf("JSON parse error: %v", Err)
	}
	if CggData.SaveGameFormat != CggSaveGameFormat {
		return fmt.Errorf("unsupported save format: %q (expected %q)",
			CggData.SaveGameFormat, CggSaveGameFormat)
	}

	NewColl := NewCollection()
	var Errors []string
	var LoadedGameHistories []*GameHistory
	for GameIndex, CGame := range CggData.Games {
		Hist, GameErr := DecodeGame(CGame, NewColl)
		if GameErr != nil {
			Errors = append(Errors, fmt.Sprintf("game %d: %v", GameIndex, GameErr))
			Children := NewColl.CollectionNode.Children
			if len(Children) > 0 {
				LastChild := Children[len(Children)-1]
				if LastChild.Board != nil && LastChild.Board.Hist == Hist {
					NewColl.CollectionNode.Children = Children[:len(Children)-1]
				}
			}
			continue
		}
		LoadedGameHistories = append(LoadedGameHistories, Hist)
	}
	if len(LoadedGameHistories) == 0 && len(CggData.Games) > 0 {
		return fmt.Errorf("no games could be loaded:\n%s", strings.Join(Errors, "\n"))
	}

	GoWin.Coll = NewColl
	if CggData.CurrentGame >= 0 && CggData.CurrentGame < len(LoadedGameHistories) {
		NewColl.CurrentGameH = LoadedGameHistories[CggData.CurrentGame]
	} else if len(LoadedGameHistories) > 0 {
		NewColl.CurrentGameH = LoadedGameHistories[0]
	}

	GoWin.UpdateGameTreeUI()
	if NewColl.CurrentGameH != nil {
		GoWin.SetCurrentNode(NewColl.CurrentGameH.CurrentNode)
	} else {
		GoWin.SetCurrentNode(NewColl.CollectionNode)
	}
	GoWin.OpenedFilePath = Path
	GoWin.FixWindowTitle()
	GoWin.RefreshGameSelector()
	if len(Errors) > 0 {
		GoWin.ShowError(fmt.Errorf("some games failed to load:\n%s", strings.Join(Errors, "\n")))
	}
	return nil
}

// ImportCggFile reads Path from disk and imports it as a CGG file. Used by
// command-line startup loading.
func (GoWin *GoWin) ImportCggFile(Path string) {
	RawBytes, Err := os.ReadFile(Path)
	if Err != nil {
		GoWin.ShowError(fmt.Errorf("read CGG file: %v", Err))
		return
	}
	if Err := GoWin.ImportCggBytes(RawBytes, Path); Err != nil {
		GoWin.ShowError(Err)
	}
}

func (GoWin *GoWin) HandleImportCgg() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	FileDialog := dialog.NewFileOpen(func(Reader fyne.URIReadCloser, Err error) {
		GoWin.DialogClosed()
		if Err != nil || Reader == nil {
			return
		}
		defer Reader.Close()
		RawBytes, ReadErr := io.ReadAll(Reader)
		if ReadErr != nil {
			GoWin.ShowError(ReadErr)
			return
		}
		if ImportErr := GoWin.ImportCggBytes(RawBytes, Reader.URI().Path()); ImportErr != nil {
			GoWin.ShowError(ImportErr)
		}
	}, GoWin.Win)
	FileDialog.SetFilter(&CggSuffixFileFilter{Suffixes: CggAllSuffixes})
	GoWin.ResizeDialog = func() { FileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	FileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	FileDialog.Show()
}

func Uint8MapToStringMap(M map[uint8]float64) map[string]float64 {
	Result := map[string]float64{}
	for K, V := range M {
		Result[strconv.Itoa(int(K))] = V
	}
	return Result
}

func Uint8StringMapToStringMap(M map[uint8]string) map[string]string {
	Result := map[string]string{}
	for K, V := range M {
		Result[strconv.Itoa(int(K))] = V
	}
	return Result
}
