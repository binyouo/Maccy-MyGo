package app

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestSettingsPaneChangesRestorePositionNearDisplayEdge(t *testing.T) {
	area := mygo.Rectangle{X: -1710, Y: 0, Width: 1710, Height: 1080}
	anchor := mygo.Point{X: -265, Y: 100}
	for i := 0; i < 20; i++ {
		wide := settingsBoundsAtAnchor(anchor, 710, 552, area)
		if wide.X != -710 || wide.X+wide.Width > 0 || wide.Y != 100 {
			t.Fatalf("wide pane leaves secondary display: %+v", wide)
		}
		narrow := settingsBoundsAtAnchor(anchor, 510, 528, area)
		if narrow.X != -520 || narrow.Y != 100 {
			t.Fatalf("pane cycle %d shifted original position: %+v", i, narrow)
		}
	}
}

func TestGeneralSettingsFitsOriginalWindow(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	tt := ui.NewTester(a.settingsView, 510, 528)
	labels := []string{"登录时启动", "自动检查更新", "检查更新", "搜索", "自动粘贴", "纯文本粘贴"}
	if runtime.GOOS == "darwin" {
		labels = append(labels, "通知和声音 ↗")
	}
	for _, label := range labels {
		r, ok := tt.Find(label)
		if !ok || r.Y < 0 || r.Y+r.H > 528 || r.X < 0 || r.X+r.W > 510 {
			t.Fatalf("reference-size settings clip %q at %v, found=%v", label, r, ok)
		}
	}
	if tt.HasText("用于打开应用的全局快捷键。") {
		t.Fatal("recorder tooltip should not occupy permanent rows")
	}
}

func TestGeneralSearchPickerAndModifierHelp(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	tt := ui.NewTester(a.settingsView, 510, 528)
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("模糊"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.SearchMode != 1 {
		t.Fatal("search picker did not persist its choice")
	}
	before := a.modifierLegend()
	if err := tt.Click("自动粘贴"); err != nil {
		t.Fatal(err)
	}
	if got := a.modifierLegend(); got == before || !strings.Contains(got, "按住 ⌥ 选择项目以复制") {
		t.Fatalf("modifier help did not follow default action: %q", got)
	}
}

func TestShortcutRecorderCancelClearRejectAndRestore(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	tt := ui.NewTester(a.settingsView, 510, 528)
	initial := a.prefs.GlobalShortcut
	if err := tt.Click(shortcutDisplay(initial)); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyK)
	if a.prefs.GlobalShortcut != initial || a.prefsRecordingShortcut != "open" {
		t.Fatal("plain key was recorded or stopped recording")
	}
	tt.Key(ui.Alt, ui.KeyP)
	if a.prefs.GlobalShortcut != initial {
		t.Fatal("duplicate pin shortcut was recorded")
	}
	tt.Key(0, ui.KeyEscape)
	if a.prefsRecordingShortcut != "" || a.prefs.GlobalShortcut != initial {
		t.Fatal("Escape changed the shortcut")
	}
	if err := tt.Click(shortcutDisplay(initial)); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyTab)
	if a.prefsRecordingShortcut != "" {
		t.Fatal("Tab did not finish recording")
	}
	if err := tt.Click(shortcutDisplay(initial)); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyBackspace)
	if a.prefs.GlobalShortcut != "" {
		t.Fatal("Delete did not clear the shortcut")
	}
	if err := tt.RightClick("未设置"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("恢复默认"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.GlobalShortcut != initial {
		t.Fatal("context menu did not restore the default shortcut")
	}
}

func TestIgnoreEntriesEditAndValidate(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 4
	a.prefs.IgnoredApps = "com.one\ncom.two"
	tt := ui.NewTester(a.settingsView, 560, 450)
	if err := tt.Click("com.one"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("com.edited")
	tt.Key(0, ui.KeyEnter)
	if a.cfg.Snapshot().IgnoredApps[0] != "com.edited" {
		t.Fatal("inline application edit was not saved")
	}
	if err := tt.Click("com.edited"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("com.two")
	tt.Key(0, ui.KeyEnter)
	if a.prefs.IgnoredApps != "com.edited\ncom.two" {
		t.Fatal("duplicate application edit was accepted")
	}
	a.prefsIgnoreTab = 2
	a.prefs.IgnoreRegexp = "^alpha$"
	tt.Frame()
	if err := tt.Click("^alpha$"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("[")
	tt.Key(0, ui.KeyEnter)
	if a.prefs.IgnoreRegexp != "^alpha$" {
		t.Fatal("invalid edited regexp was saved")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("^beta$")
	tt.Key(0, ui.KeyEnter)
	if a.cfg.Snapshot().IgnoreRegexp[0] != "^beta$" {
		t.Fatal("valid edited regexp was not saved")
	}
}

func TestStorageSizeAppliesLimitAndPreservesPins(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 1
	for _, text := range []string{"one", "two", "three", "four", "five"} {
		a.addItem(makeItem(t, a, text, text))
	}
	a.togglePin(a.items[0])
	tt := ui.NewTester(a.settingsView, 510, 300)
	if err := tt.Click("数量"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2")
	if len(a.items) != 3 || a.cfg.Snapshot().HistorySize != 2 {
		t.Fatalf("size limit not applied: items=%d size=%d", len(a.items), a.cfg.Snapshot().HistorySize)
	}
	pins := 0
	for _, item := range a.items {
		if item.Pin != "" {
			pins++
		}
	}
	if pins != 1 {
		t.Fatal("reducing history removed a pinned item")
	}
}

func TestSettingsTabChangeCancelsRecorder(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	tt := ui.NewTester(a.settingsView, 720, 660)
	if err := tt.Click("⌘⇧C"); err != nil {
		t.Fatal(err)
	}
	if a.prefsRecordingShortcut == "" {
		t.Fatal("recorder did not start")
	}
	if err := tt.Click("存储"); err != nil {
		t.Fatal(err)
	}
	if a.prefsRecordingShortcut != "" {
		t.Fatal("hidden shortcut recorder is still active after changing panes")
	}
}

func TestSettingsTabStartsAtTop(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	tt := ui.NewTester(a.settingsView, 720, 480)
	tt.Scroll(500, 350, 0, 420)
	tt.Advance(time.Second)
	if err := tt.Click("存储"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	r, ok := tt.Find("文本")
	if !ok || r.Y < 75 || r.Y > 220 {
		t.Fatalf("storage pane inherited another pane's scroll: %+v, found=%v", r, ok)
	}
}

func TestSettingsShortcutRecorder(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 0
	tt := ui.NewTester(func(c *ui.Context) { a.settingsView(c) }, 680, 760)
	if err := tt.Click("⌘⇧C"); err != nil {
		t.Fatalf("click recorder: %v", err)
	}
	tt.Key(ui.Super|ui.Shift, ui.KeyB)
	if got := a.prefs.GlobalShortcut; got != "Cmd+Shift+B" {
		t.Fatalf("draft shortcut = %q, texts: %v", got, tt.Texts())
	}
	if got := a.cfg.Snapshot().GlobalShortcut; got != "Cmd+Shift+B" {
		t.Fatalf("stored shortcut = %q", got)
	}
	if _, _, ok := parseShortcut(a.cfg.Snapshot().GlobalShortcut); !ok {
		t.Fatalf("recorded shortcut does not parse")
	}
}

func TestSettingsPinsPane(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.addItem(makeItem(t, a, "pinned item", "pinned body"))
	it := a.items[0]
	a.togglePin(it)

	// Empty pane hints before any pin is set; with a pin the table shows.
	a.prefsTab = 3
	tt := ui.NewTester(func(c *ui.Context) { a.settingsView(c) }, 680, 760)
	for _, want := range []string{"键位", "标题", "内容"} {
		if !tt.HasText(want) {
			t.Fatalf("pins table missing %q: %v", want, tt.Texts())
		}
	}

	// Renaming through the draft applies to the item.
	d := a.pinDraftFor(it)
	d.title = "改名"
	a.applyPinTitle(it, d)
	if it.Title != "改名" {
		t.Fatalf("title = %q", it.Title)
	}

	// Editing content swaps the plain-text blob.
	d.content = "新内容"
	a.applyPinContent(it, d)
	if got := a.itemText(it); got != "新内容" {
		t.Fatalf("content = %q", got)
	}

	// A key change keeps the pin's slot in the display order.
	old := it.Pin
	it.Pin = "x"
	a.applyPinKey(it, old)
	order := a.cfg.Snapshot().PinOrder
	if len(order) != 1 || order[0] != "x" {
		t.Fatalf("pin order = %v", order)
	}

	// Unpinning empties the table again.
	a.togglePin(it)
	tt2 := ui.NewTester(func(c *ui.Context) { a.settingsView(c) }, 680, 760)
	if !tt2.HasText("尚无置顶条目") {
		t.Fatalf("expected empty state: %v", tt2.Texts())
	}
}

func TestSettingsIgnoreListRemoves(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefs.IgnoredApps = "com.one\ncom.two"
	a.prefsTab = 4
	a.prefsIgnoreTab = 0
	tt := ui.NewTester(func(c *ui.Context) { a.settingsView(c) }, 680, 760)
	if !tt.HasText("com.one") || !tt.HasText("com.two") {
		t.Fatalf("ignore rows missing: %v", tt.Texts())
	}
	if err := tt.Click("移除"); err != nil {
		t.Fatalf("click remove: %v", err)
	}
	if got := a.prefs.IgnoredApps; got != "com.two" {
		t.Fatalf("ignored apps = %q", got)
	}
}
