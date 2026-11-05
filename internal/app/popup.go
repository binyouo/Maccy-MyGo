package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

var (
	// Maccy paints the selection with the accent color at 80%.
	selColor = ui.Hex("#0A84FF").Alpha(0.8)
	selText  = ui.RGB(255, 255, 255)
)

// Layout mirrors Maccy's Popup constants (the macOS ≤15 values).
const (
	popupRowHeight = 22
	popupRowRadius = 4
	popupPadX      = 5
	popupSepPad    = 6
)

type popupSnapshot struct {
	cfg         config.Config
	items       []*model.Item
	ranges      map[string][]search.Range
	shortcuts   map[string]string
	selected    int
	pinnedCount int
	query       string
}

func (a *App) snapshotPopup() popupSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := append([]*model.Item(nil), a.vitems...)
	ranges := make(map[string][]search.Range, len(a.ranges))
	for k, v := range a.ranges {
		ranges[k] = v
	}
	shortcuts := make(map[string]string, len(a.shortcuts))
	for k, v := range a.shortcuts {
		shortcuts[k] = v
	}
	return popupSnapshot{
		cfg:         a.popupConfig(),
		items:       items,
		ranges:      ranges,
		shortcuts:   shortcuts,
		selected:    a.selected,
		pinnedCount: a.pinnedCount,
		query:       a.query,
	}
}

// popupViewPanel builds the popup on its regular state, for the panel
// and the headless tests.
func (a *App) popupViewPanel(c *ui.Context) { a.popupView(c, &a.list) }

// view builds the popup interface; each popup host owns its ListState —
// a state shown by two Lists at once panics.
func (a *App) popupView(c *ui.Context, list *ui.ListState) {
	s := a.snapshotPopup()
	t := c.Theme()
	a.reduceMotion = c.Preferences().ReduceMotion
	c.Root().Background(ui.Transparent)

	box := ui.Column(c).Fill()
	if !c.Vibrancy() {
		box.Background(t.Surface).Radius(10).Border(1, t.Border)
	}
	box.Gap(0).Children(func() {
		if a.editing != nil {
			a.editorView(c, t, s)
			return
		}
		main := func() {
			a.header(c, t, s)
			a.listArea(c, t, s, list)
			if s.cfg.ShowFooter {
				a.footer(c, t)
			}
		}
		// Keep the list and search field in the same parent when the sidebar
		// toggles, preserving their focus, selection and scroll position.
		host := a.panel.Load()
		if list == &a.devList {
			host = a.devWin
		}
		left := a.popupLeft(host)
		row := ui.Row(c).Key("browser").Fill().Transition(paneTransition)
		if left {
			row.Reverse()
		}
		row.Children(func() {
			width := s.cfg.WindowWidth
			if width <= 0 {
				width = 450
			}
			ui.Column(c).Key("history").Width(float32(width)).Shrink(0).FillHeight().Padding(0, popupPadX).Children(main)
			if a.previewOpen || a.previewClosing {
				// Reveal a fixed-width sidebar through a changing viewport.
				// Its text and toolbar keep their layout while the host resizes.
				viewport := ui.Row(c).Key("preview-viewport").Grow(1).FillHeight().Clip()
				if left {
					viewport.Reverse()
				}
				viewport.Children(func() {
					a.previewDivider(c, t, host, left)
					a.preview(c, t, s)
				})
			}
		})
	})
}

func (a *App) listArea(c *ui.Context, t *ui.Theme, s popupSnapshot, list *ui.ListState) {
	ui.Column(c).Key("results").Grow(1).FillWidth().Clip().Children(func() {
		if len(s.items) == 0 {
			msg := "剪贴板暂无内容"
			if s.query != "" {
				msg = "无结果"
			}
			ui.Text(c, msg).FontSize(12).TextColor(t.TextMuted).Padding(16)
			return
		}
		a.listView(c, t, s, list)
	})
}

// preview renders the slideout pane as Maccy's SlideoutContentView does:
// the item's toolbar on top, the content, then a metadata block pinned to
// the bottom.
func (a *App) preview(c *ui.Context, t *ui.Theme, s popupSnapshot) {
	var it *model.Item
	if s.selected >= 0 && s.selected < len(s.items) {
		it = s.items[s.selected]
	}
	width := s.cfg.PreviewWidth
	if width <= 0 {
		width = 400
	}
	ui.Column(c).Key("preview").Width(float32(width)).Shrink(0).FillHeight().Clip().Padding(popupPadX, 10, 10, 10).Children(func() {
		a.previewToolbar(c, t, it)
		if it == nil {
			ui.Text(c, "选择条目以预览").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			return
		}
		ui.Column(c).Key(it.ID).Grow(1).FillWidth().Clip().Children(func() {
			if clipboard.HasImage(it) {
				if bmp := a.bitmapFor(it); bmp != nil {
					ui.Image(c, bmp).Fit(ui.Contain).Radius(5).Grow(1)
				}
			} else {
				ui.Scroll(c).Grow(1).Children(func() {
					text, _ := sizedPreviewText(a.itemText(it))
					ui.Text(c, text).FontSize(12).Wrap()
				})
			}
			ui.Spacer(c)
			ui.Divider(c).Margin(0, 0, 10, 0)
			meta := func(k, v string) {
				ui.Row(c).Gap(4).Children(func() {
					ui.Text(c, k).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, v).FontSize(11).Wrap().Grow(1)
				})
			}
			if it.ApplicationName != "" || it.Application != "" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "应用").FontSize(11).TextColor(t.TextMuted)
					if bmp := a.appIcon(it.Application); bmp != nil {
						ui.Image(c, bmp).Size(11, 11)
					}
					name := it.ApplicationName
					if name == "" {
						name = it.Application
					}
					ui.Text(c, name).FontSize(11).Wrap().Grow(1)
				})
			}
			if clipboard.HasImage(it) {
				if bmp := a.bitmapFor(it); bmp != nil {
					w, h := bmp.Size()
					meta("尺寸", fmt.Sprintf("%d×%d", w, h))
				}
				meta("大小", humanBytes(a.imageByteSize(it)))
			}
			meta("首次复制", it.FirstCopiedAt.Format("2006-01-02 15:04"))
			meta("最近复制", it.LastCopiedAt.Format("2006-01-02 15:04"))
			meta("复制次数", fmt.Sprintf("%d", it.NumberOfCopies))
			if it.Pin != "" {
				meta("置顶键", it.Pin)
			}
		})
	})
}

// previewToolbar draws the actions of Maccy's ToolbarView: copy the
// recognized text of an image, pin, edit and delete the selected item.
func (a *App) previewToolbar(c *ui.Context, t *ui.Theme, it *model.Item) {
	if it == nil {
		return
	}
	cfg := a.cfg.Snapshot()
	ui.Row(c).Height(26).AlignItems(ui.Center).Gap(4).Children(func() {
		ui.Spacer(c)
		if clipboard.HasImage(it) && strings.TrimSpace(it.Title) != "" {
			toolbarButton(c, "ocr", iconTextFinder, "复制识别文本").Tooltip("复制识别出的文本").OnClick(func() {
				mygo.Clipboard.WriteText(it.Title)
			})
		}
		pinIcon := iconPin
		pinAction := "置顶"
		if it.IsPinned() {
			pinIcon = iconPinSlash
			pinAction = "取消置顶"
		}
		pin := toolbarButton(c, "pin", pinIcon, "置顶/取消置顶").Tooltip(shortcutTooltip(pinAction, cfg.PinShortcut))
		pin.OnClick(func() {
			if pin.Clicks()%2 != 0 {
				a.togglePin(it)
			}
			a.focusSearch = true
			a.refresh()
		})
		toolbarButton(c, "edit", iconPencil, "编辑条目").Tooltip("编辑条目").OnClick(func() { a.beginEdit(it) })
		toolbarButton(c, "delete", iconTrash, "删除条目").Tooltip(shortcutTooltip("删除", cfg.DeleteShortcut)).OnClick(func() {
			a.deleteItem(it)
			a.focusSearch = true
			a.refresh()
		})
	})
}

// Give icon actions a complete button hit target, keyboard activation and
// visible hover, pressed and focus feedback.
func toolbarButton(c *ui.Context, key string, icon *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Key(key).Size(26, 24).Radius(4).Cursor(ui.CursorPointer).Label(label)
	background := ui.Transparent
	if b.Hovered() {
		background = t.SurfaceHover
	}
	if b.Pressed() {
		background = t.Accent.Alpha(0.2)
	}
	if b.Focused() {
		b.Border(1, t.Accent)
	}
	b.Background(background).Transition(ui.ElementTransition{Duration: 100 * time.Millisecond, Colors: true})
	b.Children(func() { ui.Icon(c, icon).Size(14, 14).TextColor(t.TextMuted).PassThrough() })
	return b
}

func contentSize(it *model.Item) int64 {
	var n int64
	for _, c := range it.Contents {
		n += int64(c.Size)
	}
	return n
}

func (a *App) imageByteSize(it *model.Item) int64 {
	a.imageMu.Lock()
	defer a.imageMu.Unlock()
	if n, ok := a.imageBytes[it.ID]; ok {
		return n
	}
	return contentSize(it)
}

// History owns bitmap lifetimes, including duplicate replacements and
// capacity eviction; a removed item's thumbnail must not grow the cache.
func (a *App) pruneImageCacheLocked() {
	live := make(map[string]bool, len(a.items))
	for _, it := range a.items {
		live[it.ID] = true
	}
	a.imageMu.Lock()
	defer a.imageMu.Unlock()
	for id := range a.images {
		if !live[id] {
			delete(a.images, id)
			delete(a.imageBytes, id)
			delete(a.imageFailedAt, id)
		}
	}
	for id := range a.imageLoading {
		if !live[id] {
			delete(a.imageLoading, id)
		}
	}
}

func (a *App) itemText(it *model.Item) string {
	return clipboard.Text(a.store, it)
}

// togglePreview shows or hides the preview pane.
func (a *App) togglePreview() {
	a.togglePreviewClicks(1)
}

func (a *App) requestPreview(open bool) {
	if a.previewRequestedOpen() != open {
		a.togglePreview()
	}
}

func (a *App) previewRequestedOpen() bool { return a.previewOpen != a.previewQueued }

func (a *App) togglePreviewClicks(clicks int) {
	if clicks <= 0 {
		return
	}
	// Manual suppression applies to the current item, not the next one.
	a.previewSelectionID = a.selectedPreviewID()
	a.previewAutoQueuedFor = ""
	a.previewGen.Add(1)
	a.cancelAutoPreview()
	// Requests arriving within one transition cancel in pairs. Complete
	// the visible transition before applying the final pending intent.
	if clicks%2 != 0 {
		a.previewQueued = !a.previewQueued
	}
	a.previewAutoSuppressed = !a.previewRequestedOpen()
	a.focusSearch = true
	a.applyQueuedPreview()
	a.refresh()
}

func (a *App) applyQueuedPreview() {
	if !a.previewQueued || a.popupMotionActive() || a.editing != nil {
		return
	}
	a.previewQueued = false
	a.previewAutoQueuedFor = ""
	if !a.previewOpen && a.selectedPreviewID() == "" {
		return
	}
	a.previewOpen = !a.previewOpen
	a.previewClosing = !a.previewOpen
	a.reflow()
}

func (a *App) header(c *ui.Context, t *ui.Theme, s popupSnapshot) {
	visible := s.cfg.ShowSearch && (s.cfg.SearchVisibility != config.SearchDuringSearch || s.query != "")
	if !visible {
		inp := ui.TextInput(c, &a.query).Width(1).Height(1).Opacity(0)
		inp.AutoFocus()
		if a.focusSearch {
			inp.Focus()
			a.focusSearch = false
		}
		inp.OnTextKey(a.handleKey)
		inp.OnChange(a.onQueryChanged)
		return
	}
	// Maccy's header holds only the search field and the preview toggle;
	// the item actions live in the slideout's toolbar.
	ui.Row(c).Height(28).AlignItems(ui.Center).Padding(5, 10, 0, 10).Children(func() {
		ui.Row(c).Grow(1).AlignItems(ui.Center).Padding(0, 5).Children(func() {
			if s.cfg.ShowTitle {
				ui.Text(c, "Maccy MyGo").FontSize(12).TextColor(t.TextMuted).Padding(0, 5, 0, 0)
			}
			a.searchField(c, t)
		})
		previewIcon := iconSidebarRight
		if a.previewOpen {
			previewIcon = iconSidebarLeft
		}
		preview := toolbarButton(c, "toggle-preview", previewIcon, "切换预览").Tooltip(shortcutTooltip("切换预览", s.cfg.PreviewShortcut))
		preview.OnClick(func() {
			// Several presses can arrive before the next frame is built.
			// Preserve their toggle parity instead of treating them as one.
			a.togglePreviewClicks(preview.Clicks())
		})
	})
}

// searchField draws Maccy's search field: a quiet rounded background with
// a magnifying glass, the editable query and a clear button. The icon and
// clear control live inside the field so the header does not lean on text
// glyphs (which fonts often lack) for its chrome.
func (a *App) searchField(c *ui.Context, t *ui.Theme) {
	var inp ui.Element
	row := ui.Row(c).Height(23).Grow(1).Gap(4).AlignItems(ui.Center).Padding(0, 5).
		Radius(popupRowRadius).Background(t.TextMuted.Alpha(0.1)).Cursor(ui.CursorText)
	row.Children(func() {
		ui.Icon(c, iconSearch).Size(11, 11).TextColor(t.TextMuted)
		inp = ui.TextInputBase(c, &a.query).Grow(1)
		inp.Label("搜索").Placeholder("搜索")
		inp.AutoFocus()
		if a.focusSearch {
			inp.Focus()
			a.focusSearch = false
		}
		inp.OnTextKey(a.handleKey)
		inp.OnChange(a.onQueryChanged)
		if a.query != "" {
			ui.Icon(c, iconClear).Size(11, 11).TextColor(t.TextMuted).Cursor(ui.CursorPointer).
				Label("清除搜索").Tooltip("清除搜索").OnClick(func() {
				a.query = ""
				inp.Focus()
				a.onQueryChanged()
			})
		}
	})
	row.OnClick(func() { inp.Focus() })
}

func (a *App) listView(c *ui.Context, t *ui.Theme, s popupSnapshot, list *ui.ListState) {
	var chosen func()
	hovered := -1
	list.Key = func(i int) any { return s.items[i].ID }
	list.KeepOffset = true
	if s.query == "" && s.pinnedCount >= 2 && s.cfg.PinTo == config.PinTop {
		pinned := s.pinnedCount
		list.Reorder = func(rows []int, to int) {
			if len(rows) != 1 || rows[0] >= pinned {
				return
			}
			if to > pinned {
				to = pinned
			}
			a.movePin(rows[0], to)
			a.refresh()
		}
	} else {
		list.Reorder = nil
	}
	// Pinned items sit as a block at the configured edge, separated from
	// the rest by a divider as Maccy's PinsView is.
	trailPins := 0
	if s.cfg.PinTo == config.PinBottom {
		for i := len(s.items) - 1; i >= 0 && s.items[i].IsPinned(); i-- {
			trailPins++
		}
	}
	sepAbove := func(i int) bool {
		switch s.cfg.PinTo {
		case config.PinTop:
			return s.pinnedCount > 0 && s.pinnedCount < len(s.items) && i == s.pinnedCount
		default:
			return trailPins > 0 && trailPins < len(s.items) && i == len(s.items)-trailPins
		}
	}
	ui.List(c, list, len(s.items), func(i int) {
		if sepAbove(i) {
			ui.Column(c).FillWidth().Children(func() {
				// Maccy's topSeparator pads the divider above only.
				ui.Box(c).Height(popupSepPad+1).Padding(popupSepPad, popupSepPad, 0, popupSepPad).Children(func() {
					ui.Divider(c)
				})
				a.row(c, t, s, i, &chosen, &hovered)
			})
			return
		}
		a.row(c, t, s, i, &chosen, &hovered)
	}).Grow(1)
	if chosen != nil {
		chosen()
	}
	if hovered >= 0 {
		// As Maccy's isKeyboardNavigating does, a resting pointer does not
		// steal the selection back while the keyboard is navigating; only
		// the pointer moving to another row does.
		if !a.kbdNav && (hovered != a.selected || a.footerIndex >= 0) {
			a.selected = hovered
			a.footerIndex = -1
			a.refresh()
		}
		a.lastHovered = hovered
	}
	if a.scrollToSelected {
		list.ScrollIntoView(s.selected)
		a.scrollToSelected = false
	}
}

// rowHint is the shortcut badge Maccy draws at the trailing edge of a row:
// the item's key with the modifiers that would act on it, switching live
// as modifier keys are pressed.
func (a *App) rowHint(c *ui.Context, it *model.Item, s popupSnapshot) string {
	var char string
	if it.IsPinned() {
		char = strings.ToUpper(it.Pin)
	} else if label := s.shortcuts[it.ID]; label != "" {
		char = label[len(label)-1:]
	} else {
		return ""
	}
	if char == "" {
		return ""
	}
	switch mods := c.Modifiers(); {
	case mods == ui.Alt:
		return "⌥" + char
	case mods == ui.Cmd|ui.Shift && s.cfg.PasteByDefault:
		return "⇧⌘" + char
	case mods == ui.Alt|ui.Shift && !s.cfg.PasteByDefault:
		return "⌥⇧" + char
	default:
		return "⌘" + char
	}
}

func (a *App) row(c *ui.Context, t *ui.Theme, s popupSnapshot, i int, chosen *func(), hovered *int) {
	it := s.items[i]
	selected := i == s.selected
	hasImage := clipboard.HasImage(it)
	row := ui.ButtonBase(c).Gap(0).AlignItems(ui.Center).Radius(popupRowRadius).FillWidth().Cursor(ui.CursorPointer).Label(it.Title)
	if hasImage {
		row.MinHeight(float32(s.cfg.ImageMaxHeight) + 10)
	} else {
		row.Height(popupRowHeight)
	}
	if selected {
		row.Background(selColor)
	} else if row.Hovered() {
		row.Background(ui.RGBA(127, 127, 127, 0.22))
	}
	if row.Hovered() {
		*hovered = i
	}
	row.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputPointerMove {
			// A different row can move under a stationary pointer when
			// keyboard navigation scrolls. Only a real mouse move leaves
			// keyboard mode, as Maccy's onMouseMove modifier does.
			a.kbdNav = false
			if a.selected != i || a.footerIndex >= 0 {
				a.selected, a.footerIndex = i, -1
				a.refresh()
			}
		}
		return false
	})
	row.Children(func() {
		if s.cfg.ShowApplicationIcons {
			if it.Application != "" {
				if bmp := a.appIcon(it.Application); bmp != nil {
					ui.Image(c, bmp).Size(15, 15).Padding(0, 0, 0, 4).PassThrough()
				}
			}
			ui.Box(c).Width(5).PassThrough()
		} else {
			ui.Box(c).Width(10).PassThrough()
		}
		if !hasImage && s.cfg.ShowHexColorSwatch {
			if col, ok := hexColor(it.Title); ok {
				ui.Box(c).Size(14, 14).Radius(3).Background(col).Margin(0, 5, 0, 0).PassThrough()
			}
		}
		ranges := s.ranges[it.ID]
		switch {
		case hasImage:
			if bmp := a.bitmapFor(it); bmp != nil {
				bw, bh := bmp.Size()
				iw, ih := thumbSize(bw, bh, s.cfg.ImageMaxHeight)
				ui.Image(c, bmp).Size(iw, ih).Radius(4).Margin(0, 5, 0, 0).PassThrough()
			} else {
				txt := ui.Text(c, it.Title).MaxLines(1).FontSize(13).Grow(1).Padding(0, 5, 0, 0).PassThrough()
				if selected {
					txt.TextColor(selText)
				}
			}
		case !selected && len(ranges) > 0:
			matchedTitle(c, t, it.Title, ranges, s.cfg.HighlightMatch)
		default:
			txt := ui.Text(c, it.Title).MaxLines(1).FontSize(13).Grow(1).Padding(0, 5, 0, 0).PassThrough()
			if selected {
				txt.TextColor(selText)
			}
		}
		ui.Spacer(c).PassThrough()
		if hint := a.rowHint(c, it, s); hint != "" {
			hintColor := t.TextMuted.Alpha(0.7)
			if selected {
				hintColor = selText.Alpha(0.7)
			}
			ui.Text(c, hint).FontSize(11).PassThrough().Padding(0, 10, 0, 0).TextColor(hintColor)
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("复制").Shortcut(ui.Cmd, ui.KeyC).Chosen() {
			*chosen = func() { a.copyOnly(it, false) }
		}
		if m.Item("粘贴").Shortcut(ui.Cmd, ui.KeyV).Chosen() {
			*chosen = func() { a.pasteOnly(it, false) }
		}
		if m.Item("粘贴并去除格式").Shortcut(ui.Cmd|ui.Shift, ui.KeyV).Chosen() {
			*chosen = func() { a.pasteOnly(it, true) }
		}
		m.Separator()
		pinLabel := "置顶"
		if it.IsPinned() {
			pinLabel = "取消置顶"
		}
		if m.Item(pinLabel).Checked(it.IsPinned()).Chosen() {
			*chosen = func() {
				a.togglePin(it)
				a.refresh()
			}
		}
		if m.Item("编辑…").Chosen() {
			*chosen = func() { a.beginEdit(it) }
		}
		m.Separator()
		if m.Item("删除").Shortcut(0, ui.KeyDelete).Chosen() {
			*chosen = func() {
				a.deleteItem(it)
				a.refresh()
			}
		}
	})
	row.OnClick(func() { a.activateIndex(i, row.ClickModifiers()) })
}

func (a *App) copyOnly(it *model.Item, removeFormatting bool) {
	a.closePopup()
	a.copyItem(it, removeFormatting)
}

func (a *App) pasteOnly(it *model.Item, removeFormatting bool) {
	a.closePopup()
	if !a.copyItem(it, removeFormatting) {
		return
	}
	go func() {
		time.Sleep(120 * time.Millisecond)
		_ = clipboard.Paste()
	}()
}

type footerAction struct {
	label    string
	shortcut string
	tip      string
	fn       func()
}

// footerActions mirrors Maccy's Footer: the first slot swaps between
// Clear and Clear All as Shift is held; the rest are fixed.
func (a *App) footerActions() []footerAction {
	clearAll := a.footerClearAll
	label, hint, tip, all := "清除", "⌥⌘⌫", "清除未置顶的项目。\n按住 ⇧ 以全部清除。", false
	if clearAll {
		label, hint, tip, all = "全部清除", "⌥⇧⌘⌫", "清除全部项目。", true
	}
	return []footerAction{
		{label, hint, tip, func() { a.confirmClear(all) }},
		{"偏好设置…", "⌘,", "", func() { a.OpenPreferences() }},
		{"关于", "", "查看更多应用信息。", func() { a.OpenAbout() }},
		{"退出", "⌘Q", "退出应用。", func() { mygo.App.Quit() }},
	}
}

// footer renders Maccy's FooterView: a divider, then the actions as
// full-width rows the keyboard can reach like the history rows.
func (a *App) footer(c *ui.Context, t *ui.Theme) {
	// Read the held modifiers during the build so the Clear/Clear All
	// swap repaints as Shift goes down and up.
	mods := c.Modifiers()
	a.footerClearAll = mods&ui.Shift != 0 && mods&^(ui.Cmd|ui.Alt|ui.Shift) == 0
	actions := a.footerActions()
	// Maccy's bottomSeparator pads the divider below only.
	ui.Box(c).FillWidth().Height(popupSepPad+1).Shrink(0).Padding(0, popupSepPad, popupSepPad, popupSepPad).Children(func() {
		ui.Divider(c)
	})
	for i, action := range actions {
		a.footerRow(c, t, action, i)
	}
	ui.Box(c).Height(popupPadX).Shrink(0)
}

func (a *App) footerRow(c *ui.Context, t *ui.Theme, action footerAction, index int) {
	selected := a.footerIndex == index
	// FooterItemView uses the same leading/trailing geometry as history
	// rows, including the smaller leading spacer when app icons are shown.
	leading := float32(10)
	if a.cfg.Snapshot().ShowApplicationIcons {
		leading = 5
	}
	row := ui.ButtonBase(c).Key(index).Gap(0).AlignItems(ui.Center).Height(popupRowHeight).Radius(popupRowRadius).FillWidth().Padding(0, 10, 0, leading).Cursor(ui.CursorPointer).Label(action.label)
	if action.tip != "" {
		row.Tooltip(action.tip)
	}
	if selected {
		row.Background(selColor)
	} else if row.Hovered() {
		row.Background(ui.RGBA(127, 127, 127, 0.22))
	}
	// Only a pointer movement chooses the footer. A search or a resize
	// can move a row under a resting pointer without the user entering it.
	row.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputPointerMove && (a.footerIndex != index || a.previewRequestedOpen()) {
			a.kbdNav = false
			if a.previewRequestedOpen() {
				// Maccy folds the preview away when the pointer reaches
				// the footer so the actions stay usable with a mouse.
				a.requestPreview(false)
			}
			if a.footerIndex != index {
				a.footerIndex = index
				a.selected = -1
				a.refresh()
			}
		}
		return false
	})
	row.Children(func() {
		txt := ui.Text(c, action.label).FontSize(13).PassThrough()
		if selected {
			txt.TextColor(selText)
		}
		ui.Spacer(c).PassThrough()
		if action.shortcut != "" {
			hintColor := t.TextMuted.Alpha(0.7)
			if selected {
				hintColor = selText.Alpha(0.7)
			}
			ui.Text(c, action.shortcut).FontSize(11).PassThrough().TextColor(hintColor)
		}
	})
	row.OnClick(action.fn)
}

// thumbSize fits a bw×bh picture in Maccy's 340×maxH thumbnail box,
// keeping the aspect and never sizing up. The row sets the size on the
// image explicitly: a bare Height leaves the flex base at the bitmap's
// pixel width, which the row shrinks to full width and Contain centers.
func thumbSize(bw, bh, maxH int) (w, h float32) {
	w, h = float32(bw), float32(bh)
	if bw > 0 && bh > 0 {
		r := min(1, 340/float32(bw), float32(maxH)/float32(bh))
		w, h = float32(bw)*r, float32(bh)*r
	}
	return w, h
}

func (a *App) bitmapFor(it *model.Item) *ui.Bitmap {
	a.imageMu.Lock()
	if b, ok := a.images[it.ID]; ok {
		if b != nil || time.Since(a.imageFailedAt[it.ID]) < time.Second {
			a.imageMu.Unlock()
			return b
		}
		// Remote Clipboard may publish a JPEG URL before the file arrives.
		// A later repaint/reopen can retry without an unbounded refresh loop.
		delete(a.images, it.ID)
	}
	if a.imageLoading[it.ID] {
		a.imageMu.Unlock()
		return nil
	}
	if !clipboard.HasImage(it) {
		a.imageMu.Unlock()
		return nil
	}
	a.imageLoading[it.ID] = true
	id := it.ID
	snapshot := *it
	snapshot.Contents = append([]model.Content(nil), it.Contents...)
	snapshot.Files = append([]string(nil), it.Files...)
	a.imageMu.Unlock()
	go func() {
		var bmp *ui.Bitmap
		var size int64
		if data, err := clipboard.ImageData(a.store, &snapshot); err == nil {
			size = int64(len(data))
			bmp, _ = ui.DecodeBitmap(data)
		}
		a.post(func() {
			a.mu.Lock()
			live := false
			for _, current := range a.items {
				if current.ID == id {
					live = true
					break
				}
			}
			a.mu.Unlock()
			a.imageMu.Lock()
			delete(a.imageLoading, id)
			if !live {
				a.imageMu.Unlock()
				return
			}
			a.images[id] = bmp
			if a.imageFailedAt == nil {
				a.imageFailedAt = map[string]time.Time{}
			}
			if bmp == nil {
				a.imageFailedAt[id] = time.Now()
			} else {
				delete(a.imageFailedAt, id)
			}
			if a.imageBytes == nil {
				a.imageBytes = map[string]int64{}
			}
			a.imageBytes[id] = size
			a.imageMu.Unlock()
			a.refresh()
		})
	}()
	return nil
}

// handleKey intercepts keys in the search field for navigation and actions.
func (a *App) handleKey(k ui.TextKey) bool {
	mods := k.Mods
	cfg := a.cfg.Snapshot()
	// Preserve KeyChord's precedence when user-configured keys overlap
	// its built-in editing/navigation commands.
	if k.Key == ui.KeyBackspace && (mods == ui.Cmd|ui.Alt || mods == ui.Cmd|ui.Alt|ui.Shift) {
		a.confirmClear(mods&ui.Shift != 0)
		return true
	}
	if k.Key == ui.KeyU && mods == ui.Ctrl {
		a.editSearchQuery(k, "")
		return true
	}
	if m, key, ok := parseShortcut(cfg.DeleteShortcut); ok && k.Key == key && mods == m {
		a.deleteSelected()
		return true
	}
	if mods == ui.Ctrl && (k.Key == ui.KeyH || k.Key == ui.KeyW) {
		text := ""
		if k.Key == ui.KeyH {
			text = dropLastGrapheme(a.query)
		} else {
			words := strings.FieldsFunc(a.query, func(r rune) bool { return r == ' ' })
			if len(words) > 1 {
				text = strings.Join(words[:len(words)-1], " ") + " "
			}
		}
		a.editSearchQuery(k, text)
		return true
	}
	switch {
	case (k.Key == ui.KeyDown || k.Key == ui.KeyUp) && mods&(ui.Cmd|ui.Alt) != 0,
		k.Key == ui.KeyN && (mods == ui.Ctrl|ui.Alt || mods == ui.Ctrl|ui.Alt|ui.Shift),
		k.Key == ui.KeyP && (mods == ui.Ctrl|ui.Alt || mods == ui.Ctrl|ui.Alt|ui.Shift):
		a.selectEdge(k.Key == ui.KeyUp || k.Key == ui.KeyP)
		return true
	case k.Key == ui.KeyDown && (mods == 0 || mods == ui.Shift),
		k.Key == ui.KeyN && (mods == ui.Ctrl || mods == ui.Ctrl|ui.Shift),
		k.Key == ui.KeyJ && mods == ui.Ctrl:
		a.moveSelection(1)
		return true
	case k.Key == ui.KeyUp && (mods == 0 || mods == ui.Shift),
		k.Key == ui.KeyP && (mods == ui.Ctrl || mods == ui.Ctrl|ui.Shift),
		k.Key == ui.KeyK && mods == ui.Ctrl && a.selected != 0:
		a.moveSelection(-1)
		return true
	case (k.Key == ui.KeyPageDown || k.Key == ui.KeyEnd) && mods == 0:
		a.selectEdge(false)
		return true
	case (k.Key == ui.KeyPageUp || k.Key == ui.KeyHome) && mods == 0:
		a.selectEdge(true)
		return true
	}
	if m, key, ok := parseShortcut(cfg.PinShortcut); ok && k.Key == key && mods == m {
		a.pinSelected()
		return true
	}
	switch {
	case k.Key == ui.KeyComma && mods == ui.Cmd:
		a.OpenPreferences()
		return true
	case k.Key == ui.KeyEnter:
		copyingQuery := a.selectedPreviewID() == "" && a.footerIndex < 0
		a.activateSelected(mods)
		// Copying a no-result query keeps the panel open and clears search.
		if copyingQuery {
			k.SetText(a.query)
		}
		return true
	case k.Key == ui.KeyEscape:
		a.closePopup()
		return true
	}
	if m, key, ok := parseShortcut(cfg.PreviewShortcut); ok && k.Key == key && mods == m {
		a.togglePreview()
		return true
	}
	if k.Key == ui.KeyQ && mods == ui.Cmd {
		mygo.App.Quit()
		return true
	}
	if itemAction(cfg, mods) != actUnknown {
		if k.Key >= ui.Key1 && k.Key <= ui.Key9 {
			a.activateShortcut(int(k.Key-ui.Key1)+1, mods)
			return true
		}
		if ch, ok := pinCharForKey(k.Key); ok {
			if it := a.pinnedByChar(ch); it != nil {
				a.activateItem(it, mods, cfg)
				return true
			}
		}
	}
	return false
}

func (a *App) editSearchQuery(k ui.TextKey, text string) {
	k.SetText(text)
	a.focusSearch = true
	a.onQueryChanged()
}

// pinCharForKey maps a key to the pin character it stands for.
func pinCharForKey(k ui.Key) (string, bool) {
	if k >= ui.KeyA && k <= ui.KeyZ {
		return string(rune('a' + (k - ui.KeyA))), true
	}
	switch k {
	case ui.KeyBracketLeft:
		return "[", true
	case ui.KeyBracketRight:
		return "]", true
	case ui.KeySemicolon:
		return ";", true
	case ui.KeyQuote:
		return "'", true
	case ui.KeyPeriod:
		return ".", true
	case ui.KeySlash:
		return "/", true
	case ui.KeyMinus:
		return "-", true
	case ui.KeyEqual:
		return "=", true
	case ui.KeyBackslash:
		return "\\", true
	}
	return "", false
}

// pinnedByChar returns the visible item pinned to ch, if any.
func (a *App) pinnedByChar(ch string) *model.Item {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, it := range a.vitems {
		if it.Pin == ch {
			return it
		}
	}
	return nil
}

func (a *App) moveSelection(delta int) {
	a.kbdNav = true
	a.mu.Lock()
	n := len(a.vitems)
	nFooter := len(a.footerActions())
	if !a.cfg.Snapshot().ShowFooter {
		nFooter = 0
	}
	if a.footerIndex >= 0 {
		if delta > 0 {
			a.footerIndex = (a.footerIndex + 1) % max(1, nFooter)
		} else if a.footerIndex == 0 {
			a.footerIndex = -1
			if n > 0 {
				a.selected = n - 1
				a.scrollToSelected = true
			}
		} else {
			a.footerIndex--
		}
		a.mu.Unlock()
		a.refresh()
		return
	}
	if n == 0 {
		if delta > 0 && nFooter > 0 {
			a.footerIndex = 0
		}
		a.mu.Unlock()
		a.refresh()
		return
	}
	next := a.selected + delta
	if next >= n {
		if delta > 0 && a.cfg.Snapshot().ShowFooter {
			a.footerIndex = 0
			a.selected = -1
		} else {
			a.selected = n - 1
		}
	} else if next < 0 {
		a.selected = 0
	} else {
		a.selected = next
	}
	a.scrollToSelected = true
	a.mu.Unlock()
	a.scheduleAutoPreview()
	a.refresh()
}

func (a *App) selectEdge(first bool) {
	a.kbdNav = true
	a.mu.Lock()
	footerVisible := a.cfg.Snapshot().ShowFooter
	if !first && footerVisible && a.footerIndex >= 0 {
		a.footerIndex = len(a.footerActions()) - 1
		a.mu.Unlock()
		a.refresh()
		return
	}
	if !first && footerVisible && a.selected >= 0 && a.selected == len(a.vitems)-1 {
		a.footerIndex, a.selected = 0, -1
		a.mu.Unlock()
		a.refresh()
		return
	}
	a.footerIndex = -1
	if len(a.vitems) == 0 {
		a.mu.Unlock()
		return
	}
	if first {
		a.selected = 0
	} else {
		a.selected = len(a.vitems) - 1
	}
	a.scrollToSelected = true
	a.mu.Unlock()
	a.scheduleAutoPreview()
	a.refresh()
}

func (a *App) onQueryChanged() {
	a.kbdNav = true
	a.mu.Lock()
	if a.query == "" {
		a.searchHeight = 0
	} else if a.searchHeight == 0 {
		a.searchHeight = a.lastPopupH
		if a.searchHeight == 0 {
			_, a.searchHeight = a.popupSizeLocked()
		}
	}
	a.footerIndex = -1
	a.selected = 0
	a.scrollToSelected = true
	a.rebuildLocked()
	a.mu.Unlock()
	a.scheduleAutoPreview()
	a.refresh()
}

func (a *App) activateSelected(mods ui.Modifiers) {
	if a.footerIndex >= 0 {
		actions := a.footerActions()
		if a.footerIndex < len(actions) {
			actions[a.footerIndex].fn()
		}
		return
	}
	a.mu.Lock()
	var it *model.Item
	if a.selected >= 0 && a.selected < len(a.vitems) {
		it = a.vitems[a.selected]
	}
	query := a.query
	cfg := a.cfg.Snapshot()
	a.mu.Unlock()

	if it != nil {
		a.activateItem(it, mods, cfg)
	} else {
		if mygo.App.IsReady() {
			mygo.Clipboard.WriteText(query)
		}
		a.query = ""
		a.onQueryChanged()
	}
}

func (a *App) activateIndex(i int, mods ui.Modifiers) {
	a.kbdNav = false
	a.mu.Lock()
	a.footerIndex = -1
	if i < 0 || i >= len(a.vitems) {
		a.mu.Unlock()
		return
	}
	it := a.vitems[i]
	cfg := a.cfg.Snapshot()
	a.mu.Unlock()
	a.activateItem(it, mods, cfg)
}

func (a *App) activateItem(it *model.Item, mods ui.Modifiers, cfg config.Config) {
	removeFmt := false
	paste := false
	if mods == 0 {
		removeFmt = cfg.RemoveFormattingByDefault
		paste = cfg.PasteByDefault
	} else {
		switch itemAction(cfg, mods) {
		case actCopy:
			// copy only
		case actPaste:
			paste = true
		case actPasteNoFormatting:
			paste = true
			removeFmt = true
		default:
			return
		}
	}
	a.lastCopiedID = it.ID
	a.closePopup()
	if !a.copyItem(it, removeFmt) {
		return
	}
	if paste {
		go func() {
			time.Sleep(120 * time.Millisecond)
			_ = clipboard.Paste()
		}()
	}
}

func (a *App) copyItem(it *model.Item, removeFormatting bool) bool {
	if !mygo.App.IsReady() {
		return false
	}
	data, err := clipboard.Data(a.store, it, removeFormatting)
	if err == nil && len(data.Formats()) > 0 {
		if err := mygo.Clipboard.Write(data); err == nil {
			if a.watcher != nil {
				a.watcher.Wake()
			}
			return true
		}
	}
	return false
}

func (a *App) pinSelected() {
	a.mu.Lock()
	var it *model.Item
	if a.selected >= 0 && a.selected < len(a.vitems) {
		it = a.vitems[a.selected]
	}
	a.mu.Unlock()
	if it != nil {
		a.togglePin(it)
		a.refresh()
	}
}

func (a *App) deleteSelected() {
	a.mu.Lock()
	var it *model.Item
	if a.selected >= 0 && a.selected < len(a.vitems) {
		it = a.vitems[a.selected]
	}
	a.mu.Unlock()
	if it != nil {
		a.deleteItem(it)
		a.refresh()
	}
}

// confirmClear asks before clearing the history, unless suppressed.
func (a *App) confirmClear(all bool) {
	cfg := a.cfg.Snapshot()
	if cfg.SuppressClearAlert {
		a.post(func() {
			a.clearHistory(all)
			a.closePopup()
			a.refresh()
		})
		return
	}
	a.suppressClose = true
	parent := a.panel.Load()
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		parent = a.settingsWin
	}
	go func() {
		defer a.post(func() { a.suppressClose = false })
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             parent,
			Type:               mygo.MessageWarning,
			Message:            "清空剪贴板历史？",
			Detail:             "此操作无法撤销。",
			Buttons:            []string{"取消", "清空"},
			DestructiveButtons: []int{1},
			CancelButton:       0,
			CheckboxLabel:      "不再询问",
		})
		if err != nil || res.Button != 1 {
			return
		}
		if res.CheckboxChecked {
			_ = a.cfg.Update(func(c *config.Config) { c.SuppressClearAlert = true })
		}
		a.post(func() {
			a.clearHistory(all)
			a.closePopup()
			a.refresh()
		})
	}()
}

// hexColor parses a hex color title into a ui.Color, as Maccy's swatch does.
func hexColor(title string) (ui.Color, bool) {
	s := strings.TrimSpace(title)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	case 8:
		s = s[:6]
	default:
		return ui.Color{}, false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return ui.Color{}, false
		}
	}
	return ui.Hex("#" + s), true
}

type titleSegment struct {
	text  string
	match bool
}

// titleSegments splits a title into highlighted and plain runs.
func titleSegments(title string, ranges []search.Range) []titleSegment {
	runes := []rune(title)
	rs := append([]search.Range(nil), ranges...)
	sort.Slice(rs, func(i, j int) bool { return rs[i].Start < rs[j].Start })
	var segs []titleSegment
	pos := 0
	for _, r := range rs {
		start, end := r.Start, r.End
		if start < pos {
			start = pos
		}
		if end > len(runes) {
			end = len(runes)
		}
		if start > pos {
			segs = append(segs, titleSegment{text: string(runes[pos:start])})
		}
		if end > start {
			segs = append(segs, titleSegment{text: string(runes[start:end]), match: true})
		}
		if end > pos {
			pos = end
		}
	}
	if pos < len(runes) {
		segs = append(segs, titleSegment{text: string(runes[pos:])})
	}
	return segs
}

// activateShortcut copies/pastes the nth unpinned item (⌘1..⌘9).
func (a *App) activateShortcut(digit int, mods ui.Modifiers) {
	label := "⌘" + strconv.Itoa(digit)
	a.mu.Lock()
	var it *model.Item
	for _, candidate := range a.vitems {
		if a.shortcuts[candidate.ID] == label {
			it = candidate
			break
		}
	}
	cfg := a.cfg.Snapshot()
	a.mu.Unlock()
	if it != nil {
		a.activateItem(it, mods, cfg)
	}
}
