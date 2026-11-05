package app

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func historyContent(t *testing.T, a *App, typ, body string) model.Content {
	t.Helper()
	h, size, err := a.store.PutBlob([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return model.Content{Type: typ, Hash: h, Size: size}
}

func TestHistoryDuplicateRetainsAliasFormatsAndOrigin(t *testing.T) {
	a := newTestApp(t)
	first := makeItem(t, a, "edited alias", "same plain text")
	first.Contents = append(first.Contents, historyContent(t, a, "text/rtf", "rich representation"))
	first.Application, first.ApplicationName = "original.app", "Original"
	a.addItem(first)
	a.togglePin(first)
	last := makeItem(t, a, "same plain text", "same plain text")
	last.Application, last.ApplicationName = "new.app", "New"
	a.addItem(last)
	if len(a.items) != 1 || a.items[0] != last {
		t.Fatalf("plain subset did not replace richer duplicate: %v", titles(a.items))
	}
	if last.Title != "edited alias" || len(last.Contents) != 2 || last.NumberOfCopies != 2 || !last.IsPinned() || last.Pin != first.Pin {
		t.Fatalf("duplicate lost stored state: %+v", last)
	}
	if last.Application != "original.app" || last.ApplicationName != "Original" || !last.FirstCopiedAt.Equal(first.FirstCopiedAt) {
		t.Fatal("ordinary duplicate lost its original source or first-copy date")
	}
	// Supersedes is directional: newly added formatting must not replace
	// an older plain-only entry just because their text matches.
	b := newTestApp(t)
	b.addItem(makeItem(t, b, "plain", "same"))
	rich := makeItem(t, b, "rich", "same")
	rich.Contents = append(rich.Contents, historyContent(t, b, "text/rtf", "rich"))
	b.addItem(rich)
	if len(b.items) != 2 {
		t.Fatal("richer incoming copy was incorrectly merged into plain-only history")
	}
}

func TestHistoryDuplicateIgnoresTransientMetadata(t *testing.T) {
	a := newTestApp(t)
	first := makeItem(t, a, "original", "same")
	first.Contents = append(first.Contents, historyContent(t, a, "org.chromium.source-url", "https://first.example"))
	a.addItem(first)
	second := makeItem(t, a, "incoming", "same")
	second.Contents = append(second.Contents, historyContent(t, a, "org.chromium.source-url", "https://second.example"))
	a.addItem(second)
	if len(a.items) != 1 || second.NumberOfCopies != 2 || !reflect.DeepEqual(second.Contents, first.Contents) {
		t.Fatal("volatile browser metadata created a second history entry")
	}
}

func TestHistoryCopyFromMaccyRetainsStoredSource(t *testing.T) {
	a := newTestApp(t)
	first := makeItem(t, a, "alias", "same")
	first.Application = "original.app"
	a.addItem(first)
	second := makeItem(t, a, "same", "same")
	second.Application, second.ApplicationName = "destination.app", "Destination"
	second.Contents = append(second.Contents, historyContent(t, a, "org.p0deje.Maccy", ""))
	a.addItem(second)
	if len(a.items) != 1 || second.Application != "original.app" || second.Title != "alias" {
		t.Fatalf("self-copy did not preserve the original alias/source: %+v", second)
	}
}

func TestHistoryLimitFollowsSortAndKeepsNewCopy(t *testing.T) {
	for _, sortBy := range []config.SortBy{config.SortFirstCopied, config.SortCopies} {
		t.Run(string(sortBy), func(t *testing.T) {
			a := newTestApp(t)
			_ = a.cfg.Update(func(c *config.Config) { c.HistorySize, c.SortBy = 2, sortBy })
			first := makeItem(t, a, "first", "first")
			second := makeItem(t, a, "second", "second")
			a.addItem(first)
			a.addItem(second)
			// Arrange last-copy dates opposite the selected ranking.
			first.FirstCopiedAt, first.LastCopiedAt, first.NumberOfCopies = time.Unix(20, 0), time.Unix(1, 0), 10
			second.FirstCopiedAt, second.LastCopiedAt, second.NumberOfCopies = time.Unix(10, 0), time.Unix(2, 0), 5
			incoming := makeItem(t, a, "new copy", "new copy")
			a.addItem(incoming)
			if len(a.items) != 2 || a.items[0] != first || a.items[1] != incoming {
				t.Fatalf("capacity removed a leading sorted item or the incoming copy: %v", titles(a.items))
			}
		})
	}
}

func TestHistorySearchKeepsPinsInTheirOwnGroup(t *testing.T) {
	for _, pinTo := range []config.PinTo{config.PinTop, config.PinBottom} {
		for _, mode := range []config.SearchMode{config.SearchFuzzy, config.SearchMixed} {
			t.Run(string(pinTo)+"/"+string(mode), func(t *testing.T) {
				a := newTestApp(t)
				_ = a.cfg.Update(func(c *config.Config) { c.PinTo, c.SearchMode = pinTo, mode })
				pinned := makeItem(t, a, "a long gap b", "pinned")
				a.addItem(pinned)
				a.togglePin(pinned)
				unpinned := makeItem(t, a, "ab", "unpinned")
				a.addItem(unpinned)
				a.query = "ab"
				a.rebuildLocked()
				want := []*model.Item{pinned, unpinned}
				if pinTo == config.PinBottom {
					want = []*model.Item{unpinned, pinned}
				}
				if !reflect.DeepEqual(a.vitems, want) {
					t.Fatalf("search crossed/lost a pin group: %v", titles(a.vitems))
				}
			})
		}
	}
}

func TestHistoryPinsReserveActionKeysAndSortAvailableKeys(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) {
		c.PinShortcut, c.DeleteShortcut, c.PreviewShortcut = "Option+B", "Cmd+Period", "Ctrl+Y"
	})
	available := a.availablePinsLocked()
	for _, blocked := range []string{"b", ".", "y"} {
		for _, key := range available {
			if key == blocked {
				t.Fatalf("action key %q was offered as a pin", key)
			}
		}
	}
	symbols := []string{}
	for _, key := range available {
		if key < "a" || key > "z" {
			symbols = append(symbols, key)
		}
	}
	if !sort.StringsAreSorted(symbols) {
		t.Fatalf("pin symbols are not alphabetically ordered: %v", symbols)
	}
}

func TestHistoryKeylessPinsPersistAndSurviveCapacity(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.HistorySize = 1 })
	available := a.availablePinsLocked()
	for i, pin := range available {
		it := makeItem(t, a, pin, "keyed "+pin)
		it.ID, it.Pin = "key-"+pin, pin
		it.Pinned = true
		a.items = append(a.items, it)
		if i == len(available)-1 {
			a.rebuildLocked()
		}
	}
	first := makeItem(t, a, "keyless first", "keyless first")
	a.addItem(first)
	a.togglePin(first)
	second := makeItem(t, a, "keyless second", "keyless second")
	a.addItem(second)
	a.togglePin(second)
	if !first.IsPinned() || !second.IsPinned() || first.Pin != "" || second.Pin != "" {
		t.Fatal("exhausted shortcut pool failed to create keyless pins")
	}
	if first.PinOrderKey() == second.PinOrderKey() || a.shortcuts[first.ID] != "" || a.shortcuts[second.ID] != "" {
		t.Fatal("keyless pins shared an order token or received a shortcut")
	}
	a.addItem(makeItem(t, a, "unpinned 1", "unpinned 1"))
	a.addItem(makeItem(t, a, "unpinned 2", "unpinned 2"))
	if len(a.items) != len(available)+3 {
		t.Fatalf("capacity counted keyless pins as unpinned: %d", len(a.items))
	}
	reloaded, err := model.Open(a.store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, it := range reloaded.Items {
		if it.IsPinned() && it.Pin == "" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("lost keyless pins on reload: %d", count)
	}
	// A repeat copy must replace the keyless order token in place.
	before := append([]string(nil), a.cfg.Snapshot().PinOrder...)
	copy := makeItem(t, a, "fresh title", "keyless first")
	a.addItem(copy)
	after := a.cfg.Snapshot().PinOrder
	for i, key := range before {
		if key == first.PinOrderKey() {
			before[i] = copy.PinOrderKey()
		}
	}
	if !copy.IsPinned() || !reflect.DeepEqual(after, before) {
		t.Fatal("recopy moved the keyless pin out of its stored order")
	}
}

func TestHistoryMovePinUsesBeforeRowDestination(t *testing.T) {
	a := newTestApp(t)
	for _, pin := range []string{"b", "c", "d", "e"} {
		it := makeItem(t, a, pin, pin)
		it.Pin = pin
		a.addItem(it)
	}
	_ = a.cfg.ReplacePinOrder([]string{"b", "c", "d", "e"})
	a.rebuildLocked()
	a.selected = 0
	selected := a.selectedIDLocked()
	a.movePin(0, 2)
	if got := a.cfg.Snapshot().PinOrder; !reflect.DeepEqual(got, []string{"c", "b", "d", "e"}) {
		t.Fatalf("dragging before row 2 moved one row too far: %v", got)
	}
	if a.selectedIDLocked() != selected {
		t.Fatal("dragging a pin changed the selected item")
	}
	a.query = "b"
	a.movePin(0, 4)
	if got := a.cfg.Snapshot().PinOrder; !reflect.DeepEqual(got, []string{"c", "b", "d", "e"}) {
		t.Fatal("a search-result drag reordered the stored pins")
	}
}

func TestHistoryEditingPinKeyPreservesOrderAndRejectsConflicts(t *testing.T) {
	a := newTestApp(t)
	var items []*model.Item
	for _, pin := range []string{"b", "c", "d"} {
		it := makeItem(t, a, pin, pin)
		it.Pin = pin
		a.addItem(it)
		items = append(items, it)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.updatePinKeyLocked(items[0], "c") {
		t.Fatal("conflicting pin shortcut was accepted")
	}
	if a.updatePinKeyLocked(items[0], "p") {
		t.Fatal("the pin action's own key was accepted")
	}
	if !a.updatePinKeyLocked(items[0], "x") {
		t.Fatal("free pin shortcut was rejected")
	}
	if got := a.cfg.Snapshot().PinOrder; !reflect.DeepEqual(got, []string{"x", "c", "d"}) {
		t.Fatalf("changing a key moved its pin to the end: %v", got)
	}
}

func TestHistoryModifiedCopyKeepsNewContentsAndStoredMetadata(t *testing.T) {
	a := newTestApp(t)
	first := makeItem(t, a, "edited alias", "before transform")
	first.Application, first.ApplicationName, first.ClipboardChangeCount = "source.app", "Source", 101
	a.addItem(first)
	a.togglePin(first)
	modified := makeItem(t, a, "after transform", "after transform")
	modified.ClipboardChangeCount = 102
	modified.Contents = append(modified.Contents,
		historyContent(t, a, "text/rtf", "transformed rich text"),
		historyContent(t, a, "x.nspasteboard.ModifiedType", "101"))
	a.addItem(modified)
	if len(a.items) != 1 || a.items[0] != modified || len(modified.Contents) != 3 {
		t.Fatalf("modified event added a new row or discarded transformed formats: %+v", modified)
	}
	if text, err := a.store.ReadBlob(modified.Contents[0].Hash); err != nil || string(text) != "after transform" {
		t.Fatalf("new text was replaced with the old clipboard contents: %q, %v", text, err)
	}
	if modified.Title != first.Title || modified.Application != first.Application || modified.NumberOfCopies != 2 || !modified.FirstCopiedAt.Equal(first.FirstCopiedAt) || modified.Pin != first.Pin || !modified.IsPinned() {
		t.Fatal("modified event lost the original alias, pin, source or copy metadata")
	}
}

func TestHistoryModifiedMarkerFollowsDuplicateReplacement(t *testing.T) {
	a := newTestApp(t)
	first := makeItem(t, a, "alias", "same")
	first.ClipboardChangeCount = 11
	a.addItem(first)
	duplicate := makeItem(t, a, "same", "same")
	duplicate.ClipboardChangeCount = 12
	a.addItem(duplicate)
	modified := makeItem(t, a, "transformed", "transformed")
	modified.ClipboardChangeCount = 13
	modified.Contents = append(modified.Contents, historyContent(t, a, "x.nspasteboard.ModifiedType", "11"))
	a.addItem(modified)
	if len(a.items) != 1 || a.items[0] != modified || modified.NumberOfCopies != 3 || modified.Title != "alias" {
		t.Fatal("old clipboard sequence marker did not follow the live duplicate chain")
	}
	for _, count := range []int64{11, 12, 13} {
		if a.clipboardSessionLog[count] != modified {
			t.Fatalf("sequence %d still points to a replaced history object", count)
		}
	}
	a.deleteItem(modified)
	if len(a.clipboardSessionLog) != 0 {
		t.Fatal("deleting a history item retained session aliases")
	}
}

func TestHistorySupersedesPrecedesModifiedLookup(t *testing.T) {
	a := newTestApp(t)
	duplicateTarget := makeItem(t, a, "duplicate alias", "one")
	duplicateTarget.ClipboardChangeCount = 11
	a.addItem(duplicateTarget)
	modifiedTarget := makeItem(t, a, "modified alias", "two")
	modifiedTarget.ClipboardChangeCount = 12
	a.addItem(modifiedTarget)
	incoming := makeItem(t, a, "incoming", "one")
	incoming.ClipboardChangeCount = 13
	incoming.Contents = append(incoming.Contents, historyContent(t, a, "x.nspasteboard.ModifiedType", "12"))
	a.addItem(incoming)
	if len(a.items) != 2 || incoming.Title != "duplicate alias" || a.items[0] != modifiedTarget {
		t.Fatal("ModifiedType incorrectly took precedence over an existing superseding item")
	}
	// isModified is checked independently: even when another superseding
	// item wins, a recognized marker prevents transferring old formats.
	if len(incoming.Contents) != 2 || incoming.Contents[1].Type != "x.nspasteboard.ModifiedType" {
		t.Fatal("a recognized ModifiedType event transferred the duplicate's old contents")
	}
}

func TestHistoryUnknownModifiedMarkerDoesNotMerge(t *testing.T) {
	for _, marker := range []string{"999", "11\n", "not a number", "0"} {
		t.Run(marker, func(t *testing.T) {
			a := newTestApp(t)
			first := makeItem(t, a, "old alias", "before")
			first.ClipboardChangeCount = 11
			a.addItem(first)
			incoming := makeItem(t, a, "new alias", "after")
			incoming.ClipboardChangeCount = 12
			incoming.Contents = append(incoming.Contents, historyContent(t, a, "x.nspasteboard.ModifiedType", marker))
			a.addItem(incoming)
			if len(a.items) != 2 || incoming.Title != "new alias" || incoming.NumberOfCopies != 1 {
				t.Fatal("unknown/invalid clipboard sequence merged unrelated history")
			}
		})
	}
}

func TestHistoryModifiedSessionIsNotPersisted(t *testing.T) {
	a := newTestApp(t)
	first := makeItem(t, a, "old alias", "before")
	first.ClipboardChangeCount = 11
	a.addItem(first)
	store, err := model.Open(a.store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if store.Items[0].ClipboardChangeCount != 0 {
		t.Fatal("platform clipboard sequence was written into the history index")
	}
	restarted := New(a.cfg, store)
	modified := makeItem(t, restarted, "new alias", "after")
	modified.ClipboardChangeCount = 12
	modified.Contents = append(modified.Contents, historyContent(t, restarted, "x.nspasteboard.ModifiedType", "11"))
	restarted.addItem(modified)
	if len(restarted.items) != 2 || modified.Title != "new alias" {
		t.Fatal("a previous run's clipboard sequence matched unrelated current history")
	}
}

func TestHistorySessionPrunedByCapacityAndClear(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.HistorySize = 1 })
	first := makeItem(t, a, "first", "first")
	first.ClipboardChangeCount = 11
	a.addItem(first)
	second := makeItem(t, a, "second", "second")
	second.ClipboardChangeCount = 12
	a.addItem(second)
	if a.clipboardSessionLog[11] != nil || a.clipboardSessionLog[12] != second {
		t.Fatal("capacity pruning retained a removed copy in the session log")
	}
	a.togglePin(second)
	third := makeItem(t, a, "third", "third")
	third.ClipboardChangeCount = 13
	a.addItem(third)
	a.clearHistory(false)
	if len(a.clipboardSessionLog) != 1 || a.clipboardSessionLog[12] != second {
		t.Fatal("clearing unpinned history removed the pin's session entry or retained unpinned entries")
	}
	a.clearHistory(true)
	if len(a.clipboardSessionLog) != 0 {
		t.Fatal("clearing all history retained clipboard session state")
	}
}
