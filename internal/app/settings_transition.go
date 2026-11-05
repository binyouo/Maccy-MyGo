package app

import (
	"math"
	"time"

	"github.com/egoist/mygo"
)

// Native bounds use anchor.X - width/2 with integer division. Center the
// toolbar on that same rounded coordinate: an odd host width otherwise
// shifts every label by half a DIP, reversing on the next even width.
func settingsToolbarRightInset(width float32) float32 {
	return 8 + width - 2*float32(math.Floor(float64(width)/2))
}

// Each settings host has its own clock and velocity. Changing categories
// retargets the existing motion instead of restarting a resize curve.
type settingsWindowMotion struct {
	width, height popupSpring
	opacity       popupSpring
	lastAt        time.Time
}

func newSettingsWindowMotion(bounds mygo.Rectangle, at time.Time) *settingsWindowMotion {
	m := &settingsWindowMotion{lastAt: at}
	m.width.reset(float64(bounds.Width))
	m.height.reset(float64(bounds.Height))
	m.opacity.reset(1)
	return m
}

func (m *settingsWindowMotion) advance(at time.Time) {
	dt := at.Sub(m.lastAt)
	m.width.advance(dt)
	m.height.advance(dt)
	m.opacity.advance(dt)
	m.lastAt = at
}

func (m *settingsWindowMotion) retarget(width, height int, at time.Time, reduceMotion bool) {
	m.advance(at)
	m.width.retarget(float64(width))
	m.height.retarget(float64(height))
	// Native host resizing suppresses toolkit Enter animations. Drive the
	// incoming pane's opacity explicitly on the same clock as the host.
	m.opacity.reset(0)
	m.opacity.retarget(1)
	if reduceMotion {
		m.snap()
	}
}

func (m *settingsWindowMotion) snap() {
	m.width.reset(m.width.target)
	m.height.reset(m.height.target)
	m.opacity.reset(1)
}

func (m *settingsWindowMotion) moving() bool {
	return !m.width.settled() || !m.height.settled() || !m.opacity.settled()
}

func (m *settingsWindowMotion) bounds(anchor mygo.Point, area mygo.Rectangle) mygo.Rectangle {
	return settingsBoundsAtAnchor(anchor, int(math.Round(m.width.position)), int(math.Round(m.height.position)), area)
}

func (a *App) settingsPaneOpacity() float32 {
	if a.prefsMotion == nil || a.prefsReduceMotion {
		return 1
	}
	return float32(max(0, min(1, a.prefsMotion.opacity.position)))
}

func (a *App) cancelSettingsTransition() {
	a.prefsResizeGen++
	if a.prefsResizeTimer != nil {
		a.prefsResizeTimer.Stop()
		a.prefsResizeTimer = nil
	}
}

func (a *App) animatePreferences(width, height int, area mygo.Rectangle, initial bool) {
	a.cancelSettingsTransition()
	win := a.settingsWin
	if win == nil || win.IsDestroyed() {
		return
	}
	now := time.Now()
	if a.prefsMotion == nil {
		a.prefsMotion = newSettingsWindowMotion(win.ContentBounds(), now)
	}
	a.prefsMotion.retarget(width, height, now, a.prefsReduceMotion || initial)
	gen := a.prefsResizeGen
	next := now
	var step func()
	step = func() {
		if gen != a.prefsResizeGen || win != a.settingsWin || win.IsDestroyed() {
			return
		}
		at := time.Now()
		m := a.prefsMotion
		if m == nil {
			return
		}
		if a.prefsReduceMotion {
			m.snap()
		} else {
			m.advance(at)
		}
		bounds := m.bounds(a.prefsWindowAnchor, area)
		if bounds != a.prefsWindowBounds {
			win.SetContentBounds(bounds)
			a.prefsWindowBounds = bounds
		}
		// Opacity continues changing after dimensions round to their final
		// pixel. Always repaint the pane, even if no resize was necessary.
		win.Invalidate()
		if m.moving() {
			next = next.Add(time.Second / 120)
			if next.Before(at) {
				next = at.Add(time.Second / 120)
			}
			a.prefsResizeTimer = time.AfterFunc(max(time.Until(next), time.Millisecond), func() { win.Update(step) })
		} else {
			a.prefsResizeTimer = nil
		}
	}
	if initial {
		// Initial placement happens before showing the preferences window.
		step()
	} else {
		// Tab callbacks run while ui.runFrame still owns its old surface
		// dimensions. Resize on the next main-loop task, after that frame.
		win.Update(step)
	}
}
