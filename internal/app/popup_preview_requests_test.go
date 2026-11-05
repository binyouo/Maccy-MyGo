package app

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

func previewRequestApp(t *testing.T, open bool) *App {
	t.Helper()
	a := popupTestApp(t)
	a.previewOpen = open
	a.previewClosing = !open
	a.mu.Lock()
	a.lastPopupW, a.lastPopupH = a.popupSizeLocked()
	a.mu.Unlock()
	a.lastPopupMode = a.popupMode()
	t.Cleanup(func() {
		if a.previewTimer != nil {
			a.previewTimer.Stop()
		}
		a.cancelPopupResize()
	})
	return a
}

// The nil host lets the request controller observe a moving window
// without constructing a native window or depending on a timer.
func activePreviewMotion(a *App) *popupWindowMotion {
	m := &popupWindowMotion{}
	m.width.reset(float64(a.lastPopupW))
	m.width.position = 650
	if a.previewOpen {
		m.width.velocity = 1000
	} else {
		m.width.velocity = -1000
	}
	m.height.reset(float64(a.lastPopupH))
	a.popupMotions = map[*mygo.Window]*popupWindowMotion{nil: m}
	return m
}

func finishPreviewMotion(m *popupWindowMotion) {
	m.width.reset(m.width.target)
	m.height.reset(m.height.target)
}

func TestPreviewButtonQueuesUntilCurrentAnimationFinishes(t *testing.T) {
	for _, open := range []bool{false, true} {
		t.Run(map[bool]string{false: "closing", true: "opening"}[open], func(t *testing.T) {
			a := previewRequestApp(t, open)
			m := activePreviewMotion(a)
			width, height := m.width, m.height
			tt := ui.NewTester(a.popupViewPanel, 851, 300)
			if err := tt.Click("切换预览"); err != nil {
				t.Fatal(err)
			}
			if a.previewOpen != open || !a.previewQueued {
				t.Fatalf("button reversed the visible transition instead of queuing: open=%v queued=%v", a.previewOpen, a.previewQueued)
			}
			if m.width != width || m.height != height {
				t.Fatalf("queued request changed active motion: width=%+v height=%+v", m.width, m.height)
			}
			a.applyQueuedPreview()
			if a.previewOpen != open || !a.previewQueued {
				t.Fatal("an active transition drained the queued request")
			}
			finishPreviewMotion(m)
			a.applyQueuedPreview()
			tt.Frame()
			if a.previewOpen == open || a.previewQueued {
				t.Fatalf("completed transition did not apply the queued intent: open=%v queued=%v", a.previewOpen, a.previewQueued)
			}
			if got := tt.HasText("复制次数"); got != !open {
				t.Fatalf("button result did not reach the view: metadata=%v want=%v", got, !open)
			}
			generation := a.popupResizeGen
			a.applyQueuedPreview()
			if a.previewOpen == open || a.previewQueued || a.popupResizeGen != generation {
				t.Fatal("the same queued request was applied more than once")
			}
		})
	}
}

func TestPreviewQueuedClicksCancelInPairs(t *testing.T) {
	for _, clicks := range [][]int{{2}, {1, 1}, {3, 1}, {1, 2, 1}} {
		a := previewRequestApp(t, false)
		m := activePreviewMotion(a)
		for _, n := range clicks {
			a.togglePreviewClicks(n)
		}
		if a.previewOpen || a.previewQueued || m.width.target != 450 {
			t.Fatalf("clicks %v did not cancel while closing: open=%v queued=%v target=%v", clicks, a.previewOpen, a.previewQueued, m.width.target)
		}
		finishPreviewMotion(m)
		a.applyQueuedPreview()
		if a.previewOpen || a.previewQueued {
			t.Fatalf("cancelled clicks %v opened the sidebar on completion", clicks)
		}
	}
}

func TestPreviewQueueWaitsForHeightAnimationToo(t *testing.T) {
	a := previewRequestApp(t, false)
	m := &popupWindowMotion{}
	m.width.reset(450)
	m.height.reset(float64(a.lastPopupH))
	m.height.position += 40
	m.height.velocity = -200
	a.popupMotions = map[*mygo.Window]*popupWindowMotion{nil: m}
	a.togglePreviewClicks(1)
	if a.previewOpen || !a.previewQueued {
		t.Fatal("request interrupted an active height transition")
	}
	finishPreviewMotion(m)
	a.applyQueuedPreview()
	if !a.previewOpen || a.previewQueued {
		t.Fatal("queued request was not applied after both dimensions settled")
	}
}

func TestPreviewCloseRequestIsIdempotentWhileQueued(t *testing.T) {
	a := previewRequestApp(t, true)
	m := activePreviewMotion(a)
	a.requestPreview(false)
	if !a.previewQueued || !a.previewOpen {
		t.Fatal("close request was not queued behind opening")
	}
	generation := a.previewGen.Load()
	for i := 0; i < 4; i++ {
		a.requestPreview(false)
	}
	if !a.previewQueued || !a.previewOpen || a.previewGen.Load() != generation || m.width.target != 851 {
		t.Fatal("repeated close requests cancelled or restarted the pending close")
	}
	finishPreviewMotion(m)
	a.applyQueuedPreview()
	a.requestPreview(false)
	if a.previewOpen || a.previewQueued || a.previewGen.Load() != generation {
		t.Fatal("idempotent close reopened the sidebar after completion")
	}
}

func enablePreviewAutoOpen(t *testing.T, a *App) {
	t.Helper()
	if err := a.cfg.Update(func(c *config.Config) {
		c.OpenPreviewAutomatically = true
		c.PreviewDelay = 60000
	}); err != nil {
		t.Fatal(err)
	}
}

func assertPreviewAutoSuppressed(t *testing.T, a *App) {
	t.Helper()
	generation := a.previewGen.Load()
	a.scheduleAutoPreview()
	if a.previewOpen || !a.previewAutoSuppressed || a.previewTimer != nil || a.previewGen.Load() != generation {
		t.Fatalf("manual intent allowed automatic preview: open=%v suppressed=%v timer=%v generation=%d want=%d", a.previewOpen, a.previewAutoSuppressed, a.previewTimer != nil, a.previewGen.Load(), generation)
	}
}

func TestPreviewManualCloseSurvivesSearchChanges(t *testing.T) {
	a := previewRequestApp(t, true)
	enablePreviewAutoOpen(t, a)
	tt := ui.NewTester(a.popupViewPanel, 851, 300)
	if err := tt.Click("切换预览"); err != nil {
		t.Fatal(err)
	}
	assertPreviewAutoSuppressed(t, a)
	tt.Type("bet")
	if a.query != "bet" {
		t.Fatalf("search input was lost after close: %q", a.query)
	}
	assertPreviewAutoSuppressed(t, a)
	tt.Key(ui.Ctrl, ui.KeyU)
	if a.query != "" {
		t.Fatalf("Control-U did not clear query: %q", a.query)
	}
	assertPreviewAutoSuppressed(t, a)
}

func TestPreviewEvenClickBatchSuppressesInitialAutoOpen(t *testing.T) {
	a := previewRequestApp(t, false)
	enablePreviewAutoOpen(t, a)
	a.togglePreviewClicks(2)
	if a.previewQueued {
		t.Fatal("even click batch left a pending transition")
	}
	assertPreviewAutoSuppressed(t, a)
	a.query = "be"
	a.onQueryChanged()
	assertPreviewAutoSuppressed(t, a)
}
