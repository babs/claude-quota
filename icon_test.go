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
	img := renderIcon(state, th, 34, 64, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_ErrorState(t *testing.T) {
	state := QuotaState{Error: "something broke"}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon error size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_NilUtilization(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon nil size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_TokenExpired(t *testing.T) {
	state := QuotaState{Error: "OAuth token has expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon expired size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}

	// TokenExpired should produce a different icon than a generic error.
	errState := QuotaState{Error: "something broke"}
	errImg := renderIcon(errState, th, 34, 64, "bold", 2)

	errData, err := iconToBytes(errImg)
	if err != nil {
		t.Fatalf("iconToBytes error icon: %v", err)
	}
	expData, err := iconToBytes(img)
	if err != nil {
		t.Fatalf("iconToBytes expired icon: %v", err)
	}

	if string(expData) == string(errData) {
		t.Error("expired icon should differ from generic error icon")
	}
}

func TestRenderIcon_CustomSize(t *testing.T) {
	v := 50.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 128, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("renderIcon custom size = %dx%d, want 128x128", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_CustomSize_Error(t *testing.T) {
	state := QuotaState{Error: "fail"}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 128, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("renderIcon custom size error = %dx%d, want 128x128", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_CustomSize_Expired(t *testing.T) {
	state := QuotaState{Error: "expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 128, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("renderIcon custom size expired = %dx%d, want 128x128", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_SmallSize(t *testing.T) {
	v := 75.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 24, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 24 || bounds.Dy() != 24 {
		t.Errorf("renderIcon small size = %dx%d, want 24x24", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_SmallSize_Expired(t *testing.T) {
	state := QuotaState{Error: "expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 24, "bold", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 24 || bounds.Dy() != 24 {
		t.Errorf("renderIcon small expired = %dx%d, want 24x24", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BitmapFont(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 18, 64, "bitmap", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bitmap size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_MonoFont(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "mono", 2)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon mono size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BitmapScaling(t *testing.T) {
	for _, size := range []int{24, 32, 48, 64, 128} {
		v := 42.0
		state := QuotaState{FiveHour: &v}
		th := Thresholds{Warning: 60, Critical: 85}
		img := renderIcon(state, th, 18, size, "bitmap", 2)
		bounds := img.Bounds()
		if bounds.Dx() != size || bounds.Dy() != size {
			t.Errorf("renderIcon(%d) size = %dx%d, want %dx%d",
				size, bounds.Dx(), bounds.Dy(), size, size)
		}
	}
}

func TestRenderIcon_NoHalo(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 0)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon no-halo size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_LargeHalo(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 3.0)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon large-halo size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestIconToBytes_ValidPNG(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 2)
	data, err := iconToBytes(img)
	if err != nil {
		t.Fatalf("iconToBytes error: %v", err)
	}
	// PNG magic bytes.
	if len(data) < 8 || data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Error("iconToBytes did not produce valid PNG data")
	}
}

func TestValidFontName(t *testing.T) {
	valid := []string{"bold", "regular", "mono", "monobold", "bitmap"}
	for _, name := range valid {
		if !ValidFontName(name) {
			t.Errorf("ValidFontName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", "comic-sans", "italic", "unknown"}
	for _, name := range invalid {
		if ValidFontName(name) {
			t.Errorf("ValidFontName(%q) = true, want false", name)
		}
	}
}
