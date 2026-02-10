package main

import (
	"image/color"
	"testing"
)

// testOpts returns RenderOptions with common test defaults.
func testOpts() RenderOptions {
	return RenderOptions{
		FontSize:  34,
		IconSize:  64,
		FontName:  "bold",
		HaloSize:  2,
		Indicator: "pie",
		ShowText:  true,
	}
}

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
	img := renderIcon(state, th, testOpts())
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_ErrorState(t *testing.T) {
	state := QuotaState{Error: "something broke"}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, testOpts())
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon error size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_NilUtilization(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, testOpts())
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon nil size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_TokenExpired(t *testing.T) {
	state := QuotaState{Error: "OAuth token has expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, testOpts())
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon expired size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}

	// TokenExpired should produce a different icon than a generic error.
	errState := QuotaState{Error: "something broke"}
	errImg := renderIcon(errState, th, testOpts())

	errData, err := encodePNG(errImg)
	if err != nil {
		t.Fatalf("encodePNG error icon: %v", err)
	}
	expData, err := encodePNG(img)
	if err != nil {
		t.Fatalf("encodePNG expired icon: %v", err)
	}

	if string(expData) == string(errData) {
		t.Error("expired icon should differ from generic error icon")
	}
}

func TestRenderIcon_CustomSize(t *testing.T) {
	v := 50.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.IconSize = 128
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("renderIcon custom size = %dx%d, want 128x128", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_CustomSize_Error(t *testing.T) {
	state := QuotaState{Error: "fail"}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.IconSize = 128
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("renderIcon custom size error = %dx%d, want 128x128", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_CustomSize_Expired(t *testing.T) {
	state := QuotaState{Error: "expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.IconSize = 128
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("renderIcon custom size expired = %dx%d, want 128x128", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_SmallSize(t *testing.T) {
	v := 75.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.IconSize = 24
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 24 || bounds.Dy() != 24 {
		t.Errorf("renderIcon small size = %dx%d, want 24x24", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_SmallSize_Expired(t *testing.T) {
	state := QuotaState{Error: "expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.IconSize = 24
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 24 || bounds.Dy() != 24 {
		t.Errorf("renderIcon small expired = %dx%d, want 24x24", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BitmapFont(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.FontSize = 18
	opts.FontName = "bitmap"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bitmap size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_MonoFont(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.FontName = "mono"
	img := renderIcon(state, th, opts)
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
		opts := testOpts()
		opts.FontSize = 18
		opts.FontName = "bitmap"
		opts.IconSize = size
		img := renderIcon(state, th, opts)
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
	opts := testOpts()
	opts.HaloSize = 0
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon no-halo size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_LargeHalo(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.HaloSize = 3.0
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon large-halo size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestEncodePNG_Valid(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, testOpts())
	data, err := encodePNG(img)
	if err != nil {
		t.Fatalf("encodePNG error: %v", err)
	}
	// PNG magic bytes.
	if len(data) < 8 || data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Error("encodePNG did not produce valid PNG data")
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

func TestValidIndicatorName(t *testing.T) {
	valid := []string{"pie", "bar", "arc", "bar-proj"}
	for _, name := range valid {
		if !ValidIndicatorName(name) {
			t.Errorf("ValidIndicatorName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", "circle", "gauge", "unknown"}
	for _, name := range invalid {
		if ValidIndicatorName(name) {
			t.Errorf("ValidIndicatorName(%q) = true, want false", name)
		}
	}
}

func TestRenderIcon_BarIndicator(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "bar"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bar size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BarIndicator_NilUtilization(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "bar"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bar nil size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_ArcIndicator(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "arc"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon arc size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_ArcIndicator_NilUtilization(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "arc"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon arc nil size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_IndicatorsDiffer(t *testing.T) {
	v := 50.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}

	pieOpts := testOpts()
	barOpts := testOpts()
	barOpts.Indicator = "bar"
	arcOpts := testOpts()
	arcOpts.Indicator = "arc"

	pieImg := renderIcon(state, th, pieOpts)
	barImg := renderIcon(state, th, barOpts)
	arcImg := renderIcon(state, th, arcOpts)

	pieData, _ := encodePNG(pieImg)
	barData, _ := encodePNG(barImg)
	arcData, _ := encodePNG(arcImg)

	if string(pieData) == string(barData) {
		t.Error("pie and bar icons should differ")
	}
	if string(pieData) == string(arcData) {
		t.Error("pie and arc icons should differ")
	}
	if string(barData) == string(arcData) {
		t.Error("bar and arc icons should differ")
	}
}

func TestRenderIcon_ShowTextFalse(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}

	for _, ind := range []string{"pie", "bar", "arc"} {
		withOpts := testOpts()
		withOpts.Indicator = ind
		withoutOpts := testOpts()
		withoutOpts.Indicator = ind
		withoutOpts.ShowText = false

		withText := renderIcon(state, th, withOpts)
		withoutText := renderIcon(state, th, withoutOpts)

		wData, _ := encodePNG(withText)
		woData, _ := encodePNG(withoutText)

		if string(wData) == string(woData) {
			t.Errorf("indicator %q: show_text=true and show_text=false should produce different icons", ind)
		}
	}
}

func TestRenderIcon_ErrorIgnoresIndicator(t *testing.T) {
	state := QuotaState{Error: "fail"}
	th := Thresholds{Warning: 60, Critical: 85}

	pieOpts := testOpts()
	barOpts := testOpts()
	barOpts.Indicator = "bar"
	arcOpts := testOpts()
	arcOpts.Indicator = "arc"

	pieImg := renderIcon(state, th, pieOpts)
	barImg := renderIcon(state, th, barOpts)
	arcImg := renderIcon(state, th, arcOpts)

	pieData, _ := encodePNG(pieImg)
	barData, _ := encodePNG(barImg)
	arcData, _ := encodePNG(arcImg)

	if string(pieData) != string(barData) || string(pieData) != string(arcData) {
		t.Error("error state should produce the same icon regardless of indicator type")
	}
}

func TestRenderIcon_ExpiredIgnoresIndicator(t *testing.T) {
	state := QuotaState{Error: "expired", TokenExpired: true}
	th := Thresholds{Warning: 60, Critical: 85}

	pieOpts := testOpts()
	barOpts := testOpts()
	barOpts.Indicator = "bar"
	arcOpts := testOpts()
	arcOpts.Indicator = "arc"

	pieImg := renderIcon(state, th, pieOpts)
	barImg := renderIcon(state, th, barOpts)
	arcImg := renderIcon(state, th, arcOpts)

	pieData, _ := encodePNG(pieImg)
	barData, _ := encodePNG(barImg)
	arcData, _ := encodePNG(arcImg)

	if string(pieData) != string(barData) || string(pieData) != string(arcData) {
		t.Error("expired state should produce the same icon regardless of indicator type")
	}
}

func TestRenderIcon_BarProj(t *testing.T) {
	v := 42.0
	proj := 75.0
	state := QuotaState{FiveHour: &v, FiveHourProjected: &proj}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "bar-proj"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bar-proj size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BarProj_NilProjection(t *testing.T) {
	v := 42.0
	state := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "bar-proj"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bar-proj nil proj size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BarProj_NilUtilization(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.Indicator = "bar-proj"
	img := renderIcon(state, th, opts)
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("renderIcon bar-proj nil util size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderIcon_BarProj_DiffersFromBar(t *testing.T) {
	v := 50.0
	proj := 80.0
	stateWithProj := QuotaState{FiveHour: &v, FiveHourProjected: &proj}
	stateNoProj := QuotaState{FiveHour: &v}
	th := Thresholds{Warning: 60, Critical: 85}

	barProjOpts := testOpts()
	barProjOpts.Indicator = "bar-proj"
	barOpts := testOpts()
	barOpts.Indicator = "bar"

	barProjImg := renderIcon(stateWithProj, th, barProjOpts)
	barImg := renderIcon(stateNoProj, th, barOpts)

	barProjData, _ := encodePNG(barProjImg)
	barData, _ := encodePNG(barImg)

	if string(barProjData) == string(barData) {
		t.Error("bar-proj with projection should differ from plain bar")
	}
}

func TestRenderIcon_BarProj_ShowTextFalse(t *testing.T) {
	v := 42.0
	proj := 75.0
	state := QuotaState{FiveHour: &v, FiveHourProjected: &proj}
	th := Thresholds{Warning: 60, Critical: 85}

	withOpts := testOpts()
	withOpts.Indicator = "bar-proj"
	withoutOpts := testOpts()
	withoutOpts.Indicator = "bar-proj"
	withoutOpts.ShowText = false

	withText := renderIcon(state, th, withOpts)
	withoutText := renderIcon(state, th, withoutOpts)

	wData, _ := encodePNG(withText)
	woData, _ := encodePNG(withoutText)

	if string(wData) == string(woData) {
		t.Error("bar-proj show_text=true and show_text=false should produce different icons")
	}
}

func TestMutedColor(t *testing.T) {
	// Red {220, 53, 69} blended 50% toward gray {128,128,128} → {174, 90, 98}
	c := color.RGBA{220, 53, 69, 255}
	m := mutedColor(c)
	if m.R != 174 || m.G != 90 || m.B != 98 || m.A != 255 {
		t.Errorf("mutedColor(%v) = %v, want {174, 90, 98, 255}", c, m)
	}

	// Green {40, 167, 69} → {84, 147, 98}
	g := color.RGBA{40, 167, 69, 255}
	mg := mutedColor(g)
	if mg.R != 84 || mg.G != 147 || mg.B != 98 || mg.A != 255 {
		t.Errorf("mutedColor(%v) = %v, want {84, 147, 98, 255}", g, mg)
	}
}

func TestClampFrac(t *testing.T) {
	tests := []struct {
		pct  float64
		want float64
	}{
		{0, 0},
		{50, 0.5},
		{100, 1.0},
		{-10, 0},
		{200, 1.0},
		{0.5, 0.005},
	}
	for _, tc := range tests {
		got := clampFrac(tc.pct)
		if got != tc.want {
			t.Errorf("clampFrac(%v) = %v, want %v", tc.pct, got, tc.want)
		}
	}
}
