package server

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/xbapps/xbvr/pkg/common"
	"github.com/xbapps/xbvr/pkg/config"
	"github.com/xbapps/xbvr/pkg/models"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
	"willnorris.com/go/imageproxy"
)

const thumbnailWidth = 700
const thumbnailHeight = 420
const heatmapHeight = 10
const heatmapMargin = 3
const maximumHeatmaps = 20 // maximumHeatmaps*(heatmapHeight+heatmapMargin) needs to be lower than thumbnailHeight

var badgeFace font.Face

func init() {
	f, err := opentype.Parse(gobold.TTF)
	if err == nil {
		badgeFace, _ = opentype.NewFace(f, &opentype.FaceOptions{
			Size:    15,
			DPI:     72,
			Hinting: font.HintingFull,
		})
	}
}

type BufferResponseWriter struct {
	header     http.Header
	statusCode int
	buf        *bytes.Buffer
}

func (myrw *BufferResponseWriter) Write(p []byte) (int, error) {
	return myrw.buf.Write(p)
}

func (w *BufferResponseWriter) Header() http.Header {
	return w.header
}

func (w *BufferResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

type HeatmapThumbnailProxy struct {
	ImageProxy *imageproxy.Proxy
	Cache      imageproxy.Cache
}

func NewHeatmapThumbnailProxy(imageproxy *imageproxy.Proxy, cache imageproxy.Cache) *HeatmapThumbnailProxy {
	proxy := &HeatmapThumbnailProxy{
		ImageProxy: imageproxy,
		Cache:      cache,
	}
	return proxy
}

func getSceneAndScriptFiles(urlpart string) (*models.Scene, []models.File, error) {
	sceneId, err := strconv.Atoi(urlpart)
	files := make([]models.File, 0)
	if err != nil {
		return nil, files, err
	}

	var scene models.Scene
	err = scene.GetIfExistByPK(uint(sceneId))
	if err != nil {
		return nil, files, err
	}

	scriptfiles, err := scene.GetScriptFilesSorted(config.Config.Interfaces.Players.ScriptSortSeq)
	if err != nil || len(scriptfiles) < 1 {
		return &scene, files, fmt.Errorf("scene %d has no script files", sceneId)
	}

	files = append(files, scriptfiles...)

	return &scene, files, nil
}

func getHeatmapImageForScene(fileId uint) (image.Image, error) {

	heatmapFilename := filepath.Join(common.ScriptHeatmapDir, fmt.Sprintf("heatmap-%d.png", fileId))
	heatmapFile, err := os.Open(heatmapFilename)
	if err != nil {
		return nil, err
	}

	heatmapImage, err := png.Decode(heatmapFile)
	heatmapFile.Close()
	if err != nil {
		return nil, err
	}

	return heatmapImage, nil
}

func drawSpeedBadge(dst *image.NRGBA, x, y int, value int) {
	if badgeFace == nil || value <= 0 {
		return
	}

	text := fmt.Sprintf("%d", value)
	d := font.Drawer{Face: badgeFace}
	textAdv := d.MeasureString(text)
	textWidth := textAdv.Ceil()

	padLeft := 9
	iconW := 11
	gap := 6
	padRight := 10
	badgeW := padLeft + iconW + gap + textWidth + padRight
	badgeH := 28
	radius := 6.0

	// Draw rounded rectangle background (deep dark with 84% opacity)
	bgRaster := vector.NewRasterizer(badgeW, badgeH)
	drawRoundedRect(bgRaster, 0, 0, float32(badgeW), float32(badgeH), float32(radius))
	bgColor := color.RGBA{18, 18, 18, 215}
	bgRaster.Draw(dst, image.Rect(x, y, x+badgeW, y+badgeH), image.NewUniform(bgColor), image.Point{})

	// Draw subtle 1px border
	borderRaster := vector.NewRasterizer(badgeW, badgeH)
	drawRoundedRectBorder(borderRaster, 0.5, 0.5, float32(badgeW)-1, float32(badgeH)-1, float32(radius)-0.5, 1.0)
	borderColor := color.RGBA{255, 255, 255, 40}
	borderRaster.Draw(dst, image.Rect(x, y, x+badgeW, y+badgeH), image.NewUniform(borderColor), image.Point{})

	// Draw lightning icon (Orange / Coral #F97316)
	iconX := float32(padLeft)
	iconY := float32((badgeH - 16) / 2)
	iconRaster := vector.NewRasterizer(badgeW, badgeH)
	drawLightningBolt(iconRaster, iconX, iconY)
	iconColor := color.RGBA{249, 115, 22, 255}
	iconRaster.Draw(dst, image.Rect(x, y, x+badgeW, y+badgeH), image.NewUniform(iconColor), image.Point{})

	// Draw text in crisp white
	textX := x + padLeft + iconW + gap
	textY := y + 19
	d.Dst = dst
	d.Src = image.NewUniform(color.RGBA{255, 255, 255, 255})
	d.Dot = fixed.P(textX, textY)
	d.DrawString(text)
}

func drawRoundedRect(ras *vector.Rasterizer, x, y, w, h, r float32) {
	ras.MoveTo(x+r, y)
	ras.LineTo(x+w-r, y)
	drawQuarterCircle(ras, x+w-r, y+r, r, -math.Pi/2, 0)
	ras.LineTo(x+w, y+h-r)
	drawQuarterCircle(ras, x+w-r, y+h-r, r, 0, math.Pi/2)
	ras.LineTo(x+r, y+h)
	drawQuarterCircle(ras, x+r, y+h-r, r, math.Pi/2, math.Pi)
	ras.LineTo(x, y+r)
	drawQuarterCircle(ras, x+r, y+r, r, math.Pi, 3*math.Pi/2)
	ras.ClosePath()
}

func drawRoundedRectBorder(ras *vector.Rasterizer, x, y, w, h, r, strokeWidth float32) {
	drawRoundedRect(ras, x, y, w, h, r)
	inX := x + strokeWidth
	inY := y + strokeWidth
	inW := w - 2*strokeWidth
	inH := h - 2*strokeWidth
	inR := r - strokeWidth
	if inR < 0 {
		inR = 0
	}
	drawRoundedRectReverse(ras, inX, inY, inW, inH, inR)
}

func drawRoundedRectReverse(ras *vector.Rasterizer, x, y, w, h, r float32) {
	ras.MoveTo(x+r, y)
	drawQuarterCircle(ras, x+r, y+r, r, 3*math.Pi/2, math.Pi)
	ras.LineTo(x, y+h-r)
	drawQuarterCircle(ras, x+r, y+h-r, r, math.Pi, math.Pi/2)
	ras.LineTo(x+w-r, y+h)
	drawQuarterCircle(ras, x+w-r, y+h-r, r, math.Pi/2, 0)
	ras.LineTo(x+w, y+r)
	drawQuarterCircle(ras, x+w-r, y+r, r, 0, -math.Pi/2)
	ras.LineTo(x+r, y)
	ras.ClosePath()
}

func drawQuarterCircle(ras *vector.Rasterizer, cx, cy, r float32, startAngle, endAngle float64) {
	steps := 8
	for i := 1; i <= steps; i++ {
		theta := startAngle + (endAngle-startAngle)*float64(i)/float64(steps)
		px := cx + r*float32(math.Cos(theta))
		py := cy + r*float32(math.Sin(theta))
		ras.LineTo(px, py)
	}
}

func drawLightningBolt(ras *vector.Rasterizer, ox, oy float32) {
	ras.MoveTo(ox+6.5, oy+0.0)
	ras.LineTo(ox+0.5, oy+8.5)
	ras.LineTo(ox+5.5, oy+8.5)
	ras.LineTo(ox+4.0, oy+15.5)
	ras.LineTo(ox+10.5, oy+6.5)
	ras.LineTo(ox+5.5, oy+6.5)
	ras.ClosePath()
}

func createHeatmapThumbnail(out *bytes.Buffer, r io.Reader, heatmapImages []image.Image, badgeValue int) error {
	thumbnailImage, err := jpeg.Decode(r)

	if err != nil {
		return err
	}

	heatmapsHeight := len(heatmapImages) * (heatmapHeight + heatmapMargin)
	rect := thumbnailImage.Bounds()
	if rect.Dx() != thumbnailWidth || rect.Dy() != thumbnailHeight-heatmapsHeight {
		thumbnailImage = imaging.Fill(thumbnailImage, thumbnailWidth, thumbnailHeight-heatmapsHeight, imaging.Center, imaging.Linear)
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, thumbnailWidth, thumbnailHeight))

	drawRect := image.Rect(0, 0, thumbnailWidth, thumbnailHeight-heatmapsHeight)
	draw.Draw(canvas, drawRect, thumbnailImage, image.Point{}, draw.Over)

	for i := range heatmapImages {
		heatmapImage := imaging.Resize(heatmapImages[i], thumbnailWidth, heatmapHeight, imaging.Linear)

		drawRect = image.Rect(0, thumbnailHeight-heatmapsHeight+heatmapMargin+i*(heatmapHeight+heatmapMargin), thumbnailWidth, thumbnailHeight)
		draw.Draw(canvas, drawRect, heatmapImage, image.Point{}, draw.Over)
	}

	if badgeValue > 0 {
		drawSpeedBadge(canvas, 20, 20, badgeValue)
	}

	jpeg.Encode(out, canvas, &jpeg.Options{Quality: 90})
	return nil
}

func (p *HeatmapThumbnailProxy) serveImageproxyResponse(w http.ResponseWriter, r *http.Request, imageURL string) {
	proxyURL := "/700x/" + imageURL
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = proxyURL
	p.ImageProxy.ServeHTTP(w, r2)
}

func (p *HeatmapThumbnailProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	parts := strings.SplitN(r.URL.Path, "/", 3)
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	imageURL := parts[2]
	scene, files, err := getSceneAndScriptFiles(parts[1])
	if err != nil {
		p.serveImageproxyResponse(w, r, imageURL)
		return
	}

	badgeValue := 0
	if scene != nil {
		badgeValue = scene.ScriptSpeed
		if badgeValue == 0 {
			badgeValue = scene.ScriptAPM
		}
	}
	if badgeValue == 0 && len(files) > 0 {
		badgeValue = files[0].ScriptSpeed
		if badgeValue == 0 {
			badgeValue = files[0].ScriptAPM
		}
	}

	loadFromCache := true

	for i := range files {
		if files[i].RefreshHeatmapCache {
			loadFromCache = false
			break
		}
	}

	cacheKey := fmt.Sprintf("v2:%d:%d:%s", files[0].ID, badgeValue, imageURL)

	if loadFromCache {
		cachedContent, ok := p.Cache.Get(cacheKey)
		if ok {
			w.Header().Add("Content-Type", "image/jpeg")
			w.Header().Add("Content-Length", fmt.Sprint(len(cachedContent)))
			if _, err := io.Copy(w, bytes.NewReader(cachedContent)); err != nil {
				log.Printf("Failed to send out response: %v", err)
			}
			return
		}
	}

	heatmapImages := make([]image.Image, 0)

	for i := range files {
		heatmapImage, err := getHeatmapImageForScene(files[i].ID)
		if err == nil {
			heatmapImages = append(heatmapImages, heatmapImage)
			if len(heatmapImages) == maximumHeatmaps {
				break
			}
		}
	}

	if len(heatmapImages) == 0 {
		p.serveImageproxyResponse(w, r, imageURL)
		return
	}

	for i := range files {
		file := files[i]
		file.RefreshHeatmapCache = false
		file.Save()
	}

	heatmapsHeight := len(heatmapImages) * (heatmapHeight + heatmapMargin)
	proxyURL := fmt.Sprintf("/%dx%d,jpeg/%s", thumbnailWidth, thumbnailHeight-heatmapsHeight, imageURL)
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = proxyURL
	imageproxyResponseWriter := &BufferResponseWriter{
		header: http.Header{},
		buf:    &bytes.Buffer{},
	}
	p.ImageProxy.ServeHTTP(imageproxyResponseWriter, r2)

	respbody, err := io.ReadAll(imageproxyResponseWriter.buf)
	if err == nil {
		var output bytes.Buffer
		err = createHeatmapThumbnail(&output, bytes.NewReader(respbody), heatmapImages, badgeValue)
		if err == nil {
			p.Cache.Set(cacheKey, output.Bytes())
			w.Header().Add("Content-Type", "image/jpeg")
			w.Header().Add("Content-Length", fmt.Sprint(len(output.Bytes())))
			if _, err := io.Copy(w, bytes.NewReader(output.Bytes())); err != nil {
				log.Printf("Failed to send out response: %v", err)
			}
			return
		}
	}
	if err != nil {
		log.Printf("%v", err)
		// serve original response
		if _, err := io.Copy(w, bytes.NewReader(respbody)); err != nil {
			log.Printf("Failed to send out response: %v", err)
		}
	}
}
