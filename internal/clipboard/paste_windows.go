//go:build windows

package clipboard

import (
	"syscall"
	"unsafe"
)

const (
	vkControl     = 0x11
	vkV           = 0x56
	keyUp         = 0x0002
	inputKeyboard = 1
)

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type input struct {
	typ uint32
	_   uint32
	ki  keybdInput
	pad [8]byte
}

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

// Paste types Ctrl+V into the focused window using SendInput.
func Paste() error {
	inputs := []input{
		{typ: inputKeyboard, ki: keybdInput{wVk: vkControl}},
		{typ: inputKeyboard, ki: keybdInput{wVk: vkV}},
		{typ: inputKeyboard, ki: keybdInput{wVk: vkV, dwFlags: keyUp}},
		{typ: inputKeyboard, ki: keybdInput{wVk: vkControl, dwFlags: keyUp}},
	}
	n, _, err := procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if n == 0 {
		return err
	}
	return nil
}
