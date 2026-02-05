package main

import (
	"image/color"
	"testing"
)

func TestColorForUtilization_Nil(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	got := colorForUtilization(nil, th)
	want := color.RGBA{128, 128, 128, 255}
	if got != want {
		t.Errorf("colorForUtilization(nil) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_Green(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	v := 30.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{40, 167, 69, 255}
	if got != want {
		t.Errorf("colorForUtilization(30) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_Yellow(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	v := 60.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{255, 193, 7, 255}
	if got != want {
		t.Errorf("colorForUtilization(60) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_YellowMid(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	v := 75.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{255, 193, 7, 255}
	if got != want {
		t.Errorf("colorForUtilization(75) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_Red(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	v := 85.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{220, 53, 69, 255}
	if got != want {
		t.Errorf("colorForUtilization(85) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_RedHigh(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	v := 100.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{220, 53, 69, 255}
	if got != want {
		t.Errorf("colorForUtilization(100) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_CustomThresholds(t *testing.T) {
	th := Thresholds{Warning: 40, Critical: 70}
	v := 50.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{255, 193, 7, 255} // yellow (>=40 but <70)
	if got != want {
		t.Errorf("colorForUtilization(50, w=40 c=70) = %v, want %v", got, want)
	}
}

func TestColorForUtilization_ZeroBoundary(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	v := 0.0
	got := colorForUtilization(&v, th)
	want := color.RGBA{40, 167, 69, 255} // green
	if got != want {
		t.Errorf("colorForUtilization(0) = %v, want %v", got, want)
	}
}

func TestRenderIcon_ProducesImage(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_ErrorState(t *testing.T) {
	state := QuotaState{Error: "something broke"}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon error size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_NilUtilization(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon nil size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestIconToBytes_ValidPNG(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th)
	data, err := iconToBytes(img)
	if err != nil {
		t.Fatalf("iconToBytes error: %v", err)
	}
	// PNG magic bytes.
	if len(data) < 8 || data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Error("iconToBytes did not produce valid PNG data")
	}
}
