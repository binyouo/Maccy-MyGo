//go:build darwin

package clipboard

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

var pollingLibrary struct {
	sync.Mutex
	loaded   bool
	start    func(float64, int64, objc.ID, objc.Block) uintptr
	accepted func(uintptr) unsafe.Pointer
	interval func(uintptr, float64)
	stop     func(uintptr)
}

// Keep the library loaded: timers and copied blocks can retain its code.
func loadPollingLibrary(path string) error {
	pollingLibrary.Lock()
	defer pollingLibrary.Unlock()
	if pollingLibrary.loaded {
		return nil
	}
	lib, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return err
	}
	var version func() uint32
	for name, fn := range map[string]any{
		"maccy_poll_version":  &version,
		"maccy_poll_start":    &pollingLibrary.start,
		"maccy_poll_accepted": &pollingLibrary.accepted,
		"maccy_poll_interval": &pollingLibrary.interval,
		"maccy_poll_stop":     &pollingLibrary.stop,
	} {
		address, err := purego.Dlsym(lib, name)
		if err != nil {
			purego.Dlclose(lib)
			return err
		}
		purego.RegisterFunc(fn, address)
	}
	if version() != 1 {
		purego.Dlclose(lib)
		return fmt.Errorf("unsupported clipboard polling bridge version")
	}
	pollingLibrary.loaded = true
	return nil
}

func (w *Watcher) startNativePolling(stop <-chan struct{}) *nativePolling {
	if !mygo.App.IsReady() {
		return nil
	}
	pollingLibrary.Lock()
	loaded := pollingLibrary.loaded
	pollingLibrary.Unlock()
	if !loaded {
		dir, err := mygo.App.Path(mygo.PathResources)
		if err != nil {
			return nil
		}
		// Packaged apps merge platform resources; go run uses the source tree.
		err = loadPollingLibrary(filepath.Join(dir, "libmaccy-poll.dylib"))
		if err != nil {
			err = loadPollingLibrary(filepath.Join(dir, "darwin", "libmaccy-poll.dylib"))
		}
		if err != nil {
			log.Printf("clipboard: native polling bridge unavailable; using portable polling: %v", err)
			return nil
		}
	}
	var handle uintptr
	var block objc.Block
	var accepted *atomic.Int64
	changes, unsubscribe := w.cfg.SubscribeIntervalChanges()
	interval := w.pollingInterval()
	mygo.RunOnMain(func() {
		block = objc.NewBlock(func(_ objc.Block) {
			select {
			case <-stop:
				return
			default:
				w.Wake()
			}
		})
		w.mu.Lock()
		count := w.lastChange
		w.mu.Unlock()
		handle = pollingLibrary.start(interval, count, 0, block)
		if handle == 0 {
			block.Release()
			return
		}
		accepted = (*atomic.Int64)(pollingLibrary.accepted(handle))
	})
	if handle == 0 {
		unsubscribe()
		return nil
	}
	return &nativePolling{
		accepted:        accepted,
		intervalChanges: changes,
		stop: func() {
			unsubscribe()
			mygo.RunOnMain(func() {
				if handle != 0 {
					pollingLibrary.stop(handle)
					handle = 0
					block.Release()
				}
			})
		},
		syncInterval: func() {
			next := w.pollingInterval()
			if next == interval {
				return
			}
			mygo.RunOnMain(func() {
				if handle != 0 {
					pollingLibrary.interval(handle, next)
					interval = next
				}
			})
		},
	}
}
