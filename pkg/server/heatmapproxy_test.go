package server

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDrawSpeedBadge(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	drawSpeedBadge(canvas, 20, 20, 125)

	boltPixel := canvas.NRGBAAt(34, 34)
	if boltPixel.A == 0 || (boltPixel.R == 0 && boltPixel.G == 0 && boltPixel.B == 0) {
		t.Errorf("Expected lightning bolt pixel at (34, 34) to be colored, got %v", boltPixel)
	}

	bgPixel := canvas.NRGBAAt(25, 25)
	if bgPixel.A == 0 || (bgPixel.R == 0 && bgPixel.G == 0 && bgPixel.B == 0) {
		t.Errorf("Expected background pill at (25, 25) to have non-zero color, got %v", bgPixel)
	}
}

func TestDrawSpeedBadgeEdgeCases(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

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

func TestClimaxBadgeWidth(t *testing.T) {
	if w := climaxBadgeWidth(0); w != 0 {
		t.Errorf("Expected 0 width for 0 minutes, got %d", w)
	}
	if w := climaxBadgeWidth(-5); w != 0 {
		t.Errorf("Expected 0 width for negative minutes, got %d", w)
	}

	w8 := climaxBadgeWidth(8)
	w18 := climaxBadgeWidth(18)
	w120 := climaxBadgeWidth(120)

	if w8 != 72 || w18 != 72 || w120 != 72 {
		t.Errorf("Expected fixed splash badge width of 72, got w8=%d, w18=%d, w120=%d", w8, w18, w120)
	}
}

func TestDrawClimaxSplashBadge(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	// Fill with mid-gray to verify both white splash and dark outline
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{128, 128, 128, 255}}, image.Point{}, draw.Src)

	badgeW := climaxBadgeWidth(18)
	badgeX := thumbnailWidth - badgeW - 20
	badgeY := 85

	drawClimaxSplashBadge(canvas, badgeX, badgeY, 18)

	// Center of badge should have non-gray pixel (text or white splash fill)
	centerPixel := canvas.NRGBAAt(badgeX+35, badgeY+36)
	if centerPixel.R == 128 && centerPixel.G == 128 && centerPixel.B == 128 {
		t.Errorf("Expected center of badge to be drawn, got untouched gray %v", centerPixel)
	}

	// Interior white splash area
	whitePixel := canvas.NRGBAAt(badgeX+35, badgeY+22)
	if whitePixel.R < 200 || whitePixel.G < 200 || whitePixel.B < 200 {
		t.Errorf("Expected white pixel in splash interior, got %v", whitePixel)
	}

	// Verify dark contrast accent lines and text exist in the splash (#0F172A)
	darkPixelCount := 0
	for y := badgeY; y <= badgeY+77; y++ {
		for x := badgeX; x <= badgeX+badgeW; x++ {
			p := canvas.NRGBAAt(x, y)
			if p.R < 40 && p.G < 40 && p.B < 60 && p.A > 0 {
				darkPixelCount++
			}
		}
	}
	if darkPixelCount == 0 {
		t.Errorf("Expected dark accent lines / text in splash, found 0 dark pixels")
	}
}

func TestDrawClimaxSplashBadgeEdgeCases(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	// minutes <= 0 should do nothing
	drawClimaxSplashBadge(canvas, 608, 85, 0)
	drawClimaxSplashBadge(canvas, 608, 85, -10)
	pixel := canvas.NRGBAAt(608, 85)
	if pixel.R != 0 || pixel.G != 0 || pixel.B != 0 {
		t.Errorf("Expected untouched canvas for non-positive minutes, got %v", pixel)
	}

	// nil canvas should not panic
	drawClimaxSplashBadge(nil, 608, 85, 18)

	// Auto-positioning when x <= 0 and y <= 0
	canvasAuto := image.NewNRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(canvasAuto, canvasAuto.Bounds(), &image.Uniform{color.NRGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)
	drawClimaxSplashBadge(canvasAuto, 0, 0, 18)
	expectedX := thumbnailWidth - climaxBadgeWidth(18) - 20
	expectedY := 85
	autoPixel := canvasAuto.NRGBAAt(expectedX+35, expectedY+22)
	if autoPixel.R < 200 || autoPixel.G < 200 {
		t.Errorf("Expected auto-positioned badge at x=%d, y=%d, got pixel %v", expectedX, expectedY, autoPixel)
	}
}

func TestCreateHeatmapThumbnail(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 700, 420))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{100, 100, 100, 255}}, image.Point{}, draw.Src)
	var inBuf bytes.Buffer
	err := jpeg.Encode(&inBuf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		t.Fatalf("Failed to encode test JPEG: %v", err)
	}

	// Test with both badges
	var outBuf bytes.Buffer
	err = createHeatmapThumbnail(&outBuf, bytes.NewReader(inBuf.Bytes()), nil, 142, 18)
	if err != nil {
		t.Fatalf("createHeatmapThumbnail failed: %v", err)
	}

	if outBuf.Len() == 0 {
		t.Errorf("Expected non-empty output buffer")
	}

	decoded, err := jpeg.Decode(&outBuf)
	if err != nil {
		t.Fatalf("Failed to decode resulting JPEG: %v", err)
	}
	if decoded.Bounds().Dx() != 700 || decoded.Bounds().Dy() != 420 {
		t.Errorf("Unexpected bounds: %v", decoded.Bounds())
	}

	// Save test image to artifact dir for visual verification
	artifactDir := `C:\Users\nicod\.gemini\antigravity\brain\f3d94aed-6820-4109-9d12-102b7248c148`
	if fi, err := os.Stat(artifactDir); err == nil && fi.IsDir() {
		outPath := filepath.Join(artifactDir, "test_thumbnail_preview.png")
		if f, err := os.Create(outPath); err == nil {
			_ = png.Encode(f, decoded)
			f.Close()
		}
	}
}
