//go:build windows

package main

import (
	"testing"
)

func TestIconToBytes_ValidICO(t *testing.T) {
	state := QuotaState{}
	th := Thresholds{Warning: 60, Critical: 85}
	img := renderIcon(state, th, 34, 64, "bold", 2)
	data, err := iconToBytes(img)
	if err != nil {
		t.Fatalf("iconToBytes error: %v", err)
	}
	// ICO magic: reserved=0x0000, type=0x0001
	if len(data) < 6 || data[0] != 0 || data[1] != 0 || data[2] != 1 || data[3] != 0 {
		t.Error("iconToBytes did not produce valid ICO data")
	}
}

func TestWrapPNGInICO(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	ico := wrapPNGInICO(png, 64, 64)

	// Header: 6 bytes + entry: 16 bytes + data: 8 bytes = 30
	if len(ico) != 30 {
		t.Fatalf("ICO length = %d, want 30", len(ico))
	}

	// Verify dimensions in entry
	if ico[6] != 64 || ico[7] != 64 {
		t.Errorf("ICO entry dimensions = %dx%d, want 64x64", ico[6], ico[7])
	}

	// Verify PNG data at offset 22
	for i, b := range png {
		if ico[22+i] != b {
			t.Errorf("PNG data mismatch at offset %d", i)
			break
		}
	}
}

func TestWrapPNGInICO_LargeSize(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G'}
	ico := wrapPNGInICO(png, 256, 256)

	// 256 maps to 0 in ICO format
	if ico[6] != 0 || ico[7] != 0 {
		t.Errorf("ICO entry dimensions = %d,%d, want 0,0 for 256x256", ico[6], ico[7])
	}
}
