package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

type itemActionKind int

const (
	actUnknown itemActionKind = iota
	actCopy
	actPaste
	actPasteNoFormatting
)

// itemAction maps modifier keys to a copy/paste action, as Maccy's
// HistoryItemAction does.
func itemAction(cfg config.Config, mods ui.Modifiers) itemActionKind {
	switch mods {
	case ui.Cmd:
		switch {
		case !cfg.PasteByDefault:
			return actCopy
		case !cfg.RemoveFormattingByDefault:
			return actPaste
		default:
			return actPasteNoFormatting
		}
	case ui.Alt:
		switch {
		case !cfg.PasteByDefault && !cfg.RemoveFormattingByDefault:
			return actPaste
		case !cfg.PasteByDefault && cfg.RemoveFormattingByDefault:
			return actPasteNoFormatting
		default:
			return actCopy
		}
	case ui.Alt | ui.Shift:
		switch {
		case !cfg.PasteByDefault && !cfg.RemoveFormattingByDefault:
			return actPasteNoFormatting
		case !cfg.PasteByDefault && cfg.RemoveFormattingByDefault:
			return actPaste
		default:
			return actUnknown
		}
	case ui.Cmd | ui.Shift:
		switch {
		case cfg.PasteByDefault && !cfg.RemoveFormattingByDefault:
			return actPasteNoFormatting
		case cfg.PasteByDefault && cfg.RemoveFormattingByDefault:
			return actPaste
		default:
			return actUnknown
		}
	}
	return actUnknown
}
