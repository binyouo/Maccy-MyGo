package app

import (
	"crypto/rand"
	"encoding/hex"
	mathrand "math/rand/v2"
	"sort"
	"strconv"
	"time"

	"github.com/egoist/mygo"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// supportedPins mirrors HistoryItem.supportedPins, less reserved keys.
var supportedPins = []string{
	"b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l",
	"m", "n", "o", "p", "r", "s", "t", "u", "x", "y",
	"[", "]", ";", "'", ".", "/", "-", "=", "\\",
}

func (a *App) availablePinsLocked() []string {
	assigned := map[string]bool{}
	for _, it := range a.items {
		if it.IsPinned() && it.Pin != "" {
			assigned[it.Pin] = true
		}
	}
	// HistoryItem.supportedPins reserves the physical character used by
	// each local action, independently of that shortcut's modifiers.
	cfg := a.cfg.Snapshot()
	for _, shortcut := range []string{cfg.PinShortcut, cfg.DeleteShortcut, cfg.PreviewShortcut} {
		if _, key, ok := parseShortcut(shortcut); ok {
			if character, ok := pinCharForKey(key); ok {
				assigned[character] = true
			}
		}
	}
	var out []string
	for _, p := range supportedPins {
		if !assigned[p] {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		iLetter, jLetter := out[i] >= "a" && out[i] <= "z", out[j] >= "a" && out[j] <= "z"
		if iLetter != jLetter {
			return iLetter
		}
		return out[i] < out[j]
	})
	return out
}

func (a *App) randomPinLocked() string {
	pins := a.availablePinsLocked()
	if len(pins) == 0 {
		return ""
	}
	return pins[mathrand.IntN(len(pins))]
}

// reconcilePinsLocked drops pins no item has and appends pins missing from
// the stored order.
func (a *App) reconcilePinsLocked(cfg *config.Config) {
	assigned := map[string]bool{}
	for _, it := range a.items {
		if it.IsPinned() {
			assigned[it.PinOrderKey()] = true
		}
	}
	var order []string
	seen := map[string]bool{}
	for _, p := range cfg.PinOrder {
		if assigned[p] && !seen[p] {
			order = append(order, p)
			seen[p] = true
		}
	}
	for _, it := range a.items {
		if key := it.PinOrderKey(); key != "" && !seen[key] {
			order = append(order, key)
			seen[key] = true
		}
	}
	cfg.PinOrder = order
}

// orderItems returns pinned items (in pin order) and unpinned items sorted
// per cfg, arranged according to cfg.PinTo.
func orderItems(items []*model.Item, cfg config.Config) []*model.Item {
	pinned := make([]*model.Item, 0)
	unpinned := make([]*model.Item, 0)
	for _, it := range items {
		if it.IsPinned() {
			pinned = append(pinned, it)
		} else {
			unpinned = append(unpinned, it)
		}
	}
	pos := map[string]int{}
	for i, p := range cfg.PinOrder {
		pos[p] = i
	}
	sort.SliceStable(pinned, func(i, j int) bool {
		pi, oki := pos[pinned[i].PinOrderKey()]
		pj, okj := pos[pinned[j].PinOrderKey()]
		if !oki {
			pi = 1 << 30
		}
		if !okj {
			pj = 1 << 30
		}
		return pi < pj
	})
	sort.SliceStable(unpinned, func(i, j int) bool {
		switch cfg.SortBy {
		case config.SortFirstCopied:
			return unpinned[i].FirstCopiedAt.After(unpinned[j].FirstCopiedAt)
		case config.SortCopies:
			return unpinned[i].NumberOfCopies > unpinned[j].NumberOfCopies
		default:
			return unpinned[i].LastCopiedAt.After(unpinned[j].LastCopiedAt)
		}
	})
	if cfg.PinTo == config.PinBottom {
		return append(unpinned, pinned...)
	}
	return append(pinned, unpinned...)
}

// rebuildLocked recomputes the visible list and highlight ranges.
func (a *App) rebuildLocked() {
	cfg := a.cfg.Snapshot()
	ordered := orderItems(a.items, cfg)
	// Swift searches the two groups independently. Fuzzy scores (or a
	// mixed-mode exact match in one group) must not cross the pin boundary.
	var pinned, unpinned []*model.Item
	for _, it := range ordered {
		if it.IsPinned() {
			pinned = append(pinned, it)
		} else {
			unpinned = append(unpinned, it)
		}
	}
	pinnedResults := search.Run(pinned, a.query, cfg.SearchMode)
	unpinnedResults := search.Run(unpinned, a.query, cfg.SearchMode)
	results := append(pinnedResults, unpinnedResults...)
	if cfg.PinTo == config.PinBottom {
		results = append(unpinnedResults, pinnedResults...)
	}
	a.vitems = a.vitems[:0]
	a.ranges = make(map[string][]search.Range, len(results))
	for _, r := range results {
		a.vitems = append(a.vitems, r.Item)
		if len(r.Ranges) > 0 {
			a.ranges[r.Item.ID] = r.Ranges
		}
	}
	a.pinnedCount = 0
	for _, it := range a.vitems {
		if it.IsPinned() {
			a.pinnedCount++
		} else {
			break
		}
	}
	a.shortcuts = make(map[string]string, len(a.vitems))
	unpinnedIndex := 0
	for _, it := range a.vitems {
		if it.IsPinned() {
			if it.Pin != "" {
				a.shortcuts[it.ID] = it.Pin
			}
			continue
		}
		unpinnedIndex++
		if unpinnedIndex <= 9 {
			a.shortcuts[it.ID] = "⌘" + strconv.Itoa(unpinnedIndex)
		}
	}
	if a.footerIndex >= 0 {
		a.selected = -1
	} else if len(a.vitems) == 0 {
		a.selected = -1
	} else if a.selected < 0 {
		a.selected = 0
	} else if a.selected >= len(a.vitems) {
		a.selected = len(a.vitems) - 1
	}
}

func contentSet(it *model.Item) map[string]bool {
	m := map[string]bool{}
	for _, c := range it.Contents {
		if historyTransientType(c.Type) {
			continue
		}
		m[c.Type+"|"+c.Hash] = true
	}
	for _, f := range it.Files {
		m["file|"+f] = true
	}
	return m
}

func supersedesContents(a, b *model.Item) bool {
	ma, mb := contentSet(a), contentSet(b)
	for k := range mb {
		if !ma[k] {
			return false
		}
	}
	return true
}

func historyTransientType(name string) bool {
	switch name {
	case "x.nspasteboard.ModifiedType", "org.p0deje.Maccy",
		"com.apple.linkpresentation.metadata", "com.apple.WebKit.custom-pasteboard-data",
		"org.nspasteboard.source", "org.chromium.web-custom-data",
		"org.chromium.source-url", "org.chromium.internal.source-rfh-token",
		"com.apple.notes.richtext":
		return true
	}
	return false
}

// modifiedItemLocked resolves only copies observed during this run, as
// History.isModified does through its in-memory sessionLog.
func (a *App) modifiedItemLocked(it *model.Item) *model.Item {
	for _, content := range it.Contents {
		if content.Type != "x.nspasteboard.ModifiedType" {
			continue
		}
		data, err := a.store.ReadBlob(content.Hash)
		if err != nil {
			return nil
		}
		count, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil || count <= 0 {
			return nil
		}
		target := a.clipboardSessionLog[count]
		if target == nil {
			return nil
		}
		for _, stored := range a.items {
			if stored == target {
				return target
			}
		}
		// Never resurrect a removed history object through a stale marker.
		delete(a.clipboardSessionLog, count)
		return nil
	}
	return nil
}

func (a *App) forgetSessionItemLocked(it *model.Item) {
	for count, target := range a.clipboardSessionLog {
		if target == it {
			delete(a.clipboardSessionLog, count)
		}
	}
}

func (a *App) replaceSessionItemLocked(old, replacement *model.Item) {
	for count, target := range a.clipboardSessionLog {
		if target == old {
			a.clipboardSessionLog[count] = replacement
		}
	}
}

// addItem merges a freshly captured item into the history.
func (a *App) addItem(it *model.Item) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	if it.ID == "" {
		it.ID = newID()
	}
	var existing *model.Item
	var existingIdx = -1
	modified := a.modifiedItemLocked(it)
	matchCfg := a.cfg.Snapshot()
	matchCfg.PinTo = config.PinTop
	for _, e := range orderItems(a.items, matchCfg) {
		if e != it && supersedesContents(e, it) {
			existing = e
			break
		}
	}
	if existing == nil {
		existing = modified
	}
	if existing != nil {
		for i, e := range a.items {
			if e == existing {
				existingIdx = i
				break
			}
		}
	}
	if existing != nil {
		it.FirstCopiedAt = existing.FirstCopiedAt
		it.NumberOfCopies = existing.NumberOfCopies + max(1, it.NumberOfCopies)
		it.Pin = existing.Pin
		it.Pinned = existing.IsPinned()
		it.Title = existing.Title
		if modified == nil {
			it.Contents = append([]model.Content(nil), existing.Contents...)
			it.Files = append([]string(nil), existing.Files...)
		}
		// Check fromMaccy after the conditional transfer, in Swift's order:
		// ordinary duplicates use the stored marker; modifications keep
		// their incoming marker along with their transformed contents.
		fromMaccy := false
		for _, content := range it.Contents {
			if content.Type == "org.p0deje.Maccy" {
				fromMaccy = true
				break
			}
		}
		if !fromMaccy {
			it.Application = existing.Application
			it.ApplicationName = existing.ApplicationName
		}
		it.LastCopiedAt = now
		if existing.IsPinned() && existing.PinOrderKey() != it.PinOrderKey() {
			cfg := a.cfg.Snapshot()
			for i, key := range cfg.PinOrder {
				if key == existing.PinOrderKey() {
					cfg.PinOrder[i] = it.PinOrderKey()
				}
			}
			_ = a.cfg.ReplacePinOrder(cfg.PinOrder)
		}
		a.items = append(a.items[:existingIdx], a.items[existingIdx+1:]...)
		// Keep marker aliases on the logical copy chain after replacing
		// its stored object, rather than retaining deleted SwiftData-style
		// references that can later revive a removed history entry.
		a.replaceSessionItemLocked(existing, it)
	} else {
		it.FirstCopiedAt = now
		it.LastCopiedAt = now
		it.NumberOfCopies = max(1, it.NumberOfCopies)
	}
	isNew := existing == nil
	// History.add makes room before inserting, so the item just copied
	// survives even when its current sort rank is below older entries.
	limit := a.cfg.Snapshot().HistorySize
	if limit <= 0 {
		limit = 200
	}
	if !it.IsPinned() {
		limit--
	}
	a.limitHistorySizeLocked(limit)
	a.items = append(a.items, it)
	if it.ClipboardChangeCount > 0 {
		if a.clipboardSessionLog == nil {
			a.clipboardSessionLog = make(map[int64]*model.Item)
		}
		a.clipboardSessionLog[it.ClipboardChangeCount] = it
	}
	if it.IsPinned() {
		cfg := a.cfg.Snapshot()
		a.reconcilePinsLocked(&cfg)
		_ = a.cfg.ReplacePinOrder(cfg.PinOrder)
	}
	a.rebuildLocked()
	a.saveLocked()
	if isNew && a.cfg.Snapshot().ShowNotifications && mygo.App.IsReady() {
		body := it.Title
		if r := []rune(body); len(r) > 160 {
			body = string(r[:160])
		}
		go func() {
			n := mygo.NewNotification(mygo.NotificationOptions{Title: "Maccy MyGo", Body: body})
			_ = n.Show()
		}()
	}
}

// trimLocked keeps the leading unpinned items in the configured sort order.
func (a *App) trimLocked() {
	size := a.cfg.Snapshot().HistorySize
	if size <= 0 {
		size = 200
	}
	a.limitHistorySizeLocked(size)
}

func (a *App) limitHistorySizeLocked(size int) {
	cfg := a.cfg.Snapshot()
	ordered := orderItems(a.items, cfg)
	drop := map[string]bool{}
	count := 0
	for _, it := range ordered {
		if it.IsPinned() {
			continue
		}
		if count >= size {
			drop[it.ID] = true
		}
		count++
	}
	kept := a.items[:0]
	for _, it := range a.items {
		if !drop[it.ID] {
			kept = append(kept, it)
		} else {
			a.forgetSessionItemLocked(it)
		}
	}
	a.items = kept
}

func (a *App) deleteItem(it *model.Item) {
	if it == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	selectedID := a.selectedIDLocked()
	for i, e := range a.items {
		if e == it {
			a.items = append(a.items[:i], a.items[i+1:]...)
			a.forgetSessionItemLocked(it)
			break
		}
	}
	cfg := a.cfg.Snapshot()
	a.reconcilePinsLocked(&cfg)
	_ = a.cfg.ReplacePinOrder(cfg.PinOrder)
	a.rebuildLocked()
	a.restoreSelectionLocked(selectedID)
	a.saveLocked()
	a.store.GarbageCollect()
}

func (a *App) togglePin(it *model.Item) {
	if it == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	selectedID := a.selectedIDLocked()
	if it.IsPinned() {
		it.Pin = ""
		it.Pinned = false
		cfg := a.cfg.Snapshot()
		var order []string
		for _, p := range cfg.PinOrder {
			if p != "" && !pinUsedByLocked(a.items, p) {
				continue
			}
			order = append(order, p)
		}
		_ = a.cfg.ReplacePinOrder(order)
	} else {
		it.Pin = a.randomPinLocked()
		it.Pinned = true
		cfg := a.cfg.Snapshot()
		cfg.PinOrder = append(cfg.PinOrder, it.PinOrderKey())
		_ = a.cfg.ReplacePinOrder(cfg.PinOrder)
	}
	a.query = ""
	a.searchHeight = 0
	cfg := a.cfg.Snapshot()
	a.reconcilePinsLocked(&cfg)
	_ = a.cfg.ReplacePinOrder(cfg.PinOrder)
	a.rebuildLocked()
	a.restoreSelectionLocked(selectedID)
	a.saveLocked()
}

// updatePinKeyLocked follows PinManager.updatePin/PinOrder.replace. The
// caller owns a.mu and rebuilds/saves the item after a successful change.
func (a *App) updatePinKeyLocked(it *model.Item, newPin string) bool {
	if it == nil || newPin == "" || it.Pin == newPin {
		return false
	}
	found := false
	for _, stored := range a.items {
		if stored == it {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	allowed := false
	for _, pin := range a.availablePinsLocked() {
		if pin == newPin {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	oldKey := it.PinOrderKey()
	it.Pin, it.Pinned = newPin, true
	cfg := a.cfg.Snapshot()
	order := make([]string, 0, len(cfg.PinOrder)+1)
	for _, key := range cfg.PinOrder {
		if key != newPin {
			order = append(order, key)
		}
	}
	replaced := false
	if oldKey != "" {
		for i, key := range order {
			if key == oldKey {
				order[i] = newPin
				replaced = true
				break
			}
		}
	}
	if !replaced {
		order = append(order, newPin)
	}
	cfg.PinOrder = order
	a.reconcilePinsLocked(&cfg)
	_ = a.cfg.ReplacePinOrder(cfg.PinOrder)
	return true
}

func (a *App) selectedIDLocked() string {
	if a.selected >= 0 && a.selected < len(a.vitems) {
		return a.vitems[a.selected].ID
	}
	return ""
}

func (a *App) restoreSelectionLocked(id string) {
	if id == "" {
		return
	}
	for i, it := range a.vitems {
		if it.ID == id {
			a.selected = i
			break
		}
	}
	a.lastHovered = -1
	a.scrollToSelected = true
}

func pinUsedByLocked(items []*model.Item, pin string) bool {
	for _, it := range items {
		if it.IsPinned() && it.PinOrderKey() == pin {
			return true
		}
	}
	return false
}

// clearHistory removes unpinned items, or everything when all is true.
func (a *App) clearHistory(all bool) {
	cfg := a.cfg.Snapshot()
	a.mu.Lock()
	defer a.mu.Unlock()
	if all {
		a.items = nil
		clear(a.clipboardSessionLog)
		_ = a.cfg.ReplacePinOrder(nil)
	} else {
		var kept []*model.Item
		for _, it := range a.items {
			if it.IsPinned() {
				kept = append(kept, it)
			} else {
				a.forgetSessionItemLocked(it)
			}
		}
		a.items = kept
	}
	a.rebuildLocked()
	a.saveLocked()
	a.store.GarbageCollect()
	if cfg.ClearSystemClipboard {
		mygo.Clipboard.Clear()
	}
}

func (a *App) movePin(from, to int) {
	// Reorder pinned items; from and to index the pinned portion of view.
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.query != "" {
		return
	}
	selectedID := a.selectedIDLocked()
	cfg := a.cfg.Snapshot()
	pinned := []string{}
	for _, it := range orderItems(a.items, cfg) {
		if it.IsPinned() {
			pinned = append(pinned, it.PinOrderKey())
		}
	}
	if from < 0 || from >= len(pinned) || to < 0 || to > len(pinned) {
		return
	}
	p := pinned[from]
	cp := append([]string(nil), pinned...)
	cp = append(cp[:from], cp[from+1:]...)
	insert := to
	if from < to {
		insert--
	}
	if insert > len(cp) {
		insert = len(cp)
	}
	cp = append(cp[:insert], append([]string{p}, cp[insert:]...)...)
	_ = a.cfg.ReplacePinOrder(cp)
	a.rebuildLocked()
	a.restoreSelectionLocked(selectedID)
	a.saveLocked()
}

func (a *App) saveStore() {
	a.saveLocked()
}

// saveLocked mirrors the in-memory slice into the store and persists it.
func (a *App) saveLocked() {
	a.store.Items = a.items
	_ = a.store.Save()
	a.pruneImageCacheLocked()
}

// setItemTitle updates an item's title (used after OCR).
func (a *App) setItemTitle(id, title string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, it := range a.items {
		if it.ID == id {
			if it.Title == title {
				return
			}
			it.Title = title
			a.rebuildLocked()
			a.saveLocked()
			return
		}
	}
}
