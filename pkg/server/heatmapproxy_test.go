package server

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"testing"
)

func TestDrawSpeedBadge(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	// Fill canvas with black
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	// Draw badge at (20, 20) with value 125
	drawSpeedBadge(canvas, 20, 20, 125)

	// Check that pixels in badge area have changed from pure black
	// Center of lightning bolt is around (20+9+5, 20+14) = (34, 34)
	boltPixel := canvas.NRGBAAt(34, 34)
	if boltPixel.A == 0 || (boltPixel.R == 0 && boltPixel.G == 0 && boltPixel.B == 0) {
		t.Errorf("Expected lightning bolt pixel at (34, 34) to be colored, got %v", boltPixel)
	}

	// Center of text "125" is around (20+9+11+6+10, 20+16) = (56, 36)
	// Background pill pixel at (25, 25) should have dark background alpha
	bgPixel := canvas.NRGBAAt(25, 25)
	if bgPixel.A == 0 || bgPixel.R == 0 && bgPixel.G == 0 && bgPixel.B == 0 {
		t.Errorf("Expected background pill at (25, 25) to have non-zero color, got %v", bgPixel)
	}
}

func TestDrawSpeedBadgeEdgeCases(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	// If value <= 0, badge should not be drawn
	drawSpeedBadge(canvas, 20, 20, 0)
	pixel := canvas.NRGBAAt(25, 25)
	if pixel.R != 0 || pixel.G != 0 || pixel.B != 0 {
		t.Errorf("Expected untouched canvas for value 0, got %v", pixel)
	}

	drawSpeedBadge(canvas, 20, 20, -10)
	pixel = canvas.NRGBAAt(25, 25)
	if pixel.R != 0 || pixel.G != 0 || pixel.B != 0 {
		t.Errorf("Expected untouched canvas for value -10, got %v", pixel)
	}
}

func TestCreateHeatmapThumbnail(t *testing.T) {
	// Create a dummy 700x420 JPEG
	img := image.NewRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{100, 100, 100, 255}}, image.Point{}, draw.Src)
	var inBuf bytes.Buffer
	err := jpeg.Encode(&inBuf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		t.Fatalf("Failed to encode test JPEG: %v", err)
	}

	var outBuf bytes.Buffer
	err = createHeatmapThumbnail(&outBuf, bytes.NewReader(inBuf.Bytes()), nil, 142)
	if err != nil {
		t.Fatalf("createHeatmapThumbnail failed: %v", err)
	}

	if outBuf.Len() == 0 {
		t.Errorf("Expected non-empty output buffer")
	}

	// Verify output is a valid JPEG
	decoded, err := jpeg.Decode(&outBuf)
	if err != nil {
		t.Fatalf("Failed to decode resulting JPEG: %v", err)
	}
	if decoded.Bounds().Dx() != 700 || decoded.Bounds().Dy() != 420 {
		t.Errorf("Unexpected bounds: %v", decoded.Bounds())
	}
}
