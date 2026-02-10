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

// ValidIndicatorName returns true if the name is a known indicator type.
func ValidIndicatorName(name string) bool {
	switch name {
	case "pie", "bar", "arc", "bar-proj":
		return true
	}
	return false
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

// clampFrac converts a percentage (0–100+) to a fraction clamped to [0, 1].
func clampFrac(pct float64) float64 {
	f := pct / 100.0
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// RenderOptions holds rendering configuration for icon generation.
type RenderOptions struct {
	FontSize  float64
	IconSize  int
	FontName  string
	HaloSize  float64
	Indicator string
	ShowText  bool
}

// drawParams holds shared, pre-scaled rendering parameters for internal draw functions.
type drawParams struct {
	fontSize float64 // FontSize * scale
	iconSize int
	s        float64 // scale factor (iconSize / 64)
	fontName string
	haloSize float64 // HaloSize * scale
	showText bool
}

// renderIcon creates an RGBA icon image of the given size based on the quota state.
func renderIcon(state QuotaState, thresholds Thresholds, opts RenderOptions) image.Image {
	dc := gg.NewContext(opts.IconSize, opts.IconSize)
	dc.SetColor(color.RGBA{0, 0, 0, 0})
	dc.Clear()

	utilization := state.FiveHour
	col := colorForUtilization(utilization, thresholds)

	s := float64(opts.IconSize) / 64.0 // scale factor relative to base size 64
	p := drawParams{
		fontSize: opts.FontSize * s,
		iconSize: opts.IconSize,
		s:        s,
		fontName: opts.FontName,
		haloSize: opts.HaloSize * s,
		showText: opts.ShowText,
	}

	if state.TokenExpired {
		drawExpiredIcon(dc, p)
	} else if state.Error != "" {
		drawErrorIcon(dc, p)
	} else {
		switch opts.Indicator {
		case "bar":
			drawBarIcon(dc, utilization, col, p)
		case "bar-proj":
			var projected *float64
			var projCol color.RGBA
			if state.FiveHourProjected != nil {
				projected = state.FiveHourProjected
				projCol = mutedColor(colorForUtilization(projected, thresholds))
			}
			drawBarProjIcon(dc, utilization, col, projected, projCol, p)
		case "arc":
			drawArcIcon(dc, utilization, col, p)
		default:
			drawNormalIcon(dc, utilization, col, p)
		}
		drawUtilizationText(dc, utilization, p)
	}

	return dc.Image()
}

// drawExpiredIcon draws an amber warning triangle with "!" for token expiry.
func drawExpiredIcon(dc *gg.Context, p drawParams) {
	amber := color.RGBA{255, 193, 7, 255}
	center := float64(p.iconSize) / 2
	margin := 4.0 * p.s

	// Triangle vertices: top-center, bottom-left, bottom-right
	topX, topY := center, margin
	botL, botR, botY := margin, float64(p.iconSize)-margin, float64(p.iconSize)-margin

	// Filled triangle
	dc.SetColor(amber)
	dc.MoveTo(topX, topY)
	dc.LineTo(botR, botY)
	dc.LineTo(botL, botY)
	dc.ClosePath()
	dc.Fill()

	// "!" centered in the triangle, nudged down for visual weight.
	// Black on amber — no halo needed for high contrast.
	drawCenteredText(dc, "!", center, center+6*p.s, 28*p.s, 0, p.fontName,
		color.RGBA{0, 0, 0, 255}, color.RGBA{})
}

// drawErrorIcon draws a gray circle with a red X.
func drawErrorIcon(dc *gg.Context, p drawParams) {
	center := float64(p.iconSize) / 2
	radius := float64(p.iconSize)/2 - 4*p.s

	// Gray filled circle
	dc.SetColor(color.RGBA{80, 80, 80, 255})
	dc.DrawCircle(center, center, radius)
	dc.Fill()

	// Red X
	xOff := 16.0 * p.s
	dc.SetColor(color.RGBA{220, 53, 69, 255})
	dc.SetLineWidth(6 * p.s)
	dc.DrawLine(xOff, xOff, float64(p.iconSize)-xOff, float64(p.iconSize)-xOff)
	dc.Stroke()
	dc.DrawLine(xOff, float64(p.iconSize)-xOff, float64(p.iconSize)-xOff, xOff)
	dc.Stroke()
}

// drawNormalIcon draws the ring outline, pie slice, and text.
func drawNormalIcon(dc *gg.Context, utilization *float64, col color.RGBA, p drawParams) {
	center := float64(p.iconSize) / 2
	outerRadius := float64(p.iconSize)/2 - 4*p.s

	// Outer ring
	dc.SetColor(col)
	dc.SetLineWidth(4 * p.s)
	dc.DrawCircle(center, center, outerRadius)
	dc.Stroke()

	if utilization == nil {
		return
	}

	// Pie slice
	extent := clampFrac(*utilization) * 2 * math.Pi
	if extent > 0 {
		pieRadius := float64(p.iconSize)/2 - 8*p.s
		startAngle := -math.Pi / 2 // top
		endAngle := startAngle + extent

		dc.SetColor(col)
		dc.MoveTo(center, center)
		dc.LineTo(center+pieRadius*math.Cos(startAngle), center+pieRadius*math.Sin(startAngle))
		dc.DrawArc(center, center, pieRadius, startAngle, endAngle)
		dc.LineTo(center, center)
		dc.Fill()
	}
}

// drawBarIcon draws a vertical filling bar indicator (bottom to top).
func drawBarIcon(dc *gg.Context, utilization *float64, col color.RGBA, p drawParams) {
	border := 2 * p.s
	size := float64(p.iconSize)

	// Border rectangle
	dc.SetColor(col)
	dc.SetLineWidth(border)
	dc.DrawRectangle(border/2, border/2, size-border, size-border)
	dc.Stroke()

	if utilization == nil {
		return
	}

	// Filled portion from bottom
	innerMargin := border + p.s
	innerW := size - 2*innerMargin
	innerH := size - 2*innerMargin
	fillH := innerH * clampFrac(*utilization)

	if fillH > 0 {
		dc.SetColor(col)
		dc.DrawRectangle(innerMargin, innerMargin+innerH-fillH, innerW, fillH)
		dc.Fill()
	}
}

// drawArcIcon draws a progress ring (thick arc stroke filling clockwise from 12 o'clock).
func drawArcIcon(dc *gg.Context, utilization *float64, col color.RGBA, p drawParams) {
	center := float64(p.iconSize) / 2
	strokeWidth := 6 * p.s
	radius := float64(p.iconSize)/2 - strokeWidth/2 - 2*p.s

	// Background track ring (dim gray)
	dc.SetColor(color.RGBA{60, 60, 60, 255})
	dc.SetLineWidth(strokeWidth)
	dc.DrawCircle(center, center, radius)
	dc.Stroke()

	if utilization == nil {
		return
	}

	// Foreground arc
	extent := clampFrac(*utilization) * 2 * math.Pi
	if extent > 0 {
		startAngle := -math.Pi / 2 // 12 o'clock
		endAngle := startAngle + extent

		dc.SetColor(col)
		dc.SetLineWidth(strokeWidth)
		dc.DrawArc(center, center, radius, startAngle, endAngle)
		dc.Stroke()
	}
}

// mutedColor returns a desaturated version of the color by blending 50% toward
// medium gray. This keeps the hue recognizable while being visually distinct
// from the full-brightness variant, even on dark backgrounds.
func mutedColor(c color.RGBA) color.RGBA {
	return color.RGBA{
		uint8((int(c.R) + 128) / 2),
		uint8((int(c.G) + 128) / 2),
		uint8((int(c.B) + 128) / 2),
		c.A,
	}
}

// drawBarProjIcon draws two side-by-side vertical bars: left = actual 5h consumption,
// right = projected 5h consumption at window reset (muted colors).
func drawBarProjIcon(dc *gg.Context, utilization *float64, col color.RGBA, projected *float64, projCol color.RGBA, p drawParams) {
	border := 2 * p.s
	size := float64(p.iconSize)
	gap := 1 * p.s

	// Border rectangle
	dc.SetColor(col)
	dc.SetLineWidth(border)
	dc.DrawRectangle(border/2, border/2, size-border, size-border)
	dc.Stroke()

	if utilization == nil {
		return
	}

	innerMargin := border + p.s
	innerW := size - 2*innerMargin
	innerH := size - 2*innerMargin

	// Column widths: split inner area into two columns with a gap.
	colW := (innerW - gap) / 2

	// Left bar: actual 5h utilization.
	fillH := innerH * clampFrac(*utilization)
	if fillH > 0 {
		dc.SetColor(col)
		dc.DrawRectangle(innerMargin, innerMargin+innerH-fillH, colW, fillH)
		dc.Fill()
	}

	// Right bar: projected utilization (muted color).
	if projected != nil {
		projFillH := innerH * clampFrac(*projected)
		if projFillH > 0 {
			rightX := innerMargin + colW + gap
			dc.SetColor(projCol)
			dc.DrawRectangle(rightX, innerMargin+innerH-projFillH, colW, projFillH)
			dc.Fill()
		}
	}
}

// drawUtilizationText draws the utilization percentage centered on the icon.
// Called once from renderIcon after the indicator shape has been drawn.
func drawUtilizationText(dc *gg.Context, utilization *float64, p drawParams) {
	if !p.showText || utilization == nil {
		return
	}
	center := float64(p.iconSize) / 2
	text := fmt.Sprintf("%d", int(*utilization))
	drawCenteredText(dc, text, center, center, p.fontSize, p.haloSize, p.fontName,
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

// encodePNG encodes an image as PNG bytes.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
