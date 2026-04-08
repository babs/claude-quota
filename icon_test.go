package main

import (
	"image/color"
	"testing"
)

// testOpts returns RenderOptions with common test defaults.
func testOpts() RenderOptions {
	return RenderOptions{
		FontSize:             34,
		IconSize:             64,
		FontName:             "bold",
		HaloSize:             2,
		Indicator:            "pie",
		ShowText:             true,
		ProviderMark:         false,
		ProviderMarkSize:     14,
		ProviderMarkPosition: "SE",
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
		state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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

func TestProviderAccentColor(t *testing.T) {
	if got := providerAccentColor(ProviderClaude); got != (color.RGBA{255, 140, 0, 255}) {
		t.Fatalf("providerAccentColor(claude) = %v, want orange", got)
	}
	if got := providerAccentColor(ProviderCodex); got != (color.RGBA{120, 120, 120, 255}) {
		t.Fatalf("providerAccentColor(codex) = %v, want gray", got)
	}
	if got := providerAccentColor(""); got != (color.RGBA{90, 90, 90, 255}) {
		t.Fatalf("providerAccentColor(\"\") = %v, want fallback gray", got)
	}
}

func TestRenderIcon_ProviderMarkDisabledDoesNotChangeIcon(t *testing.T) {
	v := 42.0
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()

	claudeImg := renderIcon(QuotaState{Provider: ProviderClaude, FiveHour: &v}, th, opts)
	codexImg := renderIcon(QuotaState{Provider: ProviderCodex, FiveHour: &v}, th, opts)

	claudeData, err := encodePNG(claudeImg)
	if err != nil {
		t.Fatalf("encodePNG claude: %v", err)
	}
	codexData, err := encodePNG(codexImg)
	if err != nil {
		t.Fatalf("encodePNG codex: %v", err)
	}
	if string(claudeData) != string(codexData) {
		t.Fatal("provider mark disabled should not change icon rendering")
	}
}

func TestRenderIcon_ProviderAccentDiffers(t *testing.T) {
	v := 42.0
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.ProviderMark = true

	claudeImg := renderIcon(QuotaState{Provider: ProviderClaude, FiveHour: &v}, th, opts)
	codexImg := renderIcon(QuotaState{Provider: ProviderCodex, FiveHour: &v}, th, opts)

	claudeData, err := encodePNG(claudeImg)
	if err != nil {
		t.Fatalf("encodePNG claude: %v", err)
	}
	codexData, err := encodePNG(codexImg)
	if err != nil {
		t.Fatalf("encodePNG codex: %v", err)
	}
	if string(claudeData) == string(codexData) {
		t.Fatal("provider accent should make Claude and Codex icons differ")
	}
}

func TestParseHexColor(t *testing.T) {
	cases := []struct {
		in   string
		want color.RGBA
		ok   bool
	}{
		// 6-char opaque (RRGGBB)
		{"#DE7356", color.RGBA{0xDE, 0x73, 0x56, 0xFF}, true},
		{"de7356", color.RGBA{0xDE, 0x73, 0x56, 0xFF}, true},
		{"  #FFFFFF  ", color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, true},
		{"#000000", color.RGBA{0x00, 0x00, 0x00, 0xFF}, true},
		// 3-char shorthand (RGB) — each nibble duplicated
		{"#DE7", color.RGBA{0xDD, 0xEE, 0x77, 0xFF}, true},
		{"de7", color.RGBA{0xDD, 0xEE, 0x77, 0xFF}, true},
		{"#F00", color.RGBA{0xFF, 0x00, 0x00, 0xFF}, true},
		{"#000", color.RGBA{0x00, 0x00, 0x00, 0xFF}, true},
		{"#FFF", color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, true},
		// 8-char with alpha (RRGGBBAA)
		{"#DE735680", color.RGBA{0xDE, 0x73, 0x56, 0x80}, true},
		{"de735680", color.RGBA{0xDE, 0x73, 0x56, 0x80}, true},
		{"#FFFFFFFF", color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, true},
		{"#00000001", color.RGBA{0x00, 0x00, 0x00, 0x01}, true},
		// Rejected forms
		{"", color.RGBA{}, false},
		{"#FF", color.RGBA{}, false},      // 2 chars
		{"#FFFF", color.RGBA{}, false},    // 4 chars
		{"#FFFFF", color.RGBA{}, false},   // 5 chars
		{"#FFFFFFF", color.RGBA{}, false}, // 7 chars
		{"#GGGGGG", color.RGBA{}, false},
		{"#ZZZ", color.RGBA{}, false},
		{"#1234567", color.RGBA{}, false},
		// Alpha 00 rejected: A==0 is the "no override" sentinel in renderIcon
		{"#DE735600", color.RGBA{}, false},
		{"#00000000", color.RGBA{}, false},
	}
	for _, tc := range cases {
		got, err := parseHexColor(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("parseHexColor(%q) unexpected error: %v", tc.in, err)
				continue
			}
			if got != tc.want {
				t.Errorf("parseHexColor(%q) = %v, want %v", tc.in, got, tc.want)
			}
		} else if err == nil {
			t.Errorf("parseHexColor(%q) expected error, got %v", tc.in, got)
		}
	}
}

func TestRenderIcon_ProviderMarkColorOverride(t *testing.T) {
	v := 42.0
	th := Thresholds{Warning: 60, Critical: 85}

	def := testOpts()
	def.ProviderMark = true

	override := testOpts()
	override.ProviderMark = true
	override.ProviderMarkColor = color.RGBA{0xDE, 0x73, 0x56, 0xFF}

	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
	defData, _ := encodePNG(renderIcon(state, th, def))
	overrideData, _ := encodePNG(renderIcon(state, th, override))
	if string(defData) == string(overrideData) {
		t.Fatal("provider_mark_color override should change the rendered icon")
	}
}

func TestRenderIcon_ProviderMarkColorOverridesBarBorder(t *testing.T) {
	v := 42.0
	th := Thresholds{Warning: 60, Critical: 85}

	base := testOpts()
	base.Indicator = "bar"
	base.ProviderMark = true

	override := base
	override.ProviderMarkColor = color.RGBA{0xDE, 0x73, 0x56, 0xFF}

	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
	baseData, _ := encodePNG(renderIcon(state, th, base))
	overrideData, _ := encodePNG(renderIcon(state, th, override))
	if string(baseData) == string(overrideData) {
		t.Fatal("provider_mark_color override should change the bar border")
	}
}

func TestRenderIcon_ProviderMarkColorOverridesBarProjBorder(t *testing.T) {
	v := 42.0
	proj := 70.0
	th := Thresholds{Warning: 60, Critical: 85}

	base := testOpts()
	base.Indicator = "bar-proj"
	base.ProviderMark = true

	override := base
	override.ProviderMarkColor = color.RGBA{0xDE, 0x73, 0x56, 0xFF}

	state := QuotaState{Provider: ProviderClaude, FiveHour: &v, FiveHourProjected: &proj}
	baseData, _ := encodePNG(renderIcon(state, th, base))
	overrideData, _ := encodePNG(renderIcon(state, th, override))
	if string(baseData) == string(overrideData) {
		t.Fatal("provider_mark_color override should change the bar-proj border")
	}
}

func TestRenderIcon_ProviderAccentDiffersInErrorState(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.ProviderMark = true

	claudeImg := renderIcon(QuotaState{Provider: ProviderClaude, Error: "something broke"}, th, opts)
	codexImg := renderIcon(QuotaState{Provider: ProviderCodex, Error: "something broke"}, th, opts)

	claudeData, err := encodePNG(claudeImg)
	if err != nil {
		t.Fatalf("encodePNG claude: %v", err)
	}
	codexData, err := encodePNG(codexImg)
	if err != nil {
		t.Fatalf("encodePNG codex: %v", err)
	}
	if string(claudeData) == string(codexData) {
		t.Fatal("provider accent should make Claude and Codex error icons differ")
	}
}

func TestRenderIcon_ProviderAccentDiffersInExpiredState(t *testing.T) {
	th := Thresholds{Warning: 60, Critical: 85}
	opts := testOpts()
	opts.ProviderMark = true

	claudeImg := renderIcon(QuotaState{
		Provider:     ProviderClaude,
		Error:        "OAuth token has expired",
		TokenExpired: true,
	}, th, opts)
	codexImg := renderIcon(QuotaState{
		Provider:     ProviderCodex,
		Error:        "OAuth token has expired",
		TokenExpired: true,
	}, th, opts)

	claudeData, err := encodePNG(claudeImg)
	if err != nil {
		t.Fatalf("encodePNG claude: %v", err)
	}
	codexData, err := encodePNG(codexImg)
	if err != nil {
		t.Fatalf("encodePNG codex: %v", err)
	}
	if string(claudeData) == string(codexData) {
		t.Fatal("provider accent should make Claude and Codex expired icons differ")
	}
}

func TestValidProviderMarkPosition(t *testing.T) {
	for _, pos := range []string{"NW", "NE", "SW", "SE"} {
		if !ValidProviderMarkPosition(pos) {
			t.Fatalf("ValidProviderMarkPosition(%q) = false", pos)
		}
	}
	for _, pos := range []string{"", "north", "center", "ss"} {
		if ValidProviderMarkPosition(pos) {
			t.Fatalf("ValidProviderMarkPosition(%q) = true", pos)
		}
	}
}

func TestRenderIcon_ProviderMarkPositionsDiffer(t *testing.T) {
	v := 42.0
	th := Thresholds{Warning: 60, Critical: 85}

	nw := testOpts()
	nw.ProviderMark = true
	nw.ProviderMarkPosition = "NW"
	se := testOpts()
	se.ProviderMark = true
	se.ProviderMarkPosition = "SE"

	nwData, _ := encodePNG(renderIcon(QuotaState{Provider: ProviderClaude, FiveHour: &v}, th, nw))
	seData, _ := encodePNG(renderIcon(QuotaState{Provider: ProviderClaude, FiveHour: &v}, th, se))
	if string(nwData) == string(seData) {
		t.Fatal("different provider mark positions should render differently")
	}
}

func TestRenderIcon_ProviderMarkSizeDiffers(t *testing.T) {
	v := 42.0
	th := Thresholds{Warning: 60, Critical: 85}

	small := testOpts()
	small.ProviderMark = true
	small.ProviderMarkSize = 12
	large := testOpts()
	large.ProviderMark = true
	large.ProviderMarkSize = 18

	smallData, _ := encodePNG(renderIcon(QuotaState{Provider: ProviderClaude, FiveHour: &v}, th, small))
	largeData, _ := encodePNG(renderIcon(QuotaState{Provider: ProviderClaude, FiveHour: &v}, th, large))
	if string(smallData) == string(largeData) {
		t.Fatal("different provider mark sizes should render differently")
	}
}

func TestRenderIcon_BarIndicator(t *testing.T) {
	v := 42.0
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
	state := QuotaState{Provider: ProviderClaude, FiveHour: &v}
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
