package app

import (
	"image"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

const (
	previewImageBudget     = 32 << 20
	previewImageEntryLimit = 128
	previewImageQueueLimit = 24
	previewPixelScale      = 2 // Retina quality without retaining original-size pixels.
)

type previewImageKey struct {
	id, source    string
	width, height int
}

type previewImageEntry struct {
	bitmap      *ui.Bitmap
	original    image.Point
	sourceBytes int64
	bytes       int64
	used        uint64
	failed      time.Time
	loading     bool
}

type previewImageJob struct {
	key   previewImageKey
	entry *previewImageEntry
	item  model.Item
}

// One worker bounds transient decoding memory too. Entries act as receipts:
// eviction/deletion invalidates a queued or in-flight result before publication.
type previewImageCache struct {
	mu      sync.Mutex
	entries map[previewImageKey]*previewImageEntry
	queue   []previewImageJob
	running bool
	clock   uint64
	bytes   int64
}

func imageSource(it *model.Item) string {
	if c, ok := clipboard.ImageContent(it); ok {
		return c.Type + ":" + c.Hash
	}
	return strings.Join(it.Files, "\x00")
}

func imageVariantKey(it *model.Item, width, height int) previewImageKey {
	// Bucket pane sizes so native resize frames do not each request a new image.
	bucket := func(n int) int { return min(2048, max(64, (n*previewPixelScale+63)/64*64)) }
	return previewImageKey{it.ID, imageSource(it), bucket(width), bucket(height)}
}

func fitImageSize(width, height, maxWidth, maxHeight int) image.Point {
	if width <= 0 || height <= 0 {
		return image.Point{}
	}
	scale := min(1.0, float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	return image.Pt(max(1, int(math.Round(float64(width)*scale))), max(1, int(math.Round(float64(height)*scale))))
}

func bitmapCost(b *ui.Bitmap) int64 {
	w, h := b.Size()
	// Include the mip chain created by MyGo when an image is shown smaller.
	return int64(w) * int64(h) * 4 * 4 / 3
}

func (c *previewImageCache) remove(key previewImageKey) {
	if e := c.entries[key]; e != nil {
		c.bytes -= e.bytes
		delete(c.entries, key)
	}
}

func (c *previewImageCache) evict() {
	for c.bytes > previewImageBudget || len(c.entries) > previewImageEntryLimit {
		var oldest previewImageKey
		var entry *previewImageEntry
		for key, e := range c.entries {
			if !e.loading && (entry == nil || e.used < entry.used) {
				oldest, entry = key, e
			}
		}
		if entry == nil {
			return
		}
		c.remove(oldest)
	}
}

// Keep an existing variant visible while a pane crosses a resolution bucket.
func (c *previewImageCache) fallback(key previewImageKey) *ui.Bitmap {
	var best *previewImageEntry
	var distance int
	for candidate, e := range c.entries {
		if candidate.id != key.id || candidate.source != key.source || e.bitmap == nil {
			continue
		}
		d := abs(candidate.width-key.width) + abs(candidate.height-key.height)
		if best == nil || d < distance {
			best, distance = e, d
		}
	}
	if best == nil {
		return nil
	}
	best.used = c.clock
	return best.bitmap
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// bitmapFor requests only the resolution this surface needs. Clipboard data
// remains untouched; dimensions shown in metadata come from the source image.
func (a *App) bitmapFor(it *model.Item, width, height int) *ui.Bitmap {
	if !clipboard.HasImage(it) {
		return nil
	}
	key := imageVariantKey(it, width, height)
	c := &a.imageCache
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock++
	if e := c.entries[key]; e != nil {
		e.used = c.clock
		if e.bitmap != nil || e.loading || time.Since(e.failed) < time.Second {
			if e.bitmap != nil {
				return e.bitmap
			}
			return c.fallback(key)
		}
		c.remove(key) // Remote Clipboard's file may have arrived since failure.
	}
	if len(c.queue) >= previewImageQueueLimit {
		return c.fallback(key)
	}
	if c.entries == nil {
		c.entries = make(map[previewImageKey]*previewImageEntry)
	}
	e := &previewImageEntry{loading: true, used: c.clock}
	c.entries[key] = e
	c.evict()
	snapshot := *it
	snapshot.Contents = append([]model.Content(nil), it.Contents...)
	snapshot.Files = append([]string(nil), it.Files...)
	c.queue = append(c.queue, previewImageJob{key, e, snapshot})
	if !c.running {
		c.running = true
		go a.loadPreviewImages()
	}
	return c.fallback(key)
}

func (a *App) loadPreviewImages() {
	c := &a.imageCache
	for {
		c.mu.Lock()
		if len(c.queue) == 0 {
			c.running = false
			c.mu.Unlock()
			return
		}
		job := c.queue[0]
		c.queue[0] = previewImageJob{}
		c.queue = c.queue[1:]
		valid := c.entries[job.key] == job.entry
		c.mu.Unlock()
		if !valid {
			continue
		}
		var bitmap *ui.Bitmap
		var original image.Point
		var sourceBytes int64
		if data, err := clipboard.ImageData(a.store, &job.item); err == nil {
			sourceBytes = int64(len(data))
			if pixels, size, err := decodePreviewImage(data, job.key.width, job.key.height); err == nil {
				bitmap, original = ui.NewBitmap(pixels), size
			}
		}
		valid = c.complete(job, bitmap, original, sourceBytes)
		if valid && mygo.App.IsReady() {
			a.post(a.refresh)
		}
	}
}

func (c *previewImageCache) complete(job previewImageJob, bitmap *ui.Bitmap, original image.Point, sourceBytes int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[job.key] != job.entry {
		return false
	}
	e := job.entry
	e.loading, e.bitmap, e.original, e.sourceBytes = false, bitmap, original, sourceBytes
	if bitmap == nil {
		e.failed = time.Now()
	} else {
		e.bytes = bitmapCost(bitmap)
		c.bytes += e.bytes
	}
	c.evict()
	return true
}

func (a *App) imageMetadata(it *model.Item) (image.Point, int64) {
	c := &a.imageCache
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, e := range c.entries {
		if key.id == it.ID && key.source == imageSource(it) && e.bitmap != nil {
			return e.original, e.sourceBytes
		}
	}
	return image.Point{}, contentSize(it)
}

// Called with a.mu held. A replaced item's old content must not be published
// by a background decode, even when the replacement retained its ID.
func (a *App) pruneImageCacheLocked() {
	live := make(map[string]string, len(a.items))
	for _, it := range a.items {
		live[it.ID] = imageSource(it)
	}
	c := &a.imageCache
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if source, ok := live[key.id]; !ok || source != key.source {
			c.remove(key)
		}
	}
	queue := c.queue[:0]
	for _, job := range c.queue {
		if c.entries[job.key] == job.entry {
			queue = append(queue, job)
		}
	}
	clear(c.queue[len(queue):])
	c.queue = queue
	if _, ok := live[a.textPreview.item.ID]; !ok {
		a.textPreview = textPreviewCache{}
	}
}
