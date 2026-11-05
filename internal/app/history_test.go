package app

import (
	"testing"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	cfg, err := config.Load(dir + "/config.json")
	if err != nil {
		t.Fatal(err)
	}
	store, err := model.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, store)
}

func makeItem(t *testing.T, a *App, title, text string) *model.Item {
	t.Helper()
	h, size, err := a.store.PutBlob([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return &model.Item{
		Title:    title,
		Contents: []model.Content{{Type: "text/plain", Hash: h, Size: size}},
	}
}

func TestAddItemDedup(t *testing.T) {
	a := newTestApp(t)
	a.addItem(makeItem(t, a, "hello", "hello"))
	a.addItem(makeItem(t, a, "hello", "hello"))
	if len(a.items) != 1 {
		t.Fatalf("want 1 item after dedupe, got %d", len(a.items))
	}
	if a.items[0].NumberOfCopies != 2 {
		t.Fatalf("want 2 copies, got %d", a.items[0].NumberOfCopies)
	}
}

func TestTrimHistory(t *testing.T) {
	a := newTestApp(t)
	_ = a.cfg.Update(func(c *config.Config) { c.HistorySize = 3 })
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		a.addItem(makeItem(t, a, s, s))
	}
	unpinned := 0
	for _, it := range a.items {
		if it.Pin == "" {
			unpinned++
		}
	}
	if unpinned != 3 {
		t.Fatalf("want 3 unpinned, got %d", unpinned)
	}
}

func TestTogglePin(t *testing.T) {
	a := newTestApp(t)
	a.addItem(makeItem(t, a, "keep me", "keep me"))
	a.togglePin(a.items[0])
	if a.items[0].Pin == "" {
		t.Fatal("expected a pin")
	}
	if len(a.cfg.Snapshot().PinOrder) != 1 {
		t.Fatal("expected pin order to have one entry")
	}
	a.togglePin(a.items[0])
	if a.items[0].Pin != "" {
		t.Fatal("expected pin removed")
	}
}

func TestViewOrderNewestFirst(t *testing.T) {
	a := newTestApp(t)
	a.addItem(makeItem(t, a, "first", "first"))
	a.addItem(makeItem(t, a, "second", "second"))
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rebuildLocked()
	if len(a.vitems) != 2 || a.vitems[0].Title != "second" {
		t.Fatalf("expected newest first, got %v", titles(a.vitems))
	}
}

func titles(items []*model.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}
