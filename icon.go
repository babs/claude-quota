package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"

	"github.com/fogleman/gg"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// builtinFonts maps font names to their TTF bytes.
var builtinFonts = map[string][]byte{
	"bold":     gobold.TTF,
	"regular":  goregular.TTF,
	"mono":     gomono.TTF,
	"monobold": gomonobold.TTF,
}

// ValidFontName returns true if the name is a known built-in font.
func ValidFontName(name string) bool {
	if name == "bitmap" {
		return true
	}
	_, ok := builtinFonts[name]
	return ok
}

// TTF font cache: parsed once per font name, faces cached per size.
var (
	ttfMu     sync.Mutex
	ttfParsed = map[string]*opentype.Font{}
	ttfFaces  = map[string]map[float64]font.Face{}
)

// loadTTFFace returns a font.Face for the given TTF font name and size.
// Parsed fonts and sized faces are cached for reuse.
func loadTTFFace(fontName string, size float64) (font.Face, error) {
	ttfMu.Lock()
	defer ttfMu.Unlock()

	// Check face cache first.
	if faces, ok := ttfFaces[fontName]; ok {
		if face, ok := faces[size]; ok {
			return face, nil
		}
	}

	// Parse font if not yet parsed.
	f, ok := ttfParsed[fontName]
	if !ok {
		ttfData, exists := builtinFonts[fontName]
		if !exists {
			return nil, fmt.Errorf("unknown font %q", fontName)
		}
		var err error
		f, err = opentype.Parse(ttfData)
		if err != nil {
			return nil, fmt.Errorf("parse font %q: %w", fontName, err)
		}
		ttfParsed[fontName] = f
	}

	// Create face for this size.
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create face %q@%.1f: %w", fontName, size, err)
	}

	if ttfFaces[fontName] == nil {
		ttfFaces[fontName] = map[float64]font.Face{}
	}
	ttfFaces[fontName][size] = face
	return face, nil
}

// colorForUtilization returns the appropriate color based on utilization and thresholds.
func colorForUtilization(utilization *float64, thresholds Thresholds) color.RGBA {
	if utilization == nil {
		return color.RGBA{128, 128, 128, 255} // Gray
	}
	v := *utilization
	if v >= thresholds.Critical {
		return color.RGBA{220, 53, 69, 255} // Red
	}
	if v >= thresholds.Warning {
		return color.RGBA{255, 193, 7, 255} // Yellow
	}
	return color.RGBA{40, 167, 69, 255} // Green
}

// renderIcon creates an RGBA icon image of the given size based on the quota state.
func renderIcon(state QuotaState, thresholds Thresholds, fontSize float64, iconSize int, fontName string, haloSize float64) image.Image {
	dc := gg.NewContext(iconSize, iconSize)
	dc.SetColor(color.RGBA{0, 0, 0, 0})
	dc.Clear()

	utilization := state.FiveHour
	col := colorForUtilization(utilization, thresholds)

	s := float64(iconSize) / 64.0 // scale factor relative to base size 64

	if state.TokenExpired {
		drawExpiredIcon(dc, iconSize, s, fontName)
	} else if state.Error != "" {
		drawErrorIcon(dc, iconSize, s)
	} else {
		drawNormalIcon(dc, utilization, col, fontSize*s, iconSize, s, fontName, haloSize*s)
	}

	return dc.Image()
}

// drawExpiredIcon draws an amber warning triangle with "!" for token expiry.
func drawExpiredIcon(dc *gg.Context, iconSize int, s float64, fontName string) {
	amber := color.RGBA{255, 193, 7, 255}
	center := float64(iconSize) / 2
	margin := 4.0 * s

	// Triangle vertices: top-center, bottom-left, bottom-right
	topX, topY := center, margin
	botL, botR, botY := margin, float64(iconSize)-margin, float64(iconSize)-margin

	// Filled triangle
	dc.SetColor(amber)
	dc.MoveTo(topX, topY)
	dc.LineTo(botR, botY)
	dc.LineTo(botL, botY)
	dc.ClosePath()
	dc.Fill()

	// "!" centered in the triangle, nudged down for visual weight.
	// Black on amber — no halo needed for high contrast.
	drawCenteredText(dc, "!", center, center+6*s, 28*s, 0, fontName,
		color.RGBA{0, 0, 0, 255}, color.RGBA{})
}

// drawErrorIcon draws a gray circle with a red X.
func drawErrorIcon(dc *gg.Context, iconSize int, s float64) {
	center := float64(iconSize) / 2
	radius := float64(iconSize)/2 - 4*s

	// Gray filled circle
	dc.SetColor(color.RGBA{80, 80, 80, 255})
	dc.DrawCircle(center, center, radius)
	dc.Fill()

	// Red X
	xOff := 16.0 * s
	dc.SetColor(color.RGBA{220, 53, 69, 255})
	dc.SetLineWidth(6 * s)
	dc.DrawLine(xOff, xOff, float64(iconSize)-xOff, float64(iconSize)-xOff)
	dc.Stroke()
	dc.DrawLine(xOff, float64(iconSize)-xOff, float64(iconSize)-xOff, xOff)
	dc.Stroke()
}

// drawNormalIcon draws the ring outline, pie slice, and text.
func drawNormalIcon(dc *gg.Context, utilization *float64, col color.RGBA, fontSize float64, iconSize int, s float64, fontName string, haloSize float64) {
	center := float64(iconSize) / 2
	outerRadius := float64(iconSize)/2 - 4*s

	// Outer ring
	dc.SetColor(col)
	dc.SetLineWidth(4 * s)
	dc.DrawCircle(center, center, outerRadius)
	dc.Stroke()

	if utilization == nil {
		return
	}

	// Pie slice
	extent := *utilization / 100.0 * 2 * math.Pi
	if extent > 0 {
		pieRadius := float64(iconSize)/2 - 8*s
		startAngle := -math.Pi / 2 // top
		endAngle := startAngle + extent

		dc.SetColor(col)
		dc.MoveTo(center, center)
		dc.LineTo(center+pieRadius*math.Cos(startAngle), center+pieRadius*math.Sin(startAngle))
		dc.DrawArc(center, center, pieRadius, startAngle, endAngle)
		dc.LineTo(center, center)
		dc.Fill()
	}

	// Text
	text := fmt.Sprintf("%d", int(*utilization))
	drawCenteredText(dc, text, center, center, fontSize, haloSize, fontName,
		color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255})
}

// drawCenteredText draws text centered at (cx, cy) with a halo shadow for contrast.
// The shadow is drawn at 8 compass offsets to create an outline/bleed effect.
// haloSize controls the offset distance in pixels; 0 or shadow A=0 disables it.
func drawCenteredText(dc *gg.Context, text string, cx, cy, fontSize, haloSize float64, fontName string, fg, shadow color.RGBA) {
	if fontName == "bitmap" {
		drawScaledBitmapText(dc, text, cx, cy, fontSize, haloSize, fg, shadow)
		return
	}

	face, err := loadTTFFace(fontName, fontSize)
	if err != nil {
		// Fallback to bitmap on error.
		drawScaledBitmapText(dc, text, cx, cy, fontSize, haloSize, fg, shadow)
		return
	}

	dc.SetFontFace(face)

	tw, th := dc.MeasureString(text)
	metrics := face.Metrics()
	ascent := float64(metrics.Ascent.Round())

	// Center horizontally and vertically using ascent for baseline positioning.
	x := cx - tw/2
	y := cy - th/2 + ascent

	// Halo shadow: draw at 8 compass offsets for an outline/bleed effect.
	if haloSize > 0 && shadow.A > 0 {
		d := math.Max(haloSize, 1)
		dc.SetColor(shadow)
		for _, off := range [8][2]float64{
			{-d, -d}, {0, -d}, {d, -d},
			{-d, 0}, {d, 0},
			{-d, d}, {0, d}, {d, d},
		} {
			dc.DrawString(text, x+off[0], y+off[1])
		}
	}

	// Foreground
	dc.SetColor(fg)
	dc.DrawString(text, x, y)
}

// drawScaledBitmapText renders text using the 7x13 bitmap font, scaled up
// with nearest-neighbor interpolation for pixel-crisp rendering at any size.
// Shadow is drawn as an 8-direction halo at the given offset. shadowOff <= 0
// or shadow alpha == 0 disables it.
func drawScaledBitmapText(dc *gg.Context, text string, cx, cy, targetSize, shadowOff float64, fg, shadow color.Color) {
	bmpFace := basicfont.Face7x13
	metrics := bmpFace.Metrics()
	nativeH := metrics.Height.Ceil()   // 13
	nativeAsc := metrics.Ascent.Ceil() // 11

	d := &font.Drawer{Face: bmpFace}
	nativeW := d.MeasureString(text).Ceil()
	if nativeW == 0 {
		return
	}

	// Scale factor from native 13px to target size.
	scale := targetSize / float64(nativeH)
	if scale < 1 {
		scale = 1
	}
	scaledW := int(math.Round(float64(nativeW) * scale))
	scaledH := int(math.Round(float64(nativeH) * scale))

	// renderScaled renders text in col at native size, then scales up.
	renderScaled := func(col color.Color) *image.RGBA {
		native := image.NewRGBA(image.Rect(0, 0, nativeW, nativeH))
		drawer := &font.Drawer{
			Dst:  native,
			Src:  image.NewUniform(col),
			Face: bmpFace,
			Dot:  fixed.P(0, nativeAsc),
		}
		drawer.DrawString(text)

		scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
		draw.NearestNeighbor.Scale(scaled, scaled.Bounds(), native, native.Bounds(), draw.Over, nil)
		return scaled
	}

	x := int(math.Round(cx)) - scaledW/2
	y := int(math.Round(cy)) - scaledH/2

	// Halo shadow: draw at 8 compass offsets for an outline/bleed effect.
	_, _, _, sa := shadow.RGBA()
	if shadowOff > 0 && sa > 0 {
		so := int(math.Max(math.Round(shadowOff), 1))
		shadowImg := renderScaled(shadow)
		for _, off := range [8][2]int{
			{-so, -so}, {0, -so}, {so, -so},
			{-so, 0}, {so, 0},
			{-so, so}, {0, so}, {so, so},
		} {
			dc.DrawImage(shadowImg, x+off[0], y+off[1])
		}
	}
	dc.DrawImage(renderScaled(fg), x, y)
}

// iconToBytes encodes an image as PNG bytes for systray.
func iconToBytes(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
