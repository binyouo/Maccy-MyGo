//go:build windows

package clipboard

import "syscall"

var clipboardSequenceNumber = syscall.NewLazyDLL("user32.dll").NewProc("GetClipboardSequenceNumber")

// ChangeCount uses Windows' public clipboard sequence number. Zero means
// the caller cannot read the sequence, so the watcher falls back to a hash.
func ChangeCount() (int64, bool) {
	count, _, _ := clipboardSequenceNumber.Call()
	return int64(uint32(count)), count != 0
}
