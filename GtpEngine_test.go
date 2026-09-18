package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

func TestPlayerEngineSelection(T *testing.T) {
	OriginalConfig := CurrentAppConfig
	T.Cleanup(func() { CurrentAppConfig = OriginalConfig })
	TestDir := T.TempDir()
	ScriptPath := filepath.Join(TestDir, "engine")
	ThinkingPath := filepath.Join(TestDir, "thinking")
	Script := "#!/bin/sh\nwhile read -r Command; do\ncase \"$Command\" in\nlist_commands) printf '= boardsize\\nkomi\\nplay\\ngenmove\\n\\n';;\ngenmove*) touch \"$1\"; while :; do :; done;;\n*) printf '=\\n\\n';;\nesac\ndone\n"
	if Err := os.WriteFile(ScriptPath, []byte(Script), 0700); Err != nil {
		T.Fatal(Err)
	}
	CurrentAppConfig.Engines = map[string]EngineConfig{"Zulu": {GtpPath: ScriptPath, GtpArgs: ThinkingPath}, "Alpha": {GtpPath: ScriptPath, GtpArgs: ThinkingPath}}
	CurrentAppConfig.GtpLogging = false
	Hist := &GameHistory{Players: 2, Width: 9, Height: 9, LibertySharingFixed: true, RuleSet: &RuleSet{StonePlacementsPerMove: 1}}
	Node := &GameTreeNode{Board: &BoardState{Hist: Hist}, NextStonePlacer: 1}
	Hist.RootNode, Hist.CurrentNode = Node, Node
	Win := &GoWin{Coll: &Collection{CurrentGameH: Hist}, PlayerEngineItems: []*fyne.MenuItem{fyne.NewMenuItem("Player 1", nil), fyne.NewMenuItem("Player 2", nil)}}
	T.Cleanup(Win.DetachAllEngines)
	Win.RefreshEngineMenuItems()
	Items := Win.PlayerEngineItems[1].ChildMenu.Items
	if len(Items) != 3 || Items[0].Label != "Human" || !Items[0].Checked || Items[1].Label != "Alpha" || Items[2].Label != "Zulu" {
		T.Fatal("expected Human followed by sorted engine names")
	}
	Items[1].Action()
	FirstEngine := Win.GtpEngines[2]
	if FirstEngine == nil || !Win.PlayerEngineItems[1].ChildMenu.Items[1].Checked {
		T.Fatal("selection did not attach and check engine")
	}
	Win.PlayerEngineItems[1].ChildMenu.Items[1].Action()
	SecondEngine := Win.GtpEngines[2]
	if SecondEngine == nil || SecondEngine == FirstEngine || FirstEngine.Cmd.ProcessState == nil {
		T.Fatal("selecting checked engine must replace its process")
	}
	Done := make(chan error, 1)
	go func() { _, Err := SendGtpCommandToEngine(SecondEngine, "genmove W"); Done <- Err }()
	Deadline := time.Now().Add(3 * time.Second)
	for {
		if _, Err := os.Stat(ThinkingPath); Err == nil {
			break
		}
		if time.Now().After(Deadline) {
			T.Fatal("engine did not receive genmove")
		}
		time.Sleep(time.Millisecond)
	}
	Win.PlayerEngineItems[1].ChildMenu.Items[0].Action()
	select {
	case Err := <-Done:
		if Err == nil {
			T.Fatal("detached genmove should fail")
		}
	case <-time.After(3 * time.Second):
		T.Fatal("detaching did not unblock GTP")
	}
	if Win.GtpEngines[2] != nil || !Win.PlayerEngineItems[1].ChildMenu.Items[0].Checked {
		T.Fatal("detachment must select Human")
	}
	Hist.Players = 3
	Win.RefreshEngineMenuItems()
	if !Win.PlayerEngineItems[1].ChildMenu.Items[1].Disabled || Win.PlayerEngineItems[1].ChildMenu.Items[0].Disabled {
		T.Fatal("unsupported games must still allow Human")
	}
}
