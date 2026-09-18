package main

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NewLibertySharingMatrix allocates a (Players+1)x(Players+1) matrix, filling
// off-diagonal entries with OffDiagonalDefault and diagonal entries with
// SharingUnconditional (a player always shares with themselves).
func NewLibertySharingMatrix(Players uint8, OffDiagonalDefault uint8) *LibertySharingMatrix {
	Size := int(Players) + 1
	Matrix := make(LibertySharingMatrix, Size)
	for RowIndex := 0; RowIndex < Size; RowIndex++ {
		Matrix[RowIndex] = make([]uint8, Size)
		for ColumnIndex := 0; ColumnIndex < Size; ColumnIndex++ {
			if RowIndex == ColumnIndex {
				Matrix[RowIndex][ColumnIndex] = SharingUnconditional
			} else {
				Matrix[RowIndex][ColumnIndex] = OffDiagonalDefault
			}
		}
	}
	return &Matrix
}

// Clone returns a deep copy of M. Returns nil if M is nil.
func (M *LibertySharingMatrix) Clone() *LibertySharingMatrix {
	if M == nil {
		return nil
	}
	Copied := make(LibertySharingMatrix, len(*M))
	for RowIndex, Row := range *M {
		Copied[RowIndex] = make([]uint8, len(Row))
		copy(Copied[RowIndex], Row)
	}
	return &Copied
}

// Equal returns true if M and Other have identical dimensions and cell values.
// Nil-safe: two nil pointers are equal; a nil and a non-nil pointer are not.
func (M *LibertySharingMatrix) Equal(Other *LibertySharingMatrix) bool {
	if M == nil || Other == nil {
		return M == nil && Other == nil
	}
	if len(*M) != len(*Other) {
		return false
	}
	for RowIndex, Row := range *M {
		OtherRow := (*Other)[RowIndex]
		if len(Row) != len(OtherRow) {
			return false
		}
		for ColumnIndex, Cell := range Row {
			if Cell != OtherRow[ColumnIndex] {
				return false
			}
		}
	}
	return true
}

// Get returns the stored sharing state for (RowPlayer, ColumnPlayer), or
// SharingRefused if the indices fall outside the matrix (treating out-of-range
// as no-sharing, which is the safest fallback).
func (M *LibertySharingMatrix) Get(RowPlayer, ColumnPlayer uint8) uint8 {
	if M == nil {
		return SharingRefused
	}
	RowIndex := int(RowPlayer)
	ColumnIndex := int(ColumnPlayer)
	if RowIndex >= len(*M) || ColumnIndex >= len((*M)[RowIndex]) {
		return SharingRefused
	}
	return (*M)[RowIndex][ColumnIndex]
}

// EffectivelyShares resolves the 3-state sharing rule into a boolean:
// Unconditional → true; Refused → false; Conditional → true iff the other
// side's stored state is Conditional or Unconditional.
func (M *LibertySharingMatrix) EffectivelyShares(RowPlayer, ColumnPlayer uint8) bool {
	if RowPlayer == ColumnPlayer {
		return true
	}
	switch M.Get(RowPlayer, ColumnPlayer) {
	case SharingUnconditional:
		return true
	case SharingRefused:
		return false
	case SharingConditional:
		Other := M.Get(ColumnPlayer, RowPlayer)
		return Other == SharingConditional || Other == SharingUnconditional
	}
	return false
}

// SharedSet returns all players k (including Owner itself) such that k
// effectively shares its liberties with Owner — i.e., the set of colors whose
// stones are walked through when flood-filling Owner's group. "X shares
// liberties with Y" means X donates its liberties to Y, so Y benefits when
// computing Y's group.
func (M *LibertySharingMatrix) SharedSet(Owner uint8) []uint8 {
	if M == nil {
		return []uint8{Owner}
	}
	Result := []uint8{}
	for PlayerIndex := uint8(1); int(PlayerIndex) < len(*M); PlayerIndex++ {
		if M.EffectivelyShares(PlayerIndex, Owner) {
			Result = append(Result, PlayerIndex)
		}
	}
	return Result
}

// HasAnySharing returns true if any off-diagonal pair effectively shares.
// Used to gate SGF export and GTP engine attach.
func (M *LibertySharingMatrix) HasAnySharing() bool {
	if M == nil {
		return false
	}
	for RowIndex := 1; RowIndex < len(*M); RowIndex++ {
		for ColumnIndex := 1; ColumnIndex < len((*M)[RowIndex]); ColumnIndex++ {
			if RowIndex == ColumnIndex {
				continue
			}
			if M.EffectivelyShares(uint8(RowIndex), uint8(ColumnIndex)) {
				return true
			}
		}
	}
	return false
}

// Resize returns a copy of M sized for NewPlayers, padding new rows/cols with
// SharingRefused and truncating any extras. Used when the Fresh Board dialog
// changes the player count after the matrix editor was opened.
func (M *LibertySharingMatrix) Resize(NewPlayers uint8) *LibertySharingMatrix {
	Resized := NewLibertySharingMatrix(NewPlayers, SharingRefused)
	if M == nil {
		return Resized
	}
	NewSize := int(NewPlayers) + 1
	for RowIndex := 0; RowIndex < len(*M) && RowIndex < NewSize; RowIndex++ {
		Row := (*M)[RowIndex]
		for ColumnIndex := 0; ColumnIndex < len(Row) && ColumnIndex < NewSize; ColumnIndex++ {
			if RowIndex == ColumnIndex {
				continue
			}
			(*Resized)[RowIndex][ColumnIndex] = Row[ColumnIndex]
		}
	}
	return Resized
}

// MarshalJSON encodes the matrix as a 2D array of symbolic strings
// ("No" / "Conditionally" / "Yes").
func (M LibertySharingMatrix) MarshalJSON() ([]byte, error) {
	StringRows := make([][]string, len(M))
	for RowIndex, Row := range M {
		StringRow := make([]string, len(Row))
		for ColumnIndex, Cell := range Row {
			Name, Found := SharingStateNames[Cell]
			if !Found {
				Name = "No"
			}
			StringRow[ColumnIndex] = Name
		}
		StringRows[RowIndex] = StringRow
	}
	return json.Marshal(StringRows)
}

// UnmarshalJSON decodes the 2D symbolic-string form produced by MarshalJSON.
// Unknown strings fall back to SharingRefused. Diagonal entries are forced to
// SharingUnconditional (the invariant "a player always shares with self").
func (M *LibertySharingMatrix) UnmarshalJSON(Data []byte) error {
	var StringRows [][]string
	if Err := json.Unmarshal(Data, &StringRows); Err != nil {
		return Err
	}
	Decoded := make(LibertySharingMatrix, len(StringRows))
	for RowIndex, StringRow := range StringRows {
		Row := make([]uint8, len(StringRow))
		for ColumnIndex, Name := range StringRow {
			if RowIndex == ColumnIndex {
				Row[ColumnIndex] = SharingUnconditional
				continue
			}
			State, Found := SharingStateByName[Name]
			if !Found {
				State = SharingRefused
			}
			Row[ColumnIndex] = State
		}
		Decoded[RowIndex] = Row
	}
	*M = Decoded
	return nil
}

// NewSharingCellSelect returns a widget.Select with the three sharing options
// labelled "No" / "Conditionally" / "Yes". OnChange is invoked with the uint8
// constant corresponding to the newly selected label.
func NewSharingCellSelect(Current uint8, OnChange func(uint8)) *widget.Select {
	CurrentName, Found := SharingStateNames[Current]
	if !Found {
		CurrentName = "No"
	}
	Select := widget.NewSelect(SharingStateOrder, func(Selected string) {
		State, NameFound := SharingStateByName[Selected]
		if !NameFound {
			State = SharingRefused
		}
		if OnChange != nil {
			OnChange(State)
		}
	})
	Select.SetSelected(CurrentName)
	return Select
}

// CompactSharingCell is a minimal tappable cell used by the N×N matrix editor.
// It shows a one-letter label (N / C / Y) and cycles through the three sharing
// states on each tap. Far smaller than a widget.Select: no chevron, no
// dropdown padding — just the letter.
type CompactSharingCell struct {
	widget.BaseWidget
	State    uint8
	OnChange func(uint8)
	Label    *widget.Label
}

// NewCompactSharingCell constructs a CompactSharingCell initialized at Current.
// OnChange fires with the new state after each tap (N → C → Y → N …).
func NewCompactSharingCell(Current uint8, OnChange func(uint8)) *CompactSharingCell {
	Text, Found := SharingStateShortNames[Current]
	if !Found {
		Text = "N"
	}
	Cell := &CompactSharingCell{
		State:    Current,
		OnChange: OnChange,
		Label:    widget.NewLabelWithStyle(Text, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	}
	Cell.ExtendBaseWidget(Cell)
	return Cell
}

// Tapped advances the state and updates the visible letter.
func (C *CompactSharingCell) Tapped(*fyne.PointEvent) {
	C.State = (C.State + 1) % 3
	Text, Found := SharingStateShortNames[C.State]
	if !Found {
		Text = "N"
	}
	C.Label.SetText(Text)
	if C.OnChange != nil {
		C.OnChange(C.State)
	}
}

// CreateRenderer wires the cell's internal label into the widget tree.
func (C *CompactSharingCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(C.Label)
}

// FormatSharingEffectiveLabel returns a short, human-readable description of
// the effective outcome between RowPlayer and ColumnPlayer under M, using the
// supplied per-player display names from GameHistory.PlayerNames.
func FormatSharingEffectiveLabel(M *LibertySharingMatrix, RowPlayer, ColumnPlayer uint8, PlayerNames map[uint8]string) string {
	RowName := FormatPlayerName(RowPlayer, PlayerNames)
	ColumnName := FormatPlayerName(ColumnPlayer, PlayerNames)
	if M.EffectivelyShares(RowPlayer, ColumnPlayer) {
		return fmt.Sprintf("%s shares with %s", RowName, ColumnName)
	}
	return fmt.Sprintf("%s does not share with %s", RowName, ColumnName)
}

// FormatPlayerName returns "Player N (Name)" if a name is set, else "Player N".
func FormatPlayerName(Player uint8, PlayerNames map[uint8]string) string {
	if Name, HasName := PlayerNames[Player]; HasName && Name != "" {
		return fmt.Sprintf("Player %d (%s)", Player, Name)
	}
	return fmt.Sprintf("Player %d", Player)
}

// ShowLibertySharingMatrixEditor opens a dialog presenting an N×N grid where
// each cell selects how the row player (the player sharing) shares liberties
// with the column player (the receiving player). Diagonal cells are locked at
// SharingUnconditional. Edits commit directly into the supplied Matrix
// (already sized for PlayerCount).
func ShowLibertySharingMatrixEditor(Win *GoWin, Matrix *LibertySharingMatrix, PlayerCount uint8, PlayerNames map[uint8]string) {
	if Matrix == nil || PlayerCount == 0 {
		return
	}
	Header := widget.NewLabelWithStyle(
		"The ROW player access to donates its group's liberties to the COLUMN player.\nTap a cell to cycle: N = No, C = Conditionally (shares iff the other side is C or Y), Y = Yes.\nDiagonal is locked at Y.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	GridColumns := int(PlayerCount) + 1
	GridObjects := make([]fyne.CanvasObject, 0, GridColumns*GridColumns)

	GridObjects = append(GridObjects, widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	for ColumnPlayer := uint8(1); ColumnPlayer <= PlayerCount; ColumnPlayer++ {
		GridObjects = append(GridObjects, widget.NewLabelWithStyle(
			fmt.Sprintf("%d", ColumnPlayer),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	}

	for RowPlayer := uint8(1); RowPlayer <= PlayerCount; RowPlayer++ {
		GridObjects = append(GridObjects, widget.NewLabelWithStyle(
			fmt.Sprintf("%d", RowPlayer),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		for ColumnPlayer := uint8(1); ColumnPlayer <= PlayerCount; ColumnPlayer++ {
			if RowPlayer == ColumnPlayer {
				GridObjects = append(GridObjects, widget.NewLabelWithStyle("—", fyne.TextAlignCenter, fyne.TextStyle{}))
				continue
			}
			RP, CP := RowPlayer, ColumnPlayer
			Cell := NewCompactSharingCell(Matrix.Get(RP, CP), func(NewState uint8) {
				(*Matrix)[RP][CP] = NewState
			})
			GridObjects = append(GridObjects, Cell)
		}
	}

	Grid := container.NewGridWithColumns(GridColumns, GridObjects...)
	Content := container.NewScroll(container.NewVBox(Header, widget.NewSeparator(), Grid))
	Content.SetMinSize(fyne.NewSize(480, 320))

	Dlg := dialog.NewCustom("Liberty Sharing Matrix", "Close", Content, Win.Win)
	Dlg.Resize(Win.Win.Canvas().Size())
	Dlg.Show()
}
