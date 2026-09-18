package main

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

// GetPlayerAt returns the player number (0=empty, 1=Black, 2=White) at Coord c,
// masking out any annotation bits.
func (Board *BoardState) GetPlayerAt(C Coord) uint8 { return Board.Vertices[C] & PlayerAtVertexMask }

func (Board *BoardState) SetPlayerAt(C Coord, Player uint8) error {
	if Player == Player&PlayerAtVertexMask {
		New := (Board.Vertices[C] &^ PlayerAtVertexMask) | Player
		if New == 0 {
			delete(Board.Vertices, C)
		} else {
			Board.Vertices[C] = New
		}
		return nil
	}
	return fmt.Errorf("player > 15 is out of range")
}

// SetAnnotation sets the annotation bit(s) specified by Mask on Coord c.
func (Board *BoardState) SetAnnotation(C Coord, Mask uint8) { Board.Vertices[C] |= Mask }

// ClearAnnotation clears the annotation bit(s) specified by Mask on Coord c.
func (Board *BoardState) ClearAnnotation(C Coord, Mask uint8) { Board.Vertices[C] &^= Mask }

// HasAnnotation returns true if any of the annotation bits in Mask are set on Coord c.
func (Board *BoardState) HasAnnotation(C Coord, Mask uint8) bool { return Board.Vertices[C]&Mask != 0 }

func (Board *BoardState) ToggleAnnotation(C Coord, Mask uint8) {
	if Board.HasAnnotation(C, Mask) {
		Board.ClearAnnotation(C, Mask)
	} else {
		Board.SetAnnotation(C, Mask)
	}
}

// ToString converts a Coord to a human-readable string using the current
// coordinate format. Sentinel values (Root, Collection, Error, Pass,
// Edit) are returned as fixed strings.
func (Board *BoardState) ToString(C Coord) string {
	switch C {
	case RootCoords:
		return "Root"
	case CollectionCoords:
		return "Collection"
	case ErrorCoords:
		return "Error"
	case PassCoords:
		return "Pass"
	case EditCoords:
		return "Edit"
	}
	switch GetCoordFormat() {
	case CoordFmtGtp:
		s, e := Board.Hist.ClientToGTPCoords(C)
		if e == nil {
			return s
		} else {
			SetCoordFormat(CoordFmtHikaruNoGo)
		}
	case CoordFmtSgf:
		sgfCoord, err := ConvertCoordToSgfCoord(C)
		if err == nil {
			return sgfCoord
		} else {
			SetCoordFormat(CoordFmtHikaruNoGo)
		}
	case CoordFmtComputer:
		return fmt.Sprintf("%d,%d", C.X, C.Y)
	case CoordFmtHikaruNoGo:
		return fmt.Sprintf("%dの%d", C.X+1, C.Y+1)
	case CoordFmt1stQuadrant:
		return fmt.Sprintf("(%d,%d)", C.X, Board.Hist.Height-1-C.Y)
	}
	return fmt.Sprintf("%dの%d", C.X+1, C.Y+1)
}

// InBounds returns true if C lies within the board dimensions (0..Width-1,
// 0..Height-1). Sentinel coordinates always return false.
func (GHist *GameHistory) InBounds(C Coord) bool {
	return C.X < GHist.Width && C.Y < GHist.Height
}

func Int16ToUInt8Modulo(num int16, mod uint8) uint8 {
	Int16Mod := int16(mod)
	Result := num - (num/Int16Mod)*Int16Mod
	if Result < 0 {
		Result += Int16Mod
	}
	return uint8(Result)
}

// GcdInt16 returns the non-negative greatest common divisor of A and B.
func GcdInt16(A, B int16) int16 {
	if A < 0 {
		A = -A
	}
	if B < 0 {
		B = -B
	}
	for B != 0 {
		A, B = B, A%B
	}
	return A
}

// ModInverseInt16 returns the modular inverse of A modulo Mod in [0, Mod),
// and Ok=true when it exists. The inverse exists iff gcd(|A|, Mod) == 1.
func ModInverseInt16(A int16, Mod uint8) (Inv int16, Ok bool) {
	Int16Mod := int16(Mod)
	if Int16Mod <= 0 {
		return 0, false
	}
	// Extended Euclidean algorithm on (A mod Mod) and Mod.
	Normalized := A % Int16Mod
	if Normalized < 0 {
		Normalized += Int16Mod
	}
	OldR, R := Normalized, Int16Mod
	OldS, S := int16(1), int16(0)
	for R != 0 {
		Quotient := OldR / R
		OldR, R = R, OldR-Quotient*R
		OldS, S = S, OldS-Quotient*S
	}
	if OldR != 1 {
		return 0, false
	}
	Result := OldS % Int16Mod
	if Result < 0 {
		Result += Int16Mod
	}
	return Result, true
}

// RecomputeWrapInverses refreshes the cached WrapXMulYInv / WrapYMulXInv
// fields on GHist from the current WrapXMulY / WrapYMulX and Height / Width.
// Call this after assigning any of those four fields. The cached inverses are
// only meaningful when the corresponding Mul is non-zero.
func (GHist *GameHistory) RecomputeWrapInverses() {
	if WrapXMulYInv, Ok := ModInverseInt16(int16(GHist.WrapXMulY), GHist.Height); Ok {
		GHist.WrapXMulYInv = WrapXMulYInv
	} else {
		GHist.WrapXMulYInv = 0
	}
	if WrapYMulXInv, Ok := ModInverseInt16(int16(GHist.WrapYMulX), GHist.Width); Ok {
		GHist.WrapYMulXInv = WrapYMulXInv
	} else {
		GHist.WrapYMulXInv = 0
	}
}

// IsValidWrapMul reports whether Mul is a valid wrap multiplier for a board
// dimension of Dim: either Mul==0 (no wrap), or |Mul| ≤ Dim and gcd(|Mul|, Dim)==1
// (so a modular inverse exists modulo Dim).
func IsValidWrapMul(Mul int8, Dim uint8) bool {
	if Mul == 0 {
		return true
	}
	Abs := int16(Mul)
	if Abs < 0 {
		Abs = -Abs
	}
	if Abs > int16(Dim) {
		return false
	}
	return GcdInt16(Abs, int16(Dim)) == 1
}

// Neighbors returns the orthogonally adjacent coordinates of C that lie
// within the board boundaries. Takes board wrapping into consideration, even allowing play on Klein bottles
func (GHist *GameHistory) Neighbors(C Coord) []Coord {
	Neighbors := make([]Coord, 0, 4)
	if C.Y != 0 {
		Neighbors = append(Neighbors, Coord{X: C.X, Y: C.Y - 1})
	} else if GHist.WrapYMulX != 0 {
		Neighbors = append(Neighbors, Coord{
			X: Int16ToUInt8Modulo((int16(C.X)-int16(GHist.WrapYShiftX))*GHist.WrapYMulXInv, GHist.Width),
			Y: GHist.Height - 1})
	}
	if C.Y != GHist.Height-1 {
		Neighbors = append(Neighbors, Coord{X: C.X, Y: C.Y + 1})
	} else if GHist.WrapYMulX != 0 {
		Neighbors = append(Neighbors, Coord{
			X: Int16ToUInt8Modulo(int16(C.X)*int16(GHist.WrapYMulX)+int16(GHist.WrapYShiftX), GHist.Width),
			Y: 0})
	}
	if C.X != 0 {
		Neighbors = append(Neighbors, Coord{X: C.X - 1, Y: C.Y})
	} else if GHist.WrapXMulY != 0 {
		Neighbors = append(Neighbors, Coord{X: GHist.Width - 1,
			Y: Int16ToUInt8Modulo((int16(C.Y)-int16(GHist.WrapXShiftY))*GHist.WrapXMulYInv, GHist.Height)})
	}
	if C.X != GHist.Width-1 {
		Neighbors = append(Neighbors, Coord{X: C.X + 1, Y: C.Y})
	} else if GHist.WrapXMulY != 0 {
		Neighbors = append(Neighbors, Coord{X: 0,
			Y: Int16ToUInt8Modulo(int16(C.Y)*int16(GHist.WrapXMulY)+int16(GHist.WrapXShiftY), GHist.Height)})
	}
	return Neighbors
}

// CalculateGroupAtCoord performs a flood-fill from Coord c to build and
// return the Group owned by the stone at c. The flood-fill expands through
// same-color stones AND through stones of any color that the Owner effectively
// shares liberties with (per Board.NextLibertySharingMatrix). The returned
// group's Vertices[Owner] lists only the Owner's stones, Vertices[0] lists
// the collective liberties (empty adjacent vertices anywhere in the region),
// and Vertices[k] for k != Owner lists stones of player k that are either
// inside the shared region (if k is in the Owner's shared set) or adjacent
// boundary stones (if not). Returns nil if c is empty or off-board.
func (Board *BoardState) CalculateGroupAtCoord(c Coord) *Group {
	Owner := Board.GetPlayerAt(c)
	if Owner == 0 || Owner > Board.Hist.Players {
		return nil
	}
	SharedSet := Board.NextLibertySharingMatrix.SharedSet(Owner)
	IsShared := make(map[uint8]bool, len(SharedSet))
	for _, SharedPlayer := range SharedSet {
		IsShared[SharedPlayer] = true
	}
	CalculatedGroup := &Group{
		Owner:    Owner,
		Vertices: map[uint8][]Coord{Owner: {c}},
	}
	Visited := map[Coord]bool{c: true}
	InBucket := map[uint8]map[Coord]bool{Owner: {c: true}}

	// AppendOnce adds AdjacentCoord to Vertices[Color] if not already present;
	// uses InBucket for O(1) membership instead of slices.Contains.
	AppendOnce := func(Color uint8, Where Coord) {
		Bucket := InBucket[Color]
		if Bucket == nil {
			Bucket = map[Coord]bool{}
			InBucket[Color] = Bucket
		}
		if !Bucket[Where] {
			Bucket[Where] = true
			CalculatedGroup.Vertices[Color] = append(CalculatedGroup.Vertices[Color], Where)
		}
	}

	Queue := []Coord{c}
	for len(Queue) > 0 {
		Current := Queue[0]
		Queue = Queue[1:]
		for _, AdjacentCoord := range Board.Hist.Neighbors(Current) {
			AdjacentPlayer := Board.GetPlayerAt(AdjacentCoord)
			if AdjacentPlayer == 0 {
				AppendOnce(0, AdjacentCoord)
				continue
			}
			AppendOnce(AdjacentPlayer, AdjacentCoord)
			if IsShared[AdjacentPlayer] && !Visited[AdjacentCoord] {
				Visited[AdjacentCoord] = true
				Queue = append(Queue, AdjacentCoord)
			}
		}
	}
	return CalculatedGroup
}

// GetGroupAtCoord searches the pre-computed Groups set for the Group that
// contains Coord c. Returns nil if c is empty or not found.
func (Board *BoardState) GetGroupAtCoord(c Coord) *Group {
	if group, exists := Board.CheckCoordToGroupCache(c); exists {
		return group
	}
	player := Board.GetPlayerAt(c)
	if 0 < player && player <= Board.Hist.Players {
		for group := range Board.Groups {
			// Only check groups of the same color for efficiency
			if slices.Contains(group.Vertices[group.Owner], c) {
				Board.SetCoordToGroupCache(c, group)
				return group
			}
		}
	}
	return nil
}

// RemoveGroupAtCoord removes all stones belonging to the group at Coord c
// from the Vertices map. Does nothing if c is empty.
func (Board *BoardState) RemoveGroupAtCoord(c Coord) {
	player := Board.GetPlayerAt(c)
	if 0 < player && player <= Board.Hist.Players {
		group := Board.GetGroupAtCoord(c)
		for _, stone := range group.Vertices[group.Owner] {
			delete(Board.Vertices, stone)
		}
	}
}

// RemoveGroup removes all stones in the given Group from the Vertices map.
func (Board *BoardState) RemoveGroup(group *Group) {
	for _, stone := range group.Vertices[group.Owner] {
		delete(Board.Vertices, stone)
	}
}

// CheckIsPlacementLegalCache looks up the cached legality result for placing
// Player's stone at Coordinate in the given EditMode. Returns (error, true)
// on cache hit (nil error means legal) or (_, false) on cache miss.
func (Board *BoardState) CheckIsPlacementLegalCache(Player uint8,
	EditMode bool, Coordinate Coord) (error, bool) {
	if Board == nil {
		return fmt.Errorf("Board does not exist!"), false
	}
	if Board.IsPlacementLegalCache == nil {
		Board.IsPlacementLegalCache = map[uint8]map[bool]map[Coord]error{}
	}
	if Board.IsPlacementLegalCache[Player] == nil {
		Board.IsPlacementLegalCache[Player] = map[bool]map[Coord]error{}
	}
	if Board.IsPlacementLegalCache[Player][EditMode] == nil {
		Board.IsPlacementLegalCache[Player][EditMode] = map[Coord]error{}
	}
	Err, Exists := Board.IsPlacementLegalCache[Player][EditMode][Coordinate]
	return Err, Exists
}

// SetIsPlacementLegalCache stores a legality result in the placement cache
// for Player at Coordinate in the given EditMode. A nil Err means the move is legal.
func (Board *BoardState) SetIsPlacementLegalCache(Player uint8,
	EditMode bool, Coordinate Coord, Err error) {
	if Board.IsPlacementLegalCache == nil {
		Board.IsPlacementLegalCache = map[uint8]map[bool]map[Coord]error{}
	}
	if Board.IsPlacementLegalCache[Player] == nil {
		Board.IsPlacementLegalCache[Player] = map[bool]map[Coord]error{}
	}
	if Board.IsPlacementLegalCache[Player][EditMode] == nil {
		Board.IsPlacementLegalCache[Player][EditMode] = map[Coord]error{}
	}
	Board.IsPlacementLegalCache[Player][EditMode][Coordinate] = Err
}

// CheckCoordToGroupCache returns the cached *Group for Coordinate and true,
// or nil and false on a cache miss.
func (Board *BoardState) CheckCoordToGroupCache(Coordinate Coord) (*Group, bool) {
	if Board.CoordToGroupCache == nil {
		Board.CoordToGroupCache = map[Coord]*Group{}
	}
	FoundGroup, Exists := Board.CoordToGroupCache[Coordinate]
	return FoundGroup, Exists
}

// SetCoordToGroupCache stores a coord->group mapping in the cache.
// Pass nil group for empty/not-found coords.
func (Board *BoardState) SetCoordToGroupCache(Coordinate Coord, FoundGroup *Group) {
	if Board.CoordToGroupCache == nil {
		Board.CoordToGroupCache = map[Coord]*Group{}
	}
	Board.CoordToGroupCache[Coordinate] = FoundGroup
}

// VerticesEqual compares two Vertices maps for equality, considering only
// player bits (ignoring annotation bits).
func VerticesEqual(A, B map[Coord]uint8) bool {
	if len(A) != len(B) {
		return false
	}
	for Coordinate, ValueA := range A {
		ValueB, Exists := B[Coordinate]
		if !Exists || (ValueA&PlayerAtVertexMask) != (ValueB&PlayerAtVertexMask) {
			return false
		}
	}
	return true
}

// CopyVertices deep-copies a Vertices map. When KeepAnnotations is false,
// strips annotation bits (copies player bits only). When true, copies full byte.
func CopyVertices(Vertices map[Coord]uint8, KeepAnnotations bool) map[Coord]uint8 {
	Copied := make(map[Coord]uint8, len(Vertices))
	if KeepAnnotations {
		maps.Copy(Copied, Vertices)
	} else {
		for Coordinate, Value := range Vertices {
			Copied[Coordinate] = Value & PlayerAtVertexMask
		}
	}
	return Copied
}

// ConvertGroup changes all stones in TargetGroup to TargetPlayer's color
// (or deletes them if TargetPlayer == 0). Updates ConversionsTo/ConversionsFrom
// when CaptureConverts is true, or Prisoners when CaptureConverts is false.
func ConvertGroup(Board *BoardState, TargetGroup *Group, TargetPlayer uint8, CapturingPlayer uint8) {
	StoneCount := len(TargetGroup.Vertices[TargetGroup.Owner])
	CaptureConverts := Board.Hist.RuleSet != nil && Board.Hist.RuleSet.CaptureConverts
	IsSuicide := TargetGroup.Owner == CapturingPlayer
	if TargetPlayer == 0 {
		// Removal
		Board.RemoveGroup(TargetGroup)
		if IsSuicide {
			Board.StoneSuicides[TargetGroup.Owner] += StoneCount
		} else {
			Board.Prisoners[CapturingPlayer] += StoneCount
		}
	} else if CaptureConverts {
		// Conversion: change stone colors
		for _, Stone := range TargetGroup.Vertices[TargetGroup.Owner] {
			Board.SetPlayerAt(Stone, TargetPlayer)
		}
		if IsSuicide {
			Board.StoneSuicides[TargetGroup.Owner] += StoneCount
		}
		Board.ConversionsTo[TargetPlayer] += StoneCount
		Board.ConversionsFrom[TargetGroup.Owner] += StoneCount
	}
}

// FindPreviousGtnWithPlacementByPlayer walks up the game tree from Gtn to
// find the most recent node where PreviousStonePlacer == Player.
// Returns that node, or nil if not found.
func FindPreviousGtnWithPlacementByPlayer(Gtn *GameTreeNode, Player uint8) *GameTreeNode {
	Current := Gtn.Parent
	for Current != nil {
		if Current.PreviousStonePlacer == Player {
			return Current
		}
		Current = Current.Parent
	}
	return nil
}

// StrongestRepetitionRule returns the strongest repetition-scope flag enabled
// in Flags. Priority (strongest first):
// Positional > Situational > NaturalSituational > BasicKo. Returns 0 if none
// of those flags are set. NonRepetitionRuleNoResult is a policy modifier and
// is handled separately by callers.
func StrongestRepetitionRule(Flags uint) uint {
	switch {
	case Flags&NonRepetitionRulePositionalSuperKo != 0:
		return NonRepetitionRulePositionalSuperKo
	case Flags&NonRepetitionRuleSituationalSuperKo != 0:
		return NonRepetitionRuleSituationalSuperKo
	case Flags&NonRepetitionRuleNaturalSituationalSuperKo != 0:
		return NonRepetitionRuleNaturalSituationalSuperKo
	case Flags&NonRepetitionRuleBasicKo != 0:
		return NonRepetitionRuleBasicKo
	}
	return 0
}

// FindRepeatedAncestor walks ancestors of Gtn comparing each Board.Vertices
// against SimBoard.Vertices under the semantics of Rule. Rule must be one of
// NonRepetitionRuleBasicKo, NonRepetitionRuleNaturalSituationalSuperKo,
// NonRepetitionRuleSituationalSuperKo, or NonRepetitionRulePositionalSuperKo.
// Returns the matching ancestor, or nil if none matches.
func FindRepeatedAncestor(Gtn *GameTreeNode, SimBoard *BoardState, Player uint8, Rule uint) *GameTreeNode {
	Current := Gtn.Parent
	for Current != nil {
		if Current.Board != nil && Current.PreviousStonePlacer != 0 {
			Consider := false
			switch Rule {
			case NonRepetitionRuleBasicKo:
				// Only compare the first same-player ancestor, and only if it was a placement.
				if Current.PreviousStonePlacer == Player {
					if Current.LastMove != PassCoords &&
						VerticesEqual(Current.Board.Vertices, SimBoard.Vertices) {
						return Current
					}
					return nil
				}
			case NonRepetitionRuleNaturalSituationalSuperKo:
				Consider = Current.PreviousStonePlacer == Player && Current.LastMove != PassCoords
			case NonRepetitionRuleSituationalSuperKo:
				Consider = Current.PreviousStonePlacer == Player
			case NonRepetitionRulePositionalSuperKo:
				Consider = true
			}
			if Consider && VerticesEqual(Current.Board.Vertices, SimBoard.Vertices) {
				return Current
			}
		}
		Current = Current.Parent
	}
	return nil
}

// AttemptMove tries to place Player's stone at Coordinate. When EditMode is
// true, existing-stone checks are skipped (any in-bounds non-suicide
// placement is legal), but captures are still resolved. When DryRun is true,
// it only checks legality and returns (nil, nil) if legal or (nil, error) if
// illegal. When DryRun is false, it applies the move and returns a new
// BoardState with captures resolved and pass counter incremented
// for PassCoords. Returns (nil, error) if illegal. Legality results are cached.
func (Board *BoardState) AttemptMove(Coordinate Coord, Player uint8, EditMode bool, DryRun bool) (*BoardState, error) {
	if Board == nil {
		return nil, fmt.Errorf("Board does not exist!")
	}

	// --- Legality check (with caching) ---
	CachedErr, CacheHit := Board.CheckIsPlacementLegalCache(Player, EditMode, Coordinate)
	if CachedErr != nil {
		return nil, CachedErr
	}

	// SimBoard is built once during the legality check (cache miss)
	// and reused by the apply section via this closure.
	var SimBoard *BoardState

	CaptureConverts := Board.Hist.RuleSet != nil && Board.Hist.RuleSet.CaptureConverts

	// DetermineConversionTarget checks whether TargetGroup should be
	// converted/captured. Returns (TargetPlayer, true) if so, or (0, false)
	// if no conversion/capture should occur.
	DetermineConversionTarget := func(TargetGroup *Group) (uint8, bool) {
		if len(TargetGroup.Vertices[0]) > 0 {
			// Group has liberties — no conversion/capture
			return 0, false
		}
		if !CaptureConverts {
			// Standard capture: convert to empty (removal)
			return 0, true
		}
		// CaptureConverts: find distinct non-zero neighbor colors
		var UniqueNeighborPlayer uint8
		UniqueCount := 0
		for NeighborPlayer := range TargetGroup.Vertices {
			if NeighborPlayer == 0 || NeighborPlayer == TargetGroup.Owner {
				continue
			}
			UniqueCount++
			UniqueNeighborPlayer = NeighborPlayer
			if UniqueCount > 1 {
				return 0, false
			}
		}
		if UniqueCount == 1 {
			return UniqueNeighborPlayer, true
		}
		return 0, false
	}

	BuildSimBoard := func() {
		SimBoard = Board.CopyWithoutGroups(EditMode)
		SimBoard.SetPlayerAt(Coordinate, Player)
		SimBoard.CalculateAllGroups()
	}

	// RunCascade executes the capture/conversion cascade algorithm on SimBoard.
	RunCascade := func() {
		for {
			OuterChanged := false

			// Inner loop: Player's stones are implicitly immortal/nonconvertible
			for {
				// Step 2: Find all NON-Player groups with determinable conversion target
				ToConvert := map[*Group]uint8{}
				for TargetGroup := range SimBoard.Groups {
					if TargetGroup.Owner == Player {
						continue
					}
					TargetPlayer, ShouldConvert := DetermineConversionTarget(TargetGroup)
					if ShouldConvert {
						ToConvert[TargetGroup] = TargetPlayer
					}
				}
				if len(ToConvert) == 0 {
					break
				}

				// Step 3: Convert each identified group
				OuterChanged = true
				for TargetGroup, TargetPlayer := range ToConvert {
					ConvertGroup(SimBoard, TargetGroup, TargetPlayer, Player)
				}

				// Step 4: Recalculate all groups
				SimBoard.CalculateAllGroups()
			}

			// Step 6: Find Player-owned groups with determinable conversion target
			ToConvertPlayer := map[*Group]uint8{}
			for TargetGroup := range SimBoard.Groups {
				if TargetGroup.Owner != Player {
					continue
				}
				TargetPlayer, ShouldConvert := DetermineConversionTarget(TargetGroup)
				if ShouldConvert {
					ToConvertPlayer[TargetGroup] = TargetPlayer
				}
			}
			for TargetGroup, TargetPlayer := range ToConvertPlayer {
				ConvertGroup(SimBoard, TargetGroup, TargetPlayer, Player)
			}
			if len(ToConvertPlayer) > 0 {
				OuterChanged = true
				SimBoard.CalculateAllGroups()
			}

			if !OuterChanged {
				break
			}
		}
	}

	if !CacheHit {
		// Passing is never illegal
		if Coordinate != PassCoords {
			if !Board.Hist.InBounds(Coordinate) {
				Board.SetIsPlacementLegalCache(Player, EditMode, Coordinate, NotOnBoardErr)
				return nil, NotOnBoardErr
			}
			if !EditMode {
				ExistingStone := Board.GetPlayerAt(Coordinate)
				if 0 != ExistingStone {
					// ExistingStoneErr is derived from the Board's current player
					// bits, which can be mutated by Set Vertex delete. Caching it
					// would leave a stale "illegal" verdict for a cell that later
					// becomes empty, surfacing as a spurious illegal-move dot.
					return nil, ExistingStoneErr
				}
			}
			BuildSimBoard()
			RunCascade()

			if !EditMode {
				// Suicide check: placed stone must still be Player's color after cascade
				if SimBoard.GetPlayerAt(Coordinate) != Player {
					if Board.Hist.RuleSet == nil || !Board.Hist.RuleSet.SuicideIsLegal {
						Board.SetIsPlacementLegalCache(Player, EditMode, Coordinate, SuicideErr)
						return nil, SuicideErr
					}
				}

				// Repetition check. Strongest enabled rule wins (Positional ⊃
				// Situational ⊃ NaturalSituational ⊃ BasicKo). NoResult is a
				// policy modifier: if it is set AND no hard repetition rule
				// fires, a Positional repeat instead flags the resulting board
				// so the game ends in a draw.
				if Board.Hist.RuleSet != nil {
					Flags := Board.Hist.RuleSet.NonRepetitionRules
					Rule := StrongestRepetitionRule(Flags)
					if Rule != 0 {
						if FindRepeatedAncestor(Board.Hist.CurrentNode, SimBoard, Player, Rule) != nil {
							Board.SetIsPlacementLegalCache(Player, EditMode, Coordinate, RepetitionErr)
							return nil, RepetitionErr
						}
					}
				}
			}
		}
		Board.SetIsPlacementLegalCache(Player, EditMode, Coordinate, nil)
	}

	// Move is legal
	if DryRun {
		return nil, nil
	}

	// --- Apply the move ---
	if Coordinate == PassCoords {
		PassBoard := Board.CopyWithoutGroups(EditMode)
		PassBoard.Passes[Player]++
		PassBoard.Groups = Board.Groups
		return PassBoard, nil
	}

	// Reuse SimBoard if built during the legality check (cache miss);
	// build and run cascade now on a cache hit.
	if SimBoard == nil {
		BuildSimBoard()
		RunCascade()
	}

	// NoResult check: runs on every apply (cache hit or miss) so
	// SimBoard.NoResultTriggered is always set correctly.
	if Board.Hist.RuleSet != nil && Board.Hist.RuleSet.NonRepetitionRules&NonRepetitionRuleNoResult != 0 {
		if FindRepeatedAncestor(Board.Hist.CurrentNode, SimBoard, Player, NonRepetitionRulePositionalSuperKo) != nil {
			SimBoard.NoResultTriggered = true
		}
	}

	return SimBoard, nil
}

// CopyWithoutGroups returns a shallow copy of B with cloned Vertices,
// Prisoners, Passes, ConversionsTo, ConversionsFrom, but an empty Groups set.
// TODO always copy with groups? Do efficient recalculation when placing stone.
func (Board *BoardState) CopyWithoutGroups(KeepAnnotations bool) *BoardState {
	BoardCopy := &BoardState{
		Hist:                     Board.Hist,
		Vertices:                 CopyVertices(Board.Vertices, KeepAnnotations),
		Groups:                   map[*Group]struct{}{},
		Prisoners:                maps.Clone(Board.Prisoners),
		Passes:                   maps.Clone(Board.Passes),
		StoneSuicides:            maps.Clone(Board.StoneSuicides),
		ConversionsTo:            maps.Clone(Board.ConversionsTo),
		ConversionsFrom:          maps.Clone(Board.ConversionsFrom),
		NextLibertySharingMatrix: Board.NextLibertySharingMatrix,
	}
	return BoardCopy
}

// Copy returns a deep copy of B, including all Groups.
func (Board *BoardState) Copy(KeepAnnotations bool) *BoardState {
	BoardCopy := Board.CopyWithoutGroups(KeepAnnotations)
	maps.Copy(BoardCopy.Groups, Board.Groups)
	return BoardCopy
}

// CalculateAllGroups recomputes the Groups set from scratch by scanning
// every board intersection and flood-filling ungrouped stones.
func (Board *BoardState) CalculateAllGroups() {
	Board.Groups = map[*Group]struct{}{}
	Board.CoordToGroupCache = nil
	for X := uint8(0); X < Board.Hist.Width; X++ {
		for Y := uint8(0); Y < Board.Hist.Height; Y++ {
			c := Coord{X, Y}
			player := Board.GetPlayerAt(c)
			if 0 < player && player <= Board.Hist.Players {
				if Board.GetGroupAtCoord(c) == nil {
					Board.Groups[Board.CalculateGroupAtCoord(c)] = struct{}{}
				}
			}
		}
	}
}

// NewBoardState creates and returns an empty BoardState linked to this
// GameHistory, with no stones and zeroed prisoner/pass/conversion counters.
func (GHist *GameHistory) NewBoardState() *BoardState {
	return &BoardState{
		Hist:                     GHist,
		Vertices:                 map[Coord]uint8{},
		Groups:                   map[*Group]struct{}{},
		Prisoners:                map[uint8]int{},
		Passes:                   map[uint8]int{},
		StoneSuicides:            map[uint8]int{},
		ConversionsTo:            map[uint8]int{},
		ConversionsFrom:          map[uint8]int{},
		NextLibertySharingMatrix: NewLibertySharingMatrix(GHist.Players, SharingRefused),
	}
}

// HalfIntegerMoku formats the komi value as a string for Sgf export and Gtp engines.
// For multi-player games, both only support 2-player komi, so we calculate
// the difference between player 2's komi and player 1's komi (player 1 always has 0 in such games).
func (GHist *GameHistory) HalfIntegerMoku() string {
	// Default to 0 if no komi is set
	if GHist.Komi == nil {
		return "0"
	}
	var KomiValue float64
	if Player2Komi, Exists := GHist.Komi[2]; Exists {
		if Player1Komi, Exists := GHist.Komi[1]; Exists {
			KomiValue = Player2Komi - Player1Komi
		} else {
			KomiValue = Player2Komi
		}
	} else {
		KomiValue = 0
	}
	MokuFmt := "%.1f"
	FloorKomi := math.Floor(KomiValue)
	if KomiValue == FloorKomi {
		MokuFmt = "%.0f"
	} else {
		KomiValue = FloorKomi + 0.5
	}
	return fmt.Sprintf(MokuFmt, KomiValue)
}
