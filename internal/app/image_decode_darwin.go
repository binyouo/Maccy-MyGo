//go:build darwin

package app

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ImageIO decodes the requested size, applies EXIF orientation and supports
// native clipboard TIFF/HEIC. No original-size bitmap is retained by the UI.
var previewIO struct {
	once                                    sync.Once
	err                                     error
	release                                 func(uintptr)
	dataCreate                              func(uintptr, *byte, int) uintptr
	dictionaryCreate                        func(uintptr, *uintptr, *uintptr, int, uintptr, uintptr) uintptr
	dictionaryGet                           func(uintptr, uintptr) uintptr
	numberCreate                            func(uintptr, int, *int64) uintptr
	numberGet                               func(uintptr, int, *int64) bool
	sourceCreate                            func(uintptr, uintptr) uintptr
	properties                              func(uintptr, int, uintptr) uintptr
	thumbnail                               func(uintptr, int, uintptr) uintptr
	imageWidth, imageHeight                 func(uintptr) int
	colorSpace                              func() uintptr
	contextCreate                           func(*byte, int, int, int, int, uintptr, uint32) uintptr
	draw                                    func(uintptr, previewCGRect, uintptr)
	cache, createAlways, transform, maxSize uintptr
	width, height, orientation              uintptr
	trueValue, falseValue                   uintptr
}

type previewCGRect struct{ X, Y, W, H float64 }

func loadPreviewIO() error {
	p := &previewIO
	p.once.Do(func() {
		open := func(path string) uintptr {
			lib, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_LOCAL)
			if err != nil {
				p.err = err
			}
			return lib
		}
		cf := open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		cg := open("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
		io := open("/System/Library/Frameworks/ImageIO.framework/ImageIO")
		if p.err != nil {
			return
		}
		addr := func(lib uintptr, name string) uintptr {
			a, err := purego.Dlsym(lib, name)
			if err != nil {
				p.err = err
			}
			return a
		}
		bind := func(lib uintptr, target any, name string) {
			if a := addr(lib, name); a != 0 {
				purego.RegisterFunc(target, a)
			}
		}
		value := func(lib uintptr, name string) uintptr {
			if a := addr(lib, name); a != 0 {
				return **(**uintptr)(unsafe.Pointer(&a))
			}
			return 0
		}
		bind(cf, &p.release, "CFRelease")
		bind(cf, &p.dataCreate, "CFDataCreate")
		bind(cf, &p.dictionaryCreate, "CFDictionaryCreate")
		bind(cf, &p.dictionaryGet, "CFDictionaryGetValue")
		bind(cf, &p.numberCreate, "CFNumberCreate")
		bind(cf, &p.numberGet, "CFNumberGetValue")
		bind(io, &p.sourceCreate, "CGImageSourceCreateWithData")
		bind(io, &p.properties, "CGImageSourceCopyPropertiesAtIndex")
		bind(io, &p.thumbnail, "CGImageSourceCreateThumbnailAtIndex")
		bind(cg, &p.imageWidth, "CGImageGetWidth")
		bind(cg, &p.imageHeight, "CGImageGetHeight")
		bind(cg, &p.colorSpace, "CGColorSpaceCreateDeviceRGB")
		bind(cg, &p.contextCreate, "CGBitmapContextCreate")
		bind(cg, &p.draw, "CGContextDrawImage")
		p.cache = value(io, "kCGImageSourceShouldCache")
		p.createAlways = value(io, "kCGImageSourceCreateThumbnailFromImageAlways")
		p.transform = value(io, "kCGImageSourceCreateThumbnailWithTransform")
		p.maxSize = value(io, "kCGImageSourceThumbnailMaxPixelSize")
		p.width = value(io, "kCGImagePropertyPixelWidth")
		p.height = value(io, "kCGImagePropertyPixelHeight")
		p.orientation = value(io, "kCGImagePropertyOrientation")
		p.trueValue = value(cf, "kCFBooleanTrue")
		p.falseValue = value(cf, "kCFBooleanFalse")
	})
	return p.err
}

func decodePreviewImage(data []byte, maxWidth, maxHeight int) (*image.RGBA, image.Point, error) {
	if len(data) == 0 {
		return nil, image.Point{}, errors.New("empty preview image")
	}
	if err := loadPreviewIO(); err != nil {
		return nil, image.Point{}, err
	}
	p := &previewIO
	bytes := p.dataCreate(0, &data[0], len(data))
	runtime.KeepAlive(data)
	if bytes == 0 {
		return nil, image.Point{}, errors.New("create image data")
	}
	defer p.release(bytes)
	keys, values := []uintptr{p.cache}, []uintptr{p.falseValue}
	options := p.dictionaryCreate(0, &keys[0], &values[0], len(keys), 0, 0)
	if options == 0 {
		return nil, image.Point{}, errors.New("create image options")
	}
	defer p.release(options)
	source := p.sourceCreate(bytes, options)
	if source == 0 {
		return nil, image.Point{}, errors.New("decode image source")
	}
	defer p.release(source)
	properties := p.properties(source, 0, options)
	if properties == 0 {
		return nil, image.Point{}, errors.New("read image dimensions")
	}
	defer p.release(properties)
	readNumber := func(key uintptr) int {
		var n int64
		if v := p.dictionaryGet(properties, key); v != 0 {
			p.numberGet(v, 4, &n)
		}
		return int(n)
	}
	original := image.Pt(readNumber(p.width), readNumber(p.height))
	if o := readNumber(p.orientation); o >= 5 && o <= 8 {
		original.X, original.Y = original.Y, original.X
	}
	if original.X <= 0 || original.Y <= 0 {
		return nil, original, errors.New("invalid image dimensions")
	}
	target := fitImageSize(original.X, original.Y, maxWidth, maxHeight)
	limit := int64(max(target.X, target.Y))
	number := p.numberCreate(0, 4, &limit) // kCFNumberSInt64Type
	if number == 0 {
		return nil, original, errors.New("create thumbnail limit")
	}
	defer p.release(number)
	keys = []uintptr{p.cache, p.createAlways, p.transform, p.maxSize}
	values = []uintptr{p.falseValue, p.trueValue, p.trueValue, number}
	thumbOptions := p.dictionaryCreate(0, &keys[0], &values[0], len(keys), 0, 0)
	if thumbOptions == 0 {
		return nil, original, errors.New("create thumbnail options")
	}
	defer p.release(thumbOptions)
	thumb := p.thumbnail(source, 0, thumbOptions)
	if thumb == 0 {
		return nil, original, errors.New("decode thumbnail")
	}
	defer p.release(thumb)
	w, h := p.imageWidth(thumb), p.imageHeight(thumb)
	if w <= 0 || h <= 0 || w > maxWidth+1 || h > maxHeight+1 {
		return nil, original, fmt.Errorf("invalid thumbnail size %dx%d", w, h)
	}
	color := p.colorSpace()
	if color == 0 {
		return nil, original, errors.New("create preview color space")
	}
	defer p.release(color)
	pixels := image.NewRGBA(image.Rect(0, 0, w, h))
	// Big-endian component order + premultiplied-last gives Go's RGBA bytes.
	ctx := p.contextCreate(&pixels.Pix[0], w, h, 8, pixels.Stride, color, 1<<14|1)
	if ctx == 0 {
		return nil, original, errors.New("create preview bitmap context")
	}
	p.draw(ctx, previewCGRect{W: float64(w), H: float64(h)}, thumb)
	p.release(ctx)
	runtime.KeepAlive(pixels)
	return pixels, original, nil
}
