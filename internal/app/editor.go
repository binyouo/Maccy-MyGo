package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

// beginEdit opens the inline editor for an item.
func (a *App) beginEdit(it *model.Item) {
	if it == nil {
		return
	}
	a.mu.Lock()
	avail := a.availablePinsLocked()
	a.mu.Unlock()
	choices := append([]string(nil), avail...)
	if it.IsPinned() {
		choices = append([]string{it.Pin}, choices...)
	}
	sortPinChoices(choices)

	a.editing = it
	a.previewQueued = false
	a.cancelAutoPreview()
	a.editTitle = it.Title
	a.editContent = a.itemText(it)
	a.editContent, a.editTooLarge = sizedPreviewText(a.editContent)
	a.editOrig = a.editContent
	a.editPin = it.Pin
	a.editPinChoices = choices
	a.editIsText = canEditItemContent(it)
	a.reflow()
	a.refresh()
}

func (a *App) cancelEdit() {
	a.editing = nil
	a.scheduleAutoPreview()
	a.reflow()
	a.refresh()
}

func (a *App) editorKey(k ui.TextKey) bool {
	if k.Key == ui.KeyEscape && k.Mods == 0 {
		a.cancelEdit()
		return true
	}
	if k.Key == ui.KeyEnter && k.Mods == ui.Cmd {
		a.saveEdit()
		return true
	}
	return false
}

func (a *App) saveEdit() {
	it := a.editing
	if it == nil || !a.hasEditorChanges() {
		return
	}
	a.mu.Lock()
	present := false
	for _, current := range a.items {
		if current == it {
			present = true
			break
		}
	}
	if !present {
		a.mu.Unlock()
		a.cancelEdit()
		return
	}
	if a.editIsText && !a.editTooLarge && a.editContent != a.editOrig && !a.replaceItemTextLocked(it, a.editContent) {
		a.mu.Unlock()
		return
	}
	if !clipboard.HasImage(it) {
		it.Title = a.editTitle
	}
	if a.editPin != it.Pin {
		a.updatePinKeyLocked(it, a.editPin)
	}
	a.rebuildLocked()
	a.restoreSelectionLocked(it.ID)
	a.saveLocked()
	a.mu.Unlock()
	a.editing = nil
	a.scheduleAutoPreview()
	a.reflow()
	a.refresh()
}

func (a *App) editorView(c *ui.Context, t *ui.Theme, s popupSnapshot) {
	it := a.editing
	editor := ui.Column(c).Key("editor").Fill().Gap(10).Padding(10).Transition(paneTransition)
	editor.Children(func() {
		ui.Text(c, "编辑条目").FontSize(15).Bold()
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "别名").FontSize(12).TextColor(t.TextMuted).Width(52)
			inp := ui.TextInput(c, &a.editTitle).Label("别名").Grow(1).Disabled(clipboard.HasImage(it))
			if !clipboard.HasImage(it) {
				inp.AutoFocus()
			}
			inp.OnTextKey(a.editorKey)
			inp.OnSubmit(a.saveEdit)
		})
		if it != nil && it.IsPinned() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "按键").FontSize(12).TextColor(t.TextMuted).Width(52)
				ui.Select(c, &a.editPin, a.editPinChoices).Label("置顶快捷键").Grow(1)
			})
		}
		ui.Text(c, "内容").FontSize(12).Bold()
		if a.editTooLarge {
			ui.Text(c, "文本包含过长的段落，仅供预览，无法编辑内容。").FontSize(11).TextColor(t.TextMuted)
		}
		if a.editIsText && a.editContent != a.editOrig && itemHasRichText(it) {
			ui.Text(c, "修改内容将移除原有富文本格式。").FontSize(11).TextColor(t.TextMuted)
		}
		if a.editIsText {
			ta := ui.TextArea(c, &a.editContent).Label("内容").Grow(1).FillHeight().Disabled(a.editTooLarge)
			ta.OnTextKey(a.editorKey)
		} else {
			ui.Text(c, "该条目的内容不是可编辑文本。").FontSize(11).TextColor(t.TextMuted).Grow(1)
		}
		changed := a.hasEditorChanges()
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			ui.Button(c, "取消").OnClick(a.cancelEdit)
			// Maccy disables Done until an edit actually changes.
			ui.PrimaryButton(c, "完成").Disabled(!changed).OnClick(a.saveEdit)
		})
	})
	// Match the sheet's default/cancel actions from every control,
	// including the pin picker and the text area's focusable wrapper.
	if editor.Shortcut(ui.Cmd, ui.KeyEnter) {
		a.saveEdit()
	}
	if editor.Shortcut(0, ui.KeyEscape) {
		a.cancelEdit()
	}
}

func (a *App) hasEditorChanges() bool {
	it := a.editing
	return it != nil && ((!clipboard.HasImage(it) && a.editTitle != it.Title) || a.editPin != it.Pin || (a.editIsText && !a.editTooLarge && a.editContent != a.editOrig))
}

func itemHasRichText(it *model.Item) bool {
	for _, c := range it.Contents {
		if c.Type == "text/rtf" || c.Type == "text/html" {
			return true
		}
	}
	return false
}

func canEditItemContent(it *model.Item) bool {
	if len(it.Files) != 0 || clipboard.HasImage(it) {
		return false
	}
	for _, c := range it.Contents {
		if c.Type == "text/plain" || c.Type == "text/rtf" || c.Type == "text/html" {
			return true
		}
	}
	return false
}

func (a *App) replaceItemTextLocked(it *model.Item, text string) bool {
	h, size, err := a.store.PutBlob([]byte(text))
	if err != nil {
		return false
	}
	// ItemEditorView.applyChanges removes stale RTF/HTML representations
	// only after the actual content was changed; aliases retain formatting.
	it.Contents = []model.Content{{Type: "text/plain", Hash: h, Size: size}}
	return true
}
