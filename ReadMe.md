# ConnectedGroupsGoban

A Weiqi (Go / Baduk) goban written in Go using the Fyne GUI toolkit, featuring multi-player games, custom rule sets, board wrapping, per-window symmetries, GTP engine support, animated SVG export, and an asymmetric liberty-sharing diplomacy system.

## Screenshots

![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024327.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024354.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024411.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024440.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024500.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024514.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024604.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024637.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024700.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024730.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024909.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_024951.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025110.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025150.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025245.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025328.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025535.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025610.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025630.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_025700.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_030216.png)
![ConnectedGroupsGoban screenshot](Screenshots/Screenshot_20260918_030246.png)

# Availability

- Linux
- Windows
- Android
- Source code

---

# Highlights

- **1–15 players per game** with per-player names, per-player komi, and per-player stone colors.
- **Rectangular boards up to 255×255** with optional general wrap multipliers and shifts (Klein bottle, twisted torus, cylinder, …).
- **8 built-in rule sets** plus a full editor for custom rule sets, with configurable scoring multipliers, four scope-tiered SuperKo flags, NoResult draws, suicide, capture-converts cascade, and multi-stone-per-turn placements.
- **Per-window 8-symmetry view transform** (rotations + flips) — a pure visual rotation that never modifies the board state.
- **Asymmetric liberty sharing (Diplomacy)** — each player decides whether to share their groups' liberties with each other player; can be fixed at game-creation time or editable mid-game.
- **GTP engine support** (1 or 2 engines per game; play immediately on selection; engine-vs-engine self-play; per-engine GTP logging).
- **Native CGG JSON save format** with optional Kanzi, Gzip, or Zstd compression — captures everything multi-player games need that SGF can't represent.
- **SGF import/export** (2-player games, ≤ 52×52). Partial collection load: a malformed game in a multi-game file does not abort the whole import.
- **Static and animated SVG export** of the current position or the entire game.
- **Comments, four-mark annotations (Circle / Square / Triangle / X Mark), and text labels** on any vertex.
- **Named themes** with a color editor; per-theme toggles for liberty lines, illegal-move dots, stone connections, hover-liberty markers, and child-node dots (preview where each branch child would move).
- **Five coordinate label formats** (GTP, Computer, Hikaru No Go, 1st-Quadrant math, SGF).

---

# Menus

## File

| Item | Shortcut | Action |
|------|----------|--------|
| Save Game | `Ctrl+S` | Save the entire collection as CGG JSON. Pick a compression method (Kanzi / Gzip / Zstd / Plaintext) and a per-method compression level. |
| Load Game | `Ctrl+O` | Load a CGG file (any of `.CGG.json`, `.CGG.json.kanzi`, `.CGG.json.gzip`, `.CGG.json.zstd`). |
| Export board as SVG |  | Single-frame SVG of the current position, or animated SVG of the entire game (one frame per node along the favorite-child path, with a configurable seconds-per-node). |
| Export Sgf | `Ctrl+Shift+S` | Write the collection to an SGF file (2-player games ≤ 52×52 only). |
| Import Sgf | `Ctrl+Shift+O` | Load an SGF file. Per-game parse errors no longer abort the whole import — the bad game is skipped with a warning and the rest still load. |

## Game

| Item | Shortcut | Action |
|------|----------|--------|
| New Game | `Ctrl+N` | Open the **Fresh Board** dialog (see below). |
| Game Information |  | View read-only board fields (size, players, rule set, wrap params) and edit per-player names, per-player komi, and the SGF-style information rows (Game Name, Result, Date And Time, Event, …). |
| Go To Move # | `Ctrl+G` | Jump to a specific move number along the favorite-child path from the root. |
| Search for move by text in comment |  | Case-insensitive substring search across `GameTreeNode.Comment` for the current game (default) or the entire collection (checkbox). Results show as `[game name] Move N — snippet`; pick a row + Go to navigate there. |
| Collection |  | Open the **Collection** dialog: filterable / sortable table of every game in the collection (Name, Players, Rule Set, Komi, Size, Moves, Root Time, Date, Result, Event). Filters: per-player name slots, rule set, komi, size range, root-time window, comment substring. Click a header to sort, click again to reverse; ▲/▼ marks the active sort. |
| Pass | `P` | Pass for the next player (Play mode only; not available when the next player is engine-controlled). |
| Diplomacy | `Ctrl+D` | Edit the next-to-move player's outgoing row of the liberty-sharing matrix (only available when the matrix is not fixed for this game). |
| Delete Node | `␡` | Delete the current node and select its parent. |

### Fresh Board dialog

Pick board dimensions (1–255 in each axis), number of players (1–15), the rule set (built-in + custom), per-player names, per-player komi, board wrapping (`WrapXMulY`/`WrapYMulX` may be 0 (to disable wrap) or any int8 with `|v| ≤ Dim` and `gcd(|v|, Dim) == 1`; `WrapXShiftY`/`WrapYShiftX < Dim`), the liberty-sharing-fixed flag, and an optional liberty-sharing matrix editor. Save the field set as a named preset (Save / Update / Delete buttons; the "Default" preset is protected). The "Delete other games in collection" checkbox replaces the entire collection on confirm. Saved player-name pool: a "[Save Name]" / "[Delete Name]" dropdown lets you maintain a quick-select list of names you reuse.

## App

| Item | Action |
|------|--------|
| Theme | Open the theme editor: pick the active theme; create / duplicate / modify / delete custom themes; change every role color (Goban, Liberty Lines, Coord, Annotation, Last Move, Child Node, Highlight, Neighbor, Play Area, Delete, Illegal, …) and every per-player color; choose the coordinate format; toggle Show Liberty Lines / Show Illegal Dots / Show Stone Connections / Show Hover Liberties / Show Child Node Dots. The Modify Theme dialog opens full-height with minimum width so every option is visible without deep scrolling on tall monitors. |
| Rule Sets | Open the rule-set editor: unified list of `[Built-in]` (view/duplicate only), `[Custom]` (full CRUD), and `[Game]` (loaded from collection games). Modify all multipliers and flags (LiveStones, Prisoners, Passes, StoneSuicides, Conversions, LastPlayerMustPassLast, SuicideIsLegal, CaptureConverts, StonePlacementsPerMove) and the four-flag SuperKo bitmask + NoResult. The four SuperKo flags form a strict hierarchy (BasicKo ⊂ NaturalSituational ⊂ Situational ⊂ Positional) and cascade both ways: checking a stronger flag auto-checks all weaker flags, and unchecking a weaker flag auto-unchecks all stronger flags. NoResult is independent. The "Show in score table" checkbox toggles built-in rule sets in the score table and moves custom rule sets between the shown and hidden lists. Use "Save as Custom" to persist a `[Game]` rule set into your config. Deleting a custom rule set used by any preset will list and delete those presets on confirm. |

## Mouse Mode

The Mouse Mode menu offers Play, Score, Set Label, and two submenus for selecting an editing mode and its value:

- **Play** — clicking on an intersection plays the next player's move (suicide and repetition rules apply; clicks on existing stones do nothing).
- **Score** — opens the score table, marks territory and dead stones; clicking a stone toggles its dead/alive status and propagates the change through neighboring groups.
- **Set Label** — clicking opens a small dialog to set or remove a text label on the clicked vertex.
- **Set Vertex** — a submenu selects "Empty (0)" or "Player N" for any player in the current game; clicking places (or removes) that player's stone via an edit node, with full capture/conversion cascade applied. Suicide and repetition are rejected according to the rule set, without changing the board or game tree. Illegal-move dots and hover previews reflect the selected player. Existing stones can still be replaced; Empty removes stones.
- **Toggle Annotation** — a submenu selects [Circle / Square / Triangle / X Mark]; clicking toggles that mark on the vertex. Hover shows a semi-transparent preview of the annotation that would be added (or an inverted-color preview for removal).

## Engine

| Item | Shortcut | Action |
|------|----------|--------|
| Settings | `Ctrl+E` | Add, rename, update, or delete named engine configurations (executable path and arguments), and toggle GTP logging (`EngineName.Player.GtpLog` files). |
| Player 1 | | Choose Human or a saved engine for Black. Selecting an engine readies it to play immediately; selecting the checked engine restarts it at the current position. |
| Player 2 | | Choose Human or a saved engine for White, with the same behavior as Player 1. |
| Detach All Engines | `Ctrl+Q` | Stop every engine in this window and set all players to Human. Also works when a text field has focus. |

Both players initially show Human. Selecting Human stops that player's engine; any detachment returns its check to Human. With two engines selected, turns alternate automatically until the game ends. Navigating the game tree detaches engines so you can inspect positions; select engines again to resume play. Engine choices are disabled for unsupported games, while Settings and Human remain available.

## Window

- **Symmetry** submenu — pick one of 8 view transforms: Identity, Rotate 90° CW, Rotate 180°, Rotate 270° CW, Flip Horizontal, Flip Horizontal + Rotate 90° CW, Flip Vertical, Transpose. The transform is per-window, never persisted, and never modifies the board. The active symmetry is greyed out so you can see which one is selected. Coord labels keep labelling their original columns/rows; only their on-screen position rotates.
- **Duplicate Window** — open a second window showing a deep copy of the current collection.
- **Add Window** — open a new window with an empty collection.
- **Close Window** — close the current window.

---

# Game Tree

The game tree is shown in a scrollable panel below the comment box. Each node is a button labeled with the move (e.g. `K9`, `Pass`, `Edit`, `Root`):

- `[C]` suffix: the node has a non-empty comment.
- `[A]` suffix: the node carries one or more annotations (Circle / Square / Triangle / X Mark).
- `[AC]` suffix: both annotations and a comment. Suffixes update live as you type in the comment box or toggle annotations — no navigation required.
- The currently selected node has a 3-pixel border.
- Hovering a node highlights the board vertices that node would affect (the placed move, or the diff against the parent for edit nodes).

Clicking a node navigates to it. A yellow thick connector marks every parent→child link; a thin blue connector marks only the edge from each parent to its *favorite* child — the path Page Down would follow. Parent nodes are centered exactly halfway between their leftmost and rightmost child columns. The view automatically centers on the selected node when you navigate or resize.

## Game tree keyboard shortcuts

- `↑` selects the parent node.
- `↓` selects the favorite child (or the first child if none is set).
- `←` selects the previous sibling (wraps around).
- `→` selects the next sibling (wraps around).
- `Page Up` jumps to the root node of the current game.
- `Page Down` follows the favorite-child path to the last node.
- `Home` switches to the previous game in the collection.
- `End` switches to the next game in the collection.
- `Delete` deletes the current node.

---

# Comment Box and Game Selector

- **Comment box** (top of the left panel, multi-line): edits are written into the current node's comment in real time. The `[C]` suffix appears on the node's game tree button as soon as you type. Press `Enter` inside the Comment box to insert a newline.
- **Game selector** (dropdown at the bottom of the left panel): switches between games in the collection. Display name is the SGF Game Name if set, otherwise `Player1 vs Player2`, otherwise `Game N`.

---

# Status Bar

A compact one-line bar at the bottom of the window, updated ~7×/second, showing:

- Current mouse mode.
- Time since the parent node was created (⇒ "thinking time" of the last move) and time since the current node was created (⇒ how long the current player has been thinking) — only when the node has a timestamp (SGF-imported nodes don't, so the timing fields are skipped).
- Hovered group: stone count and liberty count, when the mouse is on the board.

---

# Hovering on the Board

When the mouse hovers a board vertex (Play and Set Vertex modes):

- A translucent stone preview shows where the move would land (suppressed if illegal or already occupied by a stone, or the next player is engine-controlled).
- Small dots mark the neighbors of the hover (`Neighbor` color).
- The column and row coordinate labels are highlighted at all four edges of the board.
- Liberties of the group at the cursor are circled, color-coded by liberty count (`1-Liberty Line`, `2-Liberty Line`, …, `≥5-Liberty Line`). Toggle off via the active theme's Show Hover Liberties.
- In Toggle Annotation mode, an annotation preview (semi-transparent for "would add", inverted color for "would remove") replaces the stone preview.

---

# Rule Sets

A `RuleSet` controls how a game is scored and what moves are legal. Each rule set has scoring multipliers (`LiveStones`, `Prisoners`, `Passes`, `StoneSuicides`, `Conversions`) applied to their respective per-player counts and added to the territory total. It also has flags:

- **`SuicideIsLegal`** — allows self-capture moves; suicided stones are removed and counted under "Stone Suicides".
- **`CaptureConverts`** — captured stones change color instead of being removed. When a group with 0 liberties is surrounded by exactly one opponent color, its stones convert to that color. A full iterative cascade resolves chain reactions through any number of players.
- **`LastPlayerMustPassLast`** — the game ends only when the last player in turn order makes the final pass.
- **`StonePlacementsPerMove`** — number of stones a player places per turn (default 1; e.g. 3 in "Nazgand 3 Stone").
- **`NonRepetitionRules`** — bitmask. Four scope-tiered SuperKo rules form a strict hierarchy (only the strongest enabled flag is checked):
  - **BasicKo** — first same-player ancestor only, passes excluded. Just enough to forbid a single immediate recapture.
  - **NaturalSituationalSuperKo** — any ancestor with the same previous player, passes excluded.
  - **SituationalSuperKo** — any ancestor with the same previous player, passes included.
  - **PositionalSuperKo** — any matching board position by any player, passes included.
  - Plus a separate **NoResult** flag (Positional scope): instead of rejecting the move, it ends the game as a draw ("No Result" appears as the top row of the score table).

## Built-in rule sets

| Name | Scoring | Flags |
|------|---------|-------|
| Chinese | territory only | PositionalSuperKo |
| Japanese | LiveStones=−1, Prisoners=1, StoneSuicides=−1 | BasicKo + NoResult |
| British Go Association (BGA) | territory only | LastPlayerMustPassLast, NaturalSituationalSuperKo |
| American Go Association (AGA) | LiveStones=−1, Prisoners=1, StoneSuicides=−1, Passes=−1 | LastPlayerMustPassLast, SituationalSuperKo |
| New Zealand | territory only | SuicideIsLegal, SituationalSuperKo |
| Korean | LiveStones=−1, Prisoners=1, StoneSuicides=−1 | BasicKo + NoResult |
| Nazgand 3 Stone | StonePlacementsPerMove=3, custom multipliers | SuicideIsLegal, LastPlayerMustPassLast, NaturalSituationalSuperKo |
| Nazgand Convert | CaptureConverts, custom multipliers | SuicideIsLegal, LastPlayerMustPassLast, NaturalSituationalSuperKo |

The score table shows every rule set you've marked "Show in score table"; rows are dynamically shown or hidden based on the active rule set's flags (Prisoners hidden when CaptureConverts; Stone Suicides hidden when suicide is illegal; Conversions hidden when not CaptureConverts). Hovering a row's cells shows tooltips with multiplier breakdowns; for Live Stones, Territory, and Prisoners the corresponding board coordinates are highlighted on hover.

---

# Liberty Sharing (Diplomacy)

Each player independently chooses whether to share their groups' liberties with each other player. There are three states per ordered pair:

- **Y** — `SharingUnconditional`: I always share my liberties with this player.
- **C** — `SharingConditional`: I share iff the other side implements [Conditional or Unconditional] too.
- **N** — `SharingRefused`: I never share.

Sharing is **asymmetric** (M[A][B] and M[B][A] are independent) and **non-transitive**. Each player always shares with their self.

Two game modes:

- **Fixed** (set at New Game time): the matrix is locked for the entire game. The Diplomacy menu item is disabled. SGF and GTP-derived games always use this mode.
- **Changeable**: the next-to-move player can edit only their own outgoing row via [Game → Diplomacy] (`Ctrl+D`). On commit, groups are recomputed and the legality cache is cleared so the UI immediately reflects new sharing — but zero-liberty groups under the new matrix are NOT captured by the matrix change; captures only happen when an actual stone is played.

The matrix editor (opened from Fresh Board's "Edit liberty sharing matrix…" button) is a square grid with row/column numbers and single-letter cells; tap a cell to cycle N → C → Y → N.

---

# Boards & Wrapping

- Rectangular boards from 1×1 up to 255×255.
- Wrapping is configured per-axis:
  - `WrapXMulY` (int8, may be 0): horizontal seam multiplier — when X wraps, Y becomes `Y · WrapXMulY + WrapXShiftY` (mod Height). Must be 0, or must have `|v| ≤ Height` and `gcd(|v|, Height) == 1`.
  - `WrapYMulX` (int8, may be 0): vertical seam multiplier — when Y wraps, X becomes `X · WrapYMulX + WrapYShiftX` (mod Width). Same coprimality constraint against Width.
  - `WrapXShiftY` and `WrapYShiftX` (uint8, < Height/Width): seam offsets.
- Setting `WrapYMulX = 1`, others 0 → cylinder (left/right joined).
- Setting `WrapXMulY = -1`, `WrapYMulX = 1` → Klein bottle (one axis flipped on wrap).
- Setting both multipliers to 1 and both shifts to 0 → normal torus.
- Setting both multipliers to 1 and only 1 shift to 0 → a twisted torus.

Wrap-seam visuals: liberty lines, illegal-move dots, and stone connections all extend correctly across the seam, and the goban grid lines extend to the goban rect edge along whichever axes wrap.

---

# Multi-Player Notes

- 1–15 players per game. Each player's komi is configurable.
- Each player has a base color, a hover variant (~40% opacity), a territory variant (~74% opacity), and an automatically-derived contrast color (used for text on move nodes in the game tree gui).
- The window title shows the next player's number and name.
- **GTP engines only support 2-player games** with `StonePlacementsPerMove == 1` (the protocol limitation). Engine choices are automatically disabled for other games and engines auto-detach when you switch to them.
- **SGF only supports 2-player games**; CGG JSON is the native format for everything else (including custom rule sets, liberty sharing, multi-player komi, and timestamps).

---

# Save Formats

## CGG JSON (native)

`.CGG.json` plus optional compression suffix `.kanzi`, `.gzip`, or `.zstd`. Captures the complete `GoWin.Coll`: every game, every node, board states, captures, conversions, annotations, labels, comments, timestamps, territory maps, custom rule sets, per-player liberty-sharing matrices, and which node was selected at save time. Save dialog offers method (Kanzi/Gzip/Zstd/Plaintext) and a per-method compression level slider; default is Kanzi at level 9.

Load validates strictly: move nodes are replayed via the legality engine; an illegal move skips that game (others still load).

## SGF

Standard SGF — works for 2-player games with `StonePlacementsPerMove == 1` and board sizes ≤ 52×52. Unknown SGF properties are preserved verbatim across import + export. Per-game parse errors are non-fatal; bad games are skipped with a warning so the rest of a multi-game collection still loads.

---

# SVG Export

`File → Export board as SVG` opens a dialog with:

- **Mode** — Single frame (current position only) or Animated SVG (entire game).
- **CellSize (px)** — uint16 ≥ 5; the resulting SVG dimensions are exactly `CellSize × (DisplayWidth + 2)` × `CellSize × (DisplayHeight + 2)` (the +2 covers the coord label band on each side).
- **Seconds per node** — only enabled in animated mode. Default 0.15.

The animated SVG plays back via SMIL `<set>` events, so it works in any modern SVG viewer (Chrome-based browsers, Firefox). Each frame is fully self-contained, so seeking is instant and frames render the same regardless of how far into the playback you've scrubbed. A move counter is overlaid in the upper-left cell.

Static export is synchronous; animated export shows a modal progress bar.

---

# Coordinate Formats

Selectable per-theme:

- **GTP** — `A`–`Z` skipping `I`, numbers from bottom (the standard tournament/engine format).
- **Computer** — 0-indexed `X,Y` with Y=0 at the top.
- **HikaruNoGo** — 1-indexed `XのY` (Japanese-style).
- **1stQuadrant** — 1-indexed `(X,Y)` with Y=0 at the bottom (math/graph style).
- **SGF** — lowercase/uppercase letter pairs (`a`–`z`, `A`–`Z`).

GTP is only available on boards ≤ 25×25; SGF is only available on boards ≤ 52×52; the renderer transparently falls back when the active format can't represent a coord.

---

# Themes

A **theme** holds the coordinate format, every role color (Goban, the 5 liberty-line colors, Last Move, Child Node, Highlight, Annotation, Coord, Coord Hover, Neighbor, Play Area, Illegal, Delete), every per-player base color, and five show/hide toggles (`ShowLibertyLine`, `ShowStoneConnection`, `ShowIllegalDot`, `ShowHoverLiberties`, `ShowChildNodeDots`).

The "Default" theme is always available and always regenerated from the built-in defaults on load (so it shall not be modified). You can create as many custom themes as you want via the App → Theme dialog; the modify dialog lives-previews changes on the board; the color picker has presets and an arbitrary hex input.

---

# Keyboard Shortcuts (Full List)

Menus display shortcuts separately beside their action labels, including P for Pass and Delete for Delete Node.

| Key | Action |
|-----|--------|
| `Ctrl+N` | New Game (Fresh Board dialog) |
| `Ctrl+O` | Load Game (CGG) |
| `Ctrl+S` | Save Game (CGG) |
| `Ctrl+Shift+O` | Import SGF |
| `Ctrl+Shift+S` | Export SGF |
| `Ctrl+E` | Engine Settings |
| `Ctrl+Q` | Detach All Engines (all players become Human) |
| `Ctrl+G` | Go To Move # |
| `Ctrl+D` | Diplomacy |
| `P` | Pass |
| `↑` / `↓` | Game tree: parent / favorite child |
| `←` / `→` | Game tree: previous sibling (wraps) / next sibling (wraps) |
| `Page Up` / `Page Down` | Game tree: root / end of favorite-child path |
| `Home` / `End` | Previous game / next game in the collection |
| `Delete` | Delete current node |
| `Esc` | Dismiss the open dialog |
| `Enter` | Submit the open dialog (skipped when a multi-line entry has focus) |

---

# Command-Line Usage

```
ConnectedGroupsGoban [-ConfigDir=<dir>] [file.sgf | file.CGG.json[.kanzi|.gzip|.zstd] ...]
```

Each loadable file argument opens its own window. With no arguments, a single window opens with a fresh board from the active preset. Files with unsupported extensions are skipped with a warning on stdout. Window size and the vertical-split offset are remembered across runs in the config file.
