package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAutoPreviewReturnsFromFooter(t *testing.T) {
	a := previewRequestApp(t, true)
	enablePreviewAutoOpen(t, a)
	tt := ui.NewTester(a.popupViewPanel, 851, 300)
	footer, ok := tt.Find("偏好设置…")
	if !ok {
		t.Fatal("missing footer")
	}
	tt.Move(footer.X+20, footer.Y+footer.H/2)
	if a.previewOpen || a.footerIndex < 0 {
		t.Fatal("footer did not close preview")
	}
	tt.SetSize(450, 300)
	row, ok := tt.Find("alpha one")
	if !ok {
		t.Fatal("missing history row")
	}
	tt.Move(row.X+20, row.Y+row.H/2)
	if a.footerIndex >= 0 || a.previewAutoSuppressed || a.previewTimer == nil {
		t.Fatalf("returning to history did not rearm preview: footer=%d suppressed=%v timer=%v", a.footerIndex, a.previewAutoSuppressed, a.previewTimer != nil)
	}
}

func TestAutoPreviewMouseSelectionRestartsDelay(t *testing.T) {
	a := previewRequestApp(t, false)
	enablePreviewAutoOpen(t, a)
	a.scheduleAutoPreview()
	gen := a.previewGen.Load()
	tt := ui.NewTester(a.popupViewPanel, 450, 300)
	r, _ := tt.Find("alpha one")
	tt.Move(r.X+20, r.Y+r.H/2)
	if a.previewGen.Load() == gen {
		t.Fatal("mouse selection retained the previous item's deadline")
	}
}

func TestAutoPreviewDisabledSettingCancelsTimer(t *testing.T) {
	a := previewRequestApp(t, false)
	enablePreviewAutoOpen(t, a)
	a.scheduleAutoPreview()
	a.draftFromConfig()
	a.prefs.OpenPreviewAutomatically = false
	a.syncPrefs()
	if a.previewTimer != nil {
		t.Fatal("turning off auto preview left its timer running")
	}
}

func TestAutoPreviewEmptySearchCancelsTimer(t *testing.T) {
	a := previewRequestApp(t, false)
	enablePreviewAutoOpen(t, a)
	a.scheduleAutoPreview()
	a.query = "no-match-for-auto-preview"
	a.onQueryChanged()
	if a.previewTimer != nil {
		t.Fatal("empty results retained an automatic preview timer")
	}
}

func TestAutoPreviewManualCloseOnlySuppressesCurrentItem(t *testing.T) {
	a := previewRequestApp(t, true)
	enablePreviewAutoOpen(t, a)
	a.togglePreview()
	a.scheduleAutoPreview()
	if !a.previewAutoSuppressed || a.previewTimer != nil {
		t.Fatal("same item reopened after a manual close")
	}
	a.moveSelection(1)
	if a.previewAutoSuppressed || a.previewTimer == nil {
		t.Fatal("new selection retained the previous item's suppression")
	}
}

func TestAutoPreviewDisabledSettingCancelsQueuedOpen(t *testing.T) {
	a := previewRequestApp(t, false)
	enablePreviewAutoOpen(t, a)
	m := activePreviewMotion(a)
	a.previewQueued = true
	a.previewAutoQueuedFor = a.selectedPreviewID()
	a.draftFromConfig()
	a.prefs.OpenPreviewAutomatically = false
	a.syncPrefs()
	finishPreviewMotion(m)
	a.applyQueuedPreview()
	if a.previewOpen || a.previewQueued || a.previewAutoQueuedFor != "" {
		t.Fatal("disabled auto-open survived in the resize queue")
	}
}
