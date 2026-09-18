package main

import (
	"fmt"
)

// NextPlayer returns the next player in a multi-player game.
// Wraps from player n back to player 1.
func NextPlayer(p, n uint8) uint8 {
	if p == n {
		return 1
	}
	if p >= 1 && p < n {
		return p + 1
	}
	return p
}

// PreviousPlayer returns the previous player in a multi-player game.
// Wraps from player 1 back to player n.
func PreviousPlayer(p, n uint8) uint8 {
	if p == 1 {
		return n
	}
	if p > 1 && p <= n {
		return p - 1
	}
	return p
}

// SgfCoordCharToInt converts a single SGF coordinate character to a uint8
// index: 'a'..'z' → 0..25, 'A'..'Z' → 26..51. Returns 0xff and an error
// for characters outside these ranges.
func SgfCoordCharToInt(c rune) (uint8, error) {
	if c >= 'a' && c <= 'z' {
		return uint8(c - 'a'), nil
	} else if c >= 'A' && c <= 'Z' {
		return uint8(c - 'A' + 26), nil
	} else {
		return 0xff, fmt.Errorf("Invalid Sgf coordinate character: %c", c)
	}
}

// Converts Sgf coordinates (e.g., "pd") to Coord
// TODO change *BoardState to *GameHistory
func ConvertSgfCoordToCoord(SgfCoord string, B *BoardState) (Coord, error) {
	if B == nil || B.Hist == nil {
		return ErrorCoords, fmt.Errorf("Nil board state when converting Sgf coordinate: %q", SgfCoord)
	}
	if SgfCoord == "" {
		return PassCoords, nil
	}
	if len(SgfCoord) != 2 {
		return ErrorCoords, fmt.Errorf("Invalid Sgf coordinate length %d (expected 2): %q", len(SgfCoord), SgfCoord)
	}
	X, Err := SgfCoordCharToInt(rune(SgfCoord[0]))
	if Err != nil {
		return ErrorCoords, fmt.Errorf("Invalid Sgf coordinate X character %q in %q", SgfCoord[0], SgfCoord)
	}
	Y, Err := SgfCoordCharToInt(rune(SgfCoord[1]))
	if Err != nil {
		return ErrorCoords, fmt.Errorf("Invalid Sgf coordinate Y character %q in %q", SgfCoord[1], SgfCoord)
	}
	C := Coord{X, Y}
	if B.Hist.InBounds(C) {
		return C, nil // Valid coordinate
	}
	return ErrorCoords, fmt.Errorf("Sgf coordinate %q (%d,%d) is not on board (%dx%d)",
		SgfCoord, X, Y, B.Hist.Width, B.Hist.Height)
}

// Uint8ToSgfChar converts a uint8 board index to its SGF coordinate
// character: 0..25 → 'a'..'z', 26..51 → 'A'..'Z'. Returns an error
// for values ≥ 52.
func Uint8ToSgfChar(n uint8) (string, error) {
	if n <= 25 {
		// 'a' to 'z' for indices 0 to 25
		return string(rune('a' + n)), nil
	} else if n >= 26 && n <= 51 {
		// 'A' to 'Z' for indices 26 to 51
		return string(rune('A' + n - 26)), nil
	} else {
		return "", fmt.Errorf("coordinate out of range for Sgf (max 52x52 board size)")
	}
}

// Converts Coord to Sgf coordinates.
func ConvertCoordToSgfCoord(C Coord) (string, error) {
	if C == PassCoords {
		return "", nil
	}
	SgfX, err := Uint8ToSgfChar(C.X)
	if err != nil {
		return "", err
	}
	SgfY, err := Uint8ToSgfChar(C.Y)
	if err != nil {
		return "", err
	}
	return SgfX + SgfY, nil
}

// SgfGetInformation returns the stored Information value for a given SGF property key,
// or nil if not present.
func SgfGetInformation(Hist *GameHistory, SgfKey string) *string {
	if Hist.Information == nil {
		return nil
	}
	desc, ok := SgfToDescription[SgfKey]
	if !ok {
		return nil
	}
	return Hist.Information[desc]
}
