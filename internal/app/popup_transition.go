package app

import (
	"time"

	"github.com/egoist/mygo"
)

func (a *App) popupMode() int {
	if a.editing != nil {
		return 2
	}
	if a.previewOpen {
		return 1
	}
	return 0
}

// Each host retains its anchor and velocity across repeated clicks.
type popupWindowMotion struct {
	placement     popupPlacement
	width, height popupSpring
	lastAt        time.Time
	lastApplied   mygo.Rectangle
	applying      bool
}

func (a *App) rememberPopupMotion(win *mygo.Window) *popupWindowMotion {
	if a.popupMotions == nil {
		a.popupMotions = make(map[*mygo.Window]*popupWindowMotion)
	}
	if m := a.popupMotions[win]; m != nil {
		return m
	}
	b := win.ContentBounds()
	wa := mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}).WorkArea
	if testArea, ok := a.developmentScreen(); ok {
		wa = testArea
	}
	cfg := a.popupConfig()
	base, preview := cfg.WindowWidth, cfg.PreviewWidth
	if base <= 0 {
		base = 450
	}
	if preview <= 0 {
		preview = 400
	}
	anchor := b
	anchor.Width = base
	m := &popupWindowMotion{placement: newPopupPlacement(anchor, wa, base+preview+1), lastAt: time.Now(), lastApplied: b}
	m.width.reset(float64(b.Width))
	m.height.reset(float64(b.Height))
	a.popupMotions[win] = m
	return m
}

func (a *App) popupLeft(win *mygo.Window) bool {
	if m := a.popupMotions[win]; m != nil {
		return m.placement.left
	}
	return false
}

func (a *App) popupMotionActive() bool {
	for win, m := range a.popupMotions {
		if win != nil && win.IsDestroyed() {
			continue
		}
		if !m.width.settled() || !m.height.settled() {
			return true
		}
	}
	return false
}

func (a *App) cancelPopupResize() {
	a.popupResizeGen++
	if a.popupResizeTimer != nil {
		a.popupResizeTimer.Stop()
		a.popupResizeTimer = nil
	}
}

func (a *App) resizePopup(w, h int, animate bool) {
	a.cancelPopupResize()
	gen := a.popupResizeGen
	now := time.Now()
	var hosts []*mygo.Window
	for _, win := range []*mygo.Window{a.panel.Load(), a.devWin} {
		if win == nil || win.IsDestroyed() {
			continue
		}
		m := a.rememberPopupMotion(win)
		// Changing the native style mask can synchronously resize its
		// content too; it belongs to this programmatic resize operation.
		m.applying = true
		if win.IsResizable() != (a.editing == nil) {
			win.SetResizable(a.editing == nil)
		}
		win.SetMinimumSize(popupMinimumPaneWidth, 120)
		m.applying = false
		moving := !m.width.settled() || !m.height.settled()
		m.width.advance(now.Sub(m.lastAt))
		m.height.advance(now.Sub(m.lastAt))
		m.lastAt = now
		if !moving && win.ContentBounds() != m.lastApplied {
			delete(a.popupMotions, win)
			m = a.rememberPopupMotion(win)
		}
		m.width.retarget(float64(w))
		m.height.retarget(float64(h))
		if a.reduceMotion || (!animate && !moving) {
			m.width.reset(float64(w))
			m.height.reset(float64(h))
		}
		hosts = append(hosts, win)
	}
	// Absolute deadlines avoid adding synchronous rendering cost to every tick.
	next := now
	var step func()
	step = func() {
		if a.popupResizeGen != gen {
			return
		}
		at := time.Now()
		moving := false
		for _, win := range hosts {
			if win.IsDestroyed() || (win != a.panel.Load() && win != a.devWin) {
				continue
			}
			m := a.popupMotions[win]
			if m == nil {
				continue
			}
			m.width.advance(at.Sub(m.lastAt))
			m.height.advance(at.Sub(m.lastAt))
			m.lastAt = at
			bounds := m.placement.bounds(m.width.position, m.height.position)
			if bounds != m.lastApplied {
				m.lastApplied = bounds
				m.applying = true
				win.SetContentBounds(bounds)
				m.applying = false
			}
			moving = moving || !m.width.settled() || !m.height.settled()
		}
		if moving {
			next = next.Add(time.Second / 120)
			if next.Before(at) {
				next = at.Add(time.Second / 120)
			}
			a.popupResizeTimer = time.AfterFunc(max(time.Until(next), time.Millisecond), func() { a.post(step) })
		} else {
			a.popupResizeTimer = nil
			for _, win := range hosts {
				if win.IsDestroyed() {
					continue
				}
				minimum := popupMinimumPaneWidth
				if a.previewOpen {
					minimum = 2*popupMinimumPaneWidth + 1
				}
				m := a.popupMotions[win]
				m.applying = true
				win.SetMinimumSize(minimum, 120)
				m.applying = false
				// The last spring steps may round to the same pixel. Rebuild
				// once at rest so the divider leaves its disabled state.
				win.Invalidate()
			}
			if a.previewQueued && a.editing == nil {
				a.applyQueuedPreview()
				a.refresh()
				return
			}
			if a.previewClosing {
				a.previewClosing = false
				a.refresh()
			}
		}
	}
	// Click callbacks run inside a content frame whose dimensions were
	// already captured. Resize only after that frame has been presented.
	a.post(step)
}
