package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// CollectionRow is one row of the Collection dialog table: all values are
// pre-rendered so the table's update callback does zero work per repaint.
type CollectionRow struct {
	Hist           *GameHistory
	Index          int
	Name           string
	Players        string
	RuleSet        string
	KomiStr        string
	Size           string
	Moves          int
	RootUnixMilli  int64
	RootTime       string
	Date           string
	Result         string
	Event          string
	PlayerNamesLow []string
	NameLow        string
	RuleSetLow     string
	KomiStrLow     string
	CommentsLow    string
	KomiSort       float64
}

// CollectionColumn describes a sortable/displayable column.
type CollectionColumn struct {
	Title string
	Width float32
	Cell  func(Row *CollectionRow) string
	Less  func(A, B *CollectionRow) bool
}

// BuildCollectionRows materializes a CollectionRow for every valid game child
// of CollectionNode. Children with nil Board/Hist are skipped.
func (GoWin *GoWin) BuildCollectionRows() []CollectionRow {
	ShownRS := CurrentAppConfig.ShownRuleSets()
	Rows := []CollectionRow{}
	for ChildIndex, Child := range GoWin.Coll.CollectionNode.Children {
		if Child.Board == nil || Child.Board.Hist == nil {
			continue
		}
		Hist := Child.Board.Hist

		Separator := " vs "
		if Hist.Players > 2 {
			Separator = ", "
		}
		PlayerParts := make([]string, 0, Hist.Players)
		PlayerLow := make([]string, 0, Hist.Players)
		for P := uint8(1); P <= Hist.Players; P++ {
			Name := Hist.PlayerNames[P]
			PlayerParts = append(PlayerParts, Name)
			PlayerLow = append(PlayerLow, strings.ToLower(Name))
		}
		PlayersStr := strings.Join(PlayerParts, Separator)

		KomiParts := make([]string, 0, Hist.Players)
		MaxKomi := math.Inf(-1)
		for P := uint8(1); P <= Hist.Players; P++ {
			K, Exists := Hist.Komi[P]
			if !Exists {
				K = 0
				KomiParts = append(KomiParts, "0")
			} else {
				KomiParts = append(KomiParts, FormatPoints(K))
			}
			if K > MaxKomi {
				MaxKomi = K
			}
		}
		KomiStr := strings.Join(KomiParts, ", ")

		RuleSetStr := ""
		if Hist.RuleSet != nil {
			RuleSetStr = RuleSetDisplayName(Hist.RuleSet, ShownRS)
		}

		MoveCount := len(FavoriteChildPath(Hist.RootNode)) - 1

		var CommentsBuilder strings.Builder
		CollectComments(Hist.RootNode, &CommentsBuilder)
		CommentsLow := strings.ToLower(CommentsBuilder.String())

		RootUnixMilli := int64(0)
		if Hist.RootNode != nil {
			RootUnixMilli = Hist.RootNode.UnixMilli
		}
		RootTime := "—"
		if RootUnixMilli != 0 {
			RootTime = time.UnixMilli(RootUnixMilli).Local().Format("2006-01-02 15:04:05")
		}

		Get := func(Key string) string {
			if Val, Ok := Hist.Information[Key]; Ok && Val != nil {
				return *Val
			}
			return ""
		}

		Name := GameDisplayName(Hist, ChildIndex)
		Rows = append(Rows, CollectionRow{
			Hist:           Hist,
			Index:          ChildIndex,
			Name:           Name,
			Players:        PlayersStr,
			RuleSet:        RuleSetStr,
			KomiStr:        KomiStr,
			Size:           fmt.Sprintf("%d×%d", Hist.Width, Hist.Height),
			Moves:          MoveCount,
			RootUnixMilli:  RootUnixMilli,
			RootTime:       RootTime,
			Date:           Get("Date And Time"),
			Result:         Get("Result"),
			Event:          Get("Event"),
			PlayerNamesLow: PlayerLow,
			NameLow:        strings.ToLower(Name),
			RuleSetLow:     strings.ToLower(RuleSetStr),
			KomiStrLow:     strings.ToLower(KomiStr),
			CommentsLow:    CommentsLow,
			KomiSort:       MaxKomi,
		})
	}
	return Rows
}

// CollectComments depth-first walks the game tree rooted at Node and appends
// every non-empty Comment to Builder (newline-separated, lowercased by caller).
func CollectComments(Node *GameTreeNode, Builder *strings.Builder) {
	if Node == nil {
		return
	}
	if Node.Comment != "" {
		Builder.WriteString(Node.Comment)
		Builder.WriteByte('\n')
	}
	for _, Child := range Node.Children {
		CollectComments(Child, Builder)
	}
}

// ShowCollectionDialog opens a filterable/sortable table of every game in
// GoWin.Coll. Selecting a row and confirming with Go switches the active game
// to the selected one.
func (GoWin *GoWin) ShowCollectionDialog() {
	if GoWin.DialogShowing {
		return
	}
	GoWin.DialogShowing = true

	AllRows := GoWin.BuildCollectionRows()
	Filtered := make([]CollectionRow, len(AllRows))
	copy(Filtered, AllRows)

	Columns := []CollectionColumn{
		{Title: "Game Name", Width: 220,
			Cell: func(R *CollectionRow) string { return R.Name },
			Less: func(A, B *CollectionRow) bool { return A.NameLow < B.NameLow }},
		{Title: "Players", Width: 240,
			Cell: func(R *CollectionRow) string { return R.Players },
			Less: func(A, B *CollectionRow) bool { return strings.ToLower(A.Players) < strings.ToLower(B.Players) }},
		{Title: "Rule Set", Width: 160,
			Cell: func(R *CollectionRow) string { return R.RuleSet },
			Less: func(A, B *CollectionRow) bool { return A.RuleSetLow < B.RuleSetLow }},
		{Title: "Komi", Width: 160,
			Cell: func(R *CollectionRow) string { return R.KomiStr },
			Less: func(A, B *CollectionRow) bool { return A.KomiSort < B.KomiSort }},
		{Title: "Size", Width: 80,
			Cell: func(R *CollectionRow) string { return R.Size },
			Less: func(A, B *CollectionRow) bool {
				if A.Hist.Width != B.Hist.Width {
					return A.Hist.Width < B.Hist.Width
				}
				return A.Hist.Height < B.Hist.Height
			}},
		{Title: "Moves", Width: 80,
			Cell: func(R *CollectionRow) string { return strconv.Itoa(R.Moves) },
			Less: func(A, B *CollectionRow) bool { return A.Moves < B.Moves }},
		{Title: "Root Time", Width: 160,
			Cell: func(R *CollectionRow) string { return R.RootTime },
			Less: func(A, B *CollectionRow) bool { return A.RootUnixMilli < B.RootUnixMilli }},
		{Title: "Date", Width: 140,
			Cell: func(R *CollectionRow) string { return R.Date },
			Less: func(A, B *CollectionRow) bool { return strings.ToLower(A.Date) < strings.ToLower(B.Date) }},
		{Title: "Result", Width: 100,
			Cell: func(R *CollectionRow) string { return R.Result },
			Less: func(A, B *CollectionRow) bool { return strings.ToLower(A.Result) < strings.ToLower(B.Result) }},
		{Title: "Event", Width: 180,
			Cell: func(R *CollectionRow) string { return R.Event },
			Less: func(A, B *CollectionRow) bool { return strings.ToLower(A.Event) < strings.ToLower(B.Event) }},
	}

	SortColumn := -1
	SortDescending := false
	SelectedRow := -1

	// Forward-declared so the table's header OnTapped can invoke it after
	// the table itself is constructed below.
	var RefreshTable func()

	// Filter widgets.
	NameEntry := NewDialogEntry(GoWin)
	NameEntry.SetPlaceHolder("substring")

	PlayerAEntry := NewDialogEntry(GoWin)
	PlayerAEntry.SetPlaceHolder("substring matching any player")
	PlayerBEntry := NewDialogEntry(GoWin)
	PlayerBEntry.SetPlaceHolder("substring matching any player")
	PlayerCEntry := NewDialogEntry(GoWin)
	PlayerCEntry.SetPlaceHolder("substring matching any player")

	// Rule set filter: (any) plus distinct rule-set names present in the collection.
	RuleSetOptions := []string{"(any)"}
	RuleSetSeen := map[string]bool{}
	for _, R := range AllRows {
		if R.RuleSet != "" && !RuleSetSeen[R.RuleSet] {
			RuleSetSeen[R.RuleSet] = true
			RuleSetOptions = append(RuleSetOptions, R.RuleSet)
		}
	}
	RuleSetSelect := widget.NewSelect(RuleSetOptions, nil)
	RuleSetSelect.SetSelectedIndex(0)

	KomiEntry := NewDialogEntry(GoWin)
	KomiEntry.SetPlaceHolder("exact float or substring")

	MinSizeEntry := NewDialogEntry(GoWin)
	MinSizeEntry.SetText("1")
	MaxSizeEntry := NewDialogEntry(GoWin)
	MaxSizeEntry.SetText("255")

	ApproxUnixMilliEntry := NewDialogEntry(GoWin)
	ApproxUnixMilliEntry.SetText(strconv.FormatInt(time.Now().UnixMilli(), 10))
	MaxErrorUnixMilliEntry := NewDialogEntry(GoWin)
	// 1<<62 ms ≈ 146 million years: effectively unbounded while staying within int64.
	MaxErrorUnixMilliEntry.SetText(strconv.FormatInt(int64(1)<<62, 10))

	CommentEntry := NewDialogEntry(GoWin)
	CommentEntry.SetPlaceHolder("substring in any node comment")

	StatusLabel := widget.NewLabel("")

	// Table.
	var Table *widget.Table
	Table = widget.NewTable(
		func() (int, int) { return len(Filtered), len(Columns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(Id widget.TableCellID, Cell fyne.CanvasObject) {
			Label := Cell.(*widget.Label)
			if Id.Row < 0 || Id.Row >= len(Filtered) || Id.Col < 0 || Id.Col >= len(Columns) {
				Label.SetText("")
				return
			}
			Label.SetText(Columns[Id.Col].Cell(&Filtered[Id.Row]))
		},
	)
	Table.ShowHeaderRow = true
	Table.StickyRowCount = 0
	Table.CreateHeader = func() fyne.CanvasObject {
		Btn := widget.NewButton("", nil)
		return Btn
	}
	Table.UpdateHeader = func(Id widget.TableCellID, Template fyne.CanvasObject) {
		Btn := Template.(*widget.Button)
		if Id.Col < 0 || Id.Col >= len(Columns) {
			Btn.SetText("")
			Btn.OnTapped = nil
			return
		}
		Title := Columns[Id.Col].Title
		if Id.Col == SortColumn {
			if SortDescending {
				Title += " ▼"
			} else {
				Title += " ▲"
			}
		}
		Btn.SetText(Title)
		TappedColumn := Id.Col
		Btn.OnTapped = func() {
			if SortColumn == TappedColumn {
				SortDescending = !SortDescending
			} else {
				SortColumn = TappedColumn
				SortDescending = false
			}
			RefreshTable()
		}
	}
	for ColIndex, Col := range Columns {
		Table.SetColumnWidth(ColIndex, Col.Width)
	}
	Table.OnSelected = func(Id widget.TableCellID) {
		if Id.Row < 0 || Id.Row >= len(Filtered) {
			SelectedRow = -1
			return
		}
		SelectedRow = Id.Row
	}
	Table.OnUnselected = func(_ widget.TableCellID) { SelectedRow = -1 }

	// RefreshTable re-applies filters, sorts, updates status, and refreshes the widget.
	RefreshTable = func() {
		NameNeedle := strings.ToLower(strings.TrimSpace(NameEntry.Text))
		PlayerNeedles := []string{
			strings.ToLower(strings.TrimSpace(PlayerAEntry.Text)),
			strings.ToLower(strings.TrimSpace(PlayerBEntry.Text)),
			strings.ToLower(strings.TrimSpace(PlayerCEntry.Text)),
		}
		RuleSetNeedle := ""
		if RuleSetSelect.SelectedIndex() > 0 {
			RuleSetNeedle = RuleSetSelect.Selected
		}
		KomiText := strings.TrimSpace(KomiEntry.Text)
		KomiExact, KomiExactOk := math.NaN(), false
		if KomiText != "" {
			if V, Err := strconv.ParseFloat(KomiText, 64); Err == nil {
				KomiExact, KomiExactOk = V, true
			}
		}
		KomiSub := strings.ToLower(KomiText)

		ParseU8 := func(S string, Default uint8) uint8 {
			V, Err := strconv.ParseUint(strings.TrimSpace(S), 10, 8)
			if Err != nil {
				return Default
			}
			return uint8(V)
		}
		MinSize := ParseU8(MinSizeEntry.Text, 1)
		MaxSize := ParseU8(MaxSizeEntry.Text, 255)

		ParseI64 := func(S string, Default int64) int64 {
			V, Err := strconv.ParseInt(strings.TrimSpace(S), 10, 64)
			if Err != nil {
				return Default
			}
			return V
		}
		Approx := ParseI64(ApproxUnixMilliEntry.Text, time.Now().UnixMilli())
		MaxError := ParseI64(MaxErrorUnixMilliEntry.Text, int64(1)<<62)

		CommentNeedle := strings.ToLower(strings.TrimSpace(CommentEntry.Text))

		PassesPlayerFilter := func(Row *CollectionRow, Needle string) bool {
			if Needle == "" {
				return true
			}
			for _, Name := range Row.PlayerNamesLow {
				if strings.Contains(Name, Needle) {
					return true
				}
			}
			return false
		}

		Filtered = Filtered[:0]
		for RowIndex := range AllRows {
			R := &AllRows[RowIndex]
			if NameNeedle != "" && !strings.Contains(R.NameLow, NameNeedle) {
				continue
			}
			MatchedAllPlayers := true
			for _, Needle := range PlayerNeedles {
				if !PassesPlayerFilter(R, Needle) {
					MatchedAllPlayers = false
					break
				}
			}
			if !MatchedAllPlayers {
				continue
			}
			if RuleSetNeedle != "" && R.RuleSet != RuleSetNeedle {
				continue
			}
			if KomiText != "" {
				if KomiExactOk {
					MatchesExact := false
					for P := uint8(1); P <= R.Hist.Players; P++ {
						if K, Ok := R.Hist.Komi[P]; Ok && K == KomiExact {
							MatchesExact = true
							break
						}
					}
					if !MatchesExact {
						continue
					}
				} else if !strings.Contains(R.KomiStrLow, KomiSub) {
					continue
				}
			}
			if R.Hist.Width < MinSize || R.Hist.Width > MaxSize ||
				R.Hist.Height < MinSize || R.Hist.Height > MaxSize {
				continue
			}
			if R.RootUnixMilli != 0 {
				Delta := R.RootUnixMilli - Approx
				if Delta < 0 {
					Delta = -Delta
				}
				if Delta > MaxError {
					continue
				}
			}
			if CommentNeedle != "" && !strings.Contains(R.CommentsLow, CommentNeedle) {
				continue
			}
			Filtered = append(Filtered, *R)
		}

		if SortColumn >= 0 && SortColumn < len(Columns) {
			Less := Columns[SortColumn].Less
			if SortDescending {
				sort.SliceStable(Filtered, func(I, J int) bool { return Less(&Filtered[J], &Filtered[I]) })
			} else {
				sort.SliceStable(Filtered, func(I, J int) bool { return Less(&Filtered[I], &Filtered[J]) })
			}
		}

		SelectedRow = -1
		Table.UnselectAll()
		StatusLabel.SetText(fmt.Sprintf("%d / %d games", len(Filtered), len(AllRows)))
		Table.Refresh()
	}

	// Hook all filter widgets to RefreshTable.
	NameEntry.OnChanged = func(_ string) { RefreshTable() }
	PlayerAEntry.OnChanged = func(_ string) { RefreshTable() }
	PlayerBEntry.OnChanged = func(_ string) { RefreshTable() }
	PlayerCEntry.OnChanged = func(_ string) { RefreshTable() }
	RuleSetSelect.OnChanged = func(_ string) { RefreshTable() }
	KomiEntry.OnChanged = func(_ string) { RefreshTable() }
	MinSizeEntry.OnChanged = func(_ string) { RefreshTable() }
	MaxSizeEntry.OnChanged = func(_ string) { RefreshTable() }
	ApproxUnixMilliEntry.OnChanged = func(_ string) { RefreshTable() }
	MaxErrorUnixMilliEntry.OnChanged = func(_ string) { RefreshTable() }
	CommentEntry.OnChanged = func(_ string) { RefreshTable() }

	// Initial status label.
	StatusLabel.SetText(fmt.Sprintf("%d / %d games", len(Filtered), len(AllRows)))

	// Layout. Each row in the filter bar groups related filters side-by-side.
	LabelField := func(Caption string, Widget fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewLabel(Caption), nil, Widget)
	}
	FilterBar := container.NewVBox(
		LabelField("Game Name:", NameEntry),
		container.NewGridWithColumns(3,
			LabelField("Player A:", PlayerAEntry),
			LabelField("Player B:", PlayerBEntry),
			LabelField("Player C:", PlayerCEntry),
		),
		container.NewGridWithColumns(2,
			LabelField("Rule Set:", RuleSetSelect),
			LabelField("Komi:", KomiEntry),
		),
		container.NewGridWithColumns(2,
			LabelField("Min Size:", MinSizeEntry),
			LabelField("Max Size:", MaxSizeEntry),
		),
		container.NewGridWithColumns(2,
			LabelField("Approx Unix Milli:", ApproxUnixMilliEntry),
			LabelField("Max Error Unix Milli:", MaxErrorUnixMilliEntry),
		),
		LabelField("Comment contains:", CommentEntry),
	)

	Content := container.NewBorder(
		container.NewVBox(FilterBar, StatusLabel, widget.NewSeparator()),
		nil, nil, nil,
		Table,
	)

	Dlg := dialog.NewCustomConfirm("Search Collection", "Go", "Close",
		Content,
		func(Confirmed bool) {
			GoWin.DialogClosed()
			if !Confirmed {
				return
			}
			if SelectedRow < 0 || SelectedRow >= len(Filtered) {
				return
			}
			Hist := Filtered[SelectedRow].Hist
			if Hist == nil || Hist.CurrentNode == nil {
				return
			}
			GoWin.OnUserNavigate()
			GoWin.SetCurrentNode(Hist.CurrentNode)
		},
		GoWin.Win,
	)
	GoWin.DismissDialog = func() { Dlg.Hide() }
	GoWin.SubmitDialog = func() { Dlg.Confirm() }
	GoWin.ResizeDialog = func() { Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size())) }
	GoWin.WireSubmitOnEnter(NameEntry, PlayerAEntry, PlayerBEntry, PlayerCEntry,
		KomiEntry, MinSizeEntry, MaxSizeEntry, ApproxUnixMilliEntry, MaxErrorUnixMilliEntry, CommentEntry)
	Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))
	Dlg.Show()
	GoWin.Win.Canvas().Focus(NameEntry)
}
