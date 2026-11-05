package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/egoist/mygo/ui"
)

func TestPopupEmptyResultsKeepFooterPosition(t *testing.T) {
	a := popupTestApp(t)
	a.previewOpen = true
	tt := ui.NewTester(a.popupViewPanel, 851, 300)
	before, ok := tt.Find("偏好设置…")
	if !ok {
		t.Fatal("missing footer")
	}
	tt.Type("codex-no-match")
	after, ok := tt.Find("偏好设置…")
	if !ok || after.Y != before.Y {
		t.Fatalf("empty results moved footer from %v to %v", before, after)
	}
}

func TestPopupTypingKeepsWindowHeight(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.OpenPreviewAutomatically = false })
	for i := 0; i < 16; i++ {
		a.addItem(makeItem(t, a, fmt.Sprintf("record-%02d", i), fmt.Sprint(i)))
	}
	a.previewOpen = true
	a.reflow()
	height := a.lastPopupH
	tt := ui.NewTester(a.popupViewPanel, 851, height)
	for _, s := range []string{"record-01", "codex", "codex-no-match", "record-0", ""} {
		if err := tt.Click("搜索"); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Super, ui.KeyA)
		tt.Key(0, ui.KeyBackspace)
		tt.Type(s)
		if got := a.lastPopupH; got != height {
			t.Fatalf("typing %q resized popup from %d to %d", s, height, got)
		}
	}
}

func TestPopupRestingFooterPointerDoesNotClosePreview(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 851, 300)
	r, ok := tt.Find("偏好设置…")
	if !ok {
		t.Fatal("missing footer")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	// Select a history item while the pointer remains over the footer.
	tt.Key(0, ui.KeyHome)
	tt.Key(ui.Ctrl, ui.KeySpace)
	tt.Frame()
	if !a.previewOpen {
		t.Fatal("stationary footer pointer undid the preview toggle")
	}
	tt.Type("codex")
	tt.Frame()
	if !a.previewOpen {
		t.Fatal("typing under a stationary pointer closed the preview")
	}
}

func TestPopupSidebarDoesNotRewrapWhileResizing(t *testing.T) {
	a := newTestApp(t)
	text := "A sidebar paragraph long enough to wrap into multiple lines. Its line breaks should stay fixed while the sidebar is closing and reopening."
	a.addItem(makeItem(t, a, "sidebar", text))
	a.previewOpen = true
	tt := ui.NewTester(a.popupViewPanel, 851, 380)
	before, ok := tt.Find(text)
	if !ok {
		t.Fatal("missing preview paragraph")
	}
	a.previewOpen = false
	a.previewClosing = true
	for _, width := range []int{750, 650, 550, 475, 650, 851} {
		tt.SetSize(width, 380)
		tt.Frame()
		after, ok := tt.Find(text)
		// Find reports the visible (clipped) width; the paragraph's height
		// must still reflect its original line breaks at every viewport size.
		if !ok || after.X != before.X || after.Y != before.Y || after.H != before.H {
			t.Fatalf("sidebar rewrapped at width %d: before=%v after=%v found=%v", width, before, after, ok)
		}
	}
}

func TestPopupSearchFieldClearButton(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Type("beta")
	if a.query != "beta" {
		t.Fatalf("query = %q", a.query)
	}
	tt.Frame()
	if err := tt.Click("清除搜索"); err != nil {
		t.Fatalf("click clear: %v", err)
	}
	if a.query != "" {
		t.Fatalf("clear left %q", a.query)
	}
	if !tt.Focused("搜索") {
		t.Fatal("clear button should keep the search field focused")
	}
}

func TestPopupSearchEscapeDoesNotEditQueryBeforeClosing(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Type("beta")
	tt.Key(0, ui.KeyEscape)
	// There is no native panel in this tester. Escape requests close;
	// only the native OnClosed callback resets the query, as in Maccy.
	if a.query != "beta" {
		t.Fatalf("Escape edited the query instead of closing: %q", a.query)
	}
}

func TestPopupSearchResetsScrollToTop(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) {
		c.OpenPreviewAutomatically = false
		c.WindowHeight = 320
	})
	for i := 0; i < 40; i++ {
		s := "row-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		a.addItem(makeItem(t, a, s, s))
	}
	tt := ui.NewTester(a.popupViewPanel, 450, 320)
	tt.Scroll(225, 160, 0, 600)
	tt.Advance(300 * time.Millisecond)
	if first, _ := a.list.Visible(); first == 0 {
		t.Skip("list did not scroll")
	}
	tt.Move(200, 22) // move to the search field, as a person does before typing
	tt.Type("row-A")
	tt.Frame()
	first, _ := a.list.Visible()
	if a.selected != 0 || first != 0 {
		t.Fatalf("search left selection at %d, first visible %d", a.selected, first)
	}
}

func TestPopupHiddenSearchStillFilters(t *testing.T) {
	a := popupTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.SearchVisibility = config.SearchDuringSearch })
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Type("beta")
	if a.query != "beta" {
		t.Fatalf("query = %q", a.query)
	}
	if !tt.HasText("beta two") {
		t.Fatalf("hidden field did not filter: %v", tt.Texts())
	}
}
