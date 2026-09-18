package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// handleKeyEvent dispatches keyboard events for game-tree navigation
// (arrow keys, Page Up/Down, Home/End), node deletion (Delete), and
// passing (P). Ignored while a dialog is showing except:
//   - Escape: dismisses the dialog (DismissDialog).
//   - Return/Enter: confirms the dialog (SubmitDialog) unless the focused
//     widget is a multi-line entry (where Enter inserts a newline).
func (GoWin *GoWin) HandleKeyEvent(event *fyne.KeyEvent) {
	if GoWin.DialogShowing {
		switch event.Name {
		case fyne.KeyEscape:
			if GoWin.DismissDialog != nil {
				GoWin.DismissDialog()
			}
		case fyne.KeyReturn, fyne.KeyEnter:
			if GoWin.SubmitDialog != nil {
				Focused := GoWin.Win.Canvas().Focused()
				Entry, IsEntry := Focused.(*widget.Entry)
				if !IsEntry || !Entry.MultiLine {
					GoWin.SubmitDialog()
				}
			}
		}
		return
	}
	switch event.Name {
	case fyne.KeyUp:
		if GoWin.CurrentNode().Parent != nil {
			GoWin.OnUserNavigate()
			GoWin.SetCurrentNode(GoWin.CurrentNode().Parent)
		}
	case fyne.KeyDown:
		if len(GoWin.CurrentNode().Children) > 0 {
			NeedToDrawGoban := GoWin.CurrentNode() == GoWin.Coll.CollectionNode
			GoWin.OnUserNavigate()
			if GoWin.CurrentNode().FavoriteChild == nil {
				GoWin.SetCurrentNode(GoWin.CurrentNode().Children[0])
			} else {
				GoWin.SetCurrentNode(GoWin.CurrentNode().FavoriteChild)
			}
			if NeedToDrawGoban {
				GoWin.DrawGoban()
			}
		}
	case fyne.KeyRight:
		if GoWin.CurrentNode().Parent != nil {
			Siblings := GoWin.CurrentNode().Parent.Children
			if len(Siblings) > 1 {
				for I, Node := range Siblings {
					if Node == GoWin.CurrentNode() {
						GoWin.OnUserNavigate()
						GoWin.SetCurrentNode(Siblings[(I+1)%len(Siblings)])
						break
					}
				}
			}
		}
	case fyne.KeyLeft:
		if GoWin.CurrentNode().Parent != nil {
			Siblings := GoWin.CurrentNode().Parent.Children
			if len(Siblings) > 1 {
				for I, Node := range Siblings {
					if Node == GoWin.CurrentNode() {
						GoWin.OnUserNavigate()
						GoWin.SetCurrentNode(Siblings[(I+len(Siblings)-1)%len(Siblings)])
						break
					}
				}
			}
		}
	case fyne.KeyPageUp:
		if GoWin.Coll.CurrentGameH != nil {
			Root := GoWin.Coll.CurrentGameH.RootNode
			if GoWin.CurrentNode() != Root {
				GoWin.OnUserNavigate()
				GoWin.SetCurrentNode(Root)
			}
		}
	case fyne.KeyPageDown:
		if len(GoWin.CurrentNode().Children) > 0 {
			NeedToDrawGoban := GoWin.CurrentNode() == GoWin.Coll.CollectionNode
			GoWin.OnUserNavigate()
			Path := FavoriteChildPath(GoWin.CurrentNode())
			GoWin.SetCurrentNode(Path[len(Path)-1])
			if NeedToDrawGoban {
				GoWin.DrawGoban()
			}
		}
	case fyne.KeyEnd:
		if GoWin.Coll.CurrentGameH != nil {
			Siblings := GoWin.Coll.CollectionNode.Children
			for I, Sibling := range Siblings {
				if Sibling == GoWin.Coll.CurrentGameH.RootNode {
					Next := Siblings[(I+1)%len(Siblings)]
					GoWin.OnUserNavigate()
					GoWin.SetCurrentNode(Next.Board.Hist.CurrentNode)
					break
				}
			}
		}
	case fyne.KeyHome:
		if GoWin.Coll.CurrentGameH != nil {
			Siblings := GoWin.Coll.CollectionNode.Children
			for I, Sibling := range Siblings {
				if Sibling == GoWin.Coll.CurrentGameH.RootNode {
					Prev := Siblings[(I+len(Siblings)-1)%len(Siblings)]
					GoWin.OnUserNavigate()
					GoWin.SetCurrentNode(Prev.Board.Hist.CurrentNode)
					break
				}
			}
		}
	case fyne.KeyDelete:
		GoWin.OnUserNavigate()
		GoWin.DeleteCurrentNode()
	case fyne.KeyP:
		GoWin.HandlePass()
	}
}

// HandlePass executes a pass move for the next player, provided the game
// is in Play mode and the next player is not engine-controlled.
func (GoWin *GoWin) HandlePass() {
	if GoWin.MouseMode != "Play" || GoWin.CurrentNode().Board == nil {
		return
	}
	NextPlayer := GoWin.CurrentNode().GetNextStonePlacer()
	if GoWin.GtpEngines[NextPlayer] != nil {
		return // Engine controls this player; user may not pass for it.
	}
	GoWin.PlayMove(PassCoords, NextPlayer, 0)
	GoWin.TriggerEngineIfNeeded()
}
