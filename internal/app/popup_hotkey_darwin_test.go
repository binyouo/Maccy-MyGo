//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if os.Getenv("MACCY_NATIVE_GEOMETRY_PROBE") == "1" {
		if err := probeNativePopupGeometry(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if os.Getenv("MACCY_NATIVE_HOTKEY_PROBE") == "1" {
		if err := probeNativePopupHotkey(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNativePopupHotkeyMonitor(t *testing.T) {
	if os.Getenv("MACCY_NATIVE_HOTKEY_TEST") != "1" {
		t.Skip("set MACCY_NATIVE_HOTKEY_TEST=1 for the isolated AppKit event probe")
	}
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = append(os.Environ(), "MACCY_NATIVE_HOTKEY_PROBE=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native shortcut monitor: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func TestNativePopupKeyAndModifiers(t *testing.T) {
	if nativePopupKey(0x08) != ui.KeyC || nativePopupKey(0x31) != ui.KeySpace || nativePopupKey(0xffff) != 0 {
		t.Fatal("Carbon and local key mapping differ")
	}
	if got := nativeKeyboardModifiers(1<<17 | 1<<18 | 1<<19 | 1<<20 | 1<<16); got != ui.Shift|ui.Ctrl|ui.Alt|ui.Super {
		t.Fatalf("modifiers=%v", got)
	}
}

// The subprocess owns two hidden windows and sends synthetic events only
// inside its own NSApplication. It never uses the system event tap,
// changes focus to another application or accesses the clipboard.
func probeNativePopupHotkey() error {
	var result error
	mygo.App.SetName("MaccyPopupHotkeyProbe")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		defer mygo.App.Quit()
		w := mygo.NewWindow(mygo.WindowOptions{Title: "hidden shortcut probe", Hidden: true, Panel: true, Width: 100, Height: 100})
		other := mygo.NewWindow(mygo.WindowOptions{Title: "hidden scope probe", Hidden: true, Panel: true, Width: 100, Height: 100})
		defer w.Destroy()
		defer other.Destroy()
		window, foreign := objc.ID(w.NativeHandle()), objc.ID(other.NativeHandle())
		app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		// A hidden window cannot become key. Inject ownership for this
		// bridge test while preserving native event/window filtering.
		ownsKeyboard := true
		var seen []popupKeyEvent
		cancel, err := installScopedNativeKeyMonitor(window, func() bool { return ownsKeyboard }, func(e popupKeyEvent) bool {
			seen = append(seen, e)
			return true
		})
		if err != nil {
			result = err
			return
		}
		defer cancel()
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			result = err
			return
		}
		var makeKeyEvent func(objc.ID, objc.SEL, uint64, cgPoint, uint64, float64, int64, objc.ID, objc.ID, objc.ID, bool, uint16) objc.ID
		purego.RegisterLibFunc(&makeKeyEvent, lib, "objc_msgSend")
		send := func(target objc.ID, kind, flags uint64) {
			chars := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "c")
			event := makeKeyEvent(objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
				kind, cgPoint{}, flags, 0, objc.Send[int64](target, objc.RegisterName("windowNumber")), 0, chars, chars, false, 0x08)
			app.Send(objc.RegisterName("sendEvent:"), event)
		}
		send(window, 10, 1<<17|1<<20)
		send(window, 12, 0)
		if len(seen) != 2 || seen[0].key != ui.KeyC || seen[0].mods != ui.Shift|ui.Cmd || !seen[1].flagsChanged || !seen[1].allReleased {
			result = fmt.Errorf("native event delivery differs: %+v; expected key=%d mods=%d", seen, ui.KeyC, ui.Shift|ui.Cmd)
			return
		}
		ownsKeyboard = false
		send(foreign, 10, 1<<17|1<<20)
		ownsKeyboard = true
		send(foreign, 10, 1<<17|1<<20)
		if len(seen) != 2 {
			result = fmt.Errorf("foreign window event reached popup monitor")
			return
		}
		cancel()
		cancel() // Removal is idempotent and releases the retained block.
		send(window, 10, 1<<17|1<<20)
		if len(seen) != 2 {
			result = fmt.Errorf("removed native monitor still received an event")
			return
		}
		fmt.Println("native keyDown + flagsChanged delivered; foreign window rejected; monitor removed; clipboard untouched")
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return result
}
