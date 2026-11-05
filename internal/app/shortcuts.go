package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// parseShortcut parses a shortcut like "Option+P", "Ctrl+Space" or
// "Cmd+Backspace" into modifiers and a key.
func parseShortcut(s string) (ui.Modifiers, ui.Key, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	var mods ui.Modifiers
	parts := strings.Split(s, "+")
	keyName := strings.TrimSpace(parts[len(parts)-1])
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmd", "command", "super", "meta":
			mods |= ui.Super
		case "cmdorctrl":
			mods |= ui.Cmd
		case "ctrl", "control":
			mods |= ui.Ctrl
		case "option", "opt", "alt":
			mods |= ui.Alt
		case "shift":
			mods |= ui.Shift
		default:
			return 0, 0, false
		}
	}
	key, ok := keyFromName(keyName)
	if !ok {
		return 0, 0, false
	}
	return mods, key, true
}

func keyFromName(name string) (ui.Key, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if len(n) == 1 {
		c := n[0]
		switch {
		case c >= 'a' && c <= 'z':
			return ui.KeyA + ui.Key(c-'a'), true
		case c >= '0' && c <= '9':
			return ui.Key0 + ui.Key(c-'0'), true
		}
	}
	switch n {
	case "return", "enter":
		return ui.KeyEnter, true
	case "escape", "esc":
		return ui.KeyEscape, true
	case "delete", "backspace":
		return ui.KeyBackspace, true
	case "forwarddelete":
		return ui.KeyDelete, true
	case "tab":
		return ui.KeyTab, true
	case "space":
		return ui.KeySpace, true
	case "up":
		return ui.KeyUp, true
	case "down":
		return ui.KeyDown, true
	case "left":
		return ui.KeyLeft, true
	case "right":
		return ui.KeyRight, true
	case "home":
		return ui.KeyHome, true
	case "end":
		return ui.KeyEnd, true
	case "pageup":
		return ui.KeyPageUp, true
	case "pagedown":
		return ui.KeyPageDown, true
	case "comma", ",":
		return ui.KeyComma, true
	case "period", ".":
		return ui.KeyPeriod, true
	case "slash", "/":
		return ui.KeySlash, true
	case "minus", "-":
		return ui.KeyMinus, true
	case "equal", "=":
		return ui.KeyEqual, true
	case "bracketleft", "[":
		return ui.KeyBracketLeft, true
	case "bracketright", "]":
		return ui.KeyBracketRight, true
	case "semicolon", ";":
		return ui.KeySemicolon, true
	case "quote", "'":
		return ui.KeyQuote, true
	case "backslash", "\\":
		return ui.KeyBackslash, true
	case "f1":
		return ui.KeyF1, true
	case "f2":
		return ui.KeyF2, true
	case "f3":
		return ui.KeyF3, true
	case "f4":
		return ui.KeyF4, true
	case "f5":
		return ui.KeyF5, true
	case "f6":
		return ui.KeyF6, true
	case "f7":
		return ui.KeyF7, true
	case "f8":
		return ui.KeyF8, true
	case "f9":
		return ui.KeyF9, true
	case "f10":
		return ui.KeyF10, true
	case "f11":
		return ui.KeyF11, true
	case "f12":
		return ui.KeyF12, true
	}
	return 0, false
}

// keyName returns the configuration name of a key, the inverse of
// keyFromName.
func keyName(k ui.Key) string {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return string(rune('A' + (k - ui.KeyA)))
	case k >= ui.Key0 && k <= ui.Key9:
		return string(rune('0' + (k - ui.Key0)))
	}
	switch k {
	case ui.KeyEnter:
		return "Return"
	case ui.KeyEscape:
		return "Escape"
	case ui.KeyBackspace:
		return "Delete"
	case ui.KeyDelete:
		return "ForwardDelete"
	case ui.KeyTab:
		return "Tab"
	case ui.KeySpace:
		return "Space"
	case ui.KeyUp:
		return "Up"
	case ui.KeyDown:
		return "Down"
	case ui.KeyLeft:
		return "Left"
	case ui.KeyRight:
		return "Right"
	case ui.KeyHome:
		return "Home"
	case ui.KeyEnd:
		return "End"
	case ui.KeyPageUp:
		return "PageUp"
	case ui.KeyPageDown:
		return "PageDown"
	case ui.KeyComma:
		return "Comma"
	case ui.KeyPeriod:
		return "Period"
	case ui.KeySlash:
		return "Slash"
	case ui.KeyMinus:
		return "Minus"
	case ui.KeyEqual:
		return "Equal"
	case ui.KeyBracketLeft:
		return "BracketLeft"
	case ui.KeyBracketRight:
		return "BracketRight"
	case ui.KeySemicolon:
		return "Semicolon"
	case ui.KeyQuote:
		return "Quote"
	case ui.KeyBackslash:
		return "Backslash"
	case ui.KeyF1:
		return "F1"
	case ui.KeyF2:
		return "F2"
	case ui.KeyF3:
		return "F3"
	case ui.KeyF4:
		return "F4"
	case ui.KeyF5:
		return "F5"
	case ui.KeyF6:
		return "F6"
	case ui.KeyF7:
		return "F7"
	case ui.KeyF8:
		return "F8"
	case ui.KeyF9:
		return "F9"
	case ui.KeyF10:
		return "F10"
	case ui.KeyF11:
		return "F11"
	case ui.KeyF12:
		return "F12"
	}
	return ""
}
