package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

// TextRanges only styles editable inputs in MyGo. Plain history labels
// need individual text runs for all four search highlight styles.
func matchedTitle(c *ui.Context, t *ui.Theme, title string, ranges []search.Range, style config.HighlightMatch) {
	ui.Row(c).Gap(0).Grow(1).ClipX().Padding(0, 5, 0, 0).PassThrough().Children(func() {
		for _, seg := range titleSegments(title, ranges) {
			tx := ui.Text(c, seg.text).MaxLines(1).FontSize(13).TextColor(t.Text).Shrink(0).PassThrough()
			if !seg.match {
				continue
			}
			switch style {
			case config.HighlightItalic:
				tx.Italic()
			case config.HighlightUnderline:
				tx.Underline()
			case config.HighlightColor:
				// NSColor.findHighlightColor's default yellow with black text.
				tx.TextBackground(ui.Hex("#FFFF00")).TextColor(ui.Hex("#000000"))
			default:
				tx.Bold()
			}
		}
	})
}

func shortcutTooltip(action, shortcut string) string {
	if _, _, ok := parseShortcut(shortcut); !ok {
		return action
	}
	return action + "（" + shortcutDisplay(shortcut) + "）"
}
