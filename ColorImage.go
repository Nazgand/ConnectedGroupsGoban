package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// MakeSquareMinusFourCirclesImage returns a cached canvas.Image of a filled
// square with quarter-circles cut from each corner, used for 3-stone group
// connection rendering. The image is regenerated when Size changes.
func MakeSquareMinusFourCirclesImage(Color color.NRGBA, Size int) (*canvas.Image, error) {
	//Avoid Size==0 error
	Size = max(1, Size)
	ImageResource, Exists := SquareMinusFourCircles[Color]
	if !Exists || Size != SquareMinusFourCirclesSize[Color] {
		Err := MakeSquareMinusFourCirclesPNG(Color, Size)
		if Err != nil {
			return &canvas.Image{}, Err
		}
		ImageResource = SquareMinusFourCircles[Color]
	}
	Image := canvas.NewImageFromResource(ImageResource)
	Image.ScaleMode = canvas.ImageScaleFastest
	return Image, nil
}

// MakeSquareMinusFourCirclesPNG rasterize a Size×Size PNG of a filled square
// with quarter-circle cutouts at each corner. The result is stored in the
// global SquareMinusFourCircles cache keyed by Color.
func MakeSquareMinusFourCirclesPNG(Color color.NRGBA, Size int) error {
	Image := image.NewRGBA(image.Rect(0, 0, Size, Size))
	SizeSquared := Size * Size
	RadiusRoundedUp := (Size >> 1) + (Size & 1)
	// Trace the arc of the upper left circle
	// from 0 radians to -π/4 radians
	Y := 0
	X := RadiusRoundedUp
	for X >= Y {
		X--
		// Check distance from top left of image to bottom right of current pixel
		DistanceSquaredFromTopLeft := (X+1)*(X+1) + (Y+1)*(Y+1)
		if DistanceSquaredFromTopLeft<<2 < SizeSquared {
			for SetX := X + 1; SetX <= Size-2-X; SetX++ {
				Image.Set(SetX, Y, Color)
				Image.Set(Y, SetX, Color)
				Image.Set(Size-1-Y, SetX, Color)
				Image.Set(SetX, Size-1-Y, Color)
			}
			Y++
			X++
		}
	}
	// Fill in leftover square
	for SetY := Y; SetY <= Size-1-Y; SetY++ {
		for SetX := Y; SetX <= Size-1-Y; SetX++ {
			Image.Set(SetX, SetY, Color)
		}
	}
	// Convert to bytes (PNG format)
	EncodePNG := func(img *image.RGBA) []byte {
		var buf bytes.Buffer
		png.Encode(&buf, img)
		return buf.Bytes()
	}
	SquareMinusFourCircles[Color] = fyne.NewStaticResource(
		"SquareMinusFourCircles"+RGBAtoHexColor(Color)+".png", EncodePNG(Image))
	SquareMinusFourCirclesSize[Color] = Size
	return nil
}

// RGBAtoHexColor converts an NRGBA color to a "#rrggbbaa" hex string.
func RGBAtoHexColor(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// HexColorToNRGBA parses a "#rrggbb" or "#rrggbbaa" hex string into an
// NRGBA color. A 7-character string defaults alpha to 255.
func HexColorToNRGBA(HexColor string) (color.NRGBA, error) {
	switch len(HexColor) {
	case 7, 9:
	default:
		return color.NRGBA{}, fmt.Errorf("Invalid HexColor length.")
	}
	if HexColor[:1] != "#" {
		return color.NRGBA{}, fmt.Errorf("HexColor must start with `#`.")
	}
	HexToByte := func(Hex string) (uint8, error) {
		Val, Err := strconv.ParseUint(Hex, 16, 8)
		return uint8(Val), Err
	}
	R, Err := HexToByte(HexColor[1:3])
	if Err != nil {
		return color.NRGBA{}, fmt.Errorf("HexColor R byte did not parse. %v", Err)
	}
	G, Err := HexToByte(HexColor[3:5])
	if Err != nil {
		return color.NRGBA{}, fmt.Errorf("HexColor G byte did not parse. %v", Err)
	}
	B, Err := HexToByte(HexColor[5:7])
	if Err != nil {
		return color.NRGBA{}, fmt.Errorf("HexColor B byte did not parse. %v", Err)
	}
	A := uint8(255)
	if len(HexColor) == 9 {
		A, Err = HexToByte(HexColor[7:9])
		if Err != nil {
			return color.NRGBA{}, fmt.Errorf("HexColor A byte did not parse. %v", Err)
		}
	}
	return color.NRGBA{R: R, G: G, B: B, A: A}, nil
}

// RenderColorPreview fills img with a split preview: the left half
// composites c over white and the right half composites c over black,
// showing how the color looks at its current alpha against both extremes.
func RenderColorPreview(img *image.NRGBA, c color.NRGBA) {
	w := img.Bounds().Max.X
	h := img.Bounds().Max.Y
	for y := range h {
		for x := range w {
			var bg uint8
			if x < w/2 { // TODO: more efficient to have y loop inside x loop; fix.
				bg = 255 // white on left, black on right
			}
			outR := uint8((uint16(c.R)*uint16(c.A) + uint16(bg)*uint16(255-c.A)) / 255)
			outG := uint8((uint16(c.G)*uint16(c.A) + uint16(bg)*uint16(255-c.A)) / 255)
			outB := uint8((uint16(c.B)*uint16(c.A) + uint16(bg)*uint16(255-c.A)) / 255)
			img.SetNRGBA(x, y, color.NRGBA{R: outR, G: outG, B: outB, A: 255})
		}
	}
}

// NewColorPreviewImage creates a Width×Height color-preview image and its matching
// canvas.Image, pre-rendered with Color via RenderColorPreview.
func NewColorPreviewImage(Color color.NRGBA, Width, Height int) (*image.NRGBA, *canvas.Image) {
	CPImage := image.NewNRGBA(image.Rect(0, 0, Width, Height))
	RenderColorPreview(CPImage, Color)
	CanvasImage := canvas.NewImageFromImage(CPImage)
	CanvasImage.FillMode = canvas.ImageFillStretch
	CanvasImage.ScaleMode = canvas.ImageScaleFastest
	return CPImage, CanvasImage
}

// Makes a contrasting color at same opacity
func ContrastColor(Color color.NRGBA) color.NRGBA {
	switch {
	case Color.R > 0x80:
		Color.R = 0
	default:
		Color.R = 0xff
	}
	switch {
	case Color.G > 0x80:
		Color.G = 0
	default:
		Color.G = 0xff
	}
	switch {
	case Color.B > 0x80:
		Color.B = 0
	default:
		Color.B = 0xff
	}
	return Color
}
