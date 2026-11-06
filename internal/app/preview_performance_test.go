package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/image/tiff"

	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func previewPNG(t testing.TB, width, height int) []byte {
	t.Helper()
	src := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := color.RGBA{R: 240, A: 255}
			if y >= height/2 {
				c = color.RGBA{B: 240, A: 255}
			}
			if x >= width/2 {
				c.A = 128
				c.R /= 2
				c.B /= 2
			}
			src.SetRGBA(x, y, c)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, src); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestPreviewDecodeBoundsPixelsAndMetadata(t *testing.T) {
	for _, size := range []image.Point{image.Pt(3840, 2160), image.Pt(20, 10)} {
		data := previewPNG(t, size.X, size.Y)
		pixels, original, err := decodePreviewImage(data, 680, 80)
		if err != nil {
			t.Fatal(err)
		}
		want := fitImageSize(size.X, size.Y, 680, 80)
		if original != size || pixels.Bounds().Size() != want {
			t.Fatalf("original=%v bitmap=%v, want %v / %v", original, pixels.Bounds(), size, want)
		}
		w, h := want.X, want.Y
		top := pixels.RGBAAt(w/4, h/4)
		bottom := pixels.RGBAAt(w/4, h*3/4)
		alpha := pixels.RGBAAt(w*3/4, h/4)
		if top.R < 220 || top.B > 20 || bottom.B < 220 || bottom.R > 20 {
			t.Fatalf("image flipped or colors changed: top=%v bottom=%v", top, bottom)
		}
		if alpha.A < 126 || alpha.A > 130 || alpha.R < 110 || alpha.R > 130 {
			t.Fatalf("premultiplied transparency lost: %v", alpha)
		}
	}
	if _, _, err := decodePreviewImage([]byte("invalid image"), 680, 80); err == nil {
		t.Fatal("invalid data accepted")
	}
}

func orientedJPEG(t *testing.T, orientation byte) []byte {
	t.Helper()
	src := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for y := range 80 {
		for x := range 120 {
			c := color.RGBA{R: 240, A: 255}
			if x >= 60 {
				c = color.RGBA{G: 240, A: 255}
			}
			if y >= 40 {
				c = color.RGBA{B: 240, A: 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, src, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	// EXIF little-endian TIFF with one Orientation SHORT in the first IFD.
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, orientation, 0, 0, 0, 0, 0, 0, 0}
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(exif)+2))
	data := append([]byte(nil), jpegData.Bytes()[:2]...)
	data = append(data, segment...)
	data = append(data, exif...)
	return append(data, jpegData.Bytes()[2:]...)
}

func TestPreviewJPEGOrientation(t *testing.T) {
	// Source quadrants: red/green above blue/blue. Check the transformed
	// top-left quadrant as well as dimensions, for all EXIF transformations.
	wantColors := []byte{'r', 'g', 'b', 'b', 'r', 'b', 'b', 'g'}
	for o := byte(1); o <= 8; o++ {
		t.Run(fmt.Sprint(o), func(t *testing.T) {
			pixels, original, err := decodePreviewImage(orientedJPEG(t, o), 240, 240)
			if err != nil {
				t.Fatal(err)
			}
			want := image.Pt(120, 80)
			if o >= 5 {
				want = image.Pt(80, 120)
			}
			if original != want || pixels.Bounds().Size() != want {
				t.Fatalf("dimensions=%v / %v, want %v", original, pixels.Bounds(), want)
			}
			c := pixels.RGBAAt(want.X/4, want.Y/4)
			got := byte('r')
			if c.G > c.R && c.G > c.B {
				got = 'g'
			}
			if c.B > c.R && c.B > c.G {
				got = 'b'
			}
			if got != wantColors[o-1] {
				t.Fatalf("orientation %d top-left=%v, want %c", o, c, wantColors[o-1])
			}
			portable, portableOriginal, err := decodePreviewImagePortable(orientedJPEG(t, o), 240, 240)
			if err != nil || portableOriginal != original || portable.Bounds() != pixels.Bounds() {
				t.Fatalf("portable orientation %d: %v / %v / %v", o, portableOriginal, portable, err)
			}
			other := portable.RGBAAt(want.X/4, want.Y/4)
			if abs(int(other.R)-int(c.R)) > 10 || abs(int(other.G)-int(c.G)) > 10 || abs(int(other.B)-int(c.B)) > 10 {
				t.Fatalf("portable orientation changed pixels: %v / %v", c, other)
			}
		})
	}
}

func TestPreviewTIFF(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1200, 600))
	src.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := tiff.Encode(&encoded, src, nil); err != nil {
		t.Fatal(err)
	}
	for _, decode := range []func([]byte, int, int) (*image.RGBA, image.Point, error){decodePreviewImage, decodePreviewImagePortable} {
		pixels, original, err := decode(encoded.Bytes(), 200, 200)
		if err != nil || original != image.Pt(1200, 600) || pixels.Bounds().Size() != image.Pt(200, 100) {
			t.Fatalf("TIFF decode: original=%v error=%v", original, err)
		}
	}
}

func TestPreviewImageCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := previewImageCache{entries: map[previewImageKey]*previewImageEntry{}}
	b := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1024, 1024)))
	for i := 0; i < 10; i++ {
		key := previewImageKey{id: fmt.Sprint(i)}
		cost := bitmapCost(b)
		c.entries[key] = &previewImageEntry{bitmap: b, bytes: cost, used: uint64(i + 1)}
		c.bytes += cost
		c.evict()
	}
	if c.bytes > previewImageBudget || c.entries[previewImageKey{id: "0"}] != nil || c.entries[previewImageKey{id: "9"}] == nil {
		t.Fatalf("cache not bounded/LRU: bytes=%d entries=%d", c.bytes, len(c.entries))
	}
}

func TestPreviewRetainsImageDuringResize(t *testing.T) {
	a := newTestApp(t)
	it := &model.Item{ID: "image", Contents: []model.Content{{Type: "image/png", Hash: "source"}}}
	key := imageVariantKey(it, 400, 600)
	bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 800, 400)))
	a.imageCache.entries = map[previewImageKey]*previewImageEntry{key: {bitmap: bitmap, original: image.Pt(4000, 2000)}}
	a.imageCache.running = true
	if got := a.bitmapFor(it, 600, 600); got != bitmap {
		t.Fatal("resize blanked an already loaded preview")
	}
	if got := a.bitmapFor(it, 600, 600); got != bitmap || len(a.imageCache.queue) != 1 {
		t.Fatal("resize duplicated in-flight work")
	}
}

func TestPreviewImageCacheRejectsStaleCompletion(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			a := newTestApp(t)
			it := makeItem(t, a, "image", "old image")
			it.Contents[0].Type = "image/png"
			a.addItem(it)
			key := imageVariantKey(it, 340, 40)
			e := &previewImageEntry{loading: true}
			a.imageCache.entries = map[previewImageKey]*previewImageEntry{key: e}
			job := previewImageJob{key: key, entry: e, item: *it}
			if replace {
				it.Contents[0].Hash = "replacement"
				a.pruneImageCacheLocked()
			} else {
				a.deleteItem(it)
			}
			b := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2, 2)))
			if a.imageCache.complete(job, b, image.Pt(4000, 2000), 100) {
				t.Fatal("stale decode published")
			}
			if a.imageCache.bytes != 0 {
				t.Fatal("stale decode retained pixels")
			}
		})
	}
}

func TestPreviewImageRequestsBoundedAndRetryable(t *testing.T) {
	a := newTestApp(t)
	a.imageCache.running = true // Inspect the queue before decoding it.
	for i := 0; i < 100; i++ {
		it := &model.Item{ID: fmt.Sprint(i), Contents: []model.Content{{Type: "image/jpeg", Hash: fmt.Sprint(i)}}}
		a.bitmapFor(it, 340, 40)
	}
	if len(a.imageCache.queue) != previewImageQueueLimit || len(a.imageCache.entries) != previewImageQueueLimit {
		t.Fatal("image work queue grew without bound")
	}
	job := a.imageCache.queue[0]
	a.imageCache.queue = a.imageCache.queue[1:]
	a.imageCache.complete(job, nil, image.Point{}, 0)
	a.bitmapFor(&job.item, 340, 40)
	if len(a.imageCache.queue) != previewImageQueueLimit-1 {
		t.Fatal("failed file retried without backoff")
	}
	job.entry.failed = time.Now().Add(-2 * time.Second)
	a.bitmapFor(&job.item, 340, 40)
	if len(a.imageCache.queue) != previewImageQueueLimit {
		t.Fatal("late-arriving image cannot retry")
	}
}

func TestPreviewWorkerPreservesOriginalClipboard(t *testing.T) {
	a := newTestApp(t)
	data := previewPNG(t, 1200, 600)
	h, n, err := a.store.PutBlob(data)
	if err != nil {
		t.Fatal(err)
	}
	it := &model.Item{Contents: []model.Content{{Type: "image/png", Hash: h, Size: n}}}
	a.addItem(it)
	a.bitmapFor(it, 340, 40)
	a.bitmapFor(it, 400, 600)
	deadline := time.Now().Add(10 * time.Second)
	for {
		a.imageCache.mu.Lock()
		running := a.imageCache.running
		a.imageCache.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("image worker did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	thumb := a.bitmapFor(it, 340, 40)
	preview := a.bitmapFor(it, 400, 600)
	if thumb == nil || preview == nil || thumb == preview {
		t.Fatal("thumbnail and preview not cached independently")
	}
	w, hgt := thumb.Size()
	if w > 704 || hgt > 128 {
		t.Fatalf("full-size thumbnail retained: %dx%d", w, hgt)
	}
	dimensions, encodedBytes := a.imageMetadata(it)
	if dimensions != image.Pt(1200, 600) || encodedBytes != int64(len(data)) {
		t.Fatalf("source metadata lost: %v/%d", dimensions, encodedBytes)
	}
	restored, err := clipboard.Data(a.store, it, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restored.Items()[0].Read("image/png")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("preview changed original clipboard data")
	}
}

func TestTextPreviewCachesAndInvalidates(t *testing.T) {
	a := newTestApp(t)
	it := makeItem(t, a, "fallback", "first body")
	if got := a.previewText(it); got != "first body" {
		t.Fatal(got)
	}
	if err := os.Remove(filepath.Join(a.store.Dir(), "blobs", it.Contents[0].Hash)); err != nil {
		t.Fatal(err)
	}
	if got := a.previewText(it); got != "first body" {
		t.Fatal("unchanged preview reread blob")
	}
	replacement := makeItem(t, a, "fallback", "second body")
	it.Contents = replacement.Contents
	if got := a.previewText(it); got != "second body" {
		t.Fatal("content hash did not invalidate preview")
	}
	it.Contents = nil
	it.Title = "new fallback"
	if got := a.previewText(it); got != "new fallback" {
		t.Fatal("title did not invalidate preview")
	}
	it.Contents = replacement.Contents
	if got := a.itemText(it); got != "second body" {
		t.Fatal("display cache changed full-text read")
	}
	large := makeItem(t, a, "large", strings.Repeat("x", 20000))
	if got := a.previewText(large); len(got) != previewParagraphBytes {
		t.Fatal("preview paragraph bound changed")
	}
}

func TestStartupDoesNotRewriteUnchangedFiles(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	store, err := model.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	sentinel := time.Unix(1_600_000_000, 0)
	for _, name := range []string{"config.json", "history.json"} {
		if err := os.Chtimes(filepath.Join(dir, name), sentinel, sentinel); err != nil {
			t.Fatal(err)
		}
	}
	New(cfg, store)
	for _, name := range []string{"config.json", "history.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(sentinel) {
			t.Fatalf("startup rewrote unchanged %s", name)
		}
	}
}

func TestStartupPersistsTrimmingAndPinRepair(t *testing.T) {
	a := newTestApp(t)
	a.addItem(makeItem(t, a, "first", "first"))
	a.addItem(makeItem(t, a, "second", "second"))
	a.togglePin(a.items[0])
	a.addItem(makeItem(t, a, "third", "third"))
	if err := a.cfg.Update(func(c *config.Config) { c.HistorySize = 1; c.PinOrder = []string{"nonexistent"} }); err != nil {
		t.Fatal(err)
	}
	store, err := model.Open(a.store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(a.store.Dir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := New(cfg, store)
	if len(restarted.items) != 2 {
		t.Fatal("startup did not trim unpinned history")
	}
	reopened, err := model.Open(a.store.Dir())
	if err != nil || len(reopened.Items) != 2 {
		t.Fatal("trimmed history was not persisted")
	}
	reloaded, err := config.Load(filepath.Join(a.store.Dir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	order := reloaded.Snapshot().PinOrder
	if len(order) != 1 || order[0] == "nonexistent" {
		t.Fatal("repaired pin order was not persisted")
	}
}

func BenchmarkPreviewDecode(b *testing.B) {
	data := previewPNG(b, 3840, 2160)
	for _, kind := range []string{"original-bitmap", "thumbnail", "preview"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var bitmap *ui.Bitmap
				if kind == "original-bitmap" {
					var err error
					bitmap, err = ui.DecodeBitmap(data)
					if err != nil {
						b.Fatal(err)
					}
				} else {
					width, height := 704, 128
					if kind == "preview" {
						width, height = 832, 1216
					}
					pixels, _, err := decodePreviewImage(data, width, height)
					if err != nil {
						b.Fatal(err)
					}
					bitmap = ui.NewBitmap(pixels)
				}
				w, h := bitmap.Size()
				b.ReportMetric(float64(w*h*4), "pixel-bytes")
			}
		})
	}
}

func BenchmarkTextPreview(b *testing.B) {
	dir := b.TempDir()
	store, err := model.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	data := []byte(strings.Repeat("long preview text\n", 8000))
	h, n, err := store.PutBlob(data)
	if err != nil {
		b.Fatal(err)
	}
	a := &App{store: store}
	it := &model.Item{ID: "text", Contents: []model.Content{{Type: "text/plain", Hash: h, Size: n}}}
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprint("cached=", cached), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if cached {
					a.previewText(it)
				} else {
					sizedPreviewText(a.itemText(it))
				}
			}
		})
	}
}
