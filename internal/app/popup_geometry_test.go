package app

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

func TestPreviewDividerDragKeepsTotalWidthAndPersists(t *testing.T) {
	for _, left := range []bool{false, true} {
		t.Run(map[bool]string{false: "right", true: "left"}[left], func(t *testing.T) {
			a := popupTestApp(t)
			a.previewOpen = true
			anchor := mygo.Rectangle{X: 500, Y: 50, Width: 450, Height: 300}
			m := &popupWindowMotion{placement: popupPlacement{anchor: anchor, left: left}}
			m.width.reset(851)
			m.height.reset(300)
			a.popupMotions = map[*mygo.Window]*popupWindowMotion{nil: m}
			tt := ui.NewTester(a.popupViewPanel, 851, 300)
			tt.Type("bet")
			r, ok := tt.Find("调整历史与预览宽度")
			if !ok {
				t.Fatal("missing accessible divider")
			}
			x, y := r.X+r.W/2, r.Y+r.H/2
			// Stay on the preview side of its clipped viewport.
			if left {
				x -= 2
			} else {
				x += 2
			}
			tt.Press(x, y)
			delta := float32(60)
			if left {
				delta = -delta
			}
			tt.Move(x+delta, y)
			tt.Move(x+delta+delta/2, y)
			tt.Release(x+delta+delta/2, y)
			cfg := a.cfg.Snapshot()
			if cfg.WindowWidth != 540 || cfg.PreviewWidth != 310 {
				t.Fatalf("dragged widths = %d/%d, want 540/310", cfg.WindowWidth, cfg.PreviewWidth)
			}
			if cfg.WindowWidth+cfg.PreviewWidth+1 != 851 || a.query != "bet" {
				t.Fatalf("divider changed window width or search: %+v query=%q", cfg, a.query)
			}
			if m.placement.anchor.Width != 540 {
				t.Fatalf("history anchor width = %d", m.placement.anchor.Width)
			}
			if left && m.placement.anchor.X+m.placement.anchor.Width != anchor.X+anchor.Width {
				t.Fatal("left preview drag moved the history's right edge")
			}
			loaded, err := config.Load(a.store.Dir() + "/config.json")
			if err != nil || loaded.WindowWidth != cfg.WindowWidth || loaded.PreviewWidth != cfg.PreviewWidth {
				t.Fatalf("dragged widths did not persist: %+v %v", loaded, err)
			}
		})
	}
}

func TestPreviewDividerKeyboardAndMinimumWidths(t *testing.T) {
	a := popupTestApp(t)
	a.previewOpen = true
	tt := ui.NewTester(a.popupViewPanel, 851, 300)
	r, ok := tt.Find("调整历史与预览宽度")
	if !ok {
		t.Fatal("missing divider")
	}
	x, y := r.X+r.W/2+2, r.Y+r.H/2
	tt.ClickAt(x, y)
	tt.Key(0, ui.KeyRight)
	if got := a.cfg.Snapshot(); got.WindowWidth != 460 || got.PreviewWidth != 390 {
		t.Fatalf("keyboard did not resize focused divider: %d/%d", got.WindowWidth, got.PreviewWidth)
	}
	a.resizePopupDivider(nil, 851, 10)
	if got := a.popupConfig(); got.WindowWidth != 200 || got.PreviewWidth != 650 {
		t.Fatalf("minimum history width = %d/%d", got.WindowWidth, got.PreviewWidth)
	}
	a.resizePopupDivider(nil, 851, 1000)
	a.flushPopupGeometry()
	if got := a.cfg.Snapshot(); got.WindowWidth != 650 || got.PreviewWidth != 200 {
		t.Fatalf("minimum preview width = %d/%d", got.WindowWidth, got.PreviewWidth)
	}
}
