package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
)

const iconSize = 64

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

// renderIcon creates a 64x64 RGBA icon image based on the quota state.
func renderIcon(state QuotaState, thresholds Thresholds) image.Image {
	dc := gg.NewContext(iconSize, iconSize)
	dc.SetColor(color.RGBA{0, 0, 0, 0})
	dc.Clear()

	utilization := state.FiveHour
	col := colorForUtilization(utilization, thresholds)

	if state.Error != "" {
		drawErrorIcon(dc, col)
	} else {
		drawNormalIcon(dc, utilization, col)
	}

	return dc.Image()
}

// drawErrorIcon draws a gray circle with a red X.
func drawErrorIcon(dc *gg.Context, _ color.RGBA) {
	center := float64(iconSize) / 2
	radius := float64(iconSize)/2 - 4

	// Gray filled circle
	dc.SetColor(color.RGBA{80, 80, 80, 255})
	dc.DrawCircle(center, center, radius)
	dc.Fill()

	// Red X
	dc.SetColor(color.RGBA{220, 53, 69, 255})
	dc.SetLineWidth(6)
	dc.DrawLine(16, 16, float64(iconSize)-16, float64(iconSize)-16)
	dc.Stroke()
	dc.DrawLine(16, float64(iconSize)-16, float64(iconSize)-16, 16)
	dc.Stroke()
}

// drawNormalIcon draws the ring outline, pie slice, and text.
func drawNormalIcon(dc *gg.Context, utilization *float64, col color.RGBA) {
	center := float64(iconSize) / 2
	outerRadius := float64(iconSize)/2 - 4

	// Outer ring
	dc.SetColor(col)
	dc.SetLineWidth(4)
	dc.DrawCircle(center, center, outerRadius)
	dc.Stroke()

	if utilization == nil {
		return
	}

	// Pie slice
	extent := *utilization / 100.0 * 2 * math.Pi
	if extent > 0 {
		pieRadius := float64(iconSize)/2 - 8
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
	drawCenteredText(dc, text, center, center-1)
}

// drawCenteredText draws text centered at (cx, cy) with a black shadow.
func drawCenteredText(dc *gg.Context, text string, cx, cy float64) {
	face, err := loadFontFace(18)
	if err != nil {
		return
	}
	dc.SetFontFace(face)

	w, h := dc.MeasureString(text)
	x := cx - w/2
	y := cy + h/2

	// Shadow
	dc.SetColor(color.RGBA{0, 0, 0, 255})
	dc.DrawString(text, x+1, y+1)

	// Foreground
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.DrawString(text, x, y)
}

// loadFontFace loads the embedded Go Bold font at the given size.
func loadFontFace(size float64) (font.Face, error) {
	font, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("create face: %w", err)
	}
	return face, nil
}

// iconToBytes encodes an image as PNG bytes for systray.
func iconToBytes(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
