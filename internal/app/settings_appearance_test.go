package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

func TestSettingsToolbarKeyboardNavigation(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	tt := ui.NewTester(a.settingsView, 720, 632)
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyRight)
	if a.prefsTab != 3 || !tt.HasText("尚无置顶条目") {
		t.Fatalf("Right did not open pins: pane=%d texts=%v", a.prefsTab, tt.Texts())
	}
	tt.Key(0, ui.KeyLeft)
	if a.prefsTab != 2 || !tt.HasText("图片高度") {
		t.Fatal("Left did not return to appearance")
	}
}

func TestAppearanceChoicesPersistWithKeyboard(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 2
	tt := ui.NewTester(a.settingsView, 720, 632)
	if err := tt.Click("置顶到"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("底部"); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Snapshot().PinTo != config.PinBottom {
		t.Fatal("pin position was not saved")
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyEnter)
	if a.cfg.Snapshot().PinTo != config.PinTop {
		t.Fatal("keyboard change was not saved")
	}
	if err := tt.Click("高亮匹配"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("颜色"); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Snapshot().HighlightMatch != config.HighlightColor {
		t.Fatal("highlight style was not saved")
	}
}

func TestAppearanceNumberInputBounds(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 2
	tt := ui.NewTester(a.settingsView, 720, 632)
	if err := tt.Click("图片高度"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("200")
	tt.Key(0, ui.KeyUp)
	if a.cfg.Snapshot().ImageMaxHeight != 200 {
		t.Fatal("image height exceeded its maximum or was not saved")
	}
	tt.Key(0, ui.KeyDown)
	if a.cfg.Snapshot().ImageMaxHeight != 199 {
		t.Fatal("image height did not step down")
	}
	if err := tt.Click("预览延迟"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyUp)
	if a.cfg.Snapshot().PreviewDelay != 1501 {
		t.Fatal("preview delay did not step by one millisecond")
	}
}

func TestAppearanceManyScreens(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 2
	a.prefs.PopupPosition = popupPositionIndex(config.PopupCenter)
	a.prefsScreens = []string{"活动屏幕", "屏幕 1", "屏幕 2", "屏幕 3", "屏幕 4"}
	tt := ui.NewTester(a.settingsView, 600, 632)
	if err := tt.Click("屏幕"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("屏幕 4"); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Snapshot().PopupScreen != 4 {
		t.Fatal("screen picker did not save the selected display")
	}
}

func TestAppearanceDisabledDependentControls(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 2
	tt := ui.NewTester(a.settingsView, 720, 632)
	if err := tt.Click("自动打开预览"); err != nil {
		t.Fatal(err)
	}
	// Click the disabled input and try stepping it with the keyboard.
	r, ok := tt.Find("预览延迟")
	if !ok {
		t.Fatalf("preview input missing: %v", tt.Texts())
	}
	control, ok := tt.Find("自动打开预览")
	if !ok {
		t.Fatal("preview checkbox missing")
	}
	tt.ClickAt(control.X+30, r.Y+r.H/2)
	tt.Key(0, ui.KeyUp)
	if a.prefs.PreviewDelay != 1500 || a.cfg.Snapshot().OpenPreviewAutomatically {
		t.Fatal("disabled preview control accepted input")
	}
	if err := tt.Click("显示搜索框"); err != nil {
		t.Fatal(err)
	}
	r, ok = tt.Find("搜索框显示时机")
	if !ok {
		t.Fatal("search visibility choice missing")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	if a.prefs.SearchVisibility != 0 || a.cfg.Snapshot().ShowSearch {
		t.Fatal("disabled visibility control changed")
	}
}

func TestAppearanceLastPositionFitsMinimumWindow(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 2
	a.hasLast = true
	a.lastX, a.lastY = 80, 90
	_ = a.cfg.Update(func(c *config.Config) {
		c.HasPopupPosition, c.PopupX, c.PopupY = true, 80, 90
	})
	tt := ui.NewTester(a.settingsView, 600, 632)
	if err := tt.Click("弹窗位置"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("最后位置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	r, ok := tt.Find("重置位置")
	if !ok || r.X < 0 || r.X+r.W > 576 {
		t.Fatalf("reset button outside the minimum window: %+v", r)
	}
	if err := tt.Click("重置位置"); err != nil {
		t.Fatal(err)
	}
	cfg := a.cfg.Snapshot()
	if a.hasLast || cfg.HasPopupPosition || cfg.PopupX != 0 || cfg.PopupY != 0 {
		t.Fatal("reset did not clear saved position")
	}
}
