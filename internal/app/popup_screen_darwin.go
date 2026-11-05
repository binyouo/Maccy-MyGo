//go:build darwin

package app

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// NSScreen.main follows the key window, which can be on a different
// display from the mouse pointer (NSScreen+ForPopup.swift).
func activePopupScreenID() (int64, bool) {
	if !mygo.App.IsReady() {
		return 0, false
	}
	screen := objc.ID(objc.GetClass("NSScreen")).Send(objc.RegisterName("mainScreen"))
	if screen == 0 {
		return 0, false
	}
	key := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "NSScreenNumber")
	number := screen.Send(objc.RegisterName("deviceDescription")).Send(objc.RegisterName("objectForKey:"), key)
	return int64(objc.Send[uint32](number, objc.RegisterName("unsignedIntValue"))), number != 0
}
