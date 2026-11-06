package app

import (
	"bytes"
	"encoding/binary"
	"image"

	"golang.org/x/image/draw"
)

func decodePreviewImagePortable(data []byte, maxWidth, maxHeight int) (*image.RGBA, image.Point, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, image.Point{}, err
	}
	bounds := src.Bounds()
	original := bounds.Size()
	orientation := previewJPEGOrientation(data)
	if orientation >= 5 {
		original.X, original.Y = original.Y, original.X
	}
	target := fitImageSize(original.X, original.Y, maxWidth, maxHeight)
	raw := target
	if orientation >= 5 {
		raw.X, raw.Y = raw.Y, raw.X
	}
	pixels := image.NewRGBA(image.Rect(0, 0, raw.X, raw.Y))
	draw.ApproxBiLinear.Scale(pixels, pixels.Bounds(), src, bounds, draw.Src, nil)
	if orientation <= 1 {
		return pixels, original, nil
	}
	out := image.NewRGBA(image.Rect(0, 0, target.X, target.Y))
	for y := range target.Y {
		for x := range target.X {
			sx, sy := x, y
			switch orientation {
			case 2:
				sx = raw.X - 1 - x
			case 3:
				sx, sy = raw.X-1-x, raw.Y-1-y
			case 4:
				sy = raw.Y - 1 - y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, raw.Y-1-x
			case 7:
				sx, sy = raw.X-1-y, raw.Y-1-x
			case 8:
				sx, sy = raw.X-1-y, x
			}
			out.SetRGBA(x, y, pixels.RGBAAt(sx, sy))
		}
	}
	return out, original, nil
}

// Same JPEG orientation behavior as MyGo's DecodeBitmap. Read only the first
// EXIF IFD; rotation happens after scaling so it cannot retain a full-size copy.
func previewJPEGOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xff || data[i+1] == 0xda {
			break
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if size < 2 || i+2+size > len(data) {
			break
		}
		segment := data[i+4 : i+2+size]
		if data[i+1] == 0xe1 && len(segment) >= 14 && string(segment[:6]) == "Exif\x00\x00" {
			tiff := segment[6:]
			var order binary.ByteOrder
			switch string(tiff[:2]) {
			case "II":
				order = binary.LittleEndian
			case "MM":
				order = binary.BigEndian
			default:
				return 1
			}
			offset := int(order.Uint32(tiff[4:8]))
			if offset < 8 || offset+2 > len(tiff) {
				return 1
			}
			count := int(order.Uint16(tiff[offset:]))
			for at := offset + 2; count > 0 && at+12 <= len(tiff); at, count = at+12, count-1 {
				if order.Uint16(tiff[at:]) == 0x112 && order.Uint16(tiff[at+2:]) == 3 && order.Uint32(tiff[at+4:]) == 1 {
					if o := int(order.Uint16(tiff[at+8:])); o >= 1 && o <= 8 {
						return o
					}
				}
			}
			return 1
		}
		i += 2 + size
	}
	return 1
}
