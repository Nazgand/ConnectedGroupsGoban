package main

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"
)

// MakeChild creates a new GameTreeNode as a child of Parent (or as an orphan
// if Parent is nil), assigns it a unique Id, and registers it in the NodeMap.
func (Coll *Collection) MakeChild(Parent *GameTreeNode) *GameTreeNode {
	Coll.NodeCounter++

	// Determine initial RemainingStonePlacements from RuleSet
	InitialRemainingStonePlacements := uint8(1)
	if Parent != nil && Parent.Board != nil && Parent.Board.Hist != nil && Parent.Board.Hist.RuleSet != nil {
		InitialRemainingStonePlacements = Parent.Board.Hist.RuleSet.StonePlacementsPerMove
	}

	NewNode := &GameTreeNode{
		Id:                       Coll.NodeCounter,
		Labels:                   map[Coord]string{},
		Children:                 []*GameTreeNode{},
		Parent:                   Parent,
		UnknownSgfProperties:     map[string][]string{},
		UnixMilli:                time.Now().UnixMilli(),
		RemainingStonePlacements: InitialRemainingStonePlacements,
	}
	if Parent != nil {
		Parent.Children = append(Parent.Children, NewNode)
	}
	NewNode.LastMove = ErrorCoords
	Coll.NodeMap[NewNode.Id] = NewNode
	return NewNode
}

// GetNextStonePlacer returns the player whose turn it is after this node.
// If NextStonePlacer is explicitly set it is returned; otherwise the opponent
// of PreviousStonePlacer is returned via NextPlayer.
func (Gtn *GameTreeNode) GetNextStonePlacer() uint8 {
	if Gtn.NextStonePlacer != 0 {
		return Gtn.NextStonePlacer
	}
	return NextPlayer(Gtn.PreviousStonePlacer, Gtn.Board.Hist.Players)
}

// MakeChild creates a new child node of Gtn. If CopyBoard is true
// the parent's BoardState is deep-copied into the child. If KeepAnnotations
// is true, annotation bits and Labels are preserved in the copy.
func (Gtn *GameTreeNode) MakeChild(CopyBoard bool, KeepAnnotations bool) *GameTreeNode {
	Child := Gtn.Board.Hist.Collection.MakeChild(Gtn)
	if CopyBoard {
		Child.Board = Gtn.Board.Copy(KeepAnnotations)
		if KeepAnnotations {
			Child.Labels = maps.Clone(Gtn.Labels)
		}
	}
	return Child
}

// MakeSiblingCopyBoard creates a new node as a sibling of Gtn (child of
// Gtn.Parent), copying Gtn's board state but not its children.
// If KeepAnnotations is true, annotation bits and Labels are preserved in the copy.
func (Gtn *GameTreeNode) MakeSiblingCopyBoard(KeepAnnotations bool) *GameTreeNode {
	Sibling := Gtn.Board.Hist.Collection.MakeChild(Gtn.Parent)
	Sibling.Board = Gtn.Board.Copy(KeepAnnotations)
	if KeepAnnotations {
		Sibling.Labels = maps.Clone(Gtn.Labels)
	}
	return Sibling
}

// FavoriteChildPath returns the sequence of nodes from Root to the last
// descendant reached by following FavoriteChild (falling back to
// Children[0]) at each step. Path[0] is always Root.
func FavoriteChildPath(Root *GameTreeNode) []*GameTreeNode {
	Path := []*GameTreeNode{Root}
	Node := Root
	for len(Node.Children) > 0 {
		if Node.FavoriteChild != nil {
			Node = Node.FavoriteChild
		} else {
			Node = Node.Children[0]
		}
		Path = append(Path, Node)
	}
	return Path
}

// SelectNode makes this node the current node in its GameHistory and makes
// that GameHistory the active game in the Collection.
func (Gtn *GameTreeNode) SelectNode() {
	Gtn.Board.Hist.CurrentNode = Gtn
	Gtn.Board.Hist.Collection.CurrentGameH = Gtn.Board.Hist
}

// NewRootNode creates a new GameHistory with the given board dimensions,
// attaches a root GameTreeNode with an empty board, and adds it as a child
// of the collection node. The root node's NextStonePlacer is set to 1.
func (Coll *Collection) NewRootNode(Width, Height uint8) *GameTreeNode {
	h := &GameHistory{
		Collection:          Coll,
		Width:               Width,
		Height:              Height,
		Players:             2,
		Komi:                map[uint8]float64{1: 0, 2: 7.0}, // Default per-player komi
		PlayerNames:         map[uint8]string{},
		Information:         map[string]*string{},
		RuleSet:             &ProtectedRuleSets[0],
		LibertySharingFixed: true,
	}
	h.RootNode = Coll.MakeChild(Coll.CollectionNode)
	h.RootNode.Board = h.NewBoardState()
	h.RootNode.LastMove = RootCoords
	h.RootNode.NextStonePlacer = 1
	h.RootNode.RemainingStonePlacements = h.RuleSet.StonePlacementsPerMove
	h.CurrentNode = h.RootNode
	return h.RootNode
}

// NewCollection creates and returns a new empty Collection with an
// initialized NodeMap and a CollectionNode sentinel.
func NewCollection() *Collection {
	Coll := &Collection{}
	Coll.NodeMap = map[int]*GameTreeNode{}
	Coll.CollectionNode = Coll.MakeChild(nil)
	Coll.CurrentGameH = nil
	Coll.CollectionNode.LastMove = CollectionCoords
	return Coll
}

// GTP coordinates use letters A-H, J-T (I is skipped), and numbers from 1 upwards
func (GHist *GameHistory) ClientToGTPCoords(C Coord) (string, error) {
	if C == PassCoords {
		return "PASS", nil
	}
	if C.X >= GHist.Width || C.Y >= GHist.Height {
		return "", fmt.Errorf("coordinate out of range")
	}
	// Convert X to letter
	letterRunes := []rune("ABCDEFGHJKLMNOPQRSTUVWXYZ")
	lenLetterRunes := uint8(len(letterRunes)) //25
	if C.X >= lenLetterRunes {
		return "", fmt.Errorf("coordinate out of GTP range")
	}
	letter := string(letterRunes[C.X])
	// GTP coordinates have origin at lower-left corner
	number := GHist.Height - C.Y
	return fmt.Sprintf("%s%d", letter, number), nil
}

// GtpCoordToClientCoords converts a GTP coordinate string (e.g. "D4",
// "PASS") back to an internal Coord. Letters skip 'I' per GTP convention;
// numbers are bottom-origin. Returns ErrorCoords on parse failure.
func (GHist *GameHistory) GtpCoordToClientCoords(gtpCoord string) (Coord, error) {
	gtpCoord = strings.ToUpper(gtpCoord)
	if gtpCoord == "PASS" {
		return PassCoords, nil
	}
	if len(gtpCoord) < 2 {
		return ErrorCoords, fmt.Errorf("invalid GTP coordinate: %s", gtpCoord)
	}
	letter := gtpCoord[:1]
	numberStr := gtpCoord[1:]
	letterRunes := []rune("ABCDEFGHJKLMNOPQRSTUVWXYZ")
	c := ErrorCoords
	for i, r := range letterRunes {
		if string(r) == letter {
			c.X = uint8(i)
			break
		}
	}
	if c.X >= GHist.Width {
		return ErrorCoords, fmt.Errorf("invalid GTP coordinate letter `%s` in gtpCoord `%s`", letter, gtpCoord)
	}
	number, err := strconv.ParseUint(numberStr, 10, 8)
	if err != nil {
		return ErrorCoords, err
	}
	c.Y = GHist.Height - uint8(number)
	if c.Y >= GHist.Height {
		return ErrorCoords, fmt.Errorf("invalid GTP coordinate number `%s` in gtpCoord `%s`", numberStr, gtpCoord)
	}
	return c, nil
}
