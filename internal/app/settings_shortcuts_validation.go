package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// validateRecordedShortcut checks the recorder's candidate without changing
// the draft or registering a global shortcut. Escape/Tab/Delete without a
// modifier are recorder controls and must be handled before calling this.
func (a *App) validateRecordedShortcut(field, name string) string {
	bindings := []struct {
		field, label, value string
	}{
		{"open", "打开", a.prefs.GlobalShortcut},
		{"pin", "置顶", a.prefs.PinShortcut},
		{"delete", "删除", a.prefs.DeleteShortcut},
		{"preview", "预览", a.prefs.PreviewShortcut},
	}
	current := ""
	found := false
	for _, binding := range bindings {
		if binding.field == field {
			current, found = binding.value, true
			break
		}
	}
	if !found {
		return "无法识别的快捷键操作。"
	}
	if strings.TrimSpace(name) == "" {
		return ""
	}
	mods, key, ok := parseShortcut(name)
	if !ok {
		return "无法识别此快捷键，请重新录制。"
	}
	// KeyboardShortcuts 3.1.0's recorder permits function keys on their
	// own, but ordinary keys need Command, Control or Option, not only Shift.
	if mods&(ui.Super|ui.Ctrl|ui.Alt) == 0 && (key < ui.KeyF1 || key > ui.KeyF12) {
		return "请同时按下 Command、Control 或 Option 修饰键。"
	}
	if oldMods, oldKey, valid := parseShortcut(current); valid && oldMods == mods && oldKey == key {
		return ""
	}
	for _, binding := range bindings {
		if binding.field == field {
			continue
		}
		otherMods, otherKey, valid := parseShortcut(binding.value)
		if valid && otherMods == mods && otherKey == key {
			return fmt.Sprintf("此快捷键已用于“%s”，请选择其他组合。", binding.label)
		}
	}
	return ""
}
