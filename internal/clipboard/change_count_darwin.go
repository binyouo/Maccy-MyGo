//go:build darwin

package clipboard

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

var (
	generalPasteboardSelector = objc.RegisterName("generalPasteboard")
	pasteboardChangeSelector  = objc.RegisterName("changeCount")
)

// ChangeCount reads NSPasteboard's public mutation counter. Unlike a hash
// it distinguishes copying identical bytes twice, as Maccy's watcher does.
func ChangeCount() (count int64, available bool) {
	if !mygo.App.IsReady() {
		return 0, false
	}
	mygo.RunOnMain(func() {
		count, available = changeCountOnMain()
	})
	return count, available
}

// The native polling timer already runs on AppKit's main thread.
func changeCountOnMain() (int64, bool) {
	class := objc.GetClass("NSPasteboard")
	if class == 0 {
		return 0, false
	}
	board := objc.ID(class).Send(generalPasteboardSelector)
	if board == 0 {
		return 0, false
	}
	count := objc.Send[int64](board, pasteboardChangeSelector)
	return count, count >= 0
}
