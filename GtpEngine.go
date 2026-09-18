package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// SendGtpCommandToEngine is a free function (not a method) so the move
// goroutine can hold a direct *GtpEngineState reference without touching GoWin.
func SendGtpCommandToEngine(Eng *GtpEngineState, Command string) (string, error) {
	if Eng == nil {
		return "", fmt.Errorf("engine is not attached")
	}
	Eng.CommandMutex.Lock()
	defer Eng.CommandMutex.Unlock()
	if Eng.In == nil || Eng.Reader == nil {
		return "", fmt.Errorf("engine is not attached")
	}

	_, Err := Eng.In.Write([]byte(Command + "\n"))
	if Err != nil {
		return "", Err
	}
	if Eng.LogFile != nil {
		fmt.Fprintf(Eng.LogFile, ">> %s\n", Command)
	}

	var ResponseLines []string
	for {
		Line, Err := Eng.Reader.ReadString('\n')
		if Err != nil {
			return "", Err
		}
		Line = strings.TrimSpace(Line)
		if Line == "" {
			continue
		}
		if Line[0] == '=' || Line[0] == '?' {
			if len(Line) > 1 {
				ResponseLines = append(ResponseLines, strings.TrimSpace(Line[1:]))
			}
			for {
				NextLine, Err := Eng.Reader.ReadString('\n')
				if Err != nil {
					return "", Err
				}
				NextLine = strings.TrimSpace(NextLine)
				if NextLine == "" {
					break
				}
				ResponseLines = append(ResponseLines, NextLine)
			}
			Response := strings.Join(ResponseLines, "\n")
			if Eng.LogFile != nil {
				fmt.Fprintf(Eng.LogFile, "<< %s\n", Response)
			}
			if Line[0] == '?' {
				return Response, fmt.Errorf("error from engine: %s", Response)
			}
			return Response, nil
		}
	}
}

// PlayStoneGtpToEngine sends a "play" command to Eng for the given Coord and
// Player. GTP engines only support 2-player games (B/W), so only players 1 and 2
// produce commands; other values are a no-op.
func (GoWin *GoWin) PlayStoneGtpToEngine(Eng *GtpEngineState, C Coord, Player uint8) error {
	switch Player {
	case 1, 2:
		// GTP only supports 2-player games
		GtpCoord, Err := GoWin.Coll.CurrentGameH.ClientToGTPCoords(C)
		if Err != nil {
			return Err
		}
		BW, Err := PlayerToBW(Player)
		if Err != nil {
			return Err
		}
		_, Err = SendGtpCommandToEngine(Eng, fmt.Sprintf("play %s %s", BW, GtpCoord))
		if Err != nil {
			return Err
		}
	}
	return nil
}

// AttachEngineForPlayer starts the named configuration and synchronizes its board.
func (GoWin *GoWin) AttachEngineForPlayer(Player uint8, EngineName string) error {
	EngCfg, Ok := CurrentAppConfig.Engines[EngineName]
	if !Ok {
		return fmt.Errorf("engine %q not found in config", EngineName)
	}

	Args := strings.Fields(EngCfg.GtpArgs)
	Cmd := exec.Command(EngCfg.GtpPath, Args...)

	StdinPipe, Err := Cmd.StdinPipe()
	if Err != nil {
		return Err
	}
	StdoutPipe, Err := Cmd.StdoutPipe()
	if Err != nil {
		return Err
	}
	if Err := Cmd.Start(); Err != nil {
		return Err
	}

	var LogFile *os.File
	if CurrentAppConfig.GtpLogging {
		LogFile, _ = os.OpenFile(fmt.Sprintf("%s.%d.GtpLog", EngineName, Player), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	}

	Eng := &GtpEngineState{
		Cmd:     Cmd,
		In:      StdinPipe,
		Out:     StdoutPipe,
		Reader:  bufio.NewReader(StdoutPipe),
		LogFile: LogFile,
		Name:    EngineName,
	}

	if GoWin.GtpEngines == nil {
		GoWin.GtpEngines = map[uint8]*GtpEngineState{}
	}
	GoWin.GtpEngines[Player] = Eng

	if Err := GoWin.InitializeEngineForPlayer(Player); Err != nil {
		GoWin.DetachEngineForPlayer(Player)
		return Err
	}
	return nil
}

// EngineGameSupported centralizes the GTP game restrictions for menus and attachment.
func (GoWin *GoWin) EngineGameSupported() bool {
	Hist := GoWin.Coll.CurrentGameH
	return Hist != nil && Hist.Players == 2 && Hist.RuleSet != nil &&
		Hist.RuleSet.StonePlacementsPerMove == 1 && Hist.LibertySharingFixed &&
		Hist.RootNode != nil && !Hist.RootNode.Board.NextLibertySharingMatrix.HasAnySharing()
}

// SelectPlayerEngine also restarts an already selected engine. Empty means Human.
func (GoWin *GoWin) SelectPlayerEngine(Player uint8, EngineName string) {
	GoWin.DetachEngineForPlayer(Player)
	if EngineName != "" && GoWin.EngineGameSupported() {
		if Err := GoWin.AttachEngineForPlayer(Player, EngineName); Err != nil {
			GoWin.ShowError(Err)
		}
	}
	GoWin.RefreshEngineMenuItems()
	GoWin.TriggerEngineIfNeeded()
}

// DetachEngineForPlayer kills and cleans up the engine for one player.
func (GoWin *GoWin) DetachEngineForPlayer(Player uint8) {
	Eng, Ok := GoWin.GtpEngines[Player]
	if !Ok || Eng == nil {
		return
	}
	delete(GoWin.GtpEngines, Player)
	if Eng.Cmd != nil && Eng.Cmd.Process != nil {
		_ = Eng.Cmd.Process.Kill()
		_ = Eng.Cmd.Wait()
	}
	if Eng.In != nil {
		_ = Eng.In.Close()
	}
	if Eng.Out != nil {
		_ = Eng.Out.Close()
	}
	Eng.CommandMutex.Lock()
	defer Eng.CommandMutex.Unlock()
	if Eng.LogFile != nil {
		_ = Eng.LogFile.Close()
	}
	GoWin.RefreshEngineMenuItems()
}

// DetachAllEngines cancels pending moves by removing and killing every engine.
func (GoWin *GoWin) DetachAllEngines() {
	for Player := range GoWin.GtpEngines {
		GoWin.DetachEngineForPlayer(Player)
	}
}

// initializeEngineForPlayer verifies the engine supports required GTP
// commands, sets the board size and komi, and replays the current board
// position. Skips board setup if no valid game is selected.
func (GoWin *GoWin) InitializeEngineForPlayer(Player uint8) error {
	Eng, Ok := GoWin.GtpEngines[Player]
	if !Ok || Eng == nil {
		return fmt.Errorf("engine for player %d not attached", Player)
	}

	SupportedCommands, Err := SendGtpCommandToEngine(Eng, "list_commands")
	if Err != nil {
		return Err
	}
	SupportedCommandsSplit := strings.Split(SupportedCommands, "\n")

	RequiredCommands := []string{"boardsize", "komi", "play", "genmove"}
	for _, RCmd := range RequiredCommands {
		Found := false
		for _, SCmd := range SupportedCommandsSplit {
			if SCmd == RCmd {
				Found = true
				break
			}
		}
		if !Found {
			return fmt.Errorf("engine does not support required command: %s", RCmd)
		}
	}

	// Skip board setup without a valid position. Menu attachment is
	// restricted to supported active games.
	if GoWin.CurrentNode().Board == nil || (GoWin.Width() <= 1 && GoWin.Height() <= 1) {
		return nil
	}

	// Set board size
	if GoWin.Width() == GoWin.Height() {
		if _, Err := SendGtpCommandToEngine(Eng, fmt.Sprintf("boardsize %d", GoWin.Width())); Err != nil {
			return Err
		}
	} else {
		if strings.Contains(SupportedCommands, "rectangular_boardsize") {
			if _, Err := SendGtpCommandToEngine(Eng,
				fmt.Sprintf("rectangular_boardsize %d %d", GoWin.Width(), GoWin.Height())); Err != nil {
				return Err
			}
		} else {
			return fmt.Errorf("engine does not support rectangular boards and board is not square")
		}
	}

	// Set komi
	if _, Err := SendGtpCommandToEngine(Eng, "komi "+GoWin.Coll.CurrentGameH.HalfIntegerMoku()); Err != nil {
		return Err
	}

	// Send current board state
	for C, V := range GoWin.CurrentNode().Board.Vertices {
		P := V & PlayerAtVertexMask
		if P == 0 {
			continue
		}
		if Err := GoWin.PlayStoneGtpToEngine(Eng, C, P); Err != nil {
			return Err
		}
	}
	return nil
}

// RefreshEngineMenuItems rebuilds checked player choices from live attachments.
func (GoWin *GoWin) RefreshEngineMenuItems() {
	Names := make([]string, 0, len(CurrentAppConfig.Engines))
	for Name := range CurrentAppConfig.Engines {
		Names = append(Names, Name)
	}
	sort.Strings(Names)
	for Index, MenuItem := range GoWin.PlayerEngineItems {
		Player := uint8(Index + 1)
		Eng := GoWin.GtpEngines[Player]
		HumanItem := fyne.NewMenuItem("Human", func() { GoWin.SelectPlayerEngine(Player, "") })
		HumanItem.Checked = Eng == nil
		Items := []*fyne.MenuItem{HumanItem}
		for _, Name := range Names {
			Item := fyne.NewMenuItem(Name, func() { GoWin.SelectPlayerEngine(Player, Name) })
			Item.Checked = Eng != nil && Eng.Name == Name
			Item.Disabled = !GoWin.EngineGameSupported()
			Items = append(Items, Item)
		}
		MenuItem.ChildMenu = fyne.NewMenu(MenuItem.Label, Items...)
	}
	if GoWin.MainMenu != nil {
		GoWin.Win.SetMainMenu(GoWin.MainMenu)
	}
	GoWin.RefreshPassMenuItem()
}

// TriggerEngineIfNeeded schedules one turn, for either human/engine or engine/engine play.
// Engine identity and source-node checks discard responses invalidated by detach/navigation.
func (GoWin *GoWin) TriggerEngineIfNeeded() {
	if !GoWin.EngineGameSupported() || GoWin.IsGameOver() {
		return
	}
	Node := GoWin.CurrentNode()
	Player := Node.GetNextStonePlacer()
	Eng := GoWin.GtpEngines[Player]
	if Eng == nil || Eng.Thinking {
		return
	}
	BW, Err := PlayerToBW(Player)
	if Err != nil {
		return
	}
	Eng.Thinking = true
	go func() {
		Move, Err := SendGtpCommandToEngine(Eng, fmt.Sprintf("genmove %s", BW))
		fyne.Do(func() {
			if GoWin.GtpEngines[Player] != Eng || GoWin.CurrentNode() != Node {
				return
			}
			Eng.Thinking = false
			if Err != nil {
				GoWin.DetachEngineForPlayer(Player)
				GoWin.ShowError(Err)
				return
			}
			GoWin.HandleEngineMove(Move, Player)
			if GoWin.CurrentNode() == Node {
				GoWin.DetachEngineForPlayer(Player)
				return
			}
			GoWin.TriggerEngineIfNeeded()
		})
	}()
}

// PlayMove applies a move to the game tree and notifies all attached engines
// except skipEnginePlayer (use 0 to notify all engines, e.g. for human moves).
func (GoWin *GoWin) PlayMove(C Coord, Player uint8, SkipEnginePlayer uint8) {
	// Check if the move already exists as a child of the current node.
	MoveExists := false
	for _, Child := range GoWin.CurrentNode().Children {
		if Child.LastMove == C && Child.PreviousStonePlacer == Player {
			GoWin.SetCurrentNode(Child, true)
			MoveExists = true
			break
		}
	}

	if !MoveExists {
		BoardWithMove, Err := GoWin.CurrentNode().Board.AttemptMove(C, Player, false, false)
		if Err != nil {
			GoWin.ShowError(Err)
			return
		}
		NewNode := GoWin.CurrentNode().MakeChild(false, false)
		NewNode.Board = BoardWithMove
		NewNode.LastMove = C
		NewNode.PreviousStonePlacer = Player
		NewNode.NoResultDraw = BoardWithMove.NoResultTriggered

		// Set NextStonePlacer and RemainingStonePlacements based on StonePlacementsPerMove logic
		ParentNode := GoWin.CurrentNode()
		GameHist := GoWin.Coll.CurrentGameH

		if C != PassCoords && ParentNode.RemainingStonePlacements > 1 {
			// Same player continues with one less placement
			NewNode.NextStonePlacer = Player
			NewNode.RemainingStonePlacements = ParentNode.RemainingStonePlacements - 1
		} else {
			// Pass always switches player (consuming all remaining placements).
			// RemainingStonePlacements == 1 also switches player and resets.
			NewNode.NextStonePlacer = NextPlayer(Player, GameHist.Players)
			NewNode.RemainingStonePlacements = GameHist.RuleSet.StonePlacementsPerMove
		}

		GoWin.SetCurrentNode(NewNode, true)
	}

	// Inform all attached engines except the one that generated this move.
	for P, Eng := range GoWin.GtpEngines {
		if P != SkipEnginePlayer {
			if Err := GoWin.PlayStoneGtpToEngine(Eng, C, Player); Err != nil {
				GoWin.DetachEngineForPlayer(P)
				GoWin.ShowError(Err)
			}
		}
	}
}

// HandleEngineMove converts a GTP coordinate string to an internal coord and
// applies the move, skipping re-notification of enginePlayer's engine.
func (GoWin *GoWin) HandleEngineMove(GtpCoord string, EnginePlayer uint8) {
	GtpCoord = strings.ToUpper(strings.TrimSpace(GtpCoord))
	if GtpCoord == "PASS" {
		GoWin.PlayMove(PassCoords, EnginePlayer, EnginePlayer)
		return
	}
	if GtpCoord == "RESIGN" {
		GoWin.DetachAllEngines()
		dialog.ShowInformation("Engine Resigned", "The engine has resigned.", GoWin.Win)
		return
	}
	C, Err := GoWin.Coll.CurrentGameH.GtpCoordToClientCoords(GtpCoord)
	if Err != nil {
		GoWin.ShowError(Err)
		return
	}
	GoWin.PlayMove(C, EnginePlayer, EnginePlayer)
}
