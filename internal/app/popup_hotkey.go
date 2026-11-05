package app

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The first release distinguishes an ordinary toggle from holding the
// shortcut while repeatedly pressing its main key (Popup.swift).
type popupHotkeyPhase uint8

const (
	popupHotkeyOpening popupHotkeyPhase = iota
	popupHotkeyToggle
	popupHotkeyCycle
	popupHotkeyFinishing
)

type popupHotkeyAction uint8

const (
	popupHotkeyPass popupHotkeyAction = iota
	popupHotkeyNext
	popupHotkeyClose
	popupHotkeyAccept
	popupHotkeyItem
)

func (p *popupHotkeyPhase) key(mainKey, exactModifiers, itemShortcut bool) popupHotkeyAction {
	if !mainKey || *p == popupHotkeyFinishing {
		return popupHotkeyPass
	}
	if itemShortcut {
		*p = popupHotkeyFinishing
		return popupHotkeyItem
	}
	if *p == popupHotkeyOpening {
		*p = popupHotkeyCycle
	}
	if *p == popupHotkeyCycle {
		return popupHotkeyNext
	}
	if exactModifiers {
		*p = popupHotkeyFinishing
		return popupHotkeyClose
	}
	return popupHotkeyPass
}

func (p *popupHotkeyPhase) flags(allReleased bool) popupHotkeyAction {
	if !allReleased {
		return popupHotkeyPass
	}
	switch *p {
	case popupHotkeyOpening:
		*p = popupHotkeyToggle
	case popupHotkeyCycle:
		*p = popupHotkeyFinishing
		return popupHotkeyAccept
	}
	return popupHotkeyPass
}

type popupKeyEvent struct {
	key          ui.Key
	mods         ui.Modifiers
	flagsChanged bool
	allReleased  bool
	capsLock     bool
	extraMods    bool
}

type popupHotkeySession struct {
	window *mygo.Window
	phase  popupHotkeyPhase
	acc    string
	key    ui.Key
	mods   ui.Modifiers
	cancel func()
}

func (a *App) handlePopupShortcut() {
	if a.editing != nil {
		return
	}
	if a.panel.Load() != nil {
		a.closePopup()
		return
	}
	a.openPopup()
	a.startPopupHotkeySession()
}

func (a *App) startPopupHotkeySession() {
	p := a.panel.Load()
	mods, key, ok := parseShortcut(a.currentShortcut)
	if p == nil || !ok || a.currentShortcut == "" {
		return
	}
	s := &popupHotkeySession{window: p, acc: a.currentShortcut, key: key, mods: mods}
	cancel, err := installPopupKeyMonitor(p, func(e popupKeyEvent) bool {
		if !a.popupHotkeyIsCurrent(s) {
			return false
		}
		var action popupHotkeyAction
		if e.flagsChanged {
			action = s.phase.flags(e.allReleased)
		} else {
			action = s.phase.key(e.key == s.key, e.mods == s.mods && !e.capsLock && !e.extraMods,
				!e.extraMods && a.popupHotkeyItem(e.key, e.mods) >= 0)
		}
		switch action {
		case popupHotkeyNext:
			a.cyclePopupSelection()
		case popupHotkeyClose, popupHotkeyAccept, popupHotkeyItem:
			// Leave the NSEvent callback before closing its window and
			// disposing the monitor's ObjC block. Reject stale queued work.
			p.Update(func() {
				if !a.popupHotkeyIsCurrent(s) {
					return
				}
				switch action {
				case popupHotkeyClose:
					a.closePopup()
				case popupHotkeyAccept:
					a.activateSelected(0)
				case popupHotkeyItem:
					if index := a.popupHotkeyItem(e.key, e.mods); index >= 0 {
						a.mu.Lock()
						a.selected, a.footerIndex = index, -1
						a.mu.Unlock()
						a.activateSelected(e.mods)
					}
				}
			})
		}
		return action != popupHotkeyPass
	})
	if err != nil {
		// Platforms without local flagsChanged events keep the ordinary
		// registered toggle shortcut.
		if err != errPopupKeyMonitorUnsupported {
			log.Printf("maccy: popup shortcut monitor: %v", err)
		}
		return
	}
	s.cancel = cancel
	a.popupHotkey = s
	// Carbon captures the chord before local keyDown delivery. Suspend it
	// only after the local monitor has successfully taken ownership.
	mygo.GlobalShortcut.Unregister(s.acc)
}

func (a *App) popupHotkeyIsCurrent(s *popupHotkeySession) bool {
	return a.popupHotkey == s && a.panel.Load() == s.window && a.editing == nil
}

func (a *App) stopPopupHotkeySession(restore bool) {
	s := a.popupHotkey
	if s == nil {
		return
	}
	a.popupHotkey = nil
	s.cancel()
	if restore && a.currentShortcut == s.acc {
		a.applyShortcut(s.acc)
	}
}

func (a *App) stopPopupHotkeySessionFor(window *mygo.Window, restore bool) {
	if a.popupHotkey != nil && a.popupHotkey.window == window {
		a.stopPopupHotkeySession(restore)
	}
}

// Return the visible shortcut target before cycling. A configured popup
// chord can share its main key with an item's numeric/pin shortcut.
func (a *App) popupHotkeyItem(key ui.Key, mods ui.Modifiers) int {
	if itemAction(a.cfg.Snapshot(), mods) == actUnknown {
		return -1
	}
	ch, ok := pinCharForKey(key)
	if key >= ui.Key1 && key <= ui.Key9 {
		ch, ok = keyName(key), true
	}
	if !ok {
		return -1
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, it := range a.vitems {
		if strings.TrimPrefix(a.shortcuts[it.ID], "⌘") == ch {
			return i
		}
	}
	return -1
}

func (a *App) cyclePopupSelection() {
	a.mu.Lock()
	// Maccy includes visible footer actions in its navigation cycle. With
	// no footer, wrap the last history item to the first one.
	wrap := !a.cfg.Snapshot().ShowFooter && len(a.vitems) > 0 && a.selected == len(a.vitems)-1
	if wrap {
		a.selected, a.footerIndex = 0, -1
		a.scrollToSelected, a.kbdNav = true, true
	}
	a.mu.Unlock()
	if !wrap {
		a.moveSelection(1)
		return
	}
	a.scheduleAutoPreview()
	a.refresh()
}
