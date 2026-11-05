package clipboard

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/transfer"

	"github.com/binyouo/Maccy-MyGo/internal/model"
)

const (
	FromMaccyType       = "org.p0deje.Maccy"
	SourceType          = "org.nspasteboard.source"
	remoteClipboardType = "com.apple.is-remote-clipboard"
)

// Data rebuilds a transfer.Data from a stored item, optionally keeping only
// plain text and files (removing formatting).
func Data(store *model.Store, item *model.Item, removeFormatting bool) (transfer.Data, error) {
	if item == nil {
		return transfer.Data{}, nil
	}
	// Clipboard.clearFormatting leaves image/RTF-only copies untouched
	// when there is no plain-string representation to fall back to.
	plainOnly := false
	if removeFormatting {
		for _, c := range item.Contents {
			if canonicalFormat(c.Type) == "text/plain" {
				plainOnly = true
				break
			}
		}
	}
	var reps []transfer.Representation
	formats := map[string]int{}
	for _, c := range item.Contents {
		name := canonicalFormat(c.Type)
		if name == fileListFormat || strings.EqualFold(name, FromMaccyType) || strings.EqualFold(name, SourceType) {
			continue // files are rebuilt from item.Files
		}
		if plainOnly && name != "text/plain" {
			continue
		}
		b, err := store.ReadBlob(c.Hash)
		if err != nil {
			continue
		}
		rep := transfer.Bytes(transfer.Format(name), b)
		if index, exists := formats[name]; exists {
			// Maccy merges pasteboard items into one record, then setData
			// replaces repeated formats while copying it back.
			reps[index] = rep
		} else {
			formats[name] = len(reps)
			reps = append(reps, rep)
		}
	}

	var items []transfer.Item
	if len(item.Files) > 0 {
		if fd, err := transfer.FileData(item.Files...); err == nil {
			items = append(items, fd.Items()...)
		}
	}
	if len(reps) == 0 && len(items) == 0 {
		return transfer.Data{}, nil
	}
	reps = append(reps, transfer.Bytes(transfer.Format(FromMaccyType), nil), transfer.Bytes(transfer.Format(SourceType), []byte(item.Application)))
	items = append(items, transfer.NewItem(reps...))
	return transfer.New(items...), nil
}

// HasImage reports whether the item has an image representation.
func HasImage(item *model.Item) bool {
	if item == nil {
		return false
	}
	for _, c := range item.Contents {
		if category(canonicalFormat(c.Type)) == "image" {
			return true
		}
	}
	return remoteImageFile(item) != ""
}

// ImageContent returns the first image content, if any.
func ImageContent(item *model.Item) (model.Content, bool) {
	if item == nil {
		return model.Content{}, false
	}
	for _, c := range item.Contents {
		if category(canonicalFormat(c.Type)) == "image" {
			return c, true
		}
	}
	return model.Content{}, false
}

// ImageData reads the same source as HistoryItem.imageData: the stored
// image representation, or Universal Clipboard's first local JPEG URL.
// Original file representations remain intact for clipboard restoration.
func ImageData(store *model.Store, item *model.Item) ([]byte, error) {
	if content, ok := ImageContent(item); ok {
		return store.ReadBlob(content.Hash)
	}
	if file := remoteImageFile(item); file != "" {
		return os.ReadFile(file)
	}
	return nil, transfer.ErrFormat
}

func isRemoteClipboard(item *model.Item) bool {
	if item == nil {
		return false
	}
	for _, content := range item.Contents {
		if content.Type == remoteClipboardType {
			return true
		}
	}
	return false
}

func previewFiles(item *model.Item) []string {
	if isRemoteClipboard(item) {
		// Universal Clipboard adds a temporary file URL even when the
		// copied value is text/rich text or a real image representation.
		for _, content := range item.Contents {
			if cat := category(canonicalFormat(content.Type)); cat == "text" || cat == "image" {
				return nil
			}
		}
	}
	return item.Files
}

func remoteImageFile(item *model.Item) string {
	if !isRemoteClipboard(item) {
		return ""
	}
	if files := previewFiles(item); len(files) > 0 && filepath.Ext(files[0]) == ".jpeg" {
		return files[0]
	}
	return ""
}
