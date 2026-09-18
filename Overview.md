# ConnectedGroupsGoban — Developer Overview

Quick-reference for modifying this program. Read this before making changes.

## Rules To Follow - IMPORTANT!

Use PascalCase EVERYWHERE (functions, variables, and constants) (in all scopes) unless required to not.

Use descriptive names everywhere. `RuleSet` is superior to `RS`, `Index` is superior to `I`.

Never implement backwards compatibility or migration support for the config file; only the current version shall be supported.

Optimize. Deduplicate code. Deduplicating code by adding functions within functions is good.

Use `./BuildWithNixShell.sh` when you want to verify the code can build.

Internal code must not involve SGF information (SGF property codes, `SgfGetInformation`, etc.). SGF lives only in code whose job is SGF import, SGF export, or a direct helper for one of those. Internal code reads `Hist.Information[Description]` directly using human-readable description keys such as `"Game Name"`, `"Result"`, `"Event"`, `"Date And Time"` — the same keys stored under `SgfToDescription` values in [ConstantVariable.go](ConstantVariable.go).

Each top-level dialog (one reachable directly from a menu item or keyboard shortcut) lives in its own `*Dialog.go` file. Subdialogs (those spawned from another dialog's callback) stay in the parent's file.

Package-level `const`/`var` declarations belong in [ConstantVariable.go](ConstantVariable.go). Other `.go` files should not declare top-level `const`/`var`.

Variables that are declared once and used once should be inlined when inlining improves clarity (use the if-init form, inline literals, etc.); keep the local when the name documents intent.

## Docs

- [Overview.md] The file with the purpose of making this project easier to [understand quickly and develop more efficiently]. [After completely implementing a refactor, feature, or bugfix], update this overview file: add what should be added, and remove what no longer serves the purpose of this file. This file should be very detailed, giving justification to the existence of every function and every type struct.

- [ReadMe.md] User-facing feature documentation; update whenever a new user-facing feature ships.

---

## How To Build

```
./BuildWithNixShell.sh
```

---

## File Responsibilities

| File | What lives here |
|------|----------------|
| [main.go](main.go) | Entry point; opens one window per command-line file arg (`.Sgf` or any `CggAllSuffixes` suffix — classified by `IsSupportedGamePath`); `Init()`, menus (File, Game, App, Mouse Mode, Engine, Window), keyboard shortcuts, `SaveWindowState` (persists window size/VSplit offset); `UpdateStatusBar`, `UpdateStatusBarLoop` (~139 ms tick, mouse mode + last-move time + since-move time + hover-group stones/liberties); `DeleteCurrentNode`; `RefreshSetVertexMenu` rebuilds Set Vertex player choices, clamps the selected player, disables the submenu when no game is active, and records the represented game |
| [WindowMenu.go](WindowMenu.go) | Window menu actions: `NewGoWin(SourceColl)` opens a new GoWin, calling `Init(true)` so the default `AddFreshBoard` is skipped, then — if SourceColl is non-nil — seeds the collection with a deep copy round-tripped via `EncodeCggFile` + `ImportCggBytes`. `DuplicateWindow`, `AddWindow` (zero games), `CloseWindow` methods on `*GoWin` |
| [Types.go](Types.go) | All structs: `AppConfig`, `GoWin`, `GameHistory`, `GameTreeNode`, `BoardState`, `EngineConfig`, `FreshBoardPreset`, `GtpEngineState`, `Theme`, `RuleSet`, `Coord`, `Group`, `LibertySharingMatrix`, `PlayerColor`, `ScoreDetails`, `Collection`, `GobanLayerKey`, `InputLayer`, `GoErr` (error type with pointer-identity comparison) |
| [ConstantVariable.go](ConstantVariable.go) | All package-level `const`/`var`: `DefaultConfig`; `SgfToDescription` / `DescriptionToSgf` maps; `InformationRowOrder` (display order in Game Information dialog); coordinate format names (`CoordFmtGtp`, `CoordFmtComputer`, `CoordFmtHikaruNoGo`, `CoordFmt1stQuadrant`, `CoordFmtSgf`); `CoordFmt` slice; vertex bit-mask constants (`PlayerAtVertexMask`, `XMask`, `SquareMask`, `CircleMask`, `TriangleMask`); `SgfPropertyToAnnotationMask` / `AnnotationMaskToSgfProperty` maps; `LayerOrder []string` (draw-order keys for `GoWin.Layers`); `MouseModes`; sentinel `Coord` values (`CollectionCoords`, `RootCoords`, `ErrorCoords`, `PassCoords`, `EditCoords`); `NonRepetitionRule*` flag constants; `SharingRefused`/`Conditional`/`Unconditional` and the `SharingState*Names`/`Order` lookup maps; `Ctrl*` keyboard shortcut definitions; `ProtectedRuleSets` (built-in rule sets); `NonRepetitionRuleFlags` (display-name table); `ScoreDividerWidth`/`ScoreCellPad`/`ScoreTextSize` and `ScoreColor*` palette; `SymmetryNames`; CGG-format constants (`CggSaveGameFormat`, `ZstdMaxLevel`, `CggMethodToExtension`, `CggAllSuffixes`, `NonRepetitionRuleFlag↔Name` maps); `ExportModeSingleFrame`/`ExportModeAnimated` labels; `SvgDefsBlock` shared SVG `<defs>`; `StatusBarPadY`; `LastSavedConfigBytes`; `GridLineThickness`; `Version` / `ApplicationName` / `ConfigFile`. Also `init()` builds `DescriptionToSgf` from `SgfToDescription` |
| [Config.go](Config.go) | `LoadConfig`, `SaveConfig`, `SaveConfigSnapshot`, `SnapshotCurrentConfigBytes`, `SaveConfigAsync`, `ValidateAndFillConfig`; also `GoWin.Width()` / `Height()` helpers |
| [GameHandlers.go](GameHandlers.go) | `HandleKeyEvent` (game-tree navigation, deletion, pass key, dialog Esc/Enter routing) and `HandlePass` (pass move, with engine/turn guards) |
| [FreshBoardDialog.go](FreshBoardDialog.go) | `HandleNewGame` — Fresh Board dialog (board size, players, rule set selector with custom rule sets, per-player names/komi, wrap multipliers/shifts, liberty-sharing fixed flag + matrix editor entry, "Delete other games in collection" checkbox, preset Save/Update/Delete, saved-name quick-select). Also `SortedPresetNames` and `AppendPlayerName` helpers |
| [EngineSettingsDialog.go](EngineSettingsDialog.go) | `ShowEngineSettings` — engine CRUD (Save/Update/Delete with rename), GTP logging toggle, Browse for executable |
| [ShowGameInformationDialog.go](ShowGameInformationDialog.go) | `ShowGameInformation` — Game menu dialog for viewing read-only board fields and editing per-player names/komi and SGF-style information rows. Uses `InformationRowOrder` from [ConstantVariable.go](ConstantVariable.go); excludes the `"Rules"` key (Rule Set is shown read-only at the top) |
| [GoToMoveDialog.go](GoToMoveDialog.go) | `ShowGoToMoveDialog` — Game menu dialog (Ctrl+G) to navigate to a specific move number via the favorite-child path from the root node |
| [SearchCommentDialog.go](SearchCommentDialog.go) | `ShowSearchCommentDialog` — Game menu dialog that case-insensitively searches `GameTreeNode.Comment` across the current game (default) or the entire collection (checkbox) and lists matches as `[game name] Move N — snippet`; selecting a row + Go navigates there via `SetCurrentNode` (switches games when needed). `CommentSearchMatch`, `CommentSnippet` helpers |
| [DiplomacyDialog.go](DiplomacyDialog.go) | `ShowDiplomacyDialog` — Game menu dialog (Ctrl+D) that lets the next-to-move player edit only their own outgoing row of `Board.NextLibertySharingMatrix`; live "→ share / → do not share" labels resolve the 3-state (No/Conditional/Yes) interaction. On OK: `CalculateAllGroups` + invalidate `IsPlacementLegalCache`. Disabled when `LibertySharingFixed` |
| [ShowCollectionDialog.go](ShowCollectionDialog.go) | `ShowCollectionDialog` — Game menu dialog showing all games in the collection as a filterable/sortable table. Columns: Name, Players, Rule Set, Komi, Size, Moves, Root Time, Date, Result, Event. Filters: Game Name, three independent player-name slots, Rule Set (dropdown), Komi (exact float, falls back to substring), Min/Max Size, Approx Unix Milli ± Max Error, comment substring (depth-first walk of every node's Comment). Headers are sort buttons; the active header shows ▲/▼. Selecting + Go switches to that game. `BuildCollectionRows`, `CollectComments`, `CollectionRow`, `CollectionColumn` helpers |
| `NodeLayout` (GameTreeUi.go) | Struct holding col/row/parentCol/parentRow for a node in the flat game tree layout |
| [GameTree.go](GameTree.go) | `NewRootNode`, `MakeChild`, `MakeSiblingCopyBoard`, `GetNextStonePlacer`, `FavoriteChildPath(Root) []*GameTreeNode` (shared walk used by the Page Down handler, `ShowGoToMoveDialog`, `ShowCollectionDialog` move count, and `BuildAnimatedBoardSvg`), SGF coordinate conversion; `SetCoordProperty` / `ToggleCoordProperty` / `CheckCoordProperty` / `UnsetCoordProperty` (annotations write to `Vertices` bits; AW/AB/AE write directly to `Vertices`) |
| [GameTreeUi.go](GameTreeUi.go) | `UpdateGameTreeUI`, `buildGameTreeUI` (flat two-pass layout: post-order subtree widths, pre-order positioning), `AddFreshBoard`, `AddFreshBoardFromPreset`, `GetActivePreset`, `SetCurrentNode`, `ShowError`, `FixWindowTitle`, `GameDisplayName`, `RefreshGameSelector`, `RefreshPassMenuItem`, `RefreshDiplomacyMenuItem`; fast selection-only update: `UpdateGameTreeSelection`; `CenterGameTreeOnCurrentNode`, `GameTreeLayout` (layout wrapper that re-centers on resize); `TreeNodeButton` widget (custom renderer; selected-node 3px border in TextColor); `TreeNodeAffectedCoords` for hover-highlight overlay; `FixedSizeLayout` reports content size to `container.Scroll` so scrolling actually works |
| [BoardLogic.go](BoardLogic.go) | Move legality and application via unified `AttemptMove(Coord, Player, EditMode, DryRun)`, capture/conversion cascade, group calculation, `NewBoardState`; `GetPlayerAt` / `SetPlayerAt` / `SetAnnotation` / `ClearAnnotation` / `HasAnnotation` / `ToggleAnnotation` helpers on `*BoardState`; `VerticesEqual`, `CopyVertices`, `ConvertGroup`, `FindPreviousGtnWithPlacementByPlayer` helpers; `HalfIntegerMoku()` on `*GameHistory`; `IsValidWrapMul`, `GcdInt16`, `ModInverseInt16`, `RecomputeWrapInverses` for general wrap multipliers |
| [BoardDrawing.go](BoardDrawing.go) | All canvas drawing; `DrawBoard`, `DrawGoban` (Goban layer, only on resize/dim/symmetry change), `DrawLastMoveHighlight`, `DrawChildNodeDots` (small "Child Node"-colored dots on every move-type child's LastMove, stroke-colored by the child's `PreviousStonePlacer`; gated on `Theme.ShowChildNodeDots`), `MakeSolidCircleOnBoard`, coordinate↔pixel math (`BoardCoordsToPixel`, `DisplayCoordsToPixel`, `BoardUnitsToPixel`, `BoardPointToPixel`, `BoardRectToPixel`, `RotateBoardPoint`), symmetry helpers (`TransformBoardCoord`, `InverseTransformDisplayCoord`, `DisplayDims`), unified `DrawAnnotation` function for all annotation types, `DrawStub`/`DrawStubWest/East/North/South` for wrap-seam liberty stubs, `DrawConnectionBetween` for stone connections (auto-orients to rotation), `AddHalfRect` for paired wrap-seam connection rects; coord-format dispatch via `CoordColumnLabel` / `CoordRowLabel` |
| [GtpEngine.go](GtpEngine.go) | All GTP engine logic (see Engine section below); `SendGtpCommandToEngine` (free function used by the engine-move goroutine) |
| [Scoring.go](Scoring.go) | `FindRuleSetByName`, `RegisterRuleSet`, `RuleSetDisplayName`, `FormatPoints`, `CalculateScoreDetails` (filters shown rule sets to match active rule set's `CaptureConverts`), `CalculateAndDisplayScore`, `UpdateScoreTable` (auto-adjusts HSplit after rebuild; prepends "No Result (Draw)" row when the current node is flagged), `DrawScoreTooltip`, `ClearScoreTooltip`, `IsGameOver`, `ToggleGroupStatus`, `InitializeTerritoryMap`, `ScoreColLayout`/`ScoreTableLayout` (custom layouts for compact score-table rendering), `ScoreRowInfo` (per-row metadata: label, multiplier, tooltip, highlight function) |
| [CggJsonIo.go](CggJsonIo.go) | CGG JSON save/load: `HandleImportCgg`, `ImportCggFile` (path-based, used by command-line startup), `ImportCggBytes` (shared parse + apply pipeline), `DecompressCgg` (suffix-dispatched decompression), `EncodeCggFile`, `DecodeGame`, `ShowCggSaveDialog`; wire types (`CggFile`, `CggGame`, `CggNode`, etc.); `EncodeRuleSet`/`DecodeRuleSet` with `NonRepetitionRule` string conversion; liberty matrix full/diff encode/decode; optional Kanzi, Gzip, or Zstd compression (`.CGG.json`, `.CGG.json.kanzi`, `.CGG.json.gzip`, `.CGG.json.zstd`). Save/load both update `GoWin.OpenedFilePath` and the window title. `CggSuffixFileFilter` matches full multi-dot suffixes |
| [ExportCggDialog.go](ExportCggDialog.go) | `HandleExportCgg` — File menu Save Game entry point. Method radio (Kanzi/Gzip/Zstd/Plaintext) with a per-method compression-level slider (`MaxLevelForMethod`: Kanzi=9, Gzip=9, Zstd=`ZstdMaxLevel`=22, Plaintext=0/hidden). Default is Kanzi at level 9. On confirm calls `ShowCggSaveDialog` |
| [SgfIo.go](SgfIo.go) | `ImportSgfContent`, `HandleImportSgf`, `HandleExportSgf`, SGF parsing and generation; `SgfEscapeText`, `CaptureRawTree`, `LastMainLineNode` utilities. Per-game parse errors call `SkipCurrentGame`, which removes the partial root, jumps past the malformed tree via `CaptureRawTree`, and shows a warning — remaining games still load |
| [SgfUtils.go](SgfUtils.go) | `NextPlayer`, `PreviousPlayer` (multi-player, replaces `SwitchPlayer`), SGF↔internal coord conversion (`ConvertCoordToSgfCoord`, `ConvertSgfCoordToCoord`), `getInfoProp` / `storeInfoProp` |
| [HoverMouse.go](HoverMouse.go) | Mouse move/click handlers, hover drawing, `SetMouseMode` (auto-adjusts HSplit right when entering Score mode if score table needs room), annotation hover preview, unified `DrawAnnotation` function, single "Toggle Annotation" mode selected through its submenu; `VertexOptionLabel`, `AnnotationOptionLabel` utilities; `HandleMouseClick` contains an inline "Set Label" `dialog.NewForm` (subdialog of the click handler — stays here) |
| [LibertySharing.go](LibertySharing.go) | `LibertySharingMatrix` operations: `NewLibertySharingMatrix`, `Clone`, `Equal`, `Get`, `EffectivelyShares` (resolves Conditional via mutual agreement), `Resize` (preserves diagonal Unconditional invariant), `JSON` round-trip helpers, `NewSharingCellSelect` (compact dropdown for diplomacy/matrix editor), `ShowLibertySharingMatrixEditor` (subdialog of Fresh Board's "Edit liberty sharing matrix…" button: N×N grid, row/col numbered, cells show single-letter `N`/`C`/`Y`, tap to cycle) |
| [RuleSetDialog.go](RuleSetDialog.go) | Rule set editor UI for `[Built-in]`, `[Custom]`, and imported `[Game]` rule sets; `ShowRuleSetDialog` (top-level), `ShowModifyRuleSetDialog` (subdialog), `DeepCopyRuleSet`; `PresetsUsingRuleSet`, `UpdatePresetsRuleSet`, `UpdatePresetsByValue`, `RuleSetNameConflictsWithProtected`, `DeduplicateCustomRuleSets`, `RelinkPresetsToCustomSlices`, `FindCustomRuleSetMatching`, `ProtectedRuleSetIsShown`, `SetProtectedRuleSetShown`, `RuleSetCopyNameForCustom`, `PrepareRuleSetCopyForCustom`, `RuleSetDialogEntry`/`RuleSetEntryDisplayName`/`BuildRuleSetDialogEntries` helpers for preset↔rule set integrity and score-table visibility. `[Game]` entries are collection rule sets not equal to any built-in/custom entry and can be persisted via "Save as Custom" |
| [ThemeDialog.go](ThemeDialog.go) | Theme editor UI; `InitColors(Theme)`, `ApplyThemeVisual`, `ApplyTheme`, `DeepCopyTheme`, `DefaultTheme`, `GetActiveTheme`, `ThemeByName`, `ShowThemeDialog` (top-level), `ShowModifyThemeDialog` (subdialog), `ShowCustomColorPicker` (subdialog) |
| [ExportImage.go](ExportImage.go) | `ImageRenderer` interface (`FillRect`/`FillCircle`/`StrokeCircle`/`StrokeRect`/`StrokeLine`/`StrokePolygon`/`DrawText`/`Draw3MeetShape`); `BoardDrawer` symmetry-aware traversal (`Node *GameTreeNode` defaults to `Win.CurrentNode()`, walks goban/grid/coord labels + LibertyLines iff `ShowLibertyLine`/IllegalDots iff `ShowIllegalDot` and not Score mode/StoneConnections iff `ShowStoneConnection`/Stones/LastMove ring/Annotations/Labels/Territory iff `len(Node.TerritoryMap) > 0`). `BoardDrawer.Point`/`Rect`/`CellTopLeft`/`CellCenter` reuse `RotateBoardPoint` for symmetry; output spans exactly `CellSize × (DisplayWidth+2)` × `CellSize × (DisplayHeight+2)`. `DrawAll` is split into `DrawGobanLayer` (static background, reusable across animated frames) and `DrawNodeLayers` (everything that varies by node); `DrawMoveCounterOverlay(Current, Last)` emits a two-line counter in the screen upper-left corner cell (anchored via `DisplayPoint`, bypasses `SymmetryIndex`). `ShowSvgSaveDialog(CellSize, SecondsPerNode)` is the merged save entry point: `SecondsPerNode == 0` ⇒ static single frame (synchronous `BuildBoardSvg`), `SecondsPerNode > 0` ⇒ animated export (delegates to `RunAnimatedSvgExport`, which opens a modal progress dialog with a `widget.ProgressBar` and generates the SVG on a goroutine via `BuildAnimatedBoardSvg`, using `fyne.Do` to update UI). `OpenedFilePath` is **not** updated on save |
| [ExportImageDialog.go](ExportImageDialog.go) | `HandleExportImage` — File menu "Export board as SVG" entry. `dialog.NewForm` with Mode radio (`ExportModeSingleFrame` / `ExportModeAnimated`), uint16 CellSize entry (min 5), and SecondsPerNode entry (enabled only in animated mode, default 0.15). Routes to `ShowSvgSaveDialog(CellSize, SecondsPerNode)` |
| [ExportSvg.go](ExportSvg.go) | `SvgRenderer` implements `ImageRenderer` by emitting exact SVG primitives to a `strings.Builder`. `BuildBoardSvg(CellSize)` opens the document, emits the shared `SvgDefsBlock` (`FourCirclesMask` + `SquareMinusFourCircles <symbol>` matching [SquareMinusFourCircles#039fef.svg](SquareMinusFourCircles#039fef.svg)), and runs `BoardDrawer.DrawAll`. `BuildAnimatedBoardSvg(CellSize, SecondsPerNode, Progress)` walks the favorite-child path from `RootNode` via `FavoriteChildPath` and emits one **self-contained** `<g id="FrameK">` per node — each frame calls `BoardDrawer.DrawAll` (coord bg + goban + node layers) and `DrawMoveCounterOverlay(K, N-1)` so every frame is fully opaque. Frame 0 starts visible; Frame K≥1 starts with `visibility="hidden"` and is switched on by a SMIL `<set attributeName="visibility" to="visible" begin="(K*SPN)s" fill="freeze"/>`. To keep per-tick paint cost constant regardless of K, each frame K < N-1 is **also** switched off at `begin="((K+1)*SPN)s"` by a second `<set>`, emitted after the show event inside the XML so the boundary instant is processed show-before-hide. `Draw3MeetShape` emits `<use href="#SquareMinusFourCircles">`. Font-family `"Noto Sans, sans-serif"` matches Fyne's bundled default; text is vertically centered via `y = cy + FontSize*0.35` (not `dominant-baseline`) so it centers consistently across QtSvg-based viewers. `SvgColor`/`SvgEscapeText`/`SvgFloat` helpers |
| [InputLayer.go](InputLayer.go) | Fyne widget that captures mouse events on the board; `ContentWrapper` (debounced config save on resize); `VSplitChildWrapper` (saves VSplit offset); `WindowDialogSize` (computes oversized dialog size that clamps to canvas); `DialogClosed` cleanup; `WireSubmitOnEnter`; `StatusBar` and `StatusBarLayout` (compact zero-padding status bar) |
| [ColorImage.go](ColorImage.go) | Stone image generation from color; `ContrastColor` utility |
| [DialogEntry.go](DialogEntry.go) | `DialogEntry` — `widget.Entry` subclass that routes Escape to `GoWin.DismissDialog`; constructors `NewDialogEntry` / `NewMultiLineDialogEntry`. Used in every text input inside our own dialogs so Escape dismisses even while the field has focus |

---

## AttemptMove — Move Legality and Application

`AttemptMove(Coordinate Coord, Player uint8, EditMode bool, DryRun bool) (*BoardState, error)`

- **`EditMode=false`** (Play mode): enforces existing-stone, suicide, and repetition rules. Results are cached per `(Player, false, Coord)`.
- **`EditMode=true`** (Set Vertex mode): skips existing-stone checks. Captures/conversions are resolved normally. Results are cached per `(Player, true, Coord)`. Used by the Set Vertex click handler instead of the old `SetPlayerAt` + manual capture loop.
- **`DryRun=true`**: checks legality only, returns `(nil, nil)` if legal or `(nil, err)` if illegal. Hover and illegal-move indicators use this.
- **`DryRun=false`**: applies the move, returns a new `*BoardState` with captures/conversions resolved and pass counter updated.
- `SimBoard` is hoisted to function scope — built once during the legality check (cache miss) and reused by the apply section, avoiding a redundant `CopyWithoutGroups` + `CalculateAllGroups`.
- Pass moves (`PassCoords`) are always legal and return a new board with `Passes[Player]++`.
- `ExistingStoneErr` is **not** cached, because the player-bits at a coordinate can be mutated by Set Vertex delete (`SetPlayerAt(C, 0)`) — caching it would leave a stale "illegal" verdict on coords that have since been emptied. `DrawEmptyIllegalMoves` therefore skips non-empty cells before calling `AttemptMove`.

### Cascade Algorithm

After placing the stone and `CalculateAllGroups`, `RunCascade` executes:

1. **Outer loop**: snapshot S0B.
2. **Inner loop** (opponent groups only; Player's groups are implicitly immortal):
   - Snapshot S15B. Find all non-Player groups where `DetermineConversionTarget` returns `(target, true)`. `ConvertGroup` each. `CalculateAllGroups`. Repeat if board changed.
3. **Player suicide pass**: Find Player-owned groups where `DetermineConversionTarget` returns `(target, true)`. `ConvertGroup` each. `CalculateAllGroups` (single pass, no inner loop).
4. If board changed since S0B, repeat outer loop.
5. **Step 8**: If `!SuicideIsLegal` and placed stone is no longer Player's color after cascade → `SuicideErr`.

`DetermineConversionTarget(TargetGroup)` is a closure inside `AttemptMove`. When `CaptureConverts` is false: returns `(0, true)` for groups with 0 liberties (standard capture), `(0, false)` otherwise. When `CaptureConverts` is true: if 0 liberties and exactly 1 non-owner non-empty neighbor color in `TargetGroup.Vertices` → `(color, true)`, else `(0, false)`.

### Repetition Check

After the cascade stabilizes, `AttemptMove` evaluates `RuleSet.NonRepetitionRules`. The four scope rules form a strict hierarchy — `Positional ⊃ Situational ⊃ NaturalSituational ⊃ BasicKo` — and `StrongestRepetitionRule` selects the single strongest enabled flag; weaker checks are redundant and skipped. `FindRepeatedAncestor(CurrentNode, SimBoard, Player, Rule)` walks ancestors and compares `Board.Vertices` to `SimBoard.Vertices` (player bits only) under the chosen semantics:

- **BasicKo** — first same-player ancestor only, and only if `LastMove != PassCoords`.
- **NaturalSituationalSuperKo** — any ancestor with `PreviousStonePlacer == Player && LastMove != PassCoords`.
- **SituationalSuperKo** — any ancestor with `PreviousStonePlacer == Player` (passes included).
- **PositionalSuperKo** — any ancestor (any player, passes included).

A match returns `RepetitionErr`. `NonRepetitionRuleNoResult` is a policy modifier with Positional scope: instead of rejecting the move it sets `SimBoard.NoResultTriggered`, which is copied into `GameTreeNode.NoResultDraw` by `PlayMove`. The NoResult check runs in the apply branch (not the legality branch), so it is re-evaluated even on cache hits and reliably propagates to the new node. `IsGameOver` returns true for any `NoResultDraw` node and `UpdateScoreTable` prepends a "No Result (Draw)" row. The `RuleSetDialog` checkboxes enforce the hierarchy symmetrically: checking a stronger flag auto-checks all weaker flags, and unchecking a weaker flag auto-unchecks all stronger flags, so the persisted bitmask always forms a consistent prefix of the hierarchy. `NoResult` is independent and is not part of the cascade.

**Set Vertex mode behavior:**
- Placing a stone (`SetVertexPlayer != 0`): calls `AttemptMove(C, Player, true, false)` — opponent groups with 0 liberties are removed/converted, board is always legal after edit.
- Deleting a stone (`SetVertexPlayer == 0`): calls `SetPlayerAt(C, 0)` directly (deletion cannot be expressed through `AttemptMove`).
- `DrawEmptyIllegalMoves` uses `SetVertexPlayer` + `EditMode=true` in Set Vertex mode (not `GetNextStonePlacer()`).

---

## Annotation System

**Annotation Types**: (Circle / Square / Triangle / X Mark) — stored as bit flags in `BoardState.Vertices` upper nibble (`CircleMask`=0x40, `SquareMask`=0x20, `TriangleMask`=0x80, `XMask`=0x10).

**Unified Drawing**: `DrawAnnotation(mask, coord, color, layer)` in [BoardDrawing.go](BoardDrawing.go) handles all annotation types with a single switch statement. Annotation shapes go to `Layers["Annotation"]`; LB text labels go to `Layers["Label"]`.

**UI Mode**: Mouse Mode contains Play, Score, Set Label, and the Set Vertex and Toggle Annotation submenus. Choosing [Circle/Square/Triangle/X Mark] in Toggle Annotation selects the annotation mask and enters that mode. Set Vertex offers Empty (0) and Player 1 through the current game’s player count; selecting a choice sets `SetVertexPlayer` and enters Set Vertex mode. Both former dropdown widgets are removed.

**Hover Preview**: When in "Toggle Annotation" mode, shows semi-transparent preview of annotation to be added (alpha ≈ 0x93), or inverted color preview (full opacity) for annotation removal.

**Constants** (in [ConstantVariable.go](ConstantVariable.go)):
- `SgfPropertyToAnnotationMask` maps SGF property codes to bit masks
- `AnnotationMaskToSgfProperty` provides reverse lookup for efficient conversion

---

## Liberty Sharing (Diplomacy)

Each player independently chooses whether to share their groups' liberties with each other player. Three states per ordered pair: `SharingUnconditional` (Y, always shares), `SharingConditional` (C, shares iff the other side stores Conditional or Unconditional), or `SharingRefused` (N, never shares). Sharing is asymmetric and non-transitive. The diagonal is always `SharingUnconditional` (a player always shares with themselves). Stored in `LibertySharingMatrix` (a `(Players+1)×(Players+1)` outer-allocated `[][]uint8`; index 0 unused).

**Two game modes:**
- **Fixed** (`GameHistory.LibertySharingFixed = true`): set at new-game time, immutable. `Game > Diplomacy` menu item disabled. SGF and GTP-derived games always set this to true (those formats can't represent the matrix).
- **Changeable**: `Game > Diplomacy` (`Ctrl+D`) opens the editor — only the next-to-move player edits their own outgoing row of `Board.NextLibertySharingMatrix`. On commit: `CalculateAllGroups` recomputes flood-fill (groups walk through shared-color stones; `Vertices[Owner]` keeps only Owner's stones; `CoordToGroupCache` points each stone to its owner's group), and `IsPlacementLegalCache = nil` so the UI immediately reflects new sharing. `Vertices` are intentionally left alone — zero-liberty groups under the new matrix are NOT captured by the matrix change; captures occur only when a stone is actually played.

`Board.NextLibertySharingMatrix` governs the **next** move from this board; the matrix that produced this board's Groups is whatever was set at creation time and is not stored separately. Child boards inherit the pointer via `Copy` / `CopyWithoutGroups`.

**UI:**
- Compact matrix editor (`ShowLibertySharingMatrixEditor`, subdialog of Fresh Board's "Edit liberty sharing matrix…" button): N×N grid, row/col numbered, cells show single-letter `N`/`C`/`Y`, tap to cycle.
- `ShowDiplomacyDialog` edits only the current player's row with live "→ share / → do not share" outcome labels.

---

## Board Symmetry (Display-Only)

`GoWin.SymmetryIndex uint8` (0–7) is a pure visual transform: **0** Identity, **1** Rot90 CW, **2** Rot180, **3** Rot270 CW, **4** Flip Horizontal, **5** Flip+Rot90 CW, **6** Flip Vertical, **7** Transpose. Never persisted; resets to 0 on window creation. Per-window — changing symmetry in one window leaves others untouched. `BoardState` is never modified. Selected via Window → Symmetry submenu ([main.go](main.go)); `GoWin.SymmetryItems []*fyne.MenuItem` + `RefreshSymmetryMenuItems()` disable the active entry so the chosen symmetry is indicated by its greyed-out state.

The drawing pipeline is built around a small source-space cell-unit coordinate system so every visual element rotates correctly:

- `RotateBoardPoint(px, py)` — rotate a continuous source point.
- `BoardUnitsToPixel(dpx, dpy)` — display-unit point → pixel.
- `BoardPointToPixel(px, py)` — rotate + convert.
- `BoardRectToPixel(px, py, sx, sy)` — rotate a rect, return display-aligned pixel top-left and size via the bounding box of the two rotated opposite corners.
- `BoardCoordsToPixel` delegates to `BoardRectToPixel(X, Y, 1, 1)` so the returned pixel is the display top-left of the rotated cell (not the rotated source top-left, which lands on a different display corner under non-identity symmetries).

Odd indices swap the display's Width/Height — so rectangular boards rotate correctly. `PixelOnBoard` uses `BoardUnitsToPixel(0, 0) + DisplayDims()` directly so the clickable bounding box is exact for every symmetry. `DrawGoban`'s `CoordBg` / `GobanRect` anchor via `BoardUnitsToPixel` (display-space, bypasses the transform) so their bounding boxes hug the rotated layout. `GobanLayerKey` includes `SymmetryIndex` so the Goban layer redraws on rotation. Column/row labels continue to label their original columns/rows (the label text is unchanged; only its on-screen position rotates). Wrap-seam liberty stubs (`DrawStub` with `'N'`/`'S'`/`'E'`/`'W'` source-space directions) and stone-connection half-rects (`AddHalfRect`) are also rotated through the helper functions.

---

## Coding Conventions

### Use `GoWin.Coll.CurrentGameH` instead of `GoWin.CurrentNode().Board.Hist`

`GoWin.Coll.CurrentGameH` is the canonical way to access the current game's `*GameHistory` (width, height, komi, coord conversion, neighbor lookup, etc.). **Never** write `GoWin.CurrentNode().Board.Hist` — it requires `Board != nil` and is just a longer path to the same pointer.

- `CurrentGameH` is `nil` when the Collection node is selected (no game active).
- `GoWin.Width()` / `GoWin.Height()` already guard against `nil` and return 0.

### Collection node nil-safety

The Collection node (`Coll.CollectionNode`) has `Board == nil`. Any code path reachable when no game is selected must guard against this. Key places already guarded:
- `DrawBoard` returns early for Collection node
- `DrawHover` checks `Coll.CurrentGameH == nil`
- `HandleMouseClick` / `HandlePass` check `CurrentNode().Board == nil`
- `IsGameOver` returns `false` when `Board == nil`
- `SetCurrentNode` detaches engines on navigation, including the Collection node

### Custom Rule Set Invariant

`ShownCustomRuleSets ∪ HiddenCustomRuleSets` must not contain two entries that are `RuleSet.Equal` to each other. Two custom rule sets with the same `Names[0]` but different content are allowed; the dialog disambiguates them with a ` #<RuleSetId>` suffix (mirrors `RuleSetDisplayName` in [Scoring.go](Scoring.go)).

- `DeduplicateCustomRuleSets()` in [RuleSetDialog.go](RuleSetDialog.go) enforces the invariant. It merges content-equal entries (prefers `Shown` over `Hidden`), re-aims presets via `UpdatePresetsByValue`, and calls `RelinkPresetsToCustomSlices()`. Invoked from `ValidateAndFillConfig` and from the dialog's `SaveAndRefresh` after any mutation.
- `RelinkPresetsToCustomSlices()` re-resolves every non-protected `FreshBoardPreset.RuleSet` pointer to the matching slice element's current address. Call after any `append` or splice on the custom slices, because reallocations invalidate prior pointers.
- `PresetsUsingRuleSet` / `UpdatePresetsRuleSet` match by pointer identity only; callers that need value-based matching (e.g., during dedupe) use `UpdatePresetsByValue`.
- `FindCustomRuleSetMatching(Candidate *RuleSet) *RuleSet` returns an existing custom rule set content-equal to Candidate, or nil. Used by the New / Duplicate handlers to avoid appending a duplicate.
- The dialog's `RefreshEntries(SelectRS *RuleSet)` selects an entry by pointer rather than display name, so name changes and same-name disambiguation don't desync selection.

### Game Tree UI Partial Updates

The game tree UI supports incremental updates to avoid O(n) full rebuilds. Two maps on `GoWin` enable O(1) lookup:
- `GameTreeNodeToButton map[*GameTreeNode]*TreeNodeButton` — node → its button widget
- `GameTreeNodeToContainer map[*GameTreeNode]*fyne.Container` — node → its VBox(button, childrenHBox)

Both maps are populated by `buildGameTreeUI` and cleared by `UpdateGameTreeUI`. `SetCurrentNode` uses a fast path (`UpdateGameTreeSelection`) when both the old and new nodes are in the map AND the `Gtn.Parent.FavoriteChild = Gtn` assignment doesn't actually change the favorite pointer; otherwise falls back to a full `UpdateGameTreeUI`. The favorite-change check is required because the thin blue connector is drawn only for favorite-child edges and the selection-only fast path never rebuilds connectors. When a node is deleted, its entry is removed from both maps before `SetCurrentNode` is called so the fast path is bypassed and the tree is fully rebuilt.

### Game Tree Layout (Flat, Non-Recursive)

The game tree is laid out by `buildGameTreeUI` using an iterative two-pass algorithm to avoid 255+ levels of nested VBox/HBox (which historically caused stack overflow when Fyne walked the theme):

1. **Pass 1 (post-order, explicit stack)**: compute each subtree's column-width.
2. **Pass 2 (pre-order, explicit stack)**: assign each node `(Col, Row)` so each parent is centered over the span of its children.

Buttons are placed in a single `container.New(&FixedSizeLayout{...})` at absolute positions (`TreeNodeButton.LayoutX`/`LayoutY`). Connector lines are drawn before buttons: yellow (StrokeWidth=3) and blue (StrokeWidth=1.5) L-shapes with half-stroke-width overlap at corners to eliminate gaps. `FixedSizeLayout.MinSize()` returns the true content size so `container.Scroll` enables scrolling instead of force-resizing content to viewport. `GameTreeLayout` (the wrapper layout used inside `VSplit`) calls `CenterGameTreeOnCurrentNode()` on every layout pass so resizes re-center the selected node. Deterministic centering reads `LayoutX`/`LayoutY` directly (not Fyne's transient layout state) and applies via `Scroll.ScrollToOffset`.

`TreeNodeButton` labels show `Board.ToString(LastMove)` plus a suffix computed by the shared `GameTreeNodeLabel(Node)` helper: `[AC]` when the node has both annotations and a comment, `[A]` for annotations only (via `NodeHasAnnotations`), `[C]` for a non-empty Comment only, or no suffix otherwise. Each button's pixel size is computed per-node by `TreeNodeButtonSize(Label)` (text measurement + 4 px of padding on every side — enough for the 1.93 px selection-border inset, the 1.39 px stroke, and a tiny residual gap to the text), so a `C4` button is visibly thinner than a `Root` button and `RefreshGameTreeNodeLabel` reshapes the button around its new label while keeping it centered on its column. `GoWin.RefreshGameTreeNodeLabel(Node)` updates the cached button via `GameTreeNodeToButton` without a full `UpdateGameTreeUI` — call sites: Comment entry `OnChanged` in [main.go](main.go), the "Toggle Annotation" click path and the Set Label form's OK callback in [HoverMouse.go](HoverMouse.go). Hover triggers `DrawHighlights(TreeNodeAffectedCoords(Node))` so the user sees which board vertices change at that node before clicking.

**Connector lines.** `buildGameTreeUI` draws the yellow thick line (width 3) for every parent→child pair. The thin blue line (width 1.5) is drawn only for each parent's favorite child — or `Children[0]` when `FavoriteChild == nil` — so the blue line marks the path that Page Down would follow.

**Parent centering.** The layout uses two passes plus a post-order adjustment: pass 1 computes subtree widths, pass 2 assigns each node an initial `(Col, Row)` in the pre-order recursion, and the adjustment pass walks `PostOrder` **in reverse** (leaves first, then parents) setting `Parent.Col = (FirstChild.Col + LastChild.Col) / 2` so parents sit exactly halfway between their leftmost and rightmost child column — including the single-child case, where the parent lands directly above its one child. `NodeLayout.Col` and `NodeLayout.ParentCol` are `float32` to accommodate the half-integer midpoints; `NodeLayout.HasParent bool` replaces the old `ParentCol == -1` sentinel. Leaves are never adjusted, so `MaxCol` (used for total width) can stay integer. `PostOrder` is built via a single stack pop (last-pushed first), which produces a pre-order list (root first) — iterating it in reverse is what gives post-order traversal.

**Per-column widths.** `buildGameTreeUI` no longer uses a uniform `CellW`; instead each integer column `c` gets `ColWidth[c] = max` of **leaf** button widths at that column (leaves always have integer `Col`; non-leaf parents sit at fractional midpoints and are excluded — otherwise a wide parent label would bloat its children's columns and widen the gap between sibling leaves). A cumulative sweep produces `ColCenter[c]` with a small fixed `HGap` between columns, and a `CenterOf(floatCol)` helper linearly interpolates the pixel center for parents at fractional cols. When two same-row leaf siblings share their column's max width the space between their button edges is exactly `HGap`, matching the vertical row gap. A post-pass scans every node and, if any parent's button overflows past the leftmost/rightmost column boundary, shifts every column center by the measured overrun so wide parent labels still fit within the container. `LeftPad` / `RightPad` are small fixed margins so buttons don't hug the scroll viewport edges.

---

## Key Types

### `AppConfig` (Types.go)
Persisted as JSON. Key fields:
- `Themes map[string]Theme` — named themes; "Default" is from `DefaultConfig.Themes["Default"]`, never editable
- `ActiveTheme string` — which theme is active; `GetActiveTheme()` returns it
- `ShownProtectedRuleSetNames []string` — names of protected rule sets to show in the score table (e.g. `["Chinese", "Japanese", "American Go Association"]`); resolved to `*RuleSet` pointers via `FindRuleSetByName`
- `ShownCustomRuleSets []RuleSet` — user-defined rule sets shown in the score table
- `HiddenCustomRuleSets []RuleSet` — user-defined rule sets available for new games but hidden from the score table unless the game uses one
- `ShownRuleSets() []*RuleSet` — method that returns protected (by name) + shown custom rule sets as an ordered `[]*RuleSet`
- `AllCustomRuleSets() []*RuleSet` — method returning pointers to all custom rule sets (shown + hidden)
- `Engines map[string]EngineConfig` — named GTP engine configs
- `FreshBoardPresets map[string]FreshBoardPreset` — named board presets; "Default" is protected
- `ActivePresetName string` — which preset `AddFreshBoard` uses
- `PlayerNames []string` — saved player name pool for quick selection
- `GtpLogging bool` — if true, GTP commands/responses are logged to `EngineName.Player.GtpLog` files
- `WindowWidth/WindowHeight float32` — last-known outer window size (including decorators); restored on startup
- `VSplitOffset float64` — last-known vertical split offset (0.0–1.0); restored on startup

### `Theme` (Types.go)
- `CoordFmt string` — coordinate label format (`CoordFmtGtp` / `CoordFmtComputer` / `CoordFmtHikaruNoGo` / `CoordFmt1stQuadrant` / `CoordFmtSgf`).
- `HexColors map[string]string` — role-name → hex color (e.g. `"Goban"`, `"3-Liberty Line"`, `"Last Move"`, `"Child Node"`, `"Annotation"`, `"Coord Hover"`, `"Neighbor"`).
- `PlayerHexColors map[uint8]string` — per-player base color.
- Display flags: `ShowLibertyLine`, `ShowStoneConnection`, `ShowIllegalDot`, `ShowHoverLiberties`, `ShowChildNodeDots` — toggle each visual layer independently. `ShowChildNodeDots` draws a small "Child Node" dot at every move-type child's `LastMove`, stroke-colored by the player who would move (mirrors `DrawLastMoveHighlight`).

### `GoWin` (Types.go)
- `CurrentNode() *GameTreeNode` helper (GameTreeUi.go) — returns `Coll.CurrentGameH.CurrentNode` when a game is selected, or `Coll.CollectionNode` when on the collection node. Use this everywhere instead of `Coll.CurrentNode`.

One per window. Drawing layer fields:
- `Layers map[string]*fyne.Container` — all named drawing layers, keyed by the strings in `LayerOrder`. Layers bottom-to-top: `"Goban"` (coord bg + labels + goban rect + grid lines; only redrawn on resize, board-dimension change, or symmetry change), `"LibertyLine"` (stone-to-liberty lines, Liberty color), `"IllegalDot"`, `"StoneConnection"`, `"Stone"`, `"LastMoveHighlight"` (includes child-node dots when `Theme.ShowChildNodeDots`), `"Annotation"` (Circle / Square / Triangle / X Mark), `"Label"` (LB text), `"Territory"` (Score mode markers), `"Highlights"` (game-tree node hover squares and score table cell hover squares), `"Hover"` (hover stone + coords + liberties + neighbor dots + annotation preview — rendered last so the user's pointer feedback is never obscured).
- `LibertyLayer *fyne.Container` — cached liberty-dot sub-container within the Hover layer; rebuilt only when the hovered group changes.
- `LastGobanKey GobanLayerKey` — tracks the previous `PlayContainer` size, board dimensions (`Width`, `Height`), and `SymmetryIndex`; triggers `DrawGoban` when any changes.

Game tree partial update fields:
- `GameTreeNodeToButton map[*GameTreeNode]*TreeNodeButton`
- `GameTreeNodeToContainer map[*GameTreeNode]*fyne.Container`

Key engine fields:
- `PlayerEngineItems []*fyne.MenuItem` — [Player 1]/[Player 2] submenus reflecting live attachments
- `GtpEngines map[uint8]*GtpEngineState` — player → live engine process
- `PassItem *fyne.MenuItem` — Pass menu item; disabled via `RefreshPassMenuItem()` when passing is unavailable (non-Play mode, board nil, or next player is engine-controlled)
- `DiplomacyItem *fyne.MenuItem` — Game → Diplomacy menu item; disabled via `RefreshDiplomacyMenuItem()` when the matrix is fixed or board is nil
- `MainMenu *fyne.MainMenu` — stored so `RefreshEngineMenuItems` / `RefreshPassMenuItem` / `RefreshDiplomacyMenuItem` can call `SetMainMenu` to propagate disabled state

Dialog fields:
- `DialogShowing bool` — true while any dialog is open; blocks keyboard navigation
- `DismissDialog func()` — set to `dialog.Hide()` when a dialog opens, cleared in its close callback; called by `HandleKeyEvent` on `fyne.KeyEscape` to close the current dialog
- `SubmitDialog func()` — set to `dialog.Confirm()` when a dialog opens, cleared in its close callback; called by `HandleKeyEvent` on `fyne.KeyReturn`/`fyne.KeyEnter` (skipped when the focused widget is a multi-line entry)
- `ResizeDialog func()` — set by window-sized dialogs to a closure that reads the current canvas size and calls the dialog's `Resize(...)`. Cleared in the close callback. Piggybacks on `InputLayer.Resize`'s existing ~4 ms debounced timer in [InputLayer.go](InputLayer.go): when the timer fires (drag has settled) the `fyne.Do` callback calls `DrawBoard()` and then `ResizeDialog()` if non-nil. So the dialog relayouts once per drag-settle rather than per event, with zero separate bookkeeping. Used by Collection, Game Information, Fresh Board, and SGF/CGG import/export file dialogs.
- `DialogClosed()` method (in [InputLayer.go](InputLayer.go)) — canonical dialog-close cleanup: clears `DialogShowing` / `DismissDialog` / `SubmitDialog` / `ResizeDialog`. Every window-sized dialog's close callback calls this as its first line.
- `WireSubmitOnEnter(*DialogEntry...)` — sets `OnSubmitted` on each single-line entry to call `SubmitDialog`, working around Fyne's mutually-exclusive focused-widget vs canvas key routing. Takes `*DialogEntry` rather than `*widget.Entry` so Escape handling is uniform across every dialog input.

Window state fields:
- `VSplit *container.Split` — the vertical split between the comment/game-tree panel and the board; offset is persisted to config via `SaveWindowState()`
- `HSplit *container.Split` — the horizontal split between the left panel (controls/game tree) and the goban board; auto-adjusted right when entering Score mode if the score table needs more horizontal room

Mouse mode fields:
- `SetVertexPlayer uint8` — selected player for editing, with 0 meaning empty
- `SetAnnotationMask uint8` — currently selected annotation type (CircleMask/SquareMask/TriangleMask/XMask)
- Mouse Mode → Toggle Annotation selects Circle, Square, Triangle, or X Mark.
- `SetVertexMenuItem *fyne.MenuItem` — Mouse Mode → Set Vertex submenu, with Empty (0) and one choice per player in the current game.
- `SetVertexMenuGameH *GameHistory` — game represented by the submenu, or nil for no active game. `RefreshSetVertexMenu` in [main.go](main.go) initializes it during menu creation. After assigning the active game, `SetCurrentNode` refreshes the submenu only when this pointer differs from `Coll.CurrentGameH`. Tracking the represented game also catches imports and Fresh Board actions that replace the collection before navigation. Moving between nodes in the same game and calling `FixWindowTitle` do not rebuild the submenu.

Game selector field:
- `GameSelectorSelect *widget.Select` — dropdown for selecting between games in the collection (always visible)

`PlayerColor` struct:
- `Base, Hover, Territory, Contrast color.NRGBA` — `Contrast` auto-computed by `ContrastColor()` from `Base`

### `Group` (Types.go)
- `Owner uint8` — player number that owns the group.
- `Vertices map[uint8][]Coord` — all vertices associated with this group. `Vertices[Owner]` = the stones in this group. `Vertices[0]` = liberties (empty). `Vertices[k]` for k != Owner = adjacent stones of player k.

### `BoardState` (Types.go)
- `Vertices map[Coord]uint8` — the board: bits 0–3 = player (0=empty, 1=black, 2=white, …15); bits 4–7 = annotation flags (`XMask`=0x10, `SquareMask`=0x20, `CircleMask`=0x40, `TriangleMask`=0x80). Use `GetPlayerAt(c)` / `SetPlayerAt(c, player)` to read/write the player; `SetAnnotation`/`ClearAnnotation`/`HasAnnotation`/`ToggleAnnotation` for annotation bits.
- `ConversionsTo map[uint8]int` — stones converted TO each player's color (CaptureConverts only).
- `ConversionsFrom map[uint8]int` — stones converted FROM each player's color (CaptureConverts only).
- `IsPlacementLegalCache map[uint8]map[bool]map[Coord]error` — legality cache keyed by player, EditMode flag, then coordinate.
- `NextLibertySharingMatrix *LibertySharingMatrix` — governs the next move from this board; inherited by child boards via `Copy` / `CopyWithoutGroups`; replaced only when the Diplomacy dialog commits.
- `NoResultTriggered bool` — set when a placed move triggers a `NonRepetitionRuleNoResult` positional repetition. Copied to `GameTreeNode.NoResultDraw` at node creation.
- Copies via `CopyWithoutGroups()` / `Copy()` use `CopyVertices(Vertices, KeepAnnotations)`. Non-edit copies strip annotation bits.

### `GameTreeNode` (Types.go)
- `PreviousStonePlacer uint8` — player who made the last move (renamed from `LastPlayer`).
- `NextStonePlacer uint8` — player who will make the next move (renamed from `NextPlayer`). `GetNextStonePlacer()` returns this if set, else cycles via `NextPlayer(PreviousStonePlacer, Players)`.
- `RemainingStonePlacements uint8` — how many stone placements remain in the current turn. Defaults to `RuleSet.StonePlacementsPerMove`. When >1 and the placement is not a pass, the next node keeps the same player with one fewer placement. When ==1 or on a pass, the next node switches to the next player and resets to `StonePlacementsPerMove`.
- `UnixMilli int64` — Unix timestamp in milliseconds when the node was created. For SGF-imported nodes, set to 0 to indicate no timing information; the status bar (`UpdateStatusBar` in main.go) skips timing displays when `UnixMilli == 0`.
- `NoResultDraw bool` — true when this move triggered a `NonRepetitionRuleNoResult` positional repetition. `IsGameOver` returns true and the score table prepends a "No Result (Draw)" row.
- Annotations (Circle / Square / Triangle / X Mark) are stored in the `Vertices` bits of `Board`, not in any separate map.
- `Labels map[Coord]string` — text annotations.
- `UnknownSgfProperties map[string][]string` — unknown SGF properties reproduced verbatim on export.
- `TerritoryMap map[Coord]uint8` — per-node territory ownership (preserved across Score-mode entry/exit; populated by `InitializeTerritoryMap` from `Board.Vertices` only when nil).

### `GameHistory` (Types.go)
Per-game metadata:
- `PlayerNames map[uint8]string` — per-player display names (player number → name); PB/PW in SGF map to keys 1/2
- `Information map[string]*string` — keys are human-readable descriptions, defined in `SgfToDescription` in [ConstantVariable.go](ConstantVariable.go)
- `Komi map[uint8]float64` — per-player komi; all players' komi is configurable
- `Width/Height uint8`, `Players uint8`, `RuleSet *RuleSet`
- `WrapXMulY/WrapYMulX int8`, `WrapXShiftY/WrapYShiftX uint8` — wrapping (multipliers must be 0 or have `|v| ≤ Dim` and `gcd(|v|, Dim) == 1`; cached `WrapXMulYInv`/`WrapYMulXInv int16` are recomputed by `RecomputeWrapInverses()`)
- `LibertySharingFixed bool` — when true the `Game > Diplomacy` menu item is disabled and every node's `Board.NextLibertySharingMatrix` is identical. SGF and GTP-derived games always set this to true.
- `HalfIntegerMoku() string` — formats komi for SGF/GTP (difference between player 2 and player 1 komi)
- `CurrentNode *GameTreeNode` — the currently selected node within this game; initialized to `RootNode` in `NewRootNode` and never nil

### `FreshBoardPreset` (Types.go)
- `Players uint8` — number of players for this preset (0 treated as 2)
- `PlayerNames map[uint8]string` — per-player display names, copied into GameHistory.PlayerNames on board creation
- `Komi map[uint8]float64` — per-player komi for preset, copied into GameHistory.Komi on board creation
- `Information map[string]string` — non-pointer version of GameHistory.Information; copied into board on creation using `for key, val := range preset.Information { v := val; hist.Information[key] = &v }`
- `WrapXMulY int8`, `WrapYMulX int8` — wrapping multipliers. Valid values: 0 (no wrap), or |v| ≤ Height/Width with gcd(|v|, Height/Width) == 1 (checked by `IsValidWrapMul` in [BoardLogic.go](BoardLogic.go)). On `GameHistory`, `WrapXMulYInv` / `WrapYMulXInv` (non-serialized `int16`) cache the modular inverses; `RecomputeWrapInverses()` is called once after `AddFreshBoardFromPreset` and after `DecodeGame` sets the Wrap fields, so `GameHistory.Neighbors` can wrap back across the Y=0 / X=0 edge via a cheap multiply instead of recomputing extended-GCD per call. Copied into GameHistory on board creation.
- `WrapXShiftY uint8`, `WrapYShiftX uint8` — wrapping shifts (must be < board height/width respectively); copied into GameHistory on board creation
- `LibertySharingFixed bool` — copied to `GameHistory.LibertySharingFixed`
- `LibertySharingMatrix *LibertySharingMatrix` — initial matrix; nil → filled with no-sharing default at use time

---

## Engine Lifecycle

Engine contains **Settings**, **Player 1**, **Player 2**, and **Detach All Engines**. Each player submenu contains **Human**, followed by alphabetically sorted configuration names. Checks represent actual per-window attachments; both players start as Human. Checked engine entries remain clickable: selecting one kills its old process and starts a fresh process with the current configuration. Selecting Human kills only that player's engine. There is no separate start/stop match state or persisted player assignment/auto-start setting.

- `ShowEngineSettings` edits named executable/argument configurations and GTP logging. Saving refreshes the player menus; changing, renaming, or deleting an attached configuration detaches its process. Editor fields remain usable with no configurations so a first engine can be created.
- `EngineGameSupported` centralizes attachment/menu restrictions: an active two-player game, one stone placement per move, and a fixed no-sharing liberty matrix. Settings and Human remain available for unsupported games.
- `SelectPlayerEngine(Player, EngineName)` uses an empty name for Human; otherwise detaches the old process, attaches and initializes the selected configuration, refreshes checks, and requests the next engine turn. Attachment failures leave Human checked.
- `AttachEngineForPlayer(Player, EngineName)` starts the process and creates `GtpEngineState`; `InitializeEngineForPlayer` checks required commands, configures board dimensions/komi, and replays the current stones.
- `RefreshEngineMenuItems` builds `GoWin.PlayerEngineItems` from configuration names and live `GoWin.GtpEngines` pointers, allowing checked engines to be reselected, and refreshes Pass availability.
- `TriggerEngineIfNeeded` handles human-versus-engine and engine-versus-engine play. It requests one `genmove` for the next player, then schedules the next turn after applying a successful move. It stops at game over or a human turn. `GtpEngineState.Thinking` prevents duplicate requests and is accessed only on the UI thread.
- The move callback captures the source node and engine pointer. It ignores responses whose engine was detached/replaced or whose source node is no longer current. Errors and invalid/illegal moves detach the affected engine; resignation detaches all engines and shows a notification.
- `HandleEngineMove` parses coordinates/pass/resign. `PlayMove(C, Player, SkipEnginePlayer)` advances the tree through `SetCurrentNode(Node, true)` and informs other attached engines with `play`; 0 informs every engine for human moves. Failed notifications detach the affected engine.
- `DetachEngineForPlayer` removes the runtime attachment before killing/reaping the process, closes pipes/logging, and refreshes Human selection. `DetachAllEngines` applies this to every player and runs when the window closes or via Engine → Detach All Engines (Ctrl+Q). The menu action and canvas shortcut share this method, returning all players in the window to Human. `CtrlQ` is a `desktop.CustomShortcut` for Control+Q; assigning it to the menu item allows detachment even when a text field has focus. Ctrl+X retains its standard Cut behavior.
- `OnUserNavigate` detaches engines before browsing. `SetCurrentNode(Gtn, KeepEngines...)` also detaches on navigation/import/game changes unless explicitly called with `true` by `PlayMove`. This stops automatic play during tree inspection and prevents pending GTP responses from modifying a browsed position. Select an engine again to resume at that position.

**Threading:** `genmove` runs outside `fyne.Do`; its result is applied inside `fyne.Do`. `SendGtpCommandToEngine` serializes request/response pairs with `GtpEngineState.CommandMutex`, also used during log cleanup. Detachment kills the process and closes pipes before acquiring this mutex, interrupting a pending read. Initialization and `play` notifications currently remain synchronous.

**Turn enforcement:** `HandleMouseClick`, `HandlePass`, and hover preview check the next player's live engine attachment. Humans cannot place stones or pass for an engine-controlled player.

`GtpEngine_test.go` checks sorted menu choices, initial Human selection, attaching, reselecting/replacing a live process, cancellation of pending GTP I/O, Human after detachment, and attachment restrictions using a temporary mock GTP executable.

---

## Multi-Player Support

The application supports 1-15 players per game, with full game logic working for any number of players.

### Player Number System
- **Player IDs**: 1-15 (0 = empty intersection)
- **Turn cycling**: `NextPlayer(p, n)` and `PreviousPlayer(p, n)` functions in SgfUtils.go
- **Board storage**: `BoardState.Vertices[Coord]uint8` - low 4 bits store player number
- **Validation pattern**: `if 0 < player && player <= maxPlayers` throughout codebase

### Multi-Player Game Logic
- **Group calculation**: `CalculateGroupAtCoord` works for any player (not just 1/2), and walks through liberty-shared stones according to `Board.NextLibertySharingMatrix`
- **Capture detection**: Finds captures by any adjacent opponent group
- **Repetition check**: five rules — `BasicKo`, `NaturalSituationalSuperKo`, `SituationalSuperKo`, `PositionalSuperKo` (strict hierarchy), plus `NoResult` (Positional scope but ends the game as a draw instead of rejecting the move)
- **Territory assignment**: Treats dead stones as empty for proper territory flow
- **Scoring**: Supports per-player scoring with configurable komi

### UI Indicators
- **Window title**: Shows "Player N (Name) — " or "Player N — " prefix for next player
- **Stone colors**: Uses `PlayerColors[player]` mapping for all players
- **Engine menu**: Engine choices disabled for unsupported games; Settings and Human remain available

### External Protocol Limitations
- **GTP engines**: Only support 2-player games (B/W). Engine choices disabled and engines detached for unsupported games.
- **SGF format**: Standard only supports 2 players. Multi-player games need custom SGF extensions or alternative formats — use the native CGG JSON format instead.

### Code Patterns for Multi-Player
```go
// ✅ Correct - supports any number of players
player := B.GetPlayerAt(c)
if 0 < player && player <= B.Hist.Players {
    // Handle player's stone
}

// ❌ Incorrect - hardcoded for 2 players only
switch B.PlayerAt(c) {
case 2, 1:
    // Only handles players 1 and 2
}
```

---

## Config Pattern

`LoadConfig` (Config.go):
1. Start from `DefaultConfig` copy
2. Nil all map/slice fields before JSON decode (prevents mutating `DefaultConfig`'s shared instances)
3. JSON decode into the copy
4. Regenerate "Default" theme from `DefaultConfig.Themes["Default"]` (never persisted)
5. Call `ValidateAndFillConfig()` — validates all fields, fills missing/invalid ones with defaults

`ValidateAndFillConfig` validates:
- `ShownProtectedRuleSetNames` — keeps only names that resolve via `FindRuleSetByName`; falls back to `DefaultConfig` names only when the field is missing/nil, while an explicitly empty slice means the user hid every built-in rule set from the score table
- `ShownCustomRuleSets` / `HiddenCustomRuleSets` — ensures non-nil; runs `DeduplicateCustomRuleSets`
- Engine, preset, theme, and player name fields — ensures maps exist and references are valid
- Re-links preset RuleSet pointers to canonical slice elements (`RelinkPresetsToCustomSlices`)

`SaveConfigSnapshot(Config)` — JSON-encodes the snapshot and writes to disk only if the serialized bytes differ from `LastSavedConfigBytes` (seeded at startup by `SnapshotCurrentConfigBytes`). This avoids redundant writes when no changes were made, and avoids creating a config file for users who never changed anything from the default. `SaveConfig()` snapshots `CurrentAppConfig` synchronously; `SaveConfigAsync()` runs the same on a goroutine so resize handlers don't block on disk I/O.

To add a new config field:
- Add to `AppConfig` struct in Types.go with JSON tag
- Add default to `DefaultConfig` in [ConstantVariable.go](ConstantVariable.go)
- If it's a map/slice: nil it in `LoadConfig` before decode
- Add validation in `ValidateAndFillConfig` if needed

---

## Menu Shortcut Presentation

Menu labels omit shortcut prefixes. In `main.go`, each menu item with a Ctrl shortcut uses `fyne.MenuItem{Label: ..., Action: ..., Shortcut: Ctrl...}` so Fyne renders the key combination in its separate shortcut column and dispatches it through the menu. Reuse the definitions in `ConstantVariable.go` and the existing action handlers. Canvas shortcut registrations remain available as well. Pass and Delete Node use `PassShortcut` and `DeleteNodeShortcut`, zero-modifier `desktop.CustomShortcut` definitions in `ConstantVariable.go`, to display P and Delete in the same column. Their unmodified key handling remains in `HandleKeyEvent`, preserving normal text entry and deletion in focused text fields.

## UI Dialog Pattern

**One file per top-level dialog.** Top-level dialogs (those reachable directly from a menu item or keyboard shortcut) live in their own `*.go` file ([FreshBoardDialog.go](FreshBoardDialog.go), [EngineSettingsDialog.go](EngineSettingsDialog.go), [ExportCggDialog.go](ExportCggDialog.go), [ExportImageDialog.go](ExportImageDialog.go), [GoToMoveDialog.go](GoToMoveDialog.go), [SearchCommentDialog.go](SearchCommentDialog.go), [DiplomacyDialog.go](DiplomacyDialog.go), [ShowGameInformationDialog.go](ShowGameInformationDialog.go), [ShowCollectionDialog.go](ShowCollectionDialog.go), [RuleSetDialog.go](RuleSetDialog.go), [ThemeDialog.go](ThemeDialog.go)). Subdialogs (those spawned from another dialog's callback) stay in the parent's file: e.g., `ShowModifyRuleSetDialog`/`ShowModifyThemeDialog`/`ShowCustomColorPicker` stay in their parent, the "Set Label" inline form stays in [HoverMouse.go](HoverMouse.go), and `ShowLibertySharingMatrixEditor` stays in [LibertySharing.go](LibertySharing.go).

**Simple forms** (static list of fields): use `dialog.NewForm(title, "OK", "Cancel", formItems, callback, win)`.

**Complex/dynamic dialogs** (dynamic rows, custom layout): use `dialog.NewCustomConfirm(title, "OK", "Cancel", content, callback, win)` where `content` is a `container.NewVScroll(container.NewVBox(...))`.

**Window-sized dialogs** call `Dlg.Resize(WindowDialogSize(GoWin.Win.Canvas().Size()))` and set `GoWin.ResizeDialog` so live window-resize re-fits them. Every dialog sets `GoWin.DismissDialog`/`SubmitDialog` and clears them via `DialogClosed()` in its callback so Esc/Enter behave consistently.

**Window-tall dialogs** (`ShowModifyThemeDialog` and `ShowModifyRuleSetDialog`) skip the `WindowDialogSize` helper and instead call `Dlg.Resize(fyne.NewSize(1, GoWin.Win.Canvas().Size().Height))` — width 1 collapses to the form's intrinsic minimum, height matches the canvas. `GoWin.ResizeDialog` mirrors the same closure so window resize keeps the dialog full-height.

**DialogEntry** ([DialogEntry.go](DialogEntry.go)) is a thin `widget.Entry` subclass that routes `fyne.KeyEscape` to `GoWin.DismissDialog` before delegating to the embedded Entry's `TypedKey`. Fyne's default `widget.Entry` consumes every key event while focused, so the canvas-level handler in `GameHandlers.go` never sees Escape when an input has focus. Every text input inside our own dialogs is built with `NewDialogEntry(GoWin)` / `NewMultiLineDialogEntry(GoWin)` so Escape always dismisses. `GoWin.WireSubmitOnEnter(...*DialogEntry)` stays — the signature is tightened to `*DialogEntry` so every wired-up entry participates in both behaviors. Fyne's built-in `dialog.NewFileSave` / `dialog.NewFileOpen` file dialogs are out of scope — they have their own Entry widgets that we don't subclass.

Reference implementations:
- Simple: `HandleNewGame` in [FreshBoardDialog.go](FreshBoardDialog.go)
- Complex with dropdown selector: `ShowEngineSettings` in [EngineSettingsDialog.go](EngineSettingsDialog.go) (dropdown selects engine; Save/Update/Delete buttons)
- Complex with live preview: `ShowThemeDialog` in [ThemeDialog.go](ThemeDialog.go)

---

## GTP Flow

Standard GTP command sequence for initialization:
```
list_commands        → verify required commands exist
boardsize N          → or rectangular_boardsize W H
komi K
play B/W coord       → for each existing stone (board sync)
genmove B/W          → request a move
```

Coordinate translation:
- Internal: `Coord{X, Y}` (0-origin, Y=0 at top)
- GTP: letters A–Z skipping I, numbers 1-origin from bottom
- Conversion: methods on `*GameHistory`

GTP engine response format: `= response\n\n` (success) or `? error\n\n` (failure).

---

## Scoring

`RuleSet` struct (Types.go) has `Names []string` (Names[0] is the display name; extra entries are aliases for SGF import), multipliers `LiveStones`, `Prisoners`, `Passes`, `StoneSuicides`, `Conversions float64`, flags `LastPlayerMustPassLast`, `SuicideIsLegal`, `CaptureConverts bool`, `NonRepetitionRules uint` (bitmask), and `StonePlacementsPerMove uint8` (number of stones a player places per turn; 1 for standard Go, >1 for multi-stone variants). `RuleSet.Equal(other)` performs content-based equality comparison (`reflect.DeepEqual`).

`ProtectedRuleSets []RuleSet` (in [ConstantVariable.go](ConstantVariable.go)) holds the built-in rule sets. Chinese is index 0 (default). `FindRuleSetByName(name) *RuleSet` searches all Names for SGF import. `RegisterRuleSet` assigns a unique ID per pointer for disambiguation. `RuleSetDisplayName` returns the display name, appending `#<id>` only if another shown rule set (by `Equal`) shares the same `Names[0]`.

`GameHistory.RuleSet *RuleSet` stores the active rule set for each game (set in `NewRootNode`, `AddFreshBoardFromPreset`, and SGF import). All runtime access uses `Hist.RuleSet` directly — no name-based map lookups. `AppConfig.ShownRuleSets()` returns the ordered list of rule sets shown in the score table (protected by name, then shown custom).

`BoardState.StoneSuicides map[uint8]int` tracks stones lost to suicide per player. When `SuicideIsLegal` is true, a move that self-captures removes the group and adds its stone count to `StoneSuicides[Player]`. The Stone Suicides row is hidden in the score table when `SuicideIsLegal` is false.

`BoardState.Prisoners map[uint8]int` tracks captured stones per player (standard captures only, not CaptureConverts). The `Prisoners` multiplier scores the sum of on-board and off-board prisoners.

`BoardState.ConversionsTo map[uint8]int` / `ConversionsFrom map[uint8]int` track stone conversions (CaptureConverts only). Standard captures update `Prisoners`, not ConversionsTo/From. Net conversions (`ConversionsTo[P] - ConversionsFrom[P]`) are scored via the `Conversions` multiplier.

Built-in rule set multipliers (from `ProtectedRuleSets`):
- **Chinese** — territory only, `NonRepetitionRulePositionalSuperKo`
- **Japanese** — `LiveStones=-1`, `Prisoners=1`, `StoneSuicides=-1`, `BasicKo | NoResult`
- **American Go Association (AGA)** — `LiveStones=-1`, `Prisoners=1`, `StoneSuicides=-1`, `Passes=-1`, `LastPlayerMustPassLast`, `SituationalSuperKo`
- **British Go Association (BGA)** — territory only, `LastPlayerMustPassLast`, `NaturalSituationalSuperKo`
- **New Zealand** — territory only, `SuicideIsLegal`, `SituationalSuperKo`
- **Nazgand 3 Stone** — `StonePlacementsPerMove=3`, `SuicideIsLegal`, `LastPlayerMustPassLast`, custom multipliers (`LiveStones=3.91547`, `Prisoners=5.13974`, `StoneSuicides=-1.547`, `Passes=-0.404`), `NaturalSituationalSuperKo`
- **Nazgand Convert** — `CaptureConverts`, `SuicideIsLegal`, `LastPlayerMustPassLast`, custom multipliers (`LiveStones=9.3`, `Conversions=1.5`, `Passes=-0.404`), `NaturalSituationalSuperKo`
- **Korean** — `LiveStones=-1`, `Prisoners=1`, `StoneSuicides=-1`, `BasicKo | NoResult`

Score table rows are dynamically shown/hidden: Prisoners row hidden for CaptureConverts rule sets, Stone Suicides row hidden when suicide is not legal, Conversions row hidden for non-CaptureConverts rule sets. Cell tooltips show per-row multiplier breakdowns; for Live Stones, Territory, and Prisoners hovering also highlights the relevant board coordinates via `Layers["Highlights"]`.

`FormatPoints(V float64) string` formats point values (komi, scores, etc.) with 9 digits of precision, stripping trailing zeros and trailing decimal point. Used everywhere except SGF export.

Komi applies to all players except player 1 (Black).

**`IsGameOver() bool`** (Scoring.go): unified end-of-game check. Walks back `numPlayers` nodes; returns `true` only when every node is a pass. Passes always create exactly one node (consuming all remaining stone placements), so this works correctly with `StonePlacementsPerMove > 1`. If `Hist.RuleSet.LastPlayerMustPassLast` is set (AGA/BGA/Nazgand variants), the final pass must come from player `numPlayers` (White in a 2-player game). Also returns `true` for any node whose `NoResultDraw` is true. Called by `SetCurrentNode` (triggers Score mode for human play), `TriggerEngineIfNeeded` before firing a new `genmove`.

`InitializeTerritoryMap` seeds `GoWin.CurrentNode().TerritoryMap` from `Board.Vertices` (player bits only) **only when nil**, preserving any existing dead/alive assignments across Score-mode entry/exit; `ToggleGroupStatus` flips dead/alive status with propagating rebel sets for multi-player.

---

## SGF Round-Trip

`GameHistory.Information` keys are **description strings** (e.g. `"Name Of Player 1"`), not SGF property codes. The `SgfToDescription` map in [ConstantVariable.go](ConstantVariable.go) and its inverse `DescriptionToSgf` translate between them.

`RootInfoPropertyOrder` in [ConstantVariable.go](ConstantVariable.go) defines the emit order for SGF root properties.

SGF export is restricted to 2-player games with board sizes ≤ 52×52 (the SGF coord-character limit). Games failing those constraints are silently skipped; if no games qualify, no file is written.

On SGF import, unknown properties are stored verbatim in `GameTreeNode.UnknownSgfProperties` and reproduced on export. Per-game parse errors don't abort the whole import: `SkipCurrentGame` removes the partial root node, jumps past the malformed tree via `CaptureRawTree`, and shows a warning dialog. The remaining games still load.

AW/AB/AE are **not stored** in the node — on export they are computed: root nodes emit all stones from `Board.Vertices`; edit nodes (`LastMove == EditCoords`) emit the diff against `Parent.Board`. CR/SQ/TR/MA are emitted by scanning `Board.Vertices` for set annotation bits.

---

## CGG JSON Save Format

Native save format (`.CGG.json`) that captures the full `GoWin.Coll` including multi-player games, custom rule sets, per-player liberty-sharing matrices, timestamps, territory maps, and annotations. Optional compression: `.CGG.json.kanzi` (Kanzi), `.CGG.json.gzip` (Gzip), `.CGG.json.zstd` (Zstd; `ZstdMaxLevel=22` though the pure-Go encoder clamps ≥10 to `SpeedBestCompression`).

**Top level:** `{ "SaveGameFormat": "ConnectedGroupsGoban3", "CurrentGame": <index>, "Games": [...] }`.

**Node kinds:** `"Root"` (full board state), `"Move"` (LastMove + PreviousStonePlacer, reproduced via `AttemptMove`), `"Edit"` (PlayersDiff + AnnotationsDiff vs parent). Each node may carry `"CurrentNode": true` to mark the game's selected node at save time.

**Omitted from CGG:** `UnknownSgfProperties` (SGF-only), `NonGoGameTrees` (raw SGF), `Information["Komi"]` (stored in dedicated `Komi` field).

**Non-repetition rules** are serialized as a string slice containing only the strongest scope flag plus `"NoResult"` if enabled. Weaker flags are auto-filled on load.

**Liberty matrix** is stored in full at the root; child nodes store only a sparse diff (cells that changed vs parent). Cell values: `"N"` / `"C"` / `"Y"`.

**Menu:** File > Load Game (Ctrl+O), Save Game (Ctrl+S). The Save Game dialog ([ExportCggDialog.go](ExportCggDialog.go)) offers method selection (Kanzi/Gzip/Zstd/Plaintext) and a compression-level slider whose range adjusts per method (`MaxLevelForMethod`: Kanzi=9, Gzip=9, Zstd=22, Plaintext=0/hidden). Default method is Kanzi at level 9.

**Load validates strictly:** move nodes are replayed via `AttemptMove`; illegal moves cause that game to be skipped (other games in the collection still load).

---

## Common Modification Patterns

**Add a new board/game setting to the New Game dialog:**
1. Add field to `FreshBoardPreset` in Types.go
2. Add widget to `HandleNewGame` in [FreshBoardDialog.go](FreshBoardDialog.go)
3. Apply to board in `AddFreshBoard` / `AddFreshBoardFromPreset` in GameTreeUi.go
4. Add default to DefaultConfig's "Default" preset in [ConstantVariable.go](ConstantVariable.go)
5. Wire CGG round-trip in [CggJsonIo.go](CggJsonIo.go) if the field should persist

**Add a new engine capability (e.g. time controls):**
1. Add field to `EngineConfig` in Types.go
2. Add widget to `ShowEngineSettings` in [EngineSettingsDialog.go](EngineSettingsDialog.go)
3. Use in `InitializeEngineForPlayer` in GtpEngine.go

**Add a new top-level dialog:**
1. Create `MyFeatureDialog.go` with a single exported function (`ShowMyFeatureDialog` or similar).
2. Set `DismissDialog`/`SubmitDialog` and clear them in the close callback via `DialogClosed()`.
3. Wire `ResizeDialog` if the dialog should fill the window.
4. Add a menu item or `Ctrl+*` shortcut in [main.go](main.go).
