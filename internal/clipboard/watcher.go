package clipboard

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

// Watcher polls the system clipboard and reports new copies.
type Watcher struct {
	cfg   *config.Config
	store *model.Store

	// OnNew is called (from a background goroutine) for each new copy.
	OnNew func(*model.Item)
	// OnTitle updates an item's title after background OCR.
	OnTitle func(id, title string)
	// OnCaptureState reports consumption of the one-copy pause, so the
	// menu bar can restore its recording status without waiting for a copy.
	OnCaptureState func()

	mu         sync.Mutex
	last       string
	lastChange int64
	hasChange  bool
	stopCh     chan struct{}
	started    bool
	wake       chan struct{}
	native     *nativePolling
}

// accepted points to native-owned aligned atomic storage. Access is protected
// by w.mu, so Stop clears it before the main thread releases the native handle.
type nativePolling struct {
	accepted        *atomic.Int64
	stop            func()
	syncInterval    func()
	intervalChanges <-chan struct{}
}

// New creates a watcher.
func New(cfg *config.Config, store *model.Store) *Watcher {
	return &Watcher{cfg: cfg, store: store, stopCh: make(chan struct{}), wake: make(chan struct{}, 1)}
}

// Start primes the fingerprint with the current clipboard (so copies made
// before launch are not captured) and begins polling.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.stopCh = make(chan struct{})
	stop := w.stopCh
	w.mu.Unlock()

	if mygo.App.IsReady() {
		if count, ok := ChangeCount(); ok {
			w.setSequence(count)
		} else {
			snap := w.cfg.Snapshot()
			id, _ := FrontmostApp()
			if !snap.IgnoreEvents && !shouldIgnoreApp(snap, id) {
				if data, ok, err := readForCapture(snap, mygo.Clipboard.Formats(), mygo.Clipboard.Read); err == nil && ok {
					w.setLast(Fingerprint(data))
				}
			}
		}
	}
	native := w.startNativePolling(stop)
	w.mu.Lock()
	current := w.started && w.stopCh == stop
	if current {
		w.native = native
		if native != nil {
			native.accepted.Store(w.lastChange)
		}
	}
	w.mu.Unlock()
	if !current {
		if native != nil {
			native.stop()
		}
		return
	}
	go w.loop(stop, native)
}

func (w *Watcher) setSequence(count int64) {
	w.mu.Lock()
	w.lastChange, w.hasChange = count, true
	if w.native != nil {
		w.native.accepted.Store(count)
	}
	w.mu.Unlock()
}

func (w *Watcher) sameSequence(count int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hasChange && w.lastChange == count
}

func (w *Watcher) acceptCopy(fp string, count int64, sequenced bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if sequenced {
		if w.hasChange && w.lastChange == count {
			return false
		}
	} else if w.last == fp {
		return false
	}
	w.last, w.lastChange, w.hasChange = fp, count, sequenced
	if sequenced && w.native != nil {
		w.native.accepted.Store(count)
	}
	return true
}

// Stop ends polling.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return
	}
	close(w.stopCh)
	w.started = false
	native := w.native
	w.native = nil
	w.mu.Unlock()
	// Invalidation runs on the timer's main run loop. Never hold w.mu
	// while waiting for the main thread, whose callback also uses it.
	if native != nil {
		native.stop()
	}
}

// Wake forces a poll soon (used after the app itself writes the clipboard).
func (w *Watcher) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Watcher) getLast() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.last
}

func (w *Watcher) setLast(s string) {
	w.mu.Lock()
	w.last = s
	w.mu.Unlock()
}

func (w *Watcher) pollingInterval() float64 {
	return max(.05, w.cfg.Interval())
}

func (w *Watcher) loop(stop <-chan struct{}, native *nativePolling) {
	for {
		var timer *time.Timer
		var tick <-chan time.Time
		var intervalChanges <-chan struct{}
		if native == nil {
			timer = time.NewTimer(time.Duration(w.pollingInterval() * float64(time.Second)))
			tick = timer.C
		} else {
			intervalChanges = native.intervalChanges
		}
		select {
		case <-stop:
			if timer != nil {
				timer.Stop()
			}
			return
		case <-w.wake:
			if timer != nil {
				timer.Stop()
			}
		case <-intervalChanges:
			native.syncInterval()
			continue
		case <-tick:
		}
		select {
		case <-stop:
			return
		default:
		}
		if native != nil {
			native.syncInterval()
		}
		w.poll()
	}
}

func (w *Watcher) poll() {
	if !mygo.App.IsReady() {
		return
	}
	count, sequenced := ChangeCount()
	if sequenced && w.sameSequence(count) {
		return
	}
	if sequenced && w.cfg.Snapshot().IgnoreEvents {
		// A native counter lets paused capture observe a new copy without
		// materializing its potentially sensitive clipboard payload.
		w.setSequence(count)
		w.consumeIgnoreNext()
		return
	}
	id, name := FrontmostApp()
	snap := w.cfg.Snapshot()
	if shouldIgnoreApp(snap, id) {
		if sequenced {
			w.setSequence(count)
		}
		return
	}
	data, accepted, err := readForCapture(snap, mygo.Clipboard.Formats(), mygo.Clipboard.Read)
	if err != nil {
		return
	}
	if !accepted {
		if sequenced {
			w.setSequence(count)
		}
		return
	}
	if sequenced {
		after, ok := ChangeCount()
		if !ok || after != count {
			return // ownership changed during the read; retry next poll
		}
	}
	w.capture(data, count, sequenced, id, name)
}

func (w *Watcher) capture(data transfer.Data, count int64, sequenced bool, id, name string) {
	snap := w.cfg.Snapshot()
	if sequenced && !w.acceptCopy("", count, true) {
		return
	}
	if snap.IgnoreEvents {
		if !sequenced && !w.acceptCopy(Fingerprint(data), count, false) {
			return
		}
		// A pending "ignore only the next copy" clears itself after one copy.
		w.consumeIgnoreNext()
		return
	}

	if shouldIgnoreApp(snap, id) {
		return
	}
	formats, enabled := captureFormats(snap, data.Formats())
	if !enabled {
		return
	}
	data = captureData(data, formats)
	if !sequenced && !w.acceptCopy(Fingerprint(data), count, false) {
		return
	}
	it, ok, err := Build(w.store, snap, data)
	if err != nil || !ok {
		return
	}
	it.Application, it.ApplicationName = id, name
	if sequenced {
		it.ClipboardChangeCount = count
	}
	it.ID = newItemID()
	var image []byte
	if it.Title == "Image" && w.OnTitle != nil && HasImage(it) {
		image, _ = ImageData(w.store, it)
	}
	if w.OnNew != nil {
		w.OnNew(it)
	} else {
		it.ReleaseBlobs()
	}
	// OCR image titles in the background so capture stays instant.
	if it.Title == "Image" && w.OnTitle != nil {
		if image != nil {
			go func(id string, data []byte) {
				if txt := recognizeImage(data); txt != "" {
					w.OnTitle(id, shorten(txt, 1000))
				}
			}(it.ID, image)
		}
	}
}

func (w *Watcher) consumeIgnoreNext() {
	if w.cfg.ConsumeIgnoreNext() && w.OnCaptureState != nil {
		w.OnCaptureState()
	}
}

func shouldIgnoreApp(cfg config.Config, id string) bool {
	if id == "" {
		return false
	}
	found := false
	for _, a := range cfg.IgnoredApps {
		if a == id {
			found = true
			break
		}
	}
	if cfg.IgnoreAllAppsExceptListed {
		return !found
	}
	return found
}
func newItemID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
