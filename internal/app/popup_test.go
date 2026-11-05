package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

func popupTestApp(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.OpenPreviewAutomatically = false })
	a.addItem(makeItem(t, a, "alpha one", "alpha one"))
	a.addItem(makeItem(t, a, "beta two", "beta two"))
	a.addItem(makeItem(t, a, "better beta", "better beta"))
	return a
}

func TestPopupSearchFilters(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	if !tt.HasText("better beta") {
		t.Fatalf("expected newest item, texts: %v", tt.Texts())
	}
	tt.Type("be")
	if a.query != "be" {
		t.Fatalf("query = %q", a.query)
	}
	if !tt.HasText("beta two") || !tt.HasText("better beta") {
		t.Fatalf("expected both beta items, got %v", tt.Texts())
	}
	if tt.HasText("alpha one") {
		t.Fatalf("alpha should be filtered out: %v", tt.Texts())
	}
}

func TestPopupKeyboardNavigation(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	tt.Type("bet")
	if a.selected != 0 {
		t.Fatalf("selected = %d", a.selected)
	}
	tt.Key(0, ui.KeyDown)
	if a.selected != 1 {
		t.Fatalf("after down selected = %d", a.selected)
	}
	tt.Key(0, ui.KeyDown) // past the last item -> footer
	if a.footerIndex != 0 || a.selected != -1 {
		t.Fatalf("expected footer, selected=%d footer=%d", a.selected, a.footerIndex)
	}
	tt.Key(0, ui.KeyDown)
	if a.footerIndex != 1 {
		t.Fatalf("footer index = %d", a.footerIndex)
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp) // back to list
	if a.footerIndex != -1 || a.selected != 1 {
		t.Fatalf("expected back in list, selected=%d footer=%d", a.selected, a.footerIndex)
	}
	tt.Key(0, ui.KeyHome)
	if a.selected != 0 {
		t.Fatalf("home selected = %d", a.selected)
	}
	tt.Key(0, ui.KeyEnd)
	if a.selected != 1 {
		t.Fatalf("end selected = %d", a.selected)
	}
}

func TestPopupPreviewToggle(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 900, 500)
	tt.Key(ui.Ctrl, ui.KeySpace)
	if !a.previewOpen {
		t.Fatal("preview should be open")
	}
	if !tt.HasText("复制次数") {
		t.Fatalf("preview metadata missing: %v", tt.Texts())
	}
	tt.Key(ui.Ctrl, ui.KeySpace)
	if a.previewOpen {
		t.Fatal("preview should be closed")
	}
}

func TestPopupEditAndCancel(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 560, 420)
	a.beginEdit(a.vitems[0])
	tt.Frame()
	if a.editing == nil {
		t.Fatal("expected editor open")
	}
	if !tt.HasText("编辑条目") {
		t.Fatalf("editor not shown: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.editing != nil {
		t.Fatal("expected editor closed by Escape")
	}
}
func TestTitleSegments(t *testing.T) {
	segs := titleSegments("alpha beta", []search.Range{{Start: 6, End: 10}})
	if len(segs) != 2 || segs[0].text != "alpha " || segs[0].match || segs[1].text != "beta" || !segs[1].match {
		t.Fatalf("segments = %+v", segs)
	}
}
func TestParseShortcut(t *testing.T) {
	cases := []struct {
		in   string
		mods ui.Modifiers
		key  ui.Key
	}{
		{"Option+P", ui.Alt, ui.KeyP},
		{"Ctrl+Space", ui.Ctrl, ui.KeySpace},
		{"Cmd+Backspace", ui.Super, ui.KeyBackspace},
		{"Shift+Option+Delete", ui.Shift | ui.Alt, ui.KeyBackspace},
	}
	for _, c := range cases {
		mods, key, ok := parseShortcut(c.in)
		if !ok || mods != c.mods || key != c.key {
			t.Fatalf("parse %q = %v %v %v", c.in, mods, key, ok)
		}
	}
}

func TestConfigurableDeleteShortcut(t *testing.T) {
	a := popupTestApp(t)
	before := len(a.items)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	tt.Key(ui.Alt, ui.KeyBackspace)
	if len(a.items) != before-1 {
		t.Fatalf("expected %d items, got %d", before-1, len(a.items))
	}
}
func TestPinViaToolbar(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 900, 500)
	tt.Key(ui.Ctrl, ui.KeySpace) // the item toolbar lives in the preview pane
	if err := tt.Click("置顶/取消置顶"); err != nil {
		t.Fatal(err)
	}
	pinned := 0
	for _, it := range a.items {
		if it.Pin != "" {
			pinned++
		}
	}
	if pinned != 1 {
		t.Fatalf("expected one pinned item, got %d", pinned)
	}
}

func TestToolbarPinKeepsPreviewOnItem(t *testing.T) {
	a := popupTestApp(t)
	a.previewOpen = true
	a.selected = 1
	want := a.vitems[a.selected].ID
	tt := ui.NewTester(a.popupViewPanel, 900, 500)
	for i := 0; i < 2; i++ {
		if err := tt.Click("置顶/取消置顶"); err != nil {
			t.Fatal(err)
		}
		if got := a.vitems[a.selected].ID; got != want {
			t.Fatalf("pin action changed the preview to %s, want %s", got, want)
		}
	}
}

func TestPreviewToggleKeepsSearchFocus(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 900, 500)
	tt.Type("be")
	if err := tt.Click("切换预览"); err != nil {
		t.Fatal(err)
	}
	tt.Type("t")
	if a.query != "bet" {
		t.Fatalf("search lost focus after toggling preview: %q", a.query)
	}
}

func TestToolbarEditSaveCancelAndDelete(t *testing.T) {
	a := popupTestApp(t)
	a.previewOpen = true
	a.selected = 1
	want := a.vitems[a.selected]
	tt := ui.NewTester(a.popupViewPanel, 900, 500)
	if err := tt.Click("编辑条目"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("renamed beta")
	if err := tt.Click("完成"); err != nil {
		t.Fatal(err)
	}
	if a.editing != nil || want.Title != "renamed beta" || a.vitems[a.selected] != want {
		t.Fatalf("save lost the edited item: editing=%v title=%q selected=%d", a.editing != nil, want.Title, a.selected)
	}
	if err := tt.Click("编辑条目"); err != nil {
		t.Fatal(err)
	}
	tt.Type("discard")
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if a.editing != nil || want.Title != "renamed beta" {
		t.Fatal("cancel changed the item or left the editor open")
	}
	before := len(a.items)
	if err := tt.Click("删除条目"); err != nil {
		t.Fatal(err)
	}
	if len(a.items) != before-1 {
		t.Fatal("delete did not remove exactly one item")
	}
	for _, it := range a.items {
		if it.ID == want.ID {
			t.Fatal("delete removed the wrong item")
		}
	}
	if !tt.HasText("复制次数") {
		t.Fatal("preview did not update to the next item")
	}
}

func TestToolbarDeleteLastItem(t *testing.T) {
	a := newTestApp(t)
	a.addItem(makeItem(t, a, "last", "last"))
	a.previewOpen = true
	tt := ui.NewTester(a.popupViewPanel, 900, 500)
	if err := tt.Click("删除条目"); err != nil {
		t.Fatal(err)
	}
	if a.selected != -1 || !tt.HasText("选择条目以预览") || tt.HasText("复制次数") {
		t.Fatalf("stale preview after deleting final item: %v", tt.Texts())
	}
}

func TestPopupResizeStaysOnSecondaryDisplay(t *testing.T) {
	from := mygo.Rectangle{X: -500, Y: 100, Width: 450, Height: 220}
	placement := newPopupPlacement(from, mygo.Rectangle{X: -1710, Y: 0, Width: 1710, Height: 1080}, 851)
	for i := 0; i <= 100; i++ {
		r := placement.bounds(450+401*float64(i)/100, 220+160*float64(i)/100)
		if r.X >= 0 || r.X+r.Width > 0 || r.Width < from.Width || r.Width > 851 {
			t.Fatalf("resize leaves secondary display at step %d: %+v", i, r)
		}
	}
	if got := placement.bounds(450, 220); got != from {
		t.Fatalf("closing did not return to original bounds: %+v", got)
	}
}

func TestPopupLeftSidebarKeepsHistoryScreenPosition(t *testing.T) {
	a := popupTestApp(t)
	a.previewOpen = true
	a.popupMotions = map[*mygo.Window]*popupWindowMotion{nil: {placement: popupPlacement{left: true}}}
	tt := ui.NewTester(a.popupViewPanel, 450, 300)
	before, ok := tt.Find("搜索")
	if !ok {
		t.Fatal("missing search")
	}
	for _, width := range []int{451, 520, 700, 851, 720, 450, 600, 851, 450} {
		tt.SetSize(width, 300)
		tt.Frame()
		r, ok := tt.Find("搜索")
		// The window moves left by its revealed sidebar extent.
		if !ok || r.X-float32(width-450) != before.X {
			t.Fatalf("history moved at window width %d: first=%+v current=%+v", width, before, r)
		}
	}
}
func TestPopupMouseWheelScrolls(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.OpenPreviewAutomatically = false })
	for i := 0; i < 30; i++ {
		title := "item " + string(rune('A'+i%26)) + "-" + string(rune('0'+i%10))
		a.addItem(makeItem(t, a, title, title))
	}
	tt := ui.NewTester(a.popupViewPanel, 450, 320)
	before, _ := a.list.Visible()
	tt.Scroll(225, 160, 0, 100)
	tt.Advance(300 * time.Millisecond)
	after, _ := a.list.Visible()
	if after <= before {
		t.Fatalf("list did not scroll (before=%d after=%d)", before, after)
	}
}
func TestPopupRowClickCopies(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	if err := tt.Click("beta two"); err != nil {
		t.Fatal(err)
	}
	if a.lastCopiedID == "" {
		t.Fatal("clicking a row did not trigger copy")
	}
	var want string
	for _, it := range a.items {
		if it.Title == "beta two" {
			want = it.ID
		}
	}
	if a.lastCopiedID != want {
		t.Fatalf("copied %q, want %q", a.lastCopiedID, want)
	}
}
func TestPopupHoverSelects(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	r, ok := tt.Find("beta two")
	if !ok {
		t.Fatal("row not found")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	want := -1
	for i, it := range a.vitems {
		if it.Title == "beta two" {
			want = i
		}
	}
	if want < 0 || a.selected != want {
		t.Fatalf("hover selected %d, want %d", a.selected, want)
	}
}
func TestPopupHoverAfterScrollAligns(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.OpenPreviewAutomatically = false })
	for i := 0; i < 30; i++ {
		title := "row-" + string(rune('A'+i))
		a.addItem(makeItem(t, a, title, title))
	}
	tt := ui.NewTester(a.popupViewPanel, 450, 320)
	tt.Scroll(225, 170, 0, 120)
	tt.Advance(300 * time.Millisecond)
	first, last := a.list.Visible()
	t.Logf("visible %d..%d", first, last)
	// Find a visible row and hover its center.
	probe := -1
	for i := first; i <= last && i < len(a.vitems); i++ {
		if _, ok := tt.Find(a.vitems[i].Title); ok {
			probe = i
			break
		}
	}
	if probe < 0 {
		t.Fatal("no visible row found")
	}
	r, _ := tt.Find(a.vitems[probe].Title)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if a.selected != probe {
		t.Fatalf("hover offset: selected %d, want %d (visible %d..%d)", a.selected, probe, first, last)
	}
}
func TestPopupHeightAccountsForImages(t *testing.T) {
	mk := func(withImages bool) int {
		a := newTestApp(t)
		_ = a.cfg.Update(func(c *config.Config) { c.OpenPreviewAutomatically = false })
		for i := 0; i < 10; i++ {
			if withImages {
				img := image.NewRGBA(image.Rect(0, 0, 8, 8))
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						img.Set(x, y, color.RGBA{R: 200, G: uint8(20 + i*10), B: 60, A: 255})
					}
				}
				var buf bytes.Buffer
				_ = png.Encode(&buf, img)
				h, size, err := a.store.PutBlob(buf.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				a.addItem(&model.Item{Title: "Image", Contents: []model.Content{{Type: "image/png", Hash: h, Size: size}}})
			} else {
				s := "text row " + string(rune('A'+i))
				a.addItem(makeItem(t, a, s, s))
			}
		}
		a.mu.Lock()
		_, height := a.popupSizeLocked()
		a.mu.Unlock()
		return height
	}
	if mk(true) <= mk(false) {
		t.Fatalf("image rows should make the popup taller: %d vs %d", mk(true), mk(false))
	}
}
func TestPopupHoverAlignsWithMixedHeights(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.OpenPreviewAutomatically = false })
	for i := 0; i < 24; i++ {
		if i%4 == 2 {
			img := image.NewRGBA(image.Rect(0, 0, 8, 8))
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					img.Set(x, y, color.RGBA{R: uint8(30 * i), G: 80, B: 120, A: 255})
				}
			}
			var buf bytes.Buffer
			_ = png.Encode(&buf, img)
			h, size, _ := a.store.PutBlob(buf.Bytes())
			a.addItem(&model.Item{Title: "img-" + string(rune('A'+i)), Contents: []model.Content{{Type: "image/png", Hash: h, Size: size}}})
		} else {
			s := "txt-" + string(rune('A'+i))
			a.addItem(makeItem(t, a, s, s))
		}
	}
	tt := ui.NewTester(a.popupViewPanel, 450, 320)
	tt.Scroll(225, 170, 0, 160)
	tt.Advance(300 * time.Millisecond)
	first, last := a.list.Visible()
	probe := -1
	for i := first; i <= last && i < len(a.vitems); i++ {
		if !clipboardHasImage(a.vitems[i]) {
			if _, ok := tt.Find(a.vitems[i].Title); ok {
				probe = i
				break
			}
		}
	}
	if probe < 0 {
		t.Skip("no visible text row")
	}
	r, _ := tt.Find(a.vitems[probe].Title)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if a.selected != probe {
		t.Fatalf("mixed-height hover offset: selected %d, want %d (visible %d..%d)", a.selected, probe, first, last)
	}
}

func TestPopupFooterRowsAndClearAllSwap(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	for _, want := range []string{"清除", "偏好设置…", "关于", "退出"} {
		if !tt.HasText(want) {
			t.Fatalf("footer missing %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText("全部清除") {
		t.Fatal("Clear All should only show while Shift is held")
	}
	tt.HoldModifiers(ui.Shift)
	if !tt.HasText("全部清除") {
		t.Fatalf("expected 全部清除 under Shift: %v", tt.Texts())
	}
	tt.HoldModifiers(0)
	if tt.HasText("全部清除") {
		t.Fatal("Clear All should revert to 清除 after release")
	}
}

func TestPopupPinKeyActivates(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	target := a.vitems[0]
	a.togglePin(target)
	tt.Frame()
	key, ok := keyFromName(target.Pin)
	if !ok {
		t.Fatalf("pin is not a shortcut key: %q", target.Pin)
	}
	// A bare pin key types into the search field, as Maccy does.
	tt.Key(0, key)
	if a.lastCopiedID == target.ID {
		t.Fatal("bare pin key should not activate the item")
	}
	tt.Key(ui.Ctrl, ui.KeyU)
	tt.Key(ui.Cmd, key)
	if a.lastCopiedID != target.ID {
		t.Fatalf("⌘B should activate the pinned item, lastCopiedID=%q", a.lastCopiedID)
	}
}

func TestEveryAvailablePinActivatesWithPrimaryModifier(t *testing.T) {
	a := popupTestApp(t)
	target := a.vitems[0]
	choices := a.availablePinsLocked()
	a.togglePin(target)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	for _, pin := range choices {
		a.mu.Lock()
		if pin != target.Pin && !a.updatePinKeyLocked(target, pin) {
			a.mu.Unlock()
			t.Fatalf("available pin %q was rejected", pin)
		}
		a.rebuildLocked()
		a.mu.Unlock()
		a.lastCopiedID = ""
		tt.Frame()
		key, ok := keyFromName(pin)
		if !ok {
			t.Fatalf("available pin %q has no key", pin)
		}
		tt.Key(ui.Cmd, key)
		if a.lastCopiedID != target.ID {
			t.Fatalf("available pin %q was intercepted instead of activating its item", pin)
		}
	}
}

func TestPopupRowHintFollowsModifiers(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 500)
	a.togglePin(a.vitems[0])
	tt.Frame()
	char := strings.ToUpper(a.vitems[0].Pin)
	if !tt.HasText("⌘" + char) {
		t.Fatalf("expected pinned shortcut hint: %v", tt.Texts())
	}
	tt.HoldModifiers(ui.Alt)
	if !tt.HasText("⌥" + char) {
		t.Fatalf("expected Option pinned shortcut: %v", tt.Texts())
	}
	tt.HoldModifiers(0)
	if !tt.HasText("⌘" + char) {
		t.Fatal("hint should revert after releasing Option")
	}
}

func clipboardHasImage(it *model.Item) bool {
	for _, c := range it.Contents {
		if len(c.Type) >= 6 && c.Type[:6] == "image/" {
			return true
		}
	}
	return false
}
