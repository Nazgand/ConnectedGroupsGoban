// Constants and package-level variables for ConnectedGroupsGoban.
package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

const (
	// GridLineThickness is the fraction of a cell used for grid line stroke width.
	GridLineThickness = 0.154739
	// Version is the current application version string.
	Version = "3"
	// ApplicationNameNoVersion is the application name without the version suffix.
	ApplicationNameNoVersion = "ConnectedGroupsGoban"
	// ApplicationName is the full application name including the version suffix.
	ApplicationName = ApplicationNameNoVersion + Version
	// ConfigFile is the filename used for persisting AppConfig as JSON.
	ConfigFile = ApplicationName + ".config.json"
	// PlayerAtVertexMask isolates the player bits (0-3) from a Vertices value.
	PlayerAtVertexMask uint8 = 0x0F
	// XMask is the annotation bit for an X mark
	XMask uint8 = 0x10
	// SquareMask is the annotation bit for a square
	SquareMask uint8 = 0x20
	// CircleMask is the annotation bit for a circle
	CircleMask uint8 = 0x40
	// TriangleMask is the annotation bit for a triangle
	TriangleMask uint8 = 0x80
)

// Per-ordered-pair liberty sharing state. Stored as uint8 cells in LibertySharingMatrix.
// Zero value is SharingRefused so uninitialized matrices default to "no sharing".
const (
	SharingRefused       uint8 = iota // never shares with this player
	SharingConditional                // shares iff the other side is Conditional or Unconditional
	SharingUnconditional              // always shares with this player
)

// SharingStateNames maps each sharing state constant to the symbolic string
// used for both JSON serialization and full-length UI dropdowns.
var SharingStateNames = map[uint8]string{
	SharingRefused:       "No",
	SharingConditional:   "Conditionally",
	SharingUnconditional: "Yes",
}

// SharingStateByName is the reverse of SharingStateNames. Unknown names fall
// back to SharingRefused at the call site.
var SharingStateByName = map[string]uint8{
	"No":            SharingRefused,
	"Conditionally": SharingConditional,
	"Yes":           SharingUnconditional,
}

// SharingStateOrder lists the dropdown options in display order.
var SharingStateOrder = []string{"No", "Conditionally", "Yes"}

// SharingStateShortNames maps each sharing state to a single-letter label used
// by the compact N×N matrix editor: N (No), C (Conditionally), Y (Yes).
var SharingStateShortNames = map[uint8]string{
	SharingRefused:       "N",
	SharingConditional:   "C",
	SharingUnconditional: "Y",
}

// SharingStateByShortName is the reverse of SharingStateShortNames.
var SharingStateByShortName = map[string]uint8{
	"N": SharingRefused,
	"C": SharingConditional,
	"Y": SharingUnconditional,
}

// SharingStateShortOrder lists the compact dropdown options in order.
var SharingStateShortOrder = []string{"N", "C", "Y"}

// Coordinate format identifiers used by GetCoordFormat/SetCoordFormat and
// the theme's CoordFmt field.
const (
	// CoordFmtGtp uses GTP-style letters (A-T skipping I) and bottom-origin numbers.
	CoordFmtGtp = "GTP"
	// CoordFmtComputer uses 0-indexed "X,Y" with Y=0 at the top.
	CoordFmtComputer = "Computer"
	// CoordFmtHikaruNoGo uses 1-indexed "XのY" (Japanese-style).
	CoordFmtHikaruNoGo = "HikaruNoGo"
	// CoordFmt1stQuadrant uses first-quadrant math notation "(X,Y)" with Y=0 at the bottom.
	CoordFmt1stQuadrant = "1stQuadrant"
	// CoordFmtSgf uses SGF-style lowercase/uppercase letter pairs (a-z, A-Z).
	CoordFmtSgf = "Sgf"
)

const (
	NonRepetitionRuleBasicKo = uint(1) << iota
	NonRepetitionRuleNaturalSituationalSuperKo
	NonRepetitionRuleSituationalSuperKo
	NonRepetitionRulePositionalSuperKo
	NonRepetitionRuleNoResult
)

var (
	// SgfToDescription maps SGF property codes to human-readable description
	// strings used as keys in GameHistory.Information.
	SgfToDescription = map[string]string{
		"KM": "Komi",
		"AP": "Original Application",
		"CA": "RFC 1345 Character Set",
		"FF": "File Format",
		"GM": "Game Code",
		"BR": "Rank Of Player 1",
		"WR": "Rank Of Player 2",
		"DT": "Date And Time",
		"EV": "Event",
		"RU": "Rules",
		"HA": "Handicap",
		"RE": "Result",
		"GC": "Game Comment",
		"AN": "Annotator",
		"BT": "Team Of Player 1",
		"WT": "Team Of Player 2",
		"CP": "Copyright",
		"GN": "Game Name",
		"ON": "Opening Information",
		"OT": "Overtime Method",
		"PC": "Place",
		"RO": "Round",
		"SO": "Source Of Game",
		"TM": "Time Limit Per Player (Seconds)",
		"US": "User Who Made The Game Available On Computer",
		"AW": "Set Coordinate As Stone Of Player 2",
		"AE": "Set Coordinate As Empty",
		"AB": "Set Coordinate As Stone Of Player 1",
		"CR": "Mark Coordinate With Circle",
		"SQ": "Mark Coordinate With Square",
		"TR": "Mark Coordinate With Triangle",
		"MA": "Mark Coordinate With X",
		"PL": "Set Next Player",
		"W":  "Move Of Player 2",
		"B":  "Move Of Player 1",
		"C":  "Comment",
		"LB": "Label Coordinate",
	}
	// DescriptionToSgf is the inverse of SgfToDescription: description string → SGF property key.
	// Populated by init().
	DescriptionToSgf map[string]string
	// RootInfoPropertyOrder defines the emit order for root SGF info properties
	// on export, excluding FF, GM, CA, AP, SZ, and KM which are always emitted first.
	RootInfoPropertyOrder = []string{
		"RU", "DT", "BR", "WR", "BT", "WT", "HA",
		"TM", "OT", "RE", "EV", "GN", "RO", "PC", "AN", "GC", "CP", "SO", "ON", "US",
	}
	// InformationRowOrder is the display order for rows in the Game Information
	// dialog. Keys are the human-readable description strings used in
	// GameHistory.Information. "Rules" is intentionally omitted — the active
	// rule set is shown read-only at the top of the dialog.
	InformationRowOrder = []string{
		"Game Name", "Result", "Handicap",
		"Date And Time",
		"Rank Of Player 1", "Rank Of Player 2",
		"Team Of Player 1", "Team Of Player 2",
		"Time Limit Per Player (Seconds)", "Overtime Method",
		"Event", "Round", "Place",
		"Annotator", "Game Comment", "Copyright",
		"Source Of Game", "Opening Information",
		"User Who Made The Game Available On Computer",
	}
)

// StatusBarPadY is the vertical padding above and below status bar text.
const StatusBarPadY = float32(-3.9)

// ScoreDividerWidth is the pixel width of dividers in the score table.
const ScoreDividerWidth = 1

// ScoreCellPad is the pixel padding around each score-table cell (horizontal and vertical).
const ScoreCellPad = 1

// ScoreTextSize is the score-table text size in display pixels.
const ScoreTextSize = float32(9.3)

// Score-table palette colors used by rowColorsForMethod and updateScoreTable.
var (
	// ScoreColorLightBlue is used for player header and active-method rows.
	ScoreColorLightBlue = color.NRGBA{R: 0xad, G: 0xd8, B: 0xe6, A: 0xff}
	// ScoreColorGreen marks rows whose multiplier adds to the score.
	ScoreColorGreen = color.NRGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff}
	// ScoreColorRed marks rows whose multiplier subtracts from the score.
	ScoreColorRed = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	// ScoreColorYellow marks rows with a zero multiplier (no contribution).
	ScoreColorYellow = color.NRGBA{R: 0xff, G: 0xff, B: 0x00, A: 0xff}
	// ScoreColorBlack is the score table background color.
	ScoreColorBlack = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
)

// ProtectedRuleSets lists the built-in rule sets. Each RuleSet carries a
// Names slice; Names[0] is the display name. Additional entries are aliases
// recognized during SGF import (e.g. "NZ" → "New Zealand").
// Chinese is index 0 and used as the default.
var ProtectedRuleSets = []RuleSet{
	{
		Names:                  []string{"Chinese"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRulePositionalSuperKo,
	},
	{
		Names:                  []string{"Japanese"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRuleBasicKo | NonRepetitionRuleNoResult,
		LiveStones:             -1,
		Prisoners:              1,
		StoneSuicides:          -1,
	},
	{
		Names:                  []string{"British Go Association", "BGA"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRuleNaturalSituationalSuperKo,
		LastPlayerMustPassLast: true,
	},
	{
		Names:                  []string{"American Go Association", "AGA"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRuleSituationalSuperKo,
		LiveStones:             -1,
		Prisoners:              1,
		StoneSuicides:          -1,
		Passes:                 -1,
		LastPlayerMustPassLast: true,
	},
	{
		Names:                  []string{"New Zealand", "NZ"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRuleSituationalSuperKo,
		SuicideIsLegal:         true,
	},
	{
		Names:                  []string{"Korean"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRuleBasicKo | NonRepetitionRuleNoResult,
		LiveStones:             -1,
		Prisoners:              1,
		StoneSuicides:          -1,
	},
	{
		Names:                  []string{"Nazgand 3 Stone"},
		StonePlacementsPerMove: 3,
		NonRepetitionRules:     NonRepetitionRuleNaturalSituationalSuperKo,
		LiveStones:             3.91547,
		Prisoners:              5.13974,
		StoneSuicides:          -1.547,
		Passes:                 -0.404,
		LastPlayerMustPassLast: true,
		SuicideIsLegal:         true,
	},
	{
		/* Because both CaptureConverts and SuicideIsLegal are true,
		 * this RuleSet needs more than 2 players to be non-trivial.
		 */
		Names:                  []string{"Nazgand Convert"},
		StonePlacementsPerMove: 1,
		NonRepetitionRules:     NonRepetitionRuleNaturalSituationalSuperKo,
		LiveStones:             9.3,
		Conversions:            1.5,
		Passes:                 -0.404,
		LastPlayerMustPassLast: true,
		CaptureConverts:        true,
		SuicideIsLegal:         true,
	},
}

// NonRepetitionRuleFlags maps display names to bitmask flags for the
// NonRepetitionRules field of RuleSet.
var NonRepetitionRuleFlags = []struct {
	Name string
	Flag uint
}{
	{"Basic Ko", NonRepetitionRuleBasicKo},
	{"Natural Situational Super Ko", NonRepetitionRuleNaturalSituationalSuperKo},
	{"Situational Super Ko", NonRepetitionRuleSituationalSuperKo},
	{"Positional Super Ko", NonRepetitionRulePositionalSuperKo},
	{"No Result", NonRepetitionRuleNoResult},
}

// Export-mode labels for the HandleExportImage form's Mode selector.
const (
	ExportModeSingleFrame = "Single frame (SVG)"
	ExportModeAnimated    = "Animated SVG (entire game)"
)

// CggSaveGameFormat is the SaveGameFormat string written into every CGG JSON file.
const CggSaveGameFormat = "ConnectedGroupsGoban3"

// ZstdMaxLevel is the integer level exposed in the dialog slider for Zstd.
// The zstd spec's maximum standard compression level is 22.
// Note: github.com/klauspost/compress/zstd is a pure-Go implementation; its
// EncoderLevelFromZstd maps all levels >=10 to SpeedBestCompression.
const ZstdMaxLevel = 22

// NonRepetitionRuleFlagToName maps each NonRepetitionRule flag to its CGG JSON token.
var NonRepetitionRuleFlagToName = map[uint]string{
	NonRepetitionRuleBasicKo:                   "BasicKo",
	NonRepetitionRuleNaturalSituationalSuperKo: "NaturalSituationalSuperKo",
	NonRepetitionRuleSituationalSuperKo:        "SituationalSuperKo",
	NonRepetitionRulePositionalSuperKo:         "PositionalSuperKo",
	NonRepetitionRuleNoResult:                  "NoResult",
}

// NonRepetitionRuleNameToFlag is the reverse of NonRepetitionRuleFlagToName.
var NonRepetitionRuleNameToFlag = map[string]uint{
	"BasicKo":                   NonRepetitionRuleBasicKo,
	"NaturalSituationalSuperKo": NonRepetitionRuleNaturalSituationalSuperKo,
	"SituationalSuperKo":        NonRepetitionRuleSituationalSuperKo,
	"PositionalSuperKo":         NonRepetitionRulePositionalSuperKo,
	"NoResult":                  NonRepetitionRuleNoResult,
}

// CggMethodToExtension maps each CGG export compression method to its file suffix.
var CggMethodToExtension = map[string]string{
	"Kanzi":     ".CGG.json.kanzi",
	"Gzip":      ".CGG.json.gzip",
	"Zstd":      ".CGG.json.zstd",
	"Plaintext": ".CGG.json",
}

// CggAllSuffixes lists all supported CGG file suffixes for the open dialog.
var CggAllSuffixes = []string{".CGG.json", ".CGG.json.gzip", ".CGG.json.kanzi", ".CGG.json.zstd"}

// SymmetryNames lists the 8 board symmetries in the order selected by SymmetryIndex.
var SymmetryNames = []string{
	"Identity",
	"Rotate 90° CW",
	"Rotate 180°",
	"Rotate 270° CW",
	"Flip Horizontal",
	"Flip Horizontal + Rotate 90° CW",
	"Flip Vertical",
	"Transpose",
}

// SvgDefsBlock is the shared <defs> block emitted by both the static
// and animated SVG builders. Defines FourCirclesMask and the
// SquareMinusFourCircles symbol used by the 3-meet stone-connection
// shape.
const SvgDefsBlock = `  <defs>
    <mask id="FourCirclesMask">
      <rect x="0" y="0" width="1" height="1" fill="white"/>
      <circle cx="0" cy="0" r="0.5" fill="black"/>
      <circle cx="0" cy="1" r="0.5" fill="black"/>
      <circle cx="1" cy="0" r="0.5" fill="black"/>
      <circle cx="1" cy="1" r="0.5" fill="black"/>
    </mask>
    <symbol id="SquareMinusFourCircles" viewBox="0 0 1 1">
      <rect x="0" y="0" width="1" height="1" mask="url(#FourCirclesMask)"/>
    </symbol>
  </defs>
`

// init builds the DescriptionToSgf reverse-lookup map from SgfToDescription.
func init() {
	DescriptionToSgf = make(map[string]string, len(SgfToDescription))
	for k, v := range SgfToDescription {
		DescriptionToSgf[v] = k
	}
}

var (
	// PassShortcut and DeleteNodeShortcut display unmodified keys in menus.
	// HandleKeyEvent continues to handle these keys outside text fields.
	PassShortcut       = &desktop.CustomShortcut{KeyName: fyne.KeyP}
	DeleteNodeShortcut = &desktop.CustomShortcut{KeyName: fyne.KeyDelete}
	// CtrlN is the Ctrl+N keyboard shortcut for New Game.
	CtrlN = &desktop.CustomShortcut{KeyName: fyne.KeyN,
		Modifier: fyne.KeyModifierControl}
	// CtrlO is the Ctrl+O keyboard shortcut for Import SGF.
	CtrlO = &desktop.CustomShortcut{KeyName: fyne.KeyO,
		Modifier: fyne.KeyModifierControl}
	// CtrlS is the Ctrl+S keyboard shortcut for Export SGF.
	CtrlS = &desktop.CustomShortcut{KeyName: fyne.KeyS,
		Modifier: fyne.KeyModifierControl}
	// CtrlE is the Ctrl+E keyboard shortcut for Engine Settings.
	CtrlE = &desktop.CustomShortcut{KeyName: fyne.KeyE,
		Modifier: fyne.KeyModifierControl}
	// CtrlQ is the Ctrl+Q keyboard shortcut for Detach All Engines.
	CtrlQ = &desktop.CustomShortcut{KeyName: fyne.KeyQ,
		Modifier: fyne.KeyModifierControl}
	// CtrlG is the Ctrl+G keyboard shortcut for Go To Move #.
	CtrlG = &desktop.CustomShortcut{KeyName: fyne.KeyG,
		Modifier: fyne.KeyModifierControl}
	// CtrlD is the Ctrl+D keyboard shortcut for Game > Diplomacy.
	CtrlD = &desktop.CustomShortcut{KeyName: fyne.KeyD,
		Modifier: fyne.KeyModifierControl}
	// CtrlShiftO is the Ctrl+Shift+O keyboard shortcut for Import CGG.
	CtrlShiftO = &desktop.CustomShortcut{KeyName: fyne.KeyO,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}
	// CtrlShiftS is the Ctrl+Shift+S keyboard shortcut for Export CGG.
	CtrlShiftS = &desktop.CustomShortcut{KeyName: fyne.KeyS,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}
	// ConfigDir is the directory path where the config file is stored.
	ConfigDir string
	// CurrentAppConfig is the global, mutable application configuration.
	CurrentAppConfig AppConfig
	// CollectionCoords is the sentinel LastMove value for the collection root node.
	CollectionCoords = Coord{X: 0xff, Y: 1}
	// RootCoords is the sentinel LastMove value for a game's root node.
	RootCoords = Coord{X: 0xff, Y: 2}
	// ErrorCoords is the sentinel Coord indicating an invalid or uninitialized coordinate.
	ErrorCoords = Coord{X: 0xff, Y: 3}
	// PassCoords is the sentinel Coord representing a pass move.
	PassCoords = Coord{X: 0xff, Y: 0xff}
	// EditCoords is the sentinel LastMove value for a board-editing (setup) node.
	EditCoords = Coord{X: 0xff, Y: 4}
	// SgfPropertyToAnnotationMask maps SGF annotation property codes (CR, SQ, TR, MA)
	// to their corresponding Vertices bitmask values.
	SgfPropertyToAnnotationMask = map[string]uint8{
		"CR": CircleMask, "SQ": SquareMask, "TR": TriangleMask, "MA": XMask,
	}
	// AnnotationMaskToSgfProperty is the reverse lookup from annotation bitmask
	// to SGF property code, used during SGF export.
	AnnotationMaskToSgfProperty = map[uint8]string{
		CircleMask:   "CR",
		SquareMask:   "SQ",
		TriangleMask: "TR",
		XMask:        "MA",
	}
	// MouseModes lists all available mouse interaction modes in menu order.
	MouseModes = []string{
		"Play", "Score", "Set Label", "Set Vertex", "Toggle Annotation"}
	// SquareMinusFourCircles caches generated PNG resources for stone-connection
	// images, keyed by the player's base NRGBA color.
	SquareMinusFourCircles = map[color.NRGBA]*fyne.StaticResource{}
	// SquareMinusFourCirclesSize tracks the pixel size of each cached PNG
	// so it can be regenerated when the cell size changes.
	SquareMinusFourCirclesSize = map[color.NRGBA]int{}
	// PlayerColors maps player number to its derived PlayerColor (Base/Hover/Territory).
	PlayerColors = map[uint8]PlayerColor{}
	// DefaultConfig provides factory-default values for every AppConfig field.
	// It is used as the starting point for LoadConfig and as a fallback.
	DefaultConfig = AppConfig{
		Themes: map[string]Theme{
			"Default": {
				CoordFmt: CoordFmtHikaruNoGo,
				HexColors: map[string]string{
					"Territory Stroke":       "#00efef",
					"Annotation":             "#8da6ac",
					"Coord":                  "#475356",
					"Coord Background":       "#8da6ac",
					"Coord Hover":            "#8da6ac",
					"Coord Hover Background": "#475356",
					"Illegal":                "#ff0000",
					"Delete":                 "#ff000080",
					"Goban":                  "#776845",
					"Goban Line":             "#eed18b",
					"1-Liberty Line":         "#ff0000",
					"2-Liberty Line":         "#ff7f00",
					"3-Liberty Line":         "#ffff00",
					"4-Liberty Line":         "#7fff00",
					"≥5-Liberty Line":        "#00ff00",
					"Play Area":              "#475356",
					"Last Move":              "#800080",
					"Child Node":             "#ffff00",
					"Highlight":              "#ff69b463",
					"Neighbor":               "#4169e1",
				},
				PlayerHexColors: map[uint8]string{
					1:  "#000000",
					2:  "#ffffff",
					3:  "#6495ed",
					4:  "#ff00ff",
					5:  "#228b22",
					6:  "#ffff54",
					7:  "#8b4513",
					8:  "#ff0000",
					9:  "#00ff00",
					10: "#0000ff",
					11: "#4b0082",
					12: "#ff69b4",
					13: "#f5deb3",
					14: "#2f4f4f",
					15: "#00ffff",
				},
				ShowLibertyLine:     true,
				ShowStoneConnection: true,
				ShowIllegalDot:      true,
				ShowHoverLiberties:  true,
				ShowChildNodeDots:   true,
			},
		},
		ActiveTheme:                "Default",
		ShownProtectedRuleSetNames: []string{"Chinese", "Japanese", "American Go Association"},
		Engines: map[string]EngineConfig{
			"GnuGoLevel5": {GtpPath: "gnugo", GtpArgs: "--mode gtp --level 5 --chinese-rules"},
		},
		FreshBoardPresets: map[string]FreshBoardPreset{
			"Default": {
				Width:                19,
				Height:               19,
				Players:              2,
				Komi:                 map[uint8]float64{1: 0, 2: 7.0},
				PlayerNames:          map[uint8]string{},
				RuleSet:              &ProtectedRuleSets[0],
				Information:          map[string]string{},
				LibertySharingFixed:  true,
				LibertySharingMatrix: nil,
			},
		},
		ActivePresetName: "Default",
		PlayerNames:      []string{},
		GtpLogging:       false,
	}
	// RuleSetId assigns a temporary unique uint ID to every *RuleSet seen
	// since app start. Used to disambiguate rule sets that share a display name.
	RuleSetId     = map[*RuleSet]uint{}
	RuleSetNextId = uint(0)
	// Colors maps role names (e.g. "Goban", "3-Liberty Line") to their active NRGBA values,
	// populated by initColors from the current theme's HexColors.
	Colors = map[string]color.NRGBA{}
	// CoordFmt lists all available coordinate format identifiers for UI selection.
	CoordFmt = []string{
		CoordFmtGtp,
		CoordFmtComputer,
		CoordFmtHikaruNoGo,
		CoordFmt1stQuadrant,
		CoordFmtSgf,
	}
	// LastSavedConfigBytes holds the JSON-serialized form of the config as it
	// was last written to disk (or loaded from disk). SaveConfigSnapshot uses it
	// to skip redundant writes when nothing has changed.
	LastSavedConfigBytes []byte
	// LayerOrder lists the map keys for GoWin.Layers in the order they are drawn,
	// from bottom (index 0) to top (last index).
	LayerOrder = []string{
		"Goban",
		"LibertyLine",
		"IllegalDot",
		"StoneConnection",
		"Stone",
		"LastMoveHighlight",
		"Annotation",
		"Label",
		"Territory",
		"Highlights",
		"Hover",
	}
)
