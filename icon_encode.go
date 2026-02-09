//go:build !windows

package main

import "image"

// iconToBytes encodes an image as PNG bytes for systray.
func iconToBytes(img image.Image) ([]byte, error) {
	return encodePNG(img)
}
