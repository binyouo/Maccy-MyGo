package app

import (
	"slices"
	"strings"

	"github.com/binyouo/Maccy-MyGo/internal/model"
)

// Cache only the last selected preview, never text used to restore/copy/edit.
// Content hashes and fallback titles invalidate it without a stale-ID window.
type textPreviewCache struct {
	item  model.Item
	text  string
	valid bool
}

func (a *App) previewText(it *model.Item) string {
	p := &a.textPreview
	if p.valid && p.item.ID == it.ID && p.item.Title == it.Title && slices.Equal(p.item.Contents, it.Contents) && slices.Equal(p.item.Files, it.Files) {
		return p.text
	}
	text, _ := sizedPreviewText(a.itemText(it))
	*p = textPreviewCache{}
	if len(text) <= 4<<20 {
		p.item = *it
		p.item.PendingBlobs = nil
		p.item.Contents = append([]model.Content(nil), it.Contents...)
		p.item.Files = append([]string(nil), it.Files...)
		p.text, p.valid = strings.Clone(text), true
	}
	return text
}
