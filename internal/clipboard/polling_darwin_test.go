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
			time.Sleep(160 * time.Millisecond)
			if calls.Load() != 0 {
				result = fmt.Errorf("unchanged board entered Go callback")
				return
			}
			for i := int64(1); i <= 2; i++ {
				write()
				deadline := time.Now().Add(time.Second)
				for calls.Load() < i && time.Now().Before(deadline) {
					time.Sleep(5 * time.Millisecond)
				}
				if calls.Load() != i {
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
			write()
			time.Sleep(220 * time.Millisecond)
			if calls.Load() < 4 {
				result = fmt.Errorf("unacknowledged copy did not retry")
				return
			}
			mygo.RunOnMain(func() { pollingLibrary.interval(handle, .2) })
			before := calls.Load()
			time.Sleep(650 * time.Millisecond)
			slow := calls.Load() - before
			if slow < 2 || slow > 4 {
				result = fmt.Errorf("200ms interval: %d callbacks in 650ms", slow)
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
			time.Sleep(160 * time.Millisecond)
			stop()
			if calls.Load()-before < 2 {
				result = fmt.Errorf("restarted timer did not fire")
				return
			}
			fmt.Printf("unchanged=0 callbacks; identical copies=2; unacknowledged retry; interval update=%d/650ms; stop/restart passed; named pasteboard only\n", slow)
		}()
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return result
}
