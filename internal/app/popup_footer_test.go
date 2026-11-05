package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

func footerLayoutFixture(t *testing.T, icons bool, width int) (*App, *ui.Tester) {
	t.Helper()
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) {
		c.OpenPreviewAutomatically = false
		c.ShowApplicationIcons = icons
		c.WindowWidth = width
		c.WindowHeight = 360
	})
	// A bounded image gives both icon and thumbnail rows a real bitmap.
	imageSource := image.NewRGBA(image.Rect(0, 0, 60, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 60; x++ {
			imageSource.Set(x, y, color.RGBA{40, 80, 120, 255})
		}
	}
	bitmap := ui.NewBitmap(imageSource)
	a.appIcons["test.application"] = bitmap
	for i := 0; i < 24; i++ {
		item := makeItem(t, a, fmt.Sprintf("history item %02d", i), fmt.Sprintf("body %d", i))
		item.Application = "test.application"
		if i%3 == 0 {
			item.Contents[0].Type = "image/png"
		}
		a.addItem(item)
		if i%3 == 0 {
			a.images[item.ID] = bitmap
		}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		theme := *c.Theme()
		theme.Text = ui.RGB(255, 0, 0)
		theme.TextMuted = ui.RGB(0, 255, 0)
		c.SetTheme(&theme)
		a.popupViewPanel(c)
	}, width, 360)
	tt.Key(0, ui.KeyEnd)
	tt.Advance(time.Second)
	return a, tt
}

func footerInkLeft(t *testing.T, tt *ui.Tester, label string) int {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing footer action %q", label)
	}
	for x := int(r.X); x < int(r.X+r.W); x++ {
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			r, g, b, _ := tt.Image().At(x, y).RGBA()
			if r>>8 > 180 && g>>8 < 50 && b>>8 < 50 {
				return x
			}
		}
	}
	t.Fatalf("footer label %q has no visible text", label)
	return -1
}

func TestPopupFooterLeadingInsetFollowsApplicationIcons(t *testing.T) {
	for _, width := range []int{320, 450, 600} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			_, plain := footerLayoutFixture(t, false, width)
			_, icons := footerLayoutFixture(t, true, width)
			for _, label := range []string{"清除", "偏好设置…", "关于", "退出"} {
				without := footerInkLeft(t, plain, label)
				with := footerInkLeft(t, icons, label)
				if without-with != 5 {
					t.Errorf("%s: footer text x=%d with icons and %d without; want 5pt leading difference", label, with, without)
				}
			}
		})
	}
}

func footerSelectionSpan(t *testing.T, tt *ui.Tester, y int) (int, int) {
	t.Helper()
	left, right := -1, -1
	for x := 0; x < tt.Image().Bounds().Dx(); x++ {
		r, g, b, _ := tt.Image().At(x, y).RGBA()
		if b>>8 > 180 && b > r+50*257 && b > g+40*257 {
			if left < 0 {
				left = x
			}
			right = x + 1
		}
	}
	if left < 0 {
		t.Fatalf("no selected row at y=%d", y)
	}
	return left, right
}

func TestPopupFooterRowsAndShortcutsAlignWithHistory(t *testing.T) {
	for _, width := range []int{320, 450, 600} {
		for _, icons := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/icons=%v", width, icons), func(t *testing.T) {
				a, tt := footerLayoutFixture(t, icons, width)
				if a.selected != len(a.vitems)-1 {
					t.Fatal("fixture did not select the last history item")
				}
				clear, ok := tt.Find("清除")
				if !ok {
					t.Fatal("missing clear action")
				}
				// The last item is an image row; use its center to measure the
				// actual painted history selection, rather than a text box.
				left, right := footerSelectionSpan(t, tt, int(clear.Y)-popupSepPad-1-25)
				for i, label := range []string{"清除", "偏好设置…", "关于", "退出"} {
					row, ok := tt.Find(label)
					if !ok || row.X != float32(left) || row.X+row.W != float32(right) || row.H != popupRowHeight || row.Y != clear.Y+float32(i*popupRowHeight) {
						t.Fatalf("%s row misaligned: %+v, history spans %d..%d", label, row, left, right)
					}
				}
				for _, tc := range []struct{ label, shortcut string }{{"清除", "⌥⌘⌫"}, {"偏好设置…", "⌘,"}, {"退出", "⌘Q"}} {
					row, _ := tt.Find(tc.label)
					hint, ok := tt.Find(tc.shortcut)
					if !ok || math.Abs(float64(hint.X+hint.W-(row.X+row.W-10))) > 0.01 || math.Abs(float64(hint.Y+hint.H/2-(row.Y+row.H/2))) > 0.01 {
						t.Fatalf("%s shortcut misaligned: row=%+v shortcut=%+v", tc.label, row, hint)
					}
				}
				quit, _ := tt.Find("退出")
				if gap := 360 - 1 - (quit.Y + quit.H); gap != popupPadX {
					t.Fatalf("bottom padding = %v, want %d", gap, popupPadX)
				}
				// The divider has the same symmetric inset as the original.
				y := int(clear.Y) - popupSepPad - 1
				ink := tt.Image().RGBAAt(width/2, y)
				dividerLeft, dividerRight := -1, -1
				for x := left + 1; x < right-1; x++ {
					if tt.Image().RGBAAt(x, y) == ink {
						if dividerLeft < 0 {
							dividerLeft = x
						}
						dividerRight = x + 1
					}
				}
				if dividerLeft != left+popupSepPad || dividerRight != right-popupSepPad {
					t.Fatalf("divider spans %d..%d, row spans %d..%d", dividerLeft, dividerRight, left, right)
				}
				tt.HoldModifiers(ui.Shift)
				all, ok := tt.Find("全部清除")
				if !ok || all != clear {
					t.Fatalf("Clear All changed row geometry: before=%+v after=%+v", clear, all)
				}
				hint, ok := tt.Find("⌥⇧⌘⌫")
				if !ok || hint.X+hint.W != all.X+all.W-10 {
					t.Fatalf("Clear All shortcut lost trailing alignment: %+v", hint)
				}
			})
		}
	}
}

func TestPopupFooterWholeRowRemainsClickable(t *testing.T) {
	a := newTestApp(t)
	hits := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().PaddingX(popupPadX).Children(func() {
			a.footerRow(c, c.Theme(), footerAction{label: "test action", shortcut: "⌘K", fn: func() { hits++ }}, 0)
		})
	}, 450, 60)
	r, ok := tt.Find("test action")
	if !ok {
		t.Fatal("missing action")
	}
	for _, x := range []float32{r.X + 1, r.X + r.W/2, r.X + r.W - 1} {
		tt.ClickAt(x, r.Y+r.H/2)
	}
	if hits != 3 {
		t.Fatalf("full-row clicks activated %d times, want 3", hits)
	}
}
