//go:build windows

package main

import (
	"encoding/binary"
	"image"
)

// iconToBytes encodes an image as ICO (with embedded PNG) for Windows systray.
// Windows LoadImage requires ICO format; PNG-in-ICO is supported since Vista.
func iconToBytes(img image.Image) ([]byte, error) {
	pngData, err := encodePNG(img)
	if err != nil {
		return nil, err
	}
	return wrapPNGInICO(pngData, img.Bounds().Dx(), img.Bounds().Dy()), nil
}

// wrapPNGInICO wraps raw PNG bytes in a minimal ICO container.
func wrapPNGInICO(pngData []byte, w, h int) []byte {
	const headerSize = 6
	const entrySize = 16

	// ICO dimensions: 0 means 256 (or larger).
	bw, bh := byte(w), byte(h)
	if w >= 256 {
		bw = 0
	}
	if h >= 256 {
		bh = 0
	}

	buf := make([]byte, headerSize+entrySize+len(pngData))

	// ICONDIR header
	binary.LittleEndian.PutUint16(buf[0:], 0) // reserved
	binary.LittleEndian.PutUint16(buf[2:], 1) // type: ICO
	binary.LittleEndian.PutUint16(buf[4:], 1) // count: 1 image

	// ICONDIRENTRY
	off := headerSize
	buf[off+0] = bw                                                   // width
	buf[off+1] = bh                                                   // height
	buf[off+2] = 0                                                    // color count (0 for truecolor)
	buf[off+3] = 0                                                    // reserved
	binary.LittleEndian.PutUint16(buf[off+4:], 1)                     // planes
	binary.LittleEndian.PutUint16(buf[off+6:], 32)                    // bits per pixel
	binary.LittleEndian.PutUint32(buf[off+8:], uint32(len(pngData)))  // data size
	binary.LittleEndian.PutUint32(buf[off+12:], headerSize+entrySize) // data offset

	copy(buf[headerSize+entrySize:], pngData)
	return buf
}
