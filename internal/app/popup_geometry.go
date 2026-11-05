package app

import (
	"log"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

const popupMinimumPaneWidth = 200 // SlideoutController's minimum widths.

// Keep drag frames in memory; committing JSON on every pointer move stalls
// native resizing. Closing the host commits immediately.
type popupGeometry struct {
	content, preview, height int
	dirty                    bool
	timer                    *time.Timer
	generation               uint64
}

type popupDividerDrag struct {
	active       bool
	startPointer float32
	startContent int
}

func (a *App) popupConfig() config.Config {
	cfg := a.cfg.Snapshot()
	if g := a.popupGeometry; g.dirty {
		cfg.WindowWidth, cfg.PreviewWidth, cfg.WindowHeight = g.content, g.preview, g.height
	}
	return cfg
}

func (a *App) setPopupGeometry(content, preview, height int) {
	g := &a.popupGeometry
	g.content, g.preview, g.height = content, preview, height
	g.dirty = true
	g.generation++
	if g.timer != nil {
		g.timer.Stop()
	}
	if !mygo.App.IsReady() {
		return
	}
	generation := g.generation
	g.timer = time.AfterFunc(200*time.Millisecond, func() {
		a.post(func() {
			if a.popupGeometry.generation == generation {
				a.flushPopupGeometry()
			}
		})
	})
}

func (a *App) flushPopupGeometry() {
	g := &a.popupGeometry
	g.generation++
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
	if !g.dirty {
		return
	}
	if err := a.cfg.Update(func(c *config.Config) {
		c.WindowWidth, c.PreviewWidth, c.WindowHeight = g.content, g.preview, g.height
	}); err != nil {
		log.Printf("maccy: save popup geometry: %v", err)
	}
	g.dirty = false
	if !a.dividerDrag.active && (a.panel.Load() != nil || a.devWin != nil) {
		a.scheduleAutoPreview()
	}
}

func fitPopupPanes(total, content int) (int, int) {
	content = max(popupMinimumPaneWidth, min(content, total-1-popupMinimumPaneWidth))
	return content, max(popupMinimumPaneWidth, total-1-content)
}

func (a *App) resizePopupDivider(win *mygo.Window, total, content int) {
	cfg := a.popupConfig()
	content, preview := fitPopupPanes(total, content)
	if content == cfg.WindowWidth && preview == cfg.PreviewWidth {
		return
	}
	a.setPopupGeometry(content, preview, cfg.WindowHeight)
	if m := a.popupMotions[win]; m != nil {
		// The divider changes the history anchor inside a stationary window.
		if m.placement.left {
			m.placement.anchor.X += m.placement.anchor.Width - content
		}
		m.placement.anchor.Width = content
	}
	a.lastPopupW = total
	if win != nil {
		win.Invalidate()
	}
}

func (a *App) previewDivider(c *ui.Context, t *ui.Theme, win *mygo.Window, left bool) {
	ui.Box(c).Key("preview-divider").Width(1).Shrink(0).FillHeight().Background(t.Border)
	handle := ui.Box(c).Absolute().Top(0).Bottom(0).Width(11).
		Focusable().Role(ui.RoleSplitter).Label("调整历史与预览宽度").Cursor(ui.CursorResizeEW).Bind(&a.dividerHandle)
	if left {
		handle.Right(-5)
	} else {
		handle.Left(-5)
	}
	if a.previewClosing || a.popupMotionActive() {
		handle.Disabled(true)
		return
	}
	if handle.FocusVisible() {
		handle.Border(1, t.Accent)
	}
	total := int(c.Root().Bounds().W)
	handle.HandleInput(func(ev ui.InputEvent) bool {
		if (win != nil && win != a.panel.Load() && win != a.devWin) || !a.previewOpen || a.editing != nil {
			return false
		}
		drag := &a.dividerDrag
		pointer := a.dividerHandle.Bounds(c).X + ev.X
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			drag.active, drag.startPointer = true, pointer
			drag.startContent = a.popupConfig().WindowWidth
			a.cancelAutoPreview()
			a.dividerHandle.Focus(c)
			return true
		case ui.InputPointerMove:
			if !drag.active {
				return false
			}
			delta := int(math.Round(float64(pointer - drag.startPointer)))
			if left {
				delta = -delta
			}
			a.resizePopupDivider(win, total, drag.startContent+delta)
			return true
		case ui.InputPointerUp:
			if !drag.active {
				return false
			}
			drag.active = false
			a.flushPopupGeometry()
			return true
		case ui.InputKeyDown:
			if ev.Mods != 0 || (ev.Key != ui.KeyLeft && ev.Key != ui.KeyRight) {
				return false
			}
			delta := 10
			if (ev.Key == ui.KeyLeft) != left {
				delta = -delta
			}
			a.resizePopupDivider(win, total, a.popupConfig().WindowWidth+delta)
			a.flushPopupGeometry()
			return true
		}
		return false
	})
}

func (a *App) observePopupGeometry(win *mygo.Window) {
	win.OnResize(func() {
		m := a.popupMotions[win]
		if m == nil || m.applying || a.editing != nil || a.popupMotionActive() || win.IsDestroyed() {
			return
		}
		b := win.ContentBounds()
		if b == m.lastApplied {
			return
		}
		cfg := a.popupConfig()
		content, preview := cfg.WindowWidth, cfg.PreviewWidth
		if a.previewOpen {
			// The outer edge on the preview side resizes that pane, while
			// dragging the opposite edge keeps the preview's width.
			if (b.X != m.lastApplied.X) == m.placement.left {
				preview = max(popupMinimumPaneWidth, b.Width-content-1)
			} else {
				content = max(popupMinimumPaneWidth, b.Width-preview-1)
			}
		} else {
			content = max(popupMinimumPaneWidth, b.Width)
		}
		height := cfg.WindowHeight
		if b.Height != m.lastApplied.Height {
			height = b.Height
		}
		a.cancelAutoPreview()
		a.setPopupGeometry(content, preview, height)
		a.updatePopupAnchor(m, b, content)
		a.lastPopupW, a.lastPopupH = b.Width, b.Height
		win.Invalidate()
	})
	win.OnMove(func() {
		m := a.popupMotions[win]
		if m == nil || m.applying || a.popupMotionActive() || win.IsDestroyed() {
			return
		}
		b := win.ContentBounds()
		if b.Width != m.lastApplied.Width || b.Height != m.lastApplied.Height {
			return // A simultaneous resize must be processed by OnResize first.
		}
		a.updatePopupAnchor(m, b, a.popupConfig().WindowWidth)
	})
}

func (a *App) updatePopupAnchor(m *popupWindowMotion, b mygo.Rectangle, content int) {
	anchor := b
	anchor.Width = content
	if m.placement.left && a.previewOpen {
		anchor.X += b.Width - content
	}
	m.placement.anchor = anchor
	if mygo.App.IsReady() {
		m.placement.workArea = mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}).WorkArea
	}
	if !a.previewOpen {
		cfg := a.popupConfig()
		m.placement = newPopupPlacement(anchor, m.placement.workArea, content+cfg.PreviewWidth+1)
	}
	m.lastApplied = b
	m.width.reset(float64(b.Width))
	m.height.reset(float64(b.Height))
	m.lastAt = time.Now()
}
