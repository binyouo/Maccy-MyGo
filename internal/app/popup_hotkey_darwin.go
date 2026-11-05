//go:build darwin

package app

import (
	"errors"
	"fmt"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var errPopupKeyMonitorUnsupported = errors.New("local modifier events are unavailable")

const nativeIndependentModifierMask uint64 = 0xffff0000

func nativeKeyboardModifiers(flags uint64) ui.Modifiers {
	var mods ui.Modifiers
	for _, pair := range []struct {
		flag uint64
		mod  ui.Modifiers
	}{{1 << 17, ui.Shift}, {1 << 18, ui.Ctrl}, {1 << 19, ui.Alt}, {1 << 20, ui.Super}} {
		if flags&pair.flag != 0 {
			mods |= pair.mod
		}
	}
	return mods
}

func currentKeyboardModifiers() ui.Modifiers {
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	event := app.Send(objc.RegisterName("currentEvent"))
	flags := objc.Send[uint64](event, objc.RegisterName("modifierFlags"))
	flags |= objc.Send[uint64](objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("modifierFlags"))
	return nativeKeyboardModifiers(flags)
}

func installPopupKeyMonitor(p *mygo.Window, handle func(popupKeyEvent) bool) (func(), error) {
	if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return nil, err
	}
	window := objc.ID(p.NativeHandle())
	if window == 0 {
		return nil, fmt.Errorf("popup has no native window")
	}
	return installNativePopupKeyMonitor(window, handle)
}

func installNativePopupKeyMonitor(window objc.ID, handle func(popupKeyEvent) bool) (func(), error) {
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	return installScopedNativeKeyMonitor(window, func() bool {
		return app.Send(objc.RegisterName("keyWindow")) == window
	}, handle)
}

func installScopedNativeKeyMonitor(window objc.ID, ownsKeyboard func() bool, handle func(popupKeyEvent) bool) (func(), error) {
	eventClass := objc.ID(objc.GetClass("NSEvent"))
	active := true
	block := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		if !active || !ownsKeyboard() {
			return event
		}
		// flagsChanged can have no window, but must never be accepted while
		// preferences or another popup owns keyboard focus.
		if eventWindow := event.Send(objc.RegisterName("window")); eventWindow != 0 && eventWindow != window {
			return event
		}
		kind := objc.Send[uint64](event, objc.RegisterName("type"))
		flags := objc.Send[uint64](event, objc.RegisterName("modifierFlags"))
		e := popupKeyEvent{
			mods:         nativeKeyboardModifiers(flags),
			flagsChanged: kind == 12,
			allReleased:  flags&nativeIndependentModifierMask == 0,
			capsLock:     flags&(1<<16) != 0,
			extraMods:    flags&nativeIndependentModifierMask&^uint64(0x1f0000) != 0,
		}
		if kind == 10 {
			code := objc.Send[uint16](event, objc.RegisterName("keyCode"))
			e.key = nativePopupKey(code)
		}
		if handle(e) {
			return 0
		}
		return event
	})
	monitor := eventClass.Send(objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:"), uint64(1<<10|1<<12), block)
	block.Release() // NSEvent retains its own copied block until removal.
	if monitor == 0 {
		return nil, fmt.Errorf("NSEvent refused the popup key monitor")
	}
	return func() {
		if active {
			active = false
			eventClass.Send(objc.RegisterName("removeMonitor:"), monitor)
		}
	}, nil
}

// Use the physical ANSI codes used by MyGo's Carbon global shortcut
// registration, so monitoring follows the same key on every layout.
func nativePopupKey(code uint16) ui.Key {
	keys := map[uint16]ui.Key{
		0x00: ui.KeyA, 0x01: ui.KeyS, 0x02: ui.KeyD, 0x03: ui.KeyF, 0x04: ui.KeyH, 0x05: ui.KeyG,
		0x06: ui.KeyZ, 0x07: ui.KeyX, 0x08: ui.KeyC, 0x09: ui.KeyV, 0x0b: ui.KeyB, 0x0c: ui.KeyQ,
		0x0d: ui.KeyW, 0x0e: ui.KeyE, 0x0f: ui.KeyR, 0x10: ui.KeyY, 0x11: ui.KeyT,
		0x12: ui.Key1, 0x13: ui.Key2, 0x14: ui.Key3, 0x15: ui.Key4, 0x16: ui.Key6, 0x17: ui.Key5,
		0x18: ui.KeyEqual, 0x19: ui.Key9, 0x1a: ui.Key7, 0x1b: ui.KeyMinus, 0x1c: ui.Key8, 0x1d: ui.Key0,
		0x1e: ui.KeyBracketRight, 0x1f: ui.KeyO, 0x20: ui.KeyU, 0x21: ui.KeyBracketLeft, 0x22: ui.KeyI,
		0x23: ui.KeyP, 0x24: ui.KeyEnter, 0x25: ui.KeyL, 0x26: ui.KeyJ, 0x27: ui.KeyQuote, 0x28: ui.KeyK,
		0x29: ui.KeySemicolon, 0x2a: ui.KeyBackslash, 0x2b: ui.KeyComma, 0x2c: ui.KeySlash, 0x2d: ui.KeyN,
		0x2e: ui.KeyM, 0x2f: ui.KeyPeriod, 0x30: ui.KeyTab, 0x31: ui.KeySpace, 0x33: ui.KeyBackspace,
		0x35: ui.KeyEscape, 0x75: ui.KeyDelete, 0x73: ui.KeyHome, 0x77: ui.KeyEnd, 0x74: ui.KeyPageUp,
		0x79: ui.KeyPageDown, 0x7b: ui.KeyLeft, 0x7c: ui.KeyRight, 0x7d: ui.KeyDown, 0x7e: ui.KeyUp,
		0x7a: ui.KeyF1, 0x78: ui.KeyF2, 0x63: ui.KeyF3, 0x76: ui.KeyF4, 0x60: ui.KeyF5, 0x61: ui.KeyF6,
		0x62: ui.KeyF7, 0x64: ui.KeyF8, 0x65: ui.KeyF9, 0x6d: ui.KeyF10, 0x67: ui.KeyF11, 0x6f: ui.KeyF12,
	}
	return keys[code]
}
