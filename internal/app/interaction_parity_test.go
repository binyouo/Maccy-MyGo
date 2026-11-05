package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func TestHistoryActionsRequireExactModifiers(t *testing.T) {
	cfg := *config.Default()
	for _, mods := range []ui.Modifiers{ui.Ctrl | ui.Super, ui.Ctrl | ui.Alt, ui.Ctrl | ui.Alt | ui.Shift} {
		if got := itemAction(cfg, mods); got != actUnknown {
			t.Fatalf("unexpected action for extra modifiers %v: %v", mods, got)
		}
	}
}

func TestKeyChordNavigationMatchesReference(t *testing.T) {
	for _, tc := range []struct {
		name string
		mods ui.Modifiers
		key  ui.Key
		want int
	}{
		{"Control-N", ui.Ctrl, ui.KeyN, 6}, {"Control-J", ui.Ctrl, ui.KeyJ, 6},
		{"Control-P", ui.Ctrl, ui.KeyP, 4}, {"Control-K", ui.Ctrl, ui.KeyK, 4},
		{"Shift-Down", ui.Shift, ui.KeyDown, 6}, {"Shift-Up", ui.Shift, ui.KeyUp, 4},
		{"PageDown", 0, ui.KeyPageDown, 19}, {"PageUp", 0, ui.KeyPageUp, 0},
		{"Command-Down", ui.Cmd, ui.KeyDown, 19}, {"Option-Up", ui.Alt, ui.KeyUp, 0},
		{"Control-Option-N", ui.Ctrl | ui.Alt, ui.KeyN, 19},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := popupTestApp(t)
			for i := 0; i < 17; i++ {
				a.addItem(makeItem(t, a, fmt.Sprint("nav-", i), fmt.Sprint("nav-", i)))
			}
			a.selected = 5
			tt := ui.NewTester(a.popupViewPanel, 450, 400)
			tt.Key(tc.mods, tc.key)
			if a.selected != tc.want {
				t.Fatalf("selected %d, want %d", a.selected, tc.want)
			}
		})
	}
}

func TestKeyChordLastAndFooterCycle(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Key(0, ui.KeyPageDown)
	if a.selected != 2 {
		t.Fatal("PageDown did not reach last history item")
	}
	tt.Key(0, ui.KeyPageDown)
	if a.footerIndex != 0 {
		t.Fatal("second PageDown did not enter footer")
	}
	tt.Key(0, ui.KeyPageDown)
	if a.footerIndex != 3 {
		t.Fatal("PageDown in footer did not reach last action")
	}
	tt.Key(0, ui.KeyDown)
	if a.footerIndex != 0 {
		t.Fatal("footer navigation did not cycle to first action")
	}
}

func TestKeyChordSearchEditing(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Type("hello e\u0301")
	tt.Key(ui.Ctrl, ui.KeyH)
	if a.query != "hello " {
		t.Fatalf("Control-H split a grapheme: %q", a.query)
	}
	tt.Type("world")
	tt.Key(ui.Ctrl, ui.KeyW)
	if a.query != "hello " {
		t.Fatalf("Control-W query=%q", a.query)
	}
	tt.Key(ui.Ctrl, ui.KeyU)
	if a.query != "" {
		t.Fatalf("Control-U query=%q", a.query)
	}
}

func TestRichTextEditorPreservesAliasAndDropsStaleFormatsOnEdit(t *testing.T) {
	a := popupTestApp(t)
	it := makeItem(t, a, "alias", "plain body")
	h, n, err := a.store.PutBlob([]byte(`{\rtf1\ansi rich body}`))
	if err != nil {
		t.Fatal(err)
	}
	it.Contents = append(it.Contents, model.Content{Type: "text/rtf", Hash: h, Size: n})
	a.addItem(it)
	a.beginEdit(it)
	a.editTitle = ""
	a.saveEdit()
	if it.Title != "" || len(it.Contents) != 2 {
		t.Fatal("alias-only edit removed formatting or rejected an empty alias")
	}
	a.beginEdit(it)
	a.editContent = "changed body"
	a.saveEdit()
	if len(it.Contents) != 1 || it.Contents[0].Type != "text/plain" || a.itemText(it) != "changed body" {
		t.Fatal("content edit retained stale rich-text representations")
	}
}

func TestRichTextOnlyPreviewAndEditor(t *testing.T) {
	a := popupTestApp(t)
	h, n, err := a.store.PutBlob([]byte(`{\rtf1\ansi hello \b world\b0}`))
	if err != nil {
		t.Fatal(err)
	}
	it := &model.Item{Title: "short alias", Contents: []model.Content{{Type: "text/rtf", Hash: h, Size: n}}}
	a.addItem(it)
	a.beginEdit(it)
	if !a.editIsText || a.editContent != "hello world" {
		t.Fatalf("rich-only editor=%q editable=%v", a.editContent, a.editIsText)
	}
}

func TestIgnoreRegexValidationAcceptsLookbehind(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefs.IgnoreRegexp = "old"
	if err := a.editIgnoredLine(&a.prefs.IgnoreRegexp, 0, `(?<=foo)bar`, true); err != "" {
		t.Fatal(err)
	}
}

func TestCustomKeyChordPriority(t *testing.T) {
	a := popupTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.DeleteShortcut = "Ctrl+H"; c.PreviewShortcut = "Ctrl+N" })
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Type("bet")
	tt.Key(ui.Ctrl, ui.KeyN)
	if a.selected != 1 || a.previewOpen {
		t.Fatal("preview shortcut took priority over built-in navigation")
	}
	tt.Key(ui.Ctrl, ui.KeyH)
	if len(a.items) != 2 || a.query != "bet" {
		t.Fatal("search backspace took priority over the configured delete action")
	}
}

func TestNoResultReturnClearsSearchWithoutClosingView(t *testing.T) {
	a := popupTestApp(t)
	tt := ui.NewTester(a.popupViewPanel, 450, 400)
	tt.Type("no-result-parity")
	tt.Key(0, ui.KeyEnter)
	if a.query != "" || !tt.HasText("better beta") {
		t.Fatal("no-result Return did not reset search")
	}
	tt.Type("alpha")
	if a.query != "alpha" {
		t.Fatal("search editor retained the old query after Return")
	}
}

func TestEditorReturnAndNoChangeGuard(t *testing.T) {
	a := popupTestApp(t)
	it := a.vitems[0]
	a.beginEdit(it)
	tt := ui.NewTester(a.popupViewPanel, 560, 380)
	tt.Key(ui.Cmd, ui.KeyEnter)
	if a.editing != it {
		t.Fatal("unchanged editor closed through a disabled save action")
	}
	if a.editorKey(ui.TextKey{Key: ui.KeyEnter, Mods: ui.Cmd | ui.Shift}) {
		t.Fatal("extra modifiers triggered save")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("new alias")
	tt.Key(0, ui.KeyEnter)
	if a.editing != nil || it.Title != "new alias" {
		t.Fatal("Return in the alias field did not save")
	}
}

func TestLongParagraphEditorCannotOverwriteOriginal(t *testing.T) {
	a := popupTestApp(t)
	body := strings.Repeat("a", 10001)
	it := makeItem(t, a, "long text", body)
	a.addItem(it)
	a.beginEdit(it)
	if !a.editTooLarge || len(a.editContent) != 10000 {
		t.Fatal("oversized paragraph was not protected")
	}
	a.editTitle = "renamed"
	a.editContent = "attempted truncated edit"
	a.saveEdit()
	if it.Title != "renamed" || a.itemText(it) != body {
		t.Fatal("alias save overwrote the protected original content")
	}
}

func TestStartupTrimsLoadedHistoryAndKeepsPins(t *testing.T) {
	a := popupTestApp(t)
	pin := a.items[0]
	a.togglePin(pin)
	_ = a.cfg.Update(func(c *config.Config) { c.HistorySize = 1 })
	reloaded := New(a.cfg, a.store)
	if len(reloaded.items) != 2 || !pin.IsPinned() {
		t.Fatal("startup did not trim ordinary history while retaining pins")
	}
}

func TestPinSymbolKeysRoundTrip(t *testing.T) {
	for _, ch := range []string{"[", "]", ";", "'", "\\", ".", "/", "-", "="} {
		key, ok := keyFromName(ch)
		if !ok {
			t.Fatalf("missing pin key %q", ch)
		}
		name := keyName(key)
		again, ok := keyFromName(name)
		if !ok || again != key {
			t.Fatalf("shortcut recorder cannot round-trip %q", ch)
		}
	}
}

func TestMenuBarTextUsesContentAndCompleteCharacters(t *testing.T) {
	a := popupTestApp(t)
	a.addItem(makeItem(t, a, "custom alias", "  first\nsecond\u2028third  "))
	if got := a.recentTitle(); got != "firstsecondthird" {
		t.Fatalf("menu bar used alias or retained newlines: %q", got)
	}
	text := strings.Repeat("👩‍💻", 21)
	if got := firstCharacters(text, 20); got != strings.Repeat("👩‍💻", 20) {
		t.Fatal("menu bar limit split an emoji sequence")
	}
	if got := dropLastGrapheme(text); got != strings.Repeat("👩‍💻", 20) {
		t.Fatal("Control-H split a repeated emoji sequence")
	}
}

func TestDeletedHistoryDoesNotRetainPreviewBitmaps(t *testing.T) {
	a := popupTestApp(t)
	it := a.items[0]
	a.images[it.ID] = nil
	a.imageLoading[it.ID] = true
	a.imageBytes = map[string]int64{it.ID: 123}
	a.deleteItem(it)
	if _, ok := a.images[it.ID]; ok {
		t.Fatal("deleted history retained a bitmap cache entry")
	}
	if a.imageLoading[it.ID] || a.imageBytes[it.ID] != 0 {
		t.Fatal("deleted image retained in-flight preview metadata")
	}
}

func TestKeyboardScrollDoesNotSelectUnderRestingPointer(t *testing.T) {
	a := popupTestApp(t)
	for i := 0; i < 30; i++ {
		a.addItem(makeItem(t, a, fmt.Sprint("rest-", i), fmt.Sprint("rest-", i)))
	}
	tt := ui.NewTester(a.popupViewPanel, 450, 320)
	r, ok := tt.Find(a.vitems[1].Title)
	if !ok {
		t.Fatal("missing visible row")
	}
	tt.Move(r.X+20, r.Y+r.H/2)
	tt.Key(0, ui.KeyEnd)
	tt.Frame()
	if a.selected != len(a.vitems)-1 {
		t.Fatalf("resting pointer stole keyboard selection after scroll: %d", a.selected)
	}
}
