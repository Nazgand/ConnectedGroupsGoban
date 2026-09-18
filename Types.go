package main

import (
	"bufio"
	"fmt"
	"image/color"
	"io"
	"os/exec"
	"reflect"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ScoreDetails holds the complete scoring breakdown for all players and
// all shown scoring methods. Used by the score table UI.
type ScoreDetails struct {
	LiveStones        map[uint8]int
	Territory         map[uint8]int // Chinese-style: live stones + empty territory controlled
	OffBoardPrisoners map[uint8]int
	OnBoardPrisoners  map[uint8]int
	StoneSuicides     map[uint8]int
	Passes            map[uint8]int
	ConversionsTo     map[uint8]int                  // Stones converted into this player's color
	ConversionsFrom   map[uint8]int                  // Stones converted away from this player's color
	Komi              map[uint8]float64              // Per-player komi
	Score             map[*RuleSet]map[uint8]float64 // rule set → player → score
	ActiveRuleSet     *RuleSet
	ShownRuleSets     []*RuleSet
}

// RuleSet defines multipliers and flags that control how a game is scored.
// Each multiplier is applied to its corresponding count and added to the
// territory total. For example, Chinese rules zero out all multipliers
// (territory-only), while Japanese uses LiveStones=-1, Prisoners=1.
type RuleSet struct {
	Names                                  []string
	LiveStones, Passes, StoneSuicides      float64
	Conversions                            float64
	Prisoners                              float64
	LastPlayerMustPassLast, SuicideIsLegal bool
	CaptureConverts                        bool
	NonRepetitionRules                     uint
	StonePlacementsPerMove                 uint8
}

// Equal returns true if two RuleSets have identical contents, including Names.
func (A *RuleSet) Equal(B *RuleSet) bool {
	return reflect.DeepEqual(A, B)
}

// Theme holds the visual configuration for a single named theme, including
// the coordinate format string, general hex colors keyed by role name,
// per-player hex color strings, and display option flags for visual elements.
type Theme struct {
	CoordFmt            string            `json:"CoordFmt"`
	HexColors           map[string]string `json:"HexColors"`
	PlayerHexColors     map[uint8]string  `json:"PlayerHexColors"`
	ShowLibertyLine     bool              `json:"ShowLibertyLine"`
	ShowStoneConnection bool              `json:"ShowStoneConnection"`
	ShowIllegalDot      bool              `json:"ShowIllegalDot"`
	ShowHoverLiberties  bool              `json:"ShowHoverLiberties"`
	ShowChildNodeDots   bool              `json:"ShowChildNodeDots"`
}

// EngineConfig stores the path and arguments needed to launch a GTP engine
// process. Persisted in AppConfig.Engines keyed by a user-chosen name.
type EngineConfig struct {
	GtpPath string `json:"GtpPath"`
	GtpArgs string `json:"GtpArgs"`
}

// FreshBoardPreset stores the parameters used to create a new game board.
// Presets are persisted in AppConfig.FreshBoardPresets keyed by name.
// The "Default" preset is always present and cannot be deleted.
type FreshBoardPreset struct {
	Width                uint8                 `json:"Width"`
	Height               uint8                 `json:"Height"`
	Players              uint8                 `json:"Players"`
	Komi                 map[uint8]float64     `json:"Komi"`        // Per-player komi, player 1 always has 0 komi
	PlayerNames          map[uint8]string      `json:"PlayerNames"` // Per-player display names
	RuleSet              *RuleSet              `json:"RuleSet"`
	Information          map[string]string     `json:"Information"`
	WrapXMulY            int8                  `json:"WrapXMulY"`
	WrapYMulX            int8                  `json:"WrapYMulX"`
	WrapXShiftY          uint8                 `json:"WrapXShiftY"`
	WrapYShiftX          uint8                 `json:"WrapYShiftX"`
	LibertySharingFixed  bool                  `json:"LibertySharingFixed"`
	LibertySharingMatrix *LibertySharingMatrix `json:"LibertySharingMatrix"` // nil → filled with no-sharing default at use time
}

// GtpEngineState holds the runtime state for a single attached GTP engine
// process, including its stdin/stdout pipes, buffered reader, and optional
// log file handle.
type GtpEngineState struct {
	Cmd          *exec.Cmd
	In           io.WriteCloser
	Out          io.ReadCloser
	Reader       *bufio.Reader
	LogFile      io.WriteCloser // nil if logging disabled
	Name         string
	Thinking     bool       // UI-thread-only pending genmove flag
	CommandMutex sync.Mutex // serializes GTP commands and log cleanup
}

// AppConfig is the top-level configuration struct persisted as JSON.
// It contains engine definitions, board presets,
// and theme data.
type AppConfig struct {
	Themes                     map[string]Theme            `json:"Themes"`
	ActiveTheme                string                      `json:"ActiveTheme"`
	ShownProtectedRuleSetNames []string                    `json:"ShownProtectedRuleSetNames"`
	ShownCustomRuleSets        []RuleSet                   `json:"ShownCustomRuleSets"`
	HiddenCustomRuleSets       []RuleSet                   `json:"HiddenCustomRuleSets"`
	Engines                    map[string]EngineConfig     `json:"Engines"`
	FreshBoardPresets          map[string]FreshBoardPreset `json:"FreshBoardPresets"`
	ActivePresetName           string                      `json:"ActivePresetName"`
	PlayerNames                []string                    `json:"PlayerNames"`
	GtpLogging                 bool                        `json:"GtpLogging"`
	WindowWidth                float32                     `json:"WindowWidth"`
	WindowHeight               float32                     `json:"WindowHeight"`
	VSplitOffset               float64                     `json:"VSplitOffset"`
}

// ShownRuleSets returns the ordered list of rule sets to display in the score
// table: first the protected rule sets named in ShownProtectedRuleSetNames,
// then the ShownCustomRuleSets. Each entry is a pointer; protected names that
// don't match any ProtectedRuleSet are silently skipped.
func (Config *AppConfig) ShownRuleSets() []*RuleSet {
	Result := []*RuleSet{}
	for _, Name := range Config.ShownProtectedRuleSetNames {
		if RS := FindRuleSetByName(Name); RS != nil {
			Result = append(Result, RS)
		}
	}
	for Index := range Config.ShownCustomRuleSets {
		Result = append(Result, &Config.ShownCustomRuleSets[Index])
	}
	return Result
}

// AllCustomRuleSets returns pointers to every custom rule set (shown and
// hidden). Useful for presenting the full list of custom rule sets in UI
// dialogs such as new-game configuration.
func (Config *AppConfig) AllCustomRuleSets() []*RuleSet {
	Result := []*RuleSet{}
	for Index := range Config.ShownCustomRuleSets {
		Result = append(Result, &Config.ShownCustomRuleSets[Index])
	}
	for Index := range Config.HiddenCustomRuleSets {
		Result = append(Result, &Config.HiddenCustomRuleSets[Index])
	}
	return Result
}

// GetCoordFormat returns the current coordinate format from the active theme
func GetCoordFormat() string {
	if CurrentAppConfig.Themes != nil {
		if theme, ok := CurrentAppConfig.Themes[CurrentAppConfig.ActiveTheme]; ok {
			return theme.CoordFmt
		}
	}
	// Fallback to default if theme not found
	return CoordFmtHikaruNoGo
}

// SetCoordFormat sets the coordinate format in the active theme
func SetCoordFormat(coordFmt string) {
	if CurrentAppConfig.Themes != nil {
		if theme, ok := CurrentAppConfig.Themes[CurrentAppConfig.ActiveTheme]; ok {
			theme.CoordFmt = coordFmt
			CurrentAppConfig.Themes[CurrentAppConfig.ActiveTheme] = theme
		}
	}
}

// Coord represents a board coordinate with X (column) and Y (row) values.
// X=0xff and Y=0xff are reserved sentinel values used for label positions
// outside the board area.
type Coord struct {
	X, Y uint8
}

// Group represents a connected group of same-color stones on the board,
// along with its adjacent non-owner vertices grouped by player.
// Vertices[Owner] = the stones in this group.
// Vertices[0] = liberties (empty). Vertices[k] for k != Owner = adjacent stones of player k.
type Group struct {
	Owner    uint8
	Vertices map[uint8][]Coord
}

// LibertySharingMatrix[RowPlayer][ColumnPlayer] = how RowPlayer shares its
// groups' liberties with ColumnPlayer. Asymmetric: M[k][m] and M[m][k] are
// independent. Diagonal (k==m) is always SharingUnconditional. Index 0 is
// unused (player 0 is empty). Outer length = Players+1.
type LibertySharingMatrix [][]uint8

// BoardState represents the full state of the board at a single game tree
// node, including stone positions, annotation bits, captured prisoners,
// pass counts, conversion counts, group topology, and a legality cache.
type BoardState struct {
	Hist *GameHistory
	// Coord -> uint8; bits 0-3: player (0=empty,1=black,2=white); bits 4-7: annotation flags (XMask,SquareMask,CircleMask,TriangleMask)
	Vertices map[Coord]uint8
	// A set of every *Group on the board
	Groups map[*Group]struct{}
	// A cache map from stone coords to their groups
	CoordToGroupCache map[Coord]*Group
	// Prisoners taken by each player; key is player (1 or 2)
	Prisoners map[uint8]int
	// Stones lost to suicide by each player
	StoneSuicides map[uint8]int
	// Passes made by each player; key is player (1 or 2)
	Passes map[uint8]int
	// Stones converted TO each player's color (CaptureConverts only)
	ConversionsTo map[uint8]int
	// Stones converted FROM each player's color (CaptureConverts only)
	ConversionsFrom map[uint8]int
	// Player->EditMode->Coord->error
	IsPlacementLegalCache map[uint8]map[bool]map[Coord]error
	// NextLibertySharingMatrix is the matrix that governs the NEXT move played
	// from this board. The matrix that produced this board's Groups is whatever
	// was set at creation time and is not stored separately — Groups are never
	// retroactively recomputed. Child boards inherit this pointer via Copy /
	// CopyWithoutGroups; it is replaced only when the Diplomacy dialog commits
	// changes on the node holding this board.
	NextLibertySharingMatrix *LibertySharingMatrix
	// NoResultTriggered is set by AttemptMove when a move triggers a
	// NonRepetitionRuleNoResult positional repetition. The resulting
	// GameTreeNode copies this flag into its NoResultDraw field so the game
	// ends in a draw.
	NoResultTriggered bool
}

// GoErr is a lightweight error type backed by a shared string pointer,
// allowing error identity comparison via the pointer rather than string content.
type GoErr struct{ S *string }

// Error returns the error message string for this GoErr.
func (E GoErr) Error() string { return *E.S }

// Shared error values for move legality checks. Each pair consists of a
// string (for display) and a GoErr (for identity comparison).
var (
	NotOnBoardErrStr    = "Unable to place stone: Not on board"
	NotOnBoardErr       = GoErr{&NotOnBoardErrStr}
	ExistingStoneErrStr = "Unable to place stone: Existing stone"
	ExistingStoneErr    = GoErr{&ExistingStoneErrStr}
	SuicideErrStr       = "Unable to place stone: Suicide"
	SuicideErr          = GoErr{&SuicideErrStr}
	RepetitionErrStr    = "Unable to place stone: Board state repetition"
	RepetitionErr       = GoErr{&RepetitionErrStr}
)

// GameHistory holds per-game metadata: board dimensions, komi, rule set,
// player count, game information, and pointers to the root and
// current nodes of the game tree. One GameHistory exists per game in a
// Collection.
type GameHistory struct {
	Collection               *Collection
	Komi                     map[uint8]float64 // Per-player komi, player 1 always has 0 komi
	PlayerNames              map[uint8]string  // Per-player display names
	Information              map[string]*string
	RuleSet                  *RuleSet
	WrapXMulY, WrapYMulX     int8 // 0=no wrap; otherwise |value| ≤ Height/Width and gcd(|value|, Height/Width) == 1 so ModInverseInt16 exists (IsValidWrapMul). Applied forward as X|Y → X|Y * Mul + Shift (mod Dim) and reversed via the modular inverse.
	WrapXShiftY, WrapYShiftX uint8
	// Cached modular inverses of WrapXMulY mod Height and WrapYMulX mod Width.
	// 0 means "no usable inverse" (either Mul is 0 or the invariant was violated);
	// callers must gate on the corresponding Mul != 0 before using them.
	// Not serialized — recomputed by RecomputeWrapInverses() whenever any of
	// Width / Height / WrapXMulY / WrapYMulX is assigned.
	WrapXMulYInv           int16 `json:"-"`
	WrapYMulXInv           int16 `json:"-"`
	Width, Height, Players uint8
	RootNode, CurrentNode  *GameTreeNode
	// LibertySharingFixed is true when the liberty sharing matrix cannot change
	// mid-game. When true, the Game > Diplomacy menu item is disabled and every
	// node's Board.NextLibertySharingMatrix is identical. SGF and GTP games
	// always set this to true.
	LibertySharingFixed bool
}

// GameTreeNode is a single node in the game tree. It stores the board
// state at this point, the move that produced it, tree linkage (parent,
// children, favorite child), an optional comment, labels, and any
// unknown SGF properties preserved for round-trip fidelity.
type GameTreeNode struct {
	// Current state of the board at this node
	Board *BoardState
	// State of the territory at this node
	TerritoryMap map[Coord]uint8
	// Coordinates of the last move
	LastMove Coord
	// Player who made the last move
	PreviousStonePlacer uint8
	// Player who will make the next move
	NextStonePlacer uint8
	// Remaining stone placements in the current turn
	RemainingStonePlacements uint8
	// Child nodes representing subsequent moves
	Children []*GameTreeNode
	// The child that will be selected pressing the down arrow key
	FavoriteChild *GameTreeNode
	// Parent node in the game tree
	Parent *GameTreeNode
	// Unique identifier for the node
	Id int
	// Optional comment for the move
	Comment string
	// Labels for specific points on the board
	Labels map[Coord]string
	// Unknown/unsupported SGF properties stored verbatim for round-trip reproduction.
	// Key: property name, Value: list of raw value strings (original content between [ and ]).
	UnknownSgfProperties map[string][]string
	// Unix timestamp in milliseconds when this node was created.
	UnixMilli int64
	// NoResultDraw is true when this move triggered a positional repetition
	// under NonRepetitionRuleNoResult. Copied from Board.NoResultTriggered at
	// node creation. IsGameOver returns true for such nodes and the score
	// table displays "No Result (Draw)".
	NoResultDraw bool
}

// A collection of games
type Collection struct {
	NodeMap        map[int]*GameTreeNode // TODO Maybe []*GameTreeNode?
	NodeCounter    int
	CurrentGameH   *GameHistory
	CollectionNode *GameTreeNode
	// Raw SGF strings for non-Go game trees in the collection (reproduced verbatim on export)
	NonGoGameTrees []string
}

// GobanLayerKey captures the inputs that determine whether the Goban layer
// needs to be redrawn: container size and board dimensions.
type GobanLayerKey struct {
	Size          fyne.Size
	Width         uint8
	Height        uint8
	SymmetryIndex uint8
}

// A single window of this app
type GoWin struct {
	DialogShowing bool
	DismissDialog func()
	// SubmitDialog, when non-nil, is called by HandleKeyEvent on Return/Enter
	// to confirm the open dialog (unless a multi-line entry has focus).
	// Cleared by DialogClosed().
	SubmitDialog func()
	// ResizeDialog, when non-nil, is invoked from InputLayer's debounced
	// resize timer so window-sized dialogs (Game Information, Collection,
	// file dialogs, etc.) resize once after each window-drag settles. The
	// closure reads the latest canvas size itself — no parameter needed.
	// Dialog openers set it before Show() and clear it in their close callback.
	ResizeDialog   func()
	OpenedFilePath string
	Coll           *Collection
	PlayArea       *canvas.Rectangle
	PlayContainer  *fyne.Container
	// Layers stores each named drawing layer, keyed by the strings in LayerOrder.
	Layers                  map[string]*fyne.Container
	LibertyLayer            *fyne.Container
	HoverCoords             Coord
	HoverGroup              *Group
	LastGobanKey            GobanLayerKey
	Win                     fyne.Window
	CellSize                float32
	GameTreeContainer       *container.Scroll
	CurrentNodeGui          fyne.CanvasObject
	GameTreeNodeToButton    map[*GameTreeNode]*TreeNodeButton
	GameTreeNodeToContainer map[*GameTreeNode]*fyne.Container
	StatusLabel             *StatusBar
	ScoreTexts              [][]*canvas.Text
	ScoreContainer          *fyne.Container
	CurrentScoreDetails     ScoreDetails
	TooltipObjects          []fyne.CanvasObject
	CommentEntry            *widget.Entry
	GameSelectorSelect      *widget.Select
	MouseMode               string
	SetVertexPlayer         uint8
	SetVertexMenuItem       *fyne.MenuItem
	SetVertexMenuGameH      *GameHistory
	SetAnnotationMask       uint8
	SymmetryIndex           uint8
	SymmetryItems           []*fyne.MenuItem
	PlayerEngineItems       []*fyne.MenuItem
	GtpEngines              map[uint8]*GtpEngineState
	PassItem                *fyne.MenuItem
	DiplomacyItem           *fyne.MenuItem
	MainMenu                *fyne.MainMenu
	VSplit                  *container.Split
	HSplit                  *container.Split
}

// InputLayer is a transparent Fyne widget overlaid on the board canvas.
// It captures mouse move, click, and hover events and forwards them to
// the GoWin handler methods.
type InputLayer struct {
	widget.BaseWidget
	ResizeMutex sync.Mutex
	ResizeTimer *time.Timer
	Win         *GoWin
}

// InputLayerRenderer is the Fyne WidgetRenderer for InputLayer. It has
// no visual elements of its own; all drawing is done on the board layers.
type InputLayerRenderer struct {
	Layer *InputLayer
}

// PlayerColor holds the three NRGBA color variants used to render a
// single player's stones: Base (fully opaque), Hover (semi-transparent
// preview), and Territory (scoring overlay).
type PlayerColor struct {
	Base, Hover, Territory, Contrast color.NRGBA
}

// CreatePlayerColor derives a PlayerColor from a hex color string.
// Base is fully opaque (A=0xff), Hover is 40% opaque (A=0x64), and
// Territory is 74% opaque (A=0xbd).
func CreatePlayerColor(HexColor string) PlayerColor {
	var Base, Hover, Territory color.NRGBA
	Base, Err := HexColorToNRGBA(HexColor)
	if Err != nil {
		fmt.Println("Error parsing color: " + HexColor)
	}
	Hover = Base
	Territory = Base
	Base.A = 0xff
	Hover.A = 0x64
	Territory.A = 0xbd
	Contrast := ContrastColor(Base)
	return PlayerColor{Base: Base, Hover: Hover, Territory: Territory, Contrast: Contrast}
}
