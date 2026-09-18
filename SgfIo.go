package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

// PlayerToBW converts a player number (1=Black, 2=White) to the
// corresponding SGF/GTP letter string ("B" or "W"). Returns an error
// for any other value.
func PlayerToBW(P uint8) (string, error) {
	switch P {
	case 1:
		return "B", nil
	case 2:
		return "W", nil
	default:
		return "", fmt.Errorf("Player not 1 or 2!")
	}
}

// BWToPlayer converts an SGF/GTP color string ("B" or "W") to the
// corresponding player number (1 or 2). Returns an error for any other value.
func BWToPlayer(S string) (uint8, error) {
	switch S {
	case "B":
		return 1, nil
	case "W":
		return 2, nil
	default:
		return 0, fmt.Errorf("Player not B or W!")
	}
}

// HandleImportSgf opens a file-open dialog filtered to .Sgf files, reads
// the selected file, and imports the SGF content into a new Collection.
func (GoWin *GoWin) HandleImportSgf() {
	if GoWin.DialogShowing {
		return
	} else {
		GoWin.DialogShowing = true
	}
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		GoWin.DialogClosed()
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		SgfContent, err := io.ReadAll(reader)
		if err != nil {
			GoWin.ShowError(err)
			return
		}
		GoWin.OpenedFilePath = reader.URI().Path()
		err = GoWin.ImportSgfContent(string(SgfContent))
		if err != nil {
			GoWin.ShowError(err)
		}
		GoWin.FixWindowTitle()
	}, GoWin.Win)
	if GoWin.OpenedFilePath != "" {
		fileDialog.SetFileName(GoWin.OpenedFilePath)
	}
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".Sgf"}))
	GoWin.ResizeDialog = func() { fileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	fileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	fileDialog.Show()
}

// HandleExportSgf opens a file-save dialog filtered to .Sgf files and
// writes the entire Collection (including non-Go game trees) to the
// chosen file in SGF format.
func (GoWin *GoWin) HandleExportSgf() {
	if GoWin.DialogShowing {
		return
	} else {
		GoWin.DialogShowing = true
	}
	fileDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		GoWin.DialogClosed()
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		var generateSgf func(node *GameTreeNode) (string, error)
		generateSgf = func(node *GameTreeNode) (string, error) {
			if node.LastMove == CollectionCoords {
				result := ""
				hasValidGames := false
				for _, child := range node.Children {
					if childSgf, err := generateSgf(child); err == nil {
						result += childSgf
						hasValidGames = true
					}
				}
				for _, raw := range GoWin.Coll.NonGoGameTrees {
					result += raw
					hasValidGames = true
				}
				if !hasValidGames {
					return "", fmt.Errorf("no valid games to export")
				}
				return result, nil
			}

			// Check board size and player count constraints first
			// SGF standard only supports 2-player games with coordinates up to 52x52
			if node.Board != nil && node.Board.Hist != nil {
				if node.Board.Hist.Players != 2 {
					return "", fmt.Errorf("game has %d players; SGF only supports 2-player games", node.Board.Hist.Players)
				}
				if node.Board.Hist.Width > 52 || node.Board.Hist.Height > 52 {
					return "", fmt.Errorf("board size %dx%d exceeds SGF maximum of 52x52", node.Board.Hist.Width, node.Board.Hist.Height)
				}
				if !node.Board.Hist.LibertySharingFixed {
					return "", fmt.Errorf("game uses a changeable liberty sharing matrix; SGF requires fixed no-sharing")
				}
				if node.Board.Hist.RootNode != nil && node.Board.Hist.RootNode.Board.NextLibertySharingMatrix.HasAnySharing() {
					return "", fmt.Errorf("game has liberty sharing between players; SGF requires no sharing")
				}
			}

			Sgf := "\n(;"

			if node.LastMove == RootCoords {
				hist := node.Board.Hist
				// FF and GM are always emitted as canonical values
				Sgf += "FF[4]\n"
				Sgf += "GM[1]\n"
				// CA: use stored value or default to UTF-8
				if v := SgfGetInformation(hist, "CA"); v != nil {
					Sgf += "CA[" + SgfEscapeText(*v) + "]\n"
				} else {
					Sgf += "CA[UTF-8]\n"
				}
				// AP: use stored value or default to current app
				if v := SgfGetInformation(hist, "AP"); v != nil {
					Sgf += "AP[" + *v + "]\n"
				} else {
					Sgf += "AP[" + ApplicationNameNoVersion + ":" + Version + "]\n"
				}
				// SZ
				if hist.Width == hist.Height {
					Sgf += fmt.Sprintf("SZ[%d]\n", hist.Width)
				} else {
					Sgf += fmt.Sprintf("SZ[%d:%d]\n", hist.Width, hist.Height)
				}
				// KM
				Sgf += "KM[" + hist.HalfIntegerMoku() + "]\n"
				// PB/PW from PlayerNames
				if Name, Ok := hist.PlayerNames[1]; Ok && Name != "" {
					Sgf += "PB[" + SgfEscapeText(Name) + "]\n"
				}
				if Name, Ok := hist.PlayerNames[2]; Ok && Name != "" {
					Sgf += "PW[" + SgfEscapeText(Name) + "]\n"
				}
				// All other root info properties in defined order
				for _, key := range RootInfoPropertyOrder {
					if v := SgfGetInformation(hist, key); v != nil {
						Sgf += key + "[" + SgfEscapeText(*v) + "]\n"
					}
				}
			} else if node.Board.Hist.InBounds(node.LastMove) || node.LastMove == PassCoords {
				BW, Err := PlayerToBW(node.PreviousStonePlacer)
				if Err != nil {
					// TODO stop export of just the current game in the Collection
				}
				Sgf += BW
				moveCoord, err := ConvertCoordToSgfCoord(node.LastMove)
				if err != nil {
					return "", fmt.Errorf("invalid move coordinate %v: %v", node.LastMove, err)
				}
				Sgf += "[" + moveCoord + "]"
			}

			// Comment
			if node.Comment != "" {
				Sgf += "C[" + SgfEscapeText(node.Comment) + "]"
			}

			// AW/AB/AE: computed from board state
			if node.Board != nil {
				switch node.LastMove {
				case RootCoords:
					var ab, aw []string
					for Y := uint8(0); Y < node.Board.Hist.Height; Y++ {
						for X := uint8(0); X < node.Board.Hist.Width; X++ {
							c := Coord{X, Y}
							switch node.Board.GetPlayerAt(c) {
							case 1:
								coord, err := ConvertCoordToSgfCoord(c)
								if err != nil {
									return "", fmt.Errorf("invalid AB coordinate %v: %v", c, err)
								}
								ab = append(ab, coord)
							case 2:
								coord, err := ConvertCoordToSgfCoord(c)
								if err != nil {
									return "", fmt.Errorf("invalid AW coordinate %v: %v", c, err)
								}
								aw = append(aw, coord)
							}
						}
					}
					if len(ab) > 0 {
						Sgf += "AB"
						for _, s := range ab {
							Sgf += "[" + s + "]"
						}
					}
					if len(aw) > 0 {
						Sgf += "AW"
						for _, s := range aw {
							Sgf += "[" + s + "]"
						}
					}
				case EditCoords:
					var ab, aw, ae []string
					for Y := uint8(0); Y < node.Board.Hist.Height; Y++ {
						for X := uint8(0); X < node.Board.Hist.Width; X++ {
							c := Coord{X, Y}
							cur := node.Board.GetPlayerAt(c)
							par := node.Parent.Board.GetPlayerAt(c)
							if cur == par {
								continue
							}
							switch cur {
							case 1:
								coord, err := ConvertCoordToSgfCoord(c)
								if err != nil {
									return "", fmt.Errorf("invalid AB coordinate %v: %v", c, err)
								}
								ab = append(ab, coord)
							case 2:
								coord, err := ConvertCoordToSgfCoord(c)
								if err != nil {
									return "", fmt.Errorf("invalid AW coordinate %v: %v", c, err)
								}
								aw = append(aw, coord)
							case 0:
								coord, err := ConvertCoordToSgfCoord(c)
								if err != nil {
									return "", fmt.Errorf("invalid AE coordinate %v: %v", c, err)
								}
								ae = append(ae, coord)
							}
						}
					}
					if len(ab) > 0 {
						Sgf += "AB"
						for _, s := range ab {
							Sgf += "[" + s + "]"
						}
					}
					if len(aw) > 0 {
						Sgf += "AW"
						for _, s := range aw {
							Sgf += "[" + s + "]"
						}
					}
					if len(ae) > 0 {
						Sgf += "AE"
						for _, s := range ae {
							Sgf += "[" + s + "]"
						}
					}
				}

				// Annotation bits from Vertices
				for _, ap := range []struct {
					sgf  string
					mask uint8
				}{
					{"CR", CircleMask}, {"SQ", SquareMask}, {"TR", TriangleMask}, {"MA", XMask},
				} {
					var coords []string
					for Y := uint8(0); Y < node.Board.Hist.Height; Y++ {
						for X := uint8(0); X < node.Board.Hist.Width; X++ {
							c := Coord{X, Y}
							if node.Board.Vertices[c]&ap.mask != 0 {
								coord, err := ConvertCoordToSgfCoord(c)
								if err != nil {
									return "", fmt.Errorf("invalid %s annotation coordinate %v: %v", ap.sgf, c, err)
								}
								coords = append(coords, coord)
							}
						}
					}
					if len(coords) > 0 {
						Sgf += ap.sgf
						for _, s := range coords {
							Sgf += "[" + s + "]"
						}
					}
				}
			}

			// Labels
			if len(node.Labels) > 0 {
				Sgf += "LB"
				for c, label := range node.Labels {
					coord, err := ConvertCoordToSgfCoord(c)
					if err != nil {
						return "", fmt.Errorf("invalid label coordinate %v: %v", c, err)
					}
					escapedLabel := strings.ReplaceAll(label, "\\", "\\\\")
					escapedLabel = strings.ReplaceAll(escapedLabel, "]", "\\]")
					Sgf += "[" + coord + ":" + escapedLabel + "]"
				}
			}

			// Unknown/unsupported properties reproduced verbatim
			for propName, rawValues := range node.UnknownSgfProperties {
				Sgf += propName
				for _, raw := range rawValues {
					Sgf += "[" + raw + "]"
				}
			}

			// Recursively generate child nodes (variations)
			for _, child := range node.Children {
				if childSgf, err := generateSgf(child); err == nil {
					Sgf += childSgf
				}
				// Skip children that have export errors
			}

			Sgf += ")"
			return Sgf, nil
		}

		SgfContent, err := generateSgf(GoWin.Coll.CollectionNode)
		if err != nil {
			GoWin.ShowError(fmt.Errorf("No valid games to export: %v", err))
			return
		}
		_, err = writer.Write([]byte(SgfContent))
		if err != nil {
			GoWin.ShowError(err)
		} else {
			GoWin.OpenedFilePath = writer.URI().Path()
			GoWin.FixWindowTitle()
		}
	}, GoWin.Win)
	if GoWin.OpenedFilePath != "" {
		fileDialog.SetFileName(GoWin.OpenedFilePath)
	}
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".Sgf"}))
	GoWin.ResizeDialog = func() { fileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	fileDialog.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	fileDialog.Show()
}

// sgfEscapeText escapes backslashes and closing brackets in SGF text values.
func SgfEscapeText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}

// ImportSgfFile reads GoWin.OpenedFilePath from disk and imports it via
// ImportSgfContent. Does nothing if OpenedFilePath is empty.
func (GoWin *GoWin) ImportSgfFile() {
	if GoWin.OpenedFilePath == "" {
		return
	}
	ContentBytes, Err := os.ReadFile(GoWin.OpenedFilePath)
	if Err != nil {
		fmt.Println("Failed to read file: ", Err)
	}

	GoWin.ImportSgfContent(string(ContentBytes)) // TODO figure out why it is so slow
}

// PeekSgfGM scans from index (which should point at '(') forward to find the GM
// property value in the first node of the tree. Returns "" if GM is absent
// (caller should treat as Go/GM[1]).
func PeekSgfGM(content string, index int) string {
	n := len(content)
	// skip '('
	index++
	// skip whitespace to find ';'
	for index < n && content[index] != ';' {
		if content[index] == ')' {
			return ""
		}
		index++
	}
	if index >= n {
		return ""
	}
	index++ // skip ';'
	// scan properties until we hit '(' ')' ';' or EOF
	for index < n {
		ch := content[index]
		if ch == '(' || ch == ')' || ch == ';' {
			break
		}
		// collect property name
		propName := ""
		for index < n && content[index] >= 'A' && content[index] <= 'Z' {
			propName += string(content[index])
			index++
		}
		// skip whitespace
		for index < n && (content[index] == ' ' || content[index] == '\t' ||
			content[index] == '\n' || content[index] == '\r') {
			index++
		}
		if index >= n || content[index] != '[' {
			break
		}
		// read all values for this property
		var firstValue string
		for index < n && content[index] == '[' {
			index++ // skip '['
			val := ""
			for index < n && content[index] != ']' {
				if content[index] == '\\' {
					index++
				}
				if index < n {
					val += string(content[index])
					index++
				}
			}
			if index < n {
				index++ // skip ']'
			}
			if firstValue == "" {
				firstValue = val
			}
			// skip whitespace between values
			for index < n && (content[index] == ' ' || content[index] == '\t' ||
				content[index] == '\n' || content[index] == '\r') {
				index++
			}
		}
		if propName == "GM" {
			return firstValue
		}
	}
	return ""
}

// captureRawTree captures a complete SGF tree starting at index (pointing at '('),
// tracking bracket depth. Returns the raw string and the index after the closing ')'.
func CaptureRawTree(content string, index int) (string, int) {
	n := len(content)
	start := index
	depth := 0
	for index < n {
		ch := content[index]
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				index++
				return content[start:index], index
			}
		case '[':
			// skip property value, handling escapes
			index++
			for index < n && content[index] != ']' {
				if content[index] == '\\' {
					index++
				}
				index++
			}
		}
		index++
	}
	return content[start:], index
}

// ImportSgfContent parses a full SGF string into a new Collection, handling
// multiple game trees, variations, properties (moves, setup, annotations,
// comments, labels), and unknown properties for round-trip preservation.
// Non-Go game trees (GM != 1) are stored verbatim. If at least one game
// is loadable, skips non-loadable games and shows warnings for each.
// Returns an error only if no games could be loaded at all.
func (GoWin *GoWin) ImportSgfContent(SgfContent string) error {
	NewColl := NewCollection()
	ParentOfNextNode := NewColl.CollectionNode
	VariationParents := []*GameTreeNode{} // The parents of variations enclosed in parenthesis
	Index := 0
	SgfLength := len(SgfContent)
	if SgfLength == 0 {
		GoWin.Coll = NewColl
		GoWin.SetCurrentNode(NewColl.CollectionNode)
		return nil
	}
	GetLineAndColumn := func() string {
		// If Index points to a newline, it will be the zeroth column of that new line.
		Line := 1
		LastNewLine := -1
		for NIndex := 0; NIndex <= Index; NIndex++ {
			if SgfContent[NIndex:NIndex+1] == "\n" {
				Line++
				LastNewLine = NIndex
			}
		}
		return fmt.Sprintf("Line %d, Column %d", Line, Index-LastNewLine)
	}
	// CurrentGameStartIndex holds the index of the '(' that began the current
	// top-level game tree, so we can skip the whole tree on parse error.
	CurrentGameStartIndex := -1
	// SkipCurrentGame discards the current top-level game tree on parse error.
	// It removes any partial root node added to NewColl, resets parser state,
	// and shows a warning to the user.
	SkipCurrentGame := func(ParseErr error) {
		// Remove any partial root node that was added during this game.
		CollChildren := NewColl.CollectionNode.Children
		if len(CollChildren) > 0 {
			LastChild := CollChildren[len(CollChildren)-1]
			// Only remove if LastChild is the game we are skipping (its root
			// node was created after CurrentGameStartIndex was set).
			if LastChild.LastMove == RootCoords {
				NewColl.CollectionNode.Children = CollChildren[:len(CollChildren)-1]
			}
		}
		// Skip the rest of the raw tree.
		_, NewIndex := CaptureRawTree(SgfContent, CurrentGameStartIndex)
		Index = NewIndex
		// Reset parser state to top-level.
		VariationParents = VariationParents[:0]
		ParentOfNextNode = NewColl.CollectionNode
		CurrentGameStartIndex = -1
		GoWin.ShowError(fmt.Errorf("Skipping game due to parse error: %v", ParseErr))
	}
OuterLoop:
	for {
		if len(VariationParents) == 0 {
			for SgfContent[Index:Index+1] != "(" {
				Index++
				if Index >= SgfLength {
					break OuterLoop
				}
			}
			// Check if this is a non-Go game tree; if so, capture raw and skip
			gmVal := PeekSgfGM(SgfContent, Index)
			if gmVal != "" && gmVal != "1" {
				raw, newIndex := CaptureRawTree(SgfContent, Index)
				NewColl.NonGoGameTrees = append(NewColl.NonGoGameTrees, raw)
				Index = newIndex
				if Index >= SgfLength {
					break OuterLoop
				}
				continue OuterLoop
			}
			// Record the start of this top-level Go game tree.
			CurrentGameStartIndex = Index
		}
		switch SgfContent[Index : Index+1] {
		case "(":
			VariationParents = append(VariationParents, ParentOfNextNode)
			Index++
			if Index >= SgfLength {
				if CurrentGameStartIndex >= 0 {
					SkipCurrentGame(fmt.Errorf("Unexpected end of file."))
					break OuterLoop
				}
				return fmt.Errorf("Unexpected end of file.")
			}
		case ")":
			VariationParentsLength := len(VariationParents)
			if VariationParentsLength == 0 {
				return fmt.Errorf("Unexpected `)`: %s", GetLineAndColumn())
			}
			ParentOfNextNode = VariationParents[VariationParentsLength-1]
			VariationParents = VariationParents[0 : VariationParentsLength-1]
			Index++
			if Index >= SgfLength {
				break OuterLoop
			}
		case ";":
			Index++
			if Index >= SgfLength {
				if CurrentGameStartIndex >= 0 {
					SkipCurrentGame(fmt.Errorf("Unexpected end of file."))
					break OuterLoop
				}
				return fmt.Errorf("Unexpected end of file.")
			}
			NodeProperties := map[string][][]string{}
			NodePropertiesRaw := map[string][]string{}
			LastPropertyName := ""
			var CollectPropertiesErr error
		CollectProperties:
			for {
				PropertyName := ""
			GetPropertyName:
				for {
					CurrentChar := SgfContent[Index : Index+1]
					// Only upper case letters are allowed in property names
					if strings.Contains("ABCDEFGHIJKLMNOPQRSTUVWXYZ", CurrentChar) {
						PropertyName += CurrentChar
					}
					switch CurrentChar {
					case "(", ")", ";":
						if PropertyName != "" {
							CollectPropertiesErr = fmt.Errorf("Unexpected `%s`: %s", CurrentChar, GetLineAndColumn())
							break CollectProperties
						}
						break CollectProperties
					case "[":
						break GetPropertyName
					case "]":
						CollectPropertiesErr = fmt.Errorf("Unexpected `]`: %s", GetLineAndColumn())
						break CollectProperties
					}
					Index++
					if Index >= SgfLength {
						CollectPropertiesErr = fmt.Errorf("Unexpected end of file.")
						break CollectProperties
					}
				}
				if PropertyName == "" {
					PropertyName = LastPropertyName
				} else {
					LastPropertyName = PropertyName
				}
				if PropertyName == "" {
					CollectPropertiesErr = fmt.Errorf("Unexpected `[`: %s", GetLineAndColumn())
					break CollectProperties
				}
				Index++
				if Index >= SgfLength {
					CollectPropertiesErr = fmt.Errorf("Unexpected end of file.")
					break CollectProperties
				}
				rawStart := Index
				PropertyValue := []string{""}
			GetPropertyValue:
				for {
					CurrentChar := SgfContent[Index : Index+1]
					switch CurrentChar {
					case "]":
						rawEnd := Index
						if _, exists := NodePropertiesRaw[PropertyName]; !exists {
							NodePropertiesRaw[PropertyName] = []string{}
						}
						NodePropertiesRaw[PropertyName] = append(
							NodePropertiesRaw[PropertyName], SgfContent[rawStart:rawEnd])
						Index++
						if Index >= SgfLength {
							CollectPropertiesErr = fmt.Errorf("Unexpected end of file.")
							break CollectProperties
						}
						break GetPropertyValue
					case ":":
						PropertyValue = append(PropertyValue, "")
					case "\\":
						Index++
						if Index >= SgfLength {
							CollectPropertiesErr = fmt.Errorf("Unexpected end of file.")
							break CollectProperties
						}
						UnescapedCurrentChar := SgfContent[Index : Index+1]
						switch UnescapedCurrentChar {
						case "\n":
						case "\t", "\r", "\v", "\f", "\b":
							PropertyValue[len(PropertyValue)-1] += " "
						default:
							PropertyValue[len(PropertyValue)-1] += UnescapedCurrentChar
						}
					case "\t", "\r", "\v", "\f", "\b":
						PropertyValue[len(PropertyValue)-1] += " "
					default:
						PropertyValue[len(PropertyValue)-1] += CurrentChar
					}
					Index++
					if Index >= SgfLength {
						CollectPropertiesErr = fmt.Errorf("Unexpected end of file.")
						break CollectProperties
					}
				}
				_, Exists := NodeProperties[PropertyName]
				if !Exists {
					NodeProperties[PropertyName] = [][]string{}
				}
				NodeProperties[PropertyName] = append(NodeProperties[PropertyName], PropertyValue)
			}
			if CollectPropertiesErr != nil {
				if CurrentGameStartIndex >= 0 {
					SkipCurrentGame(CollectPropertiesErr)
					if Index >= SgfLength {
						break OuterLoop
					}
					continue OuterLoop
				}
				return CollectPropertiesErr
			}
			var NewNode *GameTreeNode
			var NodeParseErr error
			if ParentOfNextNode == NewColl.CollectionNode {
				var Width, Height uint8 = 19, 19
				SizeProperty, Exists := NodeProperties["SZ"]
				if Exists {
					if len(SizeProperty) == 1 {
						switch len(SizeProperty[0]) {
						case 1, 2:
							PWidth, Err := strconv.ParseUint(SizeProperty[0][0], 10, 8)
							if Err != nil {
								NodeParseErr = fmt.Errorf("SZ property not parsable. Node ending at %s.",
									GetLineAndColumn())
							} else {
								Width = uint8(PWidth)
							}
						default:
							NodeParseErr = fmt.Errorf("SZ properties allow at most 2 compound values. Node ending at %s.",
								GetLineAndColumn())
						}
						if NodeParseErr == nil {
							switch len(SizeProperty[0]) {
							case 1:
								Height = Width
							case 2:
								PHeight, Err := strconv.ParseUint(SizeProperty[0][1], 10, 8)
								if Err != nil {
									NodeParseErr = fmt.Errorf("SZ property not parsable. Node ending at %s.",
										GetLineAndColumn())
								} else {
									Height = uint8(PHeight)
								}
							}
						}
					} else {
						NodeParseErr = fmt.Errorf("SZ property set more than 1 time. Node ending at %s.",
							GetLineAndColumn())
					}
				}
				if NodeParseErr == nil && (Width > 52 || Height > 52) {
					NodeParseErr = fmt.Errorf("Board size exceeds maximum allowed size of 52. Node ending at %s.",
						GetLineAndColumn())
				}
				if NodeParseErr == nil && ((Width == 1 && Height == 1) || Width == 0 || Height == 0) {
					NodeParseErr = fmt.Errorf("Board size too small. Node ending at %s.",
						GetLineAndColumn())
				}
				if NodeParseErr != nil {
					if CurrentGameStartIndex >= 0 {
						SkipCurrentGame(NodeParseErr)
						if Index >= SgfLength {
							break OuterLoop
						}
						continue OuterLoop
					}
					return NodeParseErr
				}
				NewNode = NewColl.NewRootNode(Width, Height)
				NewNode.UnixMilli = 0
			} else {
				NewNode = ParentOfNextNode.MakeChild(true, true)
				NewNode.UnixMilli = 0
			}
			HasSetupProperties := false
			HasMoveProperties := false
			CoordToAwAeAb := map[Coord]string{}
			type pendingAnnotation struct {
				PropertyName string
				C            Coord
			}
			var pendingAnnotations []pendingAnnotation
			SetBoolProp := func(PropertyName string, C Coord) error {
				if _, isAnnotation := SgfPropertyToAnnotationMask[PropertyName]; isAnnotation {
					pendingAnnotations = append(pendingAnnotations, pendingAnnotation{PropertyName, C})
					return nil
				}
				// TODO refactor SetCoordProperty
				SetCoordProperty := func(NewNode *GameTreeNode, PropertyName string, C Coord) (*GameTreeNode, error) {
					if !NewNode.Board.Hist.InBounds(C) {
						return NewNode, fmt.Errorf("%s not on board.", NewNode.Board.ToString(C))
					}
					// TODO remove all SGF from core code
					// Annotation properties: store in Vertices bits, no topology change
					if mask, isAnnotation := SgfPropertyToAnnotationMask[PropertyName]; isAnnotation {
						NewNode.Board.SetAnnotation(C, mask)
						return NewNode, nil
					}
					// Stone-edit properties: may create a new node
					switch NewNode.LastMove {
					case RootCoords:
						if len(NewNode.Children) != 0 {
							NewNode = NewNode.MakeSiblingCopyBoard(true)
							NewNode.LastMove = RootCoords
						}
					case EditCoords:
						if len(NewNode.Children) != 0 {
							NewNode = NewNode.MakeSiblingCopyBoard(true)
							NewNode.LastMove = EditCoords
						}
					default:
						NewNode = NewNode.MakeChild(true, true)
						NewNode.LastMove = EditCoords
					}
					NewNode.UnixMilli = 0
					switch PropertyName {
					case "AW":
						NewNode.Board.Vertices[C] = (NewNode.Board.Vertices[C] &^ PlayerAtVertexMask) | 2
					case "AE":
						delete(NewNode.Board.Vertices, C)
					case "AB":
						NewNode.Board.Vertices[C] = (NewNode.Board.Vertices[C] &^ PlayerAtVertexMask) | 1
					}
					NewNode.Board.CalculateAllGroups()
					NewNode.Board.IsPlacementLegalCache = nil
					return NewNode, nil
				}
				Gtn, Err := SetCoordProperty(NewNode, PropertyName, C)
				if Err != nil {
					coordStr, coordErr := ConvertCoordToSgfCoord(C)
					if coordErr != nil {
						coordStr = fmt.Sprintf("Invalid(%d,%d)", C.X, C.Y)
					}
					return fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
						PropertyName, coordStr, GetLineAndColumn(), Err)
				}
				if Gtn != NewNode {
					coordStr, coordErr := ConvertCoordToSgfCoord(C)
					if coordErr != nil {
						coordStr = fmt.Sprintf("Invalid(%d,%d)", C.X, C.Y)
					}
					return fmt.Errorf("%s property has a bad coordinate `%s`. "+
						"Node ending at %s. Unexpectedly created a child.",
						PropertyName, coordStr, GetLineAndColumn())
				}
				return nil
			}
			SetAwAeAb := func(PropertyName string, C Coord) error {
				switch CoordToAwAeAb[C] {
				case PropertyName, "":
					CoordToAwAeAb[C] = PropertyName
				default:
					coordStr, coordErr := ConvertCoordToSgfCoord(C)
					if coordErr != nil {
						coordStr = fmt.Sprintf("Invalid(%d,%d)", C.X, C.Y)
					}
					return fmt.Errorf("Same coordinate `%s` used by both %s and %s. Node ending at %s.",
						coordStr, PropertyName, CoordToAwAeAb[C], GetLineAndColumn())
				}
				return SetBoolProp(PropertyName, C)
			}
			// storeInfoProp joins all parsed values and stores under the description key.
			storeInfoProp := func(sgfKey string, PropertyValue [][]string) {
				parts := []string{}
				for _, pv := range PropertyValue {
					parts = append(parts, strings.Join(pv, ":"))
				}
				joined := strings.Join(parts, "\n")
				desc := SgfToDescription[sgfKey]
				NewNode.Board.Hist.Information[desc] = &joined
			}
		ProcessProperties:
			for PropertyName, PropertyValue := range NodeProperties {
				switch PropertyName {
				case "SZ", "KM", "AP", "CA", "FF", "GM", "PB", "PW", "BR", "WR", "DT", "EV", "RU",
					"HA", "RE", "GC", "AN", "BT", "CP", "GN", "ON", "OT", "PC", "RO", "SO", "TM", "US", "WT":
					if ParentOfNextNode != NewColl.CollectionNode {
						NodeParseErr = fmt.Errorf("Non-root node has root property %s. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
				case "AW", "AE", "AB", "PL":
					HasSetupProperties = true
					if NewNode.LastMove != RootCoords {
						NewNode.LastMove = EditCoords
					}
				case "W", "B":
					if HasMoveProperties {
						NodeParseErr = fmt.Errorf("Node has both W and B properties. Node ending at %s.",
							GetLineAndColumn())
						break ProcessProperties
					}
					HasMoveProperties = true
				}
				switch PropertyName {
				case "KM":
					if len(PropertyValue) != 1 {
						NodeParseErr = fmt.Errorf("%s property set more than 1 time. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					if len(PropertyValue[0]) != 1 {
						NodeParseErr = fmt.Errorf("%s property is not compound. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					Komi, Err := strconv.ParseFloat(PropertyValue[0][0], 64)
					if Err != nil {
						NodeParseErr = fmt.Errorf("%s property not parsable. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					NewNode.Board.Hist.Komi = map[uint8]float64{1: 0, 2: Komi}
				case "FF":
					if len(PropertyValue) == 1 && len(PropertyValue[0]) == 1 &&
						PropertyValue[0][0] != "4" {
						GoWin.ShowError(fmt.Errorf("Warning: FF[%s] is not FF[4]; "+
							"some properties may not be parsed correctly. Node ending at %s.",
							PropertyValue[0][0], GetLineAndColumn()))
					}
					storeInfoProp(PropertyName, PropertyValue)
				case "GM":
					storeInfoProp(PropertyName, PropertyValue)
				case "AP":
					if len(PropertyValue) != 1 {
						NodeParseErr = fmt.Errorf("%s property set more than 1 time. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					if len(PropertyValue[0]) > 2 {
						NodeParseErr = fmt.Errorf("%s property has more than 2 compound values. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					storeInfoProp(PropertyName, PropertyValue)
				case "PB":
					if len(PropertyValue) == 1 {
						NewNode.Board.Hist.PlayerNames[1] = strings.Join(PropertyValue[0], ":")
					}
				case "PW":
					if len(PropertyValue) == 1 {
						NewNode.Board.Hist.PlayerNames[2] = strings.Join(PropertyValue[0], ":")
					}
				case "RU":
					storeInfoProp(PropertyName, PropertyValue)
					if len(PropertyValue) == 1 && len(PropertyValue[0]) == 1 {
						if RS := FindRuleSetByName(PropertyValue[0][0]); RS != nil {
							NewNode.Board.Hist.RuleSet = RS
						}
					}
				case "CA", "BR", "WR", "DT", "EV",
					"RE", "GC", "AN", "BT", "CP", "GN", "ON", "OT", "PC", "RO", "SO", "TM", "US", "WT":
					storeInfoProp(PropertyName, PropertyValue)
				case "HA":
					storeInfoProp(PropertyName, PropertyValue)
					// HA >= 2 means handicap game: White moves first after stones placed
					if len(PropertyValue) == 1 && len(PropertyValue[0]) == 1 {
						ha, err := strconv.ParseInt(PropertyValue[0][0], 10, 64)
						if err == nil && ha >= 2 {
							NewNode.NextStonePlacer = 2
						}
					}
				case "AW", "AE", "AB":
					for _, SgfCoords := range PropertyValue {
						switch len(SgfCoords) {
						case 1:
							C, Err := ConvertSgfCoordToCoord(SgfCoords[0], NewNode.Board)
							if Err != nil {
								NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
									PropertyName, SgfCoords[0], GetLineAndColumn(), Err)
								break ProcessProperties
							}
							Err = SetAwAeAb(PropertyName, C)
							if Err != nil {
								NodeParseErr = Err
								break ProcessProperties
							}
						case 2:
							C0, Err := ConvertSgfCoordToCoord(SgfCoords[0], NewNode.Board)
							if Err != nil {
								NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
									PropertyName, SgfCoords[0], GetLineAndColumn(), Err)
								break ProcessProperties
							}
							C1, Err := ConvertSgfCoordToCoord(SgfCoords[1], NewNode.Board)
							if Err != nil {
								NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
									PropertyName, SgfCoords[1], GetLineAndColumn(), Err)
								break ProcessProperties
							}
							MinX := min(C0.X, C1.X)
							MaxX := max(C0.X, C1.X)
							MinY := min(C0.Y, C1.Y)
							MaxY := max(C0.Y, C1.Y)
						OuterAEAWAB:
							for X := MinX; X <= MaxX; X++ {
								for Y := MinY; Y <= MaxY; Y++ {
									C := Coord{X: X, Y: Y}
									Err = SetAwAeAb(PropertyName, C)
									if Err != nil {
										NodeParseErr = Err
										break OuterAEAWAB
									}
								}
							}
							if NodeParseErr != nil {
								break ProcessProperties
							}
						default:
							NodeParseErr = fmt.Errorf("%s property does not allow more than 2 compound values. Node ending at %s.",
								PropertyName, GetLineAndColumn())
							break ProcessProperties
						}
						if NodeParseErr != nil {
							break ProcessProperties
						}
					}
				case "CR", "SQ", "TR", "MA":
					for _, SgfCoords := range PropertyValue {
						switch len(SgfCoords) {
						case 1:
							C, Err := ConvertSgfCoordToCoord(SgfCoords[0], NewNode.Board)
							if Err != nil {
								NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
									PropertyName, SgfCoords[0], GetLineAndColumn(), Err)
								break ProcessProperties
							}
							Err = SetBoolProp(PropertyName, C)
							if Err != nil {
								NodeParseErr = Err
								break ProcessProperties
							}
						case 2:
							C0, Err := ConvertSgfCoordToCoord(SgfCoords[0], NewNode.Board)
							if Err != nil {
								NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
									PropertyName, SgfCoords[0], GetLineAndColumn(), Err)
								break ProcessProperties
							}
							C1, Err := ConvertSgfCoordToCoord(SgfCoords[1], NewNode.Board)
							if Err != nil {
								NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
									PropertyName, SgfCoords[1], GetLineAndColumn(), Err)
								break ProcessProperties
							}
							MinX := min(C0.X, C1.X)
							MaxX := max(C0.X, C1.X)
							MinY := min(C0.Y, C1.Y)
							MaxY := max(C0.Y, C1.Y)
						OuterCRSQTRMA:
							for X := MinX; X <= MaxX; X++ {
								for Y := MinY; Y <= MaxY; Y++ {
									C := Coord{X: X, Y: Y}
									Err = SetBoolProp(PropertyName, C)
									if Err != nil {
										NodeParseErr = Err
										break OuterCRSQTRMA
									}
								}
							}
							if NodeParseErr != nil {
								break ProcessProperties
							}
						default:
							NodeParseErr = fmt.Errorf("%s property does not allow more than 2 compound values. Node ending at %s.",
								PropertyName, GetLineAndColumn())
							break ProcessProperties
						}
						if NodeParseErr != nil {
							break ProcessProperties
						}
					}
				case "PL":
					if len(PropertyValue) != 1 {
						NodeParseErr = fmt.Errorf("%s property set more than 1 time. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					if len(PropertyValue[0]) != 1 {
						NodeParseErr = fmt.Errorf("%s property is not compound. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					parsedPlayer, Err := BWToPlayer(PropertyValue[0][0])
					if Err != nil {
						NodeParseErr = fmt.Errorf("%s property has an invalid player. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					switch parsedPlayer {
					case NewNode.GetNextStonePlacer():
					default:
						if 0 < parsedPlayer && parsedPlayer <= NewNode.Board.Hist.Players {
							NewNode.NextStonePlacer = parsedPlayer
						} else {
							NodeParseErr = fmt.Errorf("%s property has an invalid player. Node ending at %s.",
								PropertyName, GetLineAndColumn())
							break ProcessProperties
						}
					}
				case "W", "B":
					if len(PropertyValue) != 1 {
						NodeParseErr = fmt.Errorf("%s property set more than 1 time. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					if len(PropertyValue[0]) != 1 {
						NodeParseErr = fmt.Errorf("%s property is not compound. Node ending at %s.",
							PropertyName, GetLineAndColumn())
						break ProcessProperties
					}
					C, Err := ConvertSgfCoordToCoord(PropertyValue[0][0], NewNode.Board)
					if Err != nil {
						NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
							PropertyName, PropertyValue[0][0], GetLineAndColumn(), Err)
						break ProcessProperties
					}
					MovePlayer, _ := BWToPlayer(PropertyName)
					BoardWithMove, Err := NewNode.Board.AttemptMove(C, MovePlayer, false, false)
					if Err != nil {
						NodeParseErr = fmt.Errorf("%s property has an illegal move at coordinate `%s`. "+
							"Node ending at %s. %v", PropertyName,
							PropertyValue[0][0], GetLineAndColumn(), Err)
						break ProcessProperties
					}
					NewNode.Board = BoardWithMove
					NewNode.LastMove = C
					NewNode.PreviousStonePlacer = MovePlayer
					NewNode.RemainingStonePlacements = 1 // SGF assumes single stone per move
				case "C":
					if len(PropertyValue) > 1 {
						GoWin.ShowError(fmt.Errorf("Warning: %s property set more than 1 time. "+
							"Concatenating. Node ending at %s.", PropertyName, GetLineAndColumn()))
					}
					CommentsToConcatenate := []string{}
					for _, CommentData := range PropertyValue {
						CommentsToConcatenate = append(CommentsToConcatenate,
							strings.Join(CommentData, ":"))
					}
					NewNode.Comment = strings.Join(CommentsToConcatenate, "\n")
				case "LB":
					for _, LabelsData := range PropertyValue {
						C, Err := ConvertSgfCoordToCoord(LabelsData[0], NewNode.Board)
						if Err != nil {
							NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s. %v",
								PropertyName, LabelsData[0], GetLineAndColumn(), Err)
							break ProcessProperties
						}
						if len(LabelsData) < 2 {
							NodeParseErr = fmt.Errorf("%s property should be compound of 2 values. Node ending at %s.",
								PropertyName, GetLineAndColumn())
							break ProcessProperties
						}
						Label := strings.Join(LabelsData[1:], ":")
						NewNode.Labels[C] = Label
					}
				case "SZ":
					// Already handled above when creating the root node; no-op here.
				default:
					// Store unknown properties verbatim for byte-by-byte round-trip reproduction
					rawVals, exists := NodePropertiesRaw[PropertyName]
					if exists {
						NewNode.UnknownSgfProperties[PropertyName] = append(
							NewNode.UnknownSgfProperties[PropertyName], rawVals...)
					}
				}
			}
			if NodeParseErr != nil {
				if CurrentGameStartIndex >= 0 {
					SkipCurrentGame(NodeParseErr)
					if Index >= SgfLength {
						break OuterLoop
					}
					continue OuterLoop
				}
				return NodeParseErr
			}
			// Apply annotations after move/setup processing so W/B can't overwrite them
			for _, pa := range pendingAnnotations {
				if !NewNode.Board.Hist.InBounds(pa.C) {
					coordStr, coordErr := ConvertCoordToSgfCoord(pa.C)
					if coordErr != nil {
						coordStr = fmt.Sprintf("Invalid(%d,%d)", pa.C.X, pa.C.Y)
					}
					NodeParseErr = fmt.Errorf("%s property has a bad coordinate `%s`. Node ending at %s.",
						pa.PropertyName, coordStr, GetLineAndColumn())
					break
				}
				mask := SgfPropertyToAnnotationMask[pa.PropertyName]
				NewNode.Board.SetAnnotation(pa.C, mask)
			}
			if NodeParseErr == nil && HasSetupProperties && HasMoveProperties {
				NodeParseErr = fmt.Errorf("Node has both setup and move properties. Node ending at %s.",
					GetLineAndColumn())
			}
			if NodeParseErr == nil && HasSetupProperties {
				NewNode.Board.CalculateAllGroups()
				for Group := range NewNode.Board.Groups {
					if len(Group.Vertices[0]) == 0 {
						NodeParseErr = fmt.Errorf("Setup resulted in a group with 0 liberties. Node ending at %s.",
							GetLineAndColumn())
						break
					}
				}
			}
			if NodeParseErr != nil {
				if CurrentGameStartIndex >= 0 {
					SkipCurrentGame(NodeParseErr)
					if Index >= SgfLength {
						break OuterLoop
					}
					continue OuterLoop
				}
				return NodeParseErr
			}
			ParentOfNextNode = NewNode
		default:
			Index++
		}
		if Index >= SgfLength {
			break
		}
	}

	if len(VariationParents) != 0 {
		if CurrentGameStartIndex >= 0 {
			SkipCurrentGame(fmt.Errorf("Missing `)`."))
		} else {
			return fmt.Errorf("Missing `)`.")
		}
	}

	// If nothing was loaded, report failure rather than replacing with an empty collection.
	if len(NewColl.CollectionNode.Children) == 0 && len(NewColl.NonGoGameTrees) == 0 {
		return fmt.Errorf("No loadable games found in SGF.")
	}

	// Update the game collection
	GoWin.Coll = NewColl
	TargetNode := LastMainLineNode(NewColl)
	NeedToDrawGoban := TargetNode != NewColl.CollectionNode
	GoWin.SetCurrentNode(TargetNode)
	if NeedToDrawGoban {
		GoWin.DrawGoban()
	}

	// Refresh game selector after SGF import
	GoWin.RefreshGameSelector()

	return nil
}

// LastMainLineNode returns the last node on the first game's main line,
// or the CollectionNode if the collection has no games.
func LastMainLineNode(Coll *Collection) *GameTreeNode {
	if len(Coll.CollectionNode.Children) == 0 {
		return Coll.CollectionNode
	}
	Node := Coll.CollectionNode.Children[0]
	for len(Node.Children) > 0 {
		Node = Node.Children[0]
	}
	return Node
}
