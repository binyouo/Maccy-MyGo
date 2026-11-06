//go:build darwin

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func TestNativePollingTimerLifecycle(t *testing.T) {
	if os.Getenv("MACCY_NATIVE_CLIPBOARD_TEST") != "1" {
		t.Skip("set MACCY_NATIVE_CLIPBOARD_TEST=1 to run the isolated AppKit probe")
	}
	library := filepath.Join(t.TempDir(), "libmaccy-poll.dylib")
	build := exec.Command("clang", "-O2", "-fobjc-arc", "-dynamiclib", "-framework", "AppKit", "../../tools/clipboard-poll.m", "-o", library)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile bridge: %v\n%s", err, out)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MACCY_NATIVE_POLLING_PROBE=1", "MACCY_NATIVE_POLLING_LIBRARY="+library)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("polling timer probe: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

// No windows, and all writes use a unique named pasteboard. Acknowledgement
// must suppress unchanged callbacks; failures must retry until acknowledged.
func probeNativePollingTimer() error {
	if err := loadPollingLibrary(os.Getenv("MACCY_NATIVE_POLLING_LIBRARY")); err != nil {
		return err
	}
	var result error
	var calls atomic.Int64
	fired := make(chan time.Time, 64)
	var acknowledge atomic.Bool
	acknowledge.Store(true)
	mygo.App.SetName("MaccyNativePollingProbe")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		board := objc.ID(objc.GetClass("NSPasteboard")).Send(objc.RegisterName("pasteboardWithUniqueName"))
		var handle uintptr
		var accepted *atomic.Int64
		block := objc.NewBlock(func(_ objc.Block) {
			calls.Add(1)
			select {
			case fired <- time.Now():
			default:
			}
			if acknowledge.Load() {
				accepted.Store(objc.Send[int64](board, pasteboardChangeSelector))
			}
		})
		handle = pollingLibrary.start(.05, objc.Send[int64](board, pasteboardChangeSelector), board, block)
		if handle == 0 {
			block.Release()
			result = fmt.Errorf("native timer unavailable")
			mygo.App.Quit()
			return
		}
		accepted = (*atomic.Int64)(pollingLibrary.accepted(handle))
		write := func() {
			mygo.RunOnMain(func() {
				cls := objc.ID(objc.GetClass("NSString"))
				text := cls.Send(objc.RegisterName("stringWithUTF8String:"), "same native poll fixture")
				format := cls.Send(objc.RegisterName("stringWithUTF8String:"), "public.utf8-plain-text")
				board.Send(objc.RegisterName("clearContents"))
				board.Send(objc.RegisterName("setString:forType:"), text, format)
			})
		}
		stop := func() {
			mygo.RunOnMain(func() {
				if handle != 0 {
					pollingLibrary.stop(handle)
					handle = 0
				}
			})
		}
		go func() {
			defer mygo.App.Quit()
			defer func() {
				stop()
				mygo.RunOnMain(func() { block.Release(); board.Send(objc.RegisterName("releaseGlobally")) })
			}()
			waitCalls := func(want int64) bool {
				deadline := time.NewTimer(3 * time.Second)
				defer deadline.Stop()
				for calls.Load() < want {
					select {
					case <-fired:
					case <-deadline.C:
						return false
					}
				}
				return true
			}
			time.Sleep(160 * time.Millisecond)
			if calls.Load() != 0 {
				result = fmt.Errorf("unchanged board entered Go callback")
				return
			}
			for i := int64(1); i <= 2; i++ {
				write()
				if !waitCalls(i) || calls.Load() != i {
					result = fmt.Errorf("identical copy %d not observed: %d", i, calls.Load())
					return
				}
			}
			time.Sleep(120 * time.Millisecond)
			if calls.Load() != 2 {
				result = fmt.Errorf("acknowledged board repeated")
				return
			}
			acknowledge.Store(false)
			before := calls.Load()
			write()
			if !waitCalls(before + 2) {
				result = fmt.Errorf("unacknowledged copy did not retry")
				return
			}
			var changedAt time.Time
			mygo.RunOnMain(func() {
				pollingLibrary.interval(handle, .2)
				changedAt = time.Now()
			})
			// Ignore notifications from before replacement. A shared CI host
			// may delay timers, so wait for events instead of requiring a
			// callback count inside a short, fixed wall-clock window.
			for len(fired) > 0 {
				<-fired
			}
			deadline := time.NewTimer(3 * time.Second)
			var last time.Time
			for i := 0; i < 3; i++ {
				select {
				case last = <-fired:
				case <-deadline.C:
					result = fmt.Errorf("updated timer did not deliver three callbacks")
					return
				}
			}
			deadline.Stop()
			if elapsed := last.Sub(changedAt); elapsed < 400*time.Millisecond {
				result = fmt.Errorf("200ms interval update was ignored: three callbacks in %v", elapsed)
				return
			}
			stop()
			stop()
			before = calls.Load()
			time.Sleep(250 * time.Millisecond)
			if calls.Load() != before {
				result = fmt.Errorf("callback after stop")
				return
			}
			mygo.RunOnMain(func() {
				handle = pollingLibrary.start(.05, -1, board, block)
				if handle != 0 {
					accepted = (*atomic.Int64)(pollingLibrary.accepted(handle))
				}
			})
			if handle == 0 {
				result = fmt.Errorf("restart unavailable")
				return
			}
			if !waitCalls(before + 2) {
				result = fmt.Errorf("restarted timer did not fire")
				return
			}
			stop()
			fmt.Printf("unchanged=0 callbacks; identical copies=2; unacknowledged retry; interval update and stop/restart passed; named pasteboard only\n")
		}()
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return result
}
