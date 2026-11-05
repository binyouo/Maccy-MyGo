package app

import (
	"testing"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestPopupHotkeyToggleAndCycle(t *testing.T) {
	phase := popupHotkeyOpening
	if got := phase.flags(false); got != popupHotkeyPass || phase != popupHotkeyOpening {
		t.Fatalf("partial release: phase=%v action=%v", phase, got)
	}
	if got := phase.key(false, true, false); got != popupHotkeyPass || phase != popupHotkeyOpening {
		t.Fatalf("other main key: phase=%v action=%v", phase, got)
	}
	if got := phase.flags(true); got != popupHotkeyPass || phase != popupHotkeyToggle {
		t.Fatalf("initial release: phase=%v action=%v", phase, got)
	}
	if got := phase.key(true, false, false); got != popupHotkeyPass {
		t.Fatalf("typing main key with different modifiers=%v", got)
	}
	if got := phase.key(true, true, false); got != popupHotkeyClose {
		t.Fatalf("toggle chord=%v", got)
	}

	phase = popupHotkeyOpening
	for i := 0; i < 3; i++ {
		if got := phase.key(true, true, false); got != popupHotkeyNext || phase != popupHotkeyCycle {
			t.Fatalf("cycle press %d: phase=%v action=%v", i, phase, got)
		}
	}
	if got := phase.flags(false); got != popupHotkeyPass || phase != popupHotkeyCycle {
		t.Fatalf("cycle partial release: phase=%v action=%v", phase, got)
	}
	if got := phase.flags(true); got != popupHotkeyAccept {
		t.Fatalf("cycle release action=%v", got)
	}
	if got := phase.flags(true); got != popupHotkeyPass {
		t.Fatalf("queued duplicate acceptance=%v", got)
	}
}

func TestPopupHotkeyItemPrecedesCycle(t *testing.T) {
	for _, initial := range []popupHotkeyPhase{popupHotkeyOpening, popupHotkeyToggle, popupHotkeyCycle} {
		phase := initial
		if got := phase.key(true, false, true); got != popupHotkeyItem || phase != popupHotkeyFinishing {
			t.Fatalf("initial=%v phase=%v action=%v", initial, phase, got)
		}
	}
	a := popupTestApp(t)
	if got := a.popupHotkeyItem(ui.Key1, ui.Cmd); got != 0 {
		t.Fatalf("first numbered item=%d", got)
	}
	a.togglePin(a.vitems[0])
	ch := a.vitems[0].Pin
	key, ok := keyFromName(ch)
	if !ok || a.popupHotkeyItem(key, ui.Alt) != 0 {
		t.Fatalf("pinned shortcut %q did not win", ch)
	}
	if got := a.popupHotkeyItem(ui.Key1, ui.Ctrl|ui.Super); got != -1 {
		t.Fatalf("unsupported modifiers matched=%d", got)
	}
}

func TestPopupHotkeyCycleWrapsWithoutFooter(t *testing.T) {
	a := popupTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.ShowFooter = false })
	a.selected = len(a.vitems) - 1
	a.cyclePopupSelection()
	if a.selected != 0 || a.footerIndex != -1 || !a.kbdNav || !a.scrollToSelected {
		t.Fatalf("cycle did not wrap: selected=%d footer=%d", a.selected, a.footerIndex)
	}
	_ = a.cfg.Update(func(c *config.Config) { c.ShowFooter = true })
	a.selected = len(a.vitems) - 1
	a.cyclePopupSelection()
	if a.selected != -1 || a.footerIndex != 0 {
		t.Fatalf("cycle skipped visible footer: selected=%d footer=%d", a.selected, a.footerIndex)
	}
}

func TestPopupHotkeyRejectsStaleWindowAndEditing(t *testing.T) {
	a := popupTestApp(t)
	window := &mygo.Window{}
	s := &popupHotkeySession{window: window}
	a.panel.Store(window)
	a.popupHotkey = s
	if !a.popupHotkeyIsCurrent(s) {
		t.Fatal("current session rejected")
	}
	a.editing = a.vitems[0]
	if a.popupHotkeyIsCurrent(s) {
		t.Fatal("editing accepted a shortcut event")
	}
	// The global shortcut must likewise leave an active editor alone.
	a.handlePopupShortcut()
	if a.panel.Load() != window || a.editing == nil {
		t.Fatal("global shortcut changed editor state")
	}
	a.editing = nil
	a.panel.Store(&mygo.Window{})
	if a.popupHotkeyIsCurrent(s) {
		t.Fatal("old window session accepted")
	}
	a.panel.Store(window)
	a.popupHotkey = &popupHotkeySession{window: window}
	if a.popupHotkeyIsCurrent(s) {
		t.Fatal("replaced session accepted")
	}
	canceled := false
	a.popupHotkey.cancel = func() { canceled = true }
	a.stopPopupHotkeySessionFor(&mygo.Window{}, false)
	if canceled || a.popupHotkey == nil {
		t.Fatal("old window close canceled the new popup monitor")
	}
	a.stopPopupHotkeySessionFor(window, false)
	if !canceled || a.popupHotkey != nil {
		t.Fatal("matching window close did not cancel its monitor")
	}
}
