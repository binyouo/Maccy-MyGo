package app

import (
	"reflect"
	"strings"
	"testing"
)

func TestRecordedShortcutValidation(t *testing.T) {
	a := &App{prefs: prefsDraft{
		GlobalShortcut:  "CmdOrCtrl+Shift+C",
		PinShortcut:     "Option+P",
		DeleteShortcut:  "Option+Delete",
		PreviewShortcut: "Ctrl+Space",
	}}
	before := a.prefs
	for _, tc := range []struct {
		name, field, shortcut, errorText string
	}{
		{"clear", "open", "", ""},
		{"plain letter", "open", "K", "修饰键"},
		{"shift-only letter", "open", "Shift+K", "修饰键"},
		{"plain function key", "open", "F1", ""},
		{"shift function key", "open", "Shift+F12", ""},
		{"modified escape", "open", "Control+Escape", ""},
		{"modified tab", "open", "Option+Tab", ""},
		{"option allowed", "pin", "Option+J", ""},
		{"invalid key", "open", "Cmd+Unrecognized", "无法识别"},
		{"invalid modifier", "open", "Something+K", "无法识别"},
		{"duplicate pin alias", "open", "Alt+p", "置顶"},
		{"duplicate delete alias", "preview", "Opt+Backspace", "删除"},
		{"duplicate preview", "delete", "Control+Space", "预览"},
		{"unchanged shortcut alias", "pin", "Alt+p", ""},
		{"unknown field", "unknown", "Cmd+J", "无法识别"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := a.validateRecordedShortcut(tc.field, tc.shortcut)
			if tc.errorText == "" && got != "" {
				t.Fatalf("rejected valid shortcut %q: %s", tc.shortcut, got)
			}
			if tc.errorText != "" && !strings.Contains(got, tc.errorText) {
				t.Fatalf("validation = %q, want reason containing %q", got, tc.errorText)
			}
		})
	}
	if !reflect.DeepEqual(a.prefs, before) {
		t.Fatal("validation changed the settings draft")
	}
}
