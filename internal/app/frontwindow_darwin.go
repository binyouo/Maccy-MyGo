//go:build darwin

package app

import (
	"errors"
	"log"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

// frontmostWindowRect returns the bounds of the frontmost application's
// focused window, for the "window" popup position — Maccy's
// NSWorkspace.frontmostApplication.windowFrame, through the same
// accessibility API. Without Accessibility permission it reports no
// window and the caller falls back to the cursor.
func frontmostWindowRect() (mygo.Rectangle, bool) {
	if err := loadWindowSyms(); err != nil {
		log.Printf("maccy: frontmost window: %v", err)
		return mygo.Rectangle{}, false
	}
	pid := frontmostPID()
	if pid == 0 {
		log.Printf("maccy: frontmost window: no frontmost app")
		return mygo.Rectangle{}, false
	}
	app := axUIElementCreateApplication(pid)
	if app == 0 {
		log.Printf("maccy: frontmost window: no AX app for pid %d", pid)
		return mygo.Rectangle{}, false
	}
	defer cfRelease(app)
	win := axAttrRef(app, "AXFocusedWindow")
	if win == 0 {
		// No focused window: the first of the app's windows, as
		// NSApplication.windowFrame does.
		if list := axAttrRef(app, "AXWindows"); list != 0 {
			defer cfRelease(list)
			if cfArrayGetCount(list) > 0 {
				win = cfArrayGetValueAtIndex(list, 0)
				cfRetain(win)
			}
		}
	}
	if win == 0 {
		log.Printf("maccy: frontmost window: pid %d has no window", pid)
		return mygo.Rectangle{}, false
	}
	defer cfRelease(win)
	var pos cgPoint
	var size cgSize
	if v := axAttrRef(win, "AXPosition"); v != 0 {
		axValueGetValue(v, axValueTypeCGPoint, unsafe.Pointer(&pos))
		cfRelease(v)
	}
	if v := axAttrRef(win, "AXSize"); v != 0 {
		axValueGetValue(v, axValueTypeCGSize, unsafe.Pointer(&size))
		cfRelease(v)
	}
	if size.W <= 0 || size.H <= 0 {
		log.Printf("maccy: frontmost window: pid %d window has no frame", pid)
		return mygo.Rectangle{}, false
	}
	return mygo.Rectangle{X: int(pos.X), Y: int(pos.Y), Width: int(size.W), Height: int(size.H)}, true
}

// frontmostPID is NSWorkspace.frontmostApplication.processIdentifier.
func frontmostPID() int32 {
	ws := objcMsgSend(objcGetClass(cstr("NSWorkspace")), sel(cstr("sharedWorkspace")))
	if ws == 0 {
		return 0
	}
	app := objcMsgSend(ws, sel(cstr("frontmostApplication")))
	if app == 0 {
		return 0
	}
	return objcMsgSendInt(app, sel(cstr("processIdentifier")))
}

// axAttrRef reads a CFTypeRef-valued attribute; the caller releases it.
func axAttrRef(el uintptr, attr string) uintptr {
	name := cfString(attr)
	defer cfRelease(name)
	var v uintptr
	if axUIElementCopyAttributeValue(el, name, &v) != 0 {
		return 0
	}
	return v
}

// cfString makes a CFString from a Go string.
func cfString(s string) uintptr {
	return cfStringCreateWithCString(0, cstr(s), kCFStringEncodingUTF8)
}

// cstr makes a NUL-terminated byte slice for the ObjC/CF entry points.
func cstr(s string) *byte {
	bs := append([]byte(s), 0)
	return &bs[0]
}

const (
	kCFStringEncodingUTF8 = 0x08000100
	axValueTypeCGPoint    = 1
	axValueTypeCGSize     = 2
)

var errWindowSyms = errors.New("app: accessibility symbols unavailable")

type cgPoint struct{ X, Y float64 }
type cgSize struct{ W, H float64 }

var (
	symsOnce sync.Once
	symsErr  error

	objcGetClass                  func(*byte) uintptr
	sel                           func(*byte) uintptr
	objcMsgSend                   func(uintptr, uintptr) uintptr
	objcMsgSendInt                func(uintptr, uintptr) int32
	cfRetain                      func(uintptr) uintptr
	cfRelease                     func(uintptr)
	cfArrayGetCount               func(uintptr) int64
	cfArrayGetValueAtIndex        func(uintptr, int64) uintptr
	cfStringCreateWithCString     func(uintptr, *byte, uint32) uintptr
	axUIElementCreateApplication  func(int32) uintptr
	axUIElementCopyAttributeValue func(uintptr, uintptr, *uintptr) int32
	axValueGetValue               func(uintptr, int, unsafe.Pointer) bool
)

func loadWindowSyms() error {
	symsOnce.Do(func() {
		reg := func(fn any, path, name string) bool {
			lib, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
			if err != nil {
				return false
			}
			addr, err := purego.Dlsym(lib, name)
			if err != nil {
				return false
			}
			purego.RegisterFunc(fn, addr)
			return true
		}
		objc := "/usr/lib/libobjc.A.dylib"
		cf := "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
		ax := "/System/Library/Frameworks/ApplicationServices.framework/Frameworks/HIServices.framework/HIServices"
		// AppKit must be loaded for NSWorkspace's class to resolve.
		if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			symsErr = err
			return
		}
		// objc_msgSend is variadic: the two views differ only by result.
		regStr := func(fn any, name string) bool { return reg(fn, objc, name) }
		ok := regStr(&objcGetClass, "objc_getClass") &&
			regStr(&sel, "sel_registerName") &&
			regStr(&objcMsgSend, "objc_msgSend") &&
			regStr(&objcMsgSendInt, "objc_msgSend") &&
			reg(&cfRetain, cf, "CFRetain") &&
			reg(&cfRelease, cf, "CFRelease") &&
			reg(&cfArrayGetCount, cf, "CFArrayGetCount") &&
			reg(&cfArrayGetValueAtIndex, cf, "CFArrayGetValueAtIndex") &&
			reg(&cfStringCreateWithCString, cf, "CFStringCreateWithCString") &&
			reg(&axUIElementCreateApplication, ax, "AXUIElementCreateApplication") &&
			reg(&axUIElementCopyAttributeValue, ax, "AXUIElementCopyAttributeValue") &&
			reg(&axValueGetValue, ax, "AXValueGetValue")
		if !ok {
			symsErr = errWindowSyms
		}
	})
	return symsErr
}
