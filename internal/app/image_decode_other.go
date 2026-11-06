//go:build !darwin

package app

import (
	_ "golang.org/x/image/tiff"
	"image"
)

func decodePreviewImage(data []byte, width, height int) (*image.RGBA, image.Point, error) {
	return decodePreviewImagePortable(data, width, height)
}
