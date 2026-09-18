package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowDiplomacyDialog opens the Game > Diplomacy editor for the player whose
// turn is next. Edits a clone of CurrentNode.Board.NextLibertySharingMatrix
// and, on OK, only replaces the pointer if any cell actually changed
// (preserving sharing across nodes when no edit was made).
//
// A player can only edit their own outgoing sharing row — the cells edited
// here are M[NextStonePlacer][OtherPlayer] for every other player.
func ShowDiplomacyDialog(Win *GoWin) {
	if Win.DialogShowing {
		return
	}
	Current := Win.CurrentNode()
	if Current == nil || Current.Board == nil || Win.Coll.CurrentGameH == nil {
		return
	}
	Hist := Win.Coll.CurrentGameH
	if Hist.LibertySharingFixed {
		return
	}

	Self := Current.GetNextStonePlacer()
	if Self == 0 || Self > Hist.Players {
		return
	}

	Original := Current.Board.NextLibertySharingMatrix
	Edited := Original.Clone()
	if Edited == nil {
		Edited = NewLibertySharingMatrix(Hist.Players, SharingRefused)
	}

	Header := widget.NewLabelWithStyle(
		"Donate access to your liberties to which players?",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// EffectiveLabels[OtherPlayer] is updated live as the user changes cells.
	EffectiveLabels := map[uint8]*widget.Label{}

	UpdateEffectiveLabel := func(OtherPlayer uint8) {
		Label := EffectiveLabels[OtherPlayer]
		if Label == nil {
			return
		}
		if Edited.EffectivelyShares(Self, OtherPlayer) {
			Label.SetText("→ share")
		} else {
			Label.SetText("→ do not share")
		}
	}

	Rows := []fyne.CanvasObject{Header, widget.NewSeparator()}
	for Other := uint8(1); Other <= Hist.Players; Other++ {
		if Other == Self {
			continue
		}
		OtherLabel := widget.NewLabel(fmt.Sprintf("%d", Other))
		EffectiveLabel := widget.NewLabel("")
		EffectiveLabels[Other] = EffectiveLabel
		Op := Other
		Cell := NewSharingCellSelect(Edited.Get(Self, Op), func(NewState uint8) {
			(*Edited)[Self][Op] = NewState
			UpdateEffectiveLabel(Op)
		})
		UpdateEffectiveLabel(Op)
		Rows = append(Rows, container.NewBorder(nil, nil, OtherLabel, EffectiveLabel, Cell))
	}

	Content := container.NewVScroll(container.NewVBox(Rows...))
	Content.SetMinSize(fyne.NewSize(520, 360))

	Win.DialogShowing = true
	Dlg := dialog.NewCustomConfirm(fmt.Sprintf("%s — Diplomacy", FormatPlayerName(Self, Hist.PlayerNames)),
		"OK", "Cancel", Content,
		func(Ok bool) {
			Win.DialogClosed()
			if !Ok {
				return
			}
			if !Edited.Equal(Original) {
				Current.Board.NextLibertySharingMatrix = Edited
				// Recompute Groups and invalidate the legality cache so UI
				// (liberty-line colors, hover group, legality dots) reflects
				// the new sharing immediately. Vertices are intentionally
				// left alone: groups with zero liberties under the new
				// matrix are NOT captured here — captures happen only when
				// a stone is actually played.
				Current.Board.CalculateAllGroups()
				Current.Board.IsPlacementLegalCache = nil
				Win.DrawBoard()
			}
		}, Win.Win)
	Win.DismissDialog = func() { Dlg.Hide() }
	Win.SubmitDialog = func() { Dlg.Confirm() }
	Dlg.Show()
}
