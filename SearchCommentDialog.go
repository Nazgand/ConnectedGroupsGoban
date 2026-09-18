package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// CommentSearchMatch is a single matching node produced by ShowSearchCommentDialog.
type CommentSearchMatch struct {
	Hist       *GameHistory
	GameIndex  int
	GameName   string
	Node       *GameTreeNode
	MoveNumber int
	Snippet    string
}

// ShowSearchCommentDialog opens a dialog that searches GameTreeNode.Comment
// for a case-insensitive substring. By default only the current game is
// searched; a checkbox extends the search to every game in the collection.
// Matches are shown as a clickable list; selecting a row and clicking Go
// navigates to that node via SetCurrentNode (which also switches the active
// game if the match is in a different one).
func (GoWin *GoWin) ShowSearchCommentDialog() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	QueryEntry := NewDialogEntry(GoWin)
	QueryEntry.SetPlaceHolder("substring (case-insensitive)")

	AllGamesCheck := widget.NewCheck("Search all games in collection", nil)

	Matches := []CommentSearchMatch{}
	SelectedIndex := -1

	StatusLabel := widget.NewLabel("Enter text to search.")

	ResultsList := widget.NewList(
		func() int { return len(Matches) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(Index widget.ListItemID, Item fyne.CanvasObject) {
			Label := Item.(*widget.Label)
			if Index < 0 || Index >= len(Matches) {
				Label.SetText("")
				return
			}
			M := Matches[Index]
			Label.SetText(fmt.Sprintf("[%s] Move %d — %s", M.GameName, M.MoveNumber, M.Snippet))
		},
	)
	ResultsList.OnSelected = func(Index widget.ListItemID) { SelectedIndex = Index }
	ResultsList.OnUnselected = func(Index widget.ListItemID) {
		if SelectedIndex == Index {
			SelectedIndex = -1
		}
	}

	RunSearch := func() {
		Matches = Matches[:0]
		SelectedIndex = -1
		ResultsList.UnselectAll()

		Needle := strings.ToLower(strings.TrimSpace(QueryEntry.Text))
		if Needle == "" {
			StatusLabel.SetText("Enter text to search.")
			ResultsList.Refresh()
			return
		}

		FindGameIndex := func(Hist *GameHistory) int {
			for I, Child := range GoWin.Coll.CollectionNode.Children {
				if Child.Board != nil && Child.Board.Hist == Hist {
					return I
				}
			}
			return -1
		}

		WalkGame := func(Hist *GameHistory, GameIndex int) {
			Name := GameDisplayName(Hist, GameIndex)
			var Recurse func(Node *GameTreeNode, Depth int)
			Recurse = func(Node *GameTreeNode, Depth int) {
				if Node == nil {
					return
				}
				if Node.Comment != "" && strings.Contains(strings.ToLower(Node.Comment), Needle) {
					Matches = append(Matches, CommentSearchMatch{
						Hist:       Hist,
						GameIndex:  GameIndex,
						GameName:   Name,
						Node:       Node,
						MoveNumber: Depth,
						Snippet:    CommentSnippet(Node.Comment, Needle),
					})
				}
				for _, Child := range Node.Children {
					Recurse(Child, Depth+1)
				}
			}
			Recurse(Hist.RootNode, 0)
		}

		if AllGamesCheck.Checked {
			for Index, Child := range GoWin.Coll.CollectionNode.Children {
				if Child.Board == nil || Child.Board.Hist == nil {
					continue
				}
				WalkGame(Child.Board.Hist, Index)
			}
		} else if GoWin.Coll.CurrentGameH != nil {
			WalkGame(GoWin.Coll.CurrentGameH, FindGameIndex(GoWin.Coll.CurrentGameH))
		}

		StatusLabel.SetText(fmt.Sprintf("%d match(es)", len(Matches)))
		ResultsList.Refresh()
	}

	QueryEntry.OnChanged = func(_ string) { RunSearch() }
	AllGamesCheck.OnChanged = func(_ bool) { RunSearch() }

	LabelField := func(Caption string, W fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewLabel(Caption), nil, W)
	}
	Header := container.NewVBox(
		LabelField("Comment contains:", QueryEntry),
		AllGamesCheck,
		StatusLabel,
		widget.NewSeparator(),
	)

	Content := container.NewBorder(Header, nil, nil, nil, ResultsList)

	Dlg := dialog.NewCustomConfirm("Search Comments", "Go", "Close", Content,
		func(Confirmed bool) {
			GoWin.DialogClosed()
			if !Confirmed {
				return
			}
			if SelectedIndex < 0 || SelectedIndex >= len(Matches) {
				return
			}
			Target := Matches[SelectedIndex].Node
			if Target == nil {
				return
			}
			GoWin.OnUserNavigate()
			GoWin.SetCurrentNode(Target)
		},
		GoWin.Win)

	GoWin.DismissDialog = func() { Dlg.Hide() }
	GoWin.SubmitDialog = func() { Dlg.Confirm() }
	GoWin.ResizeDialog = func() { Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	GoWin.WireSubmitOnEnter(QueryEntry)
	Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	Dlg.Show()
	GoWin.Win.Canvas().Focus(QueryEntry)
}

// CommentSnippet returns a one-line preview of Comment centered (when possible)
// on the first occurrence of LowerNeedle in the lower-cased comment. Newlines
// are collapsed to spaces and the result is clamped to ~120 bytes with leading
// / trailing ellipses when the comment was truncated.
func CommentSnippet(Comment string, LowerNeedle string) string {
	OneLine := strings.ReplaceAll(Comment, "\n", " ")
	OneLine = strings.ReplaceAll(OneLine, "\r", " ")
	OneLine = strings.TrimSpace(OneLine)

	const Window = 120
	if len(OneLine) <= Window {
		return OneLine
	}

	MatchIndex := strings.Index(strings.ToLower(OneLine), LowerNeedle)
	if MatchIndex < 0 {
		return OneLine[:Window] + "…"
	}

	Start := max(MatchIndex-Window/3, 0)
	End := Start + Window
	if End > len(OneLine) {
		End = len(OneLine)
		Start = max(End-Window, 0)
	}

	Prefix := ""
	Suffix := ""
	if Start > 0 {
		Prefix = "…"
	}
	if End < len(OneLine) {
		Suffix = "…"
	}
	return Prefix + OneLine[Start:End] + Suffix
}
