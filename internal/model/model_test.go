package model

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeItem(t *testing.T, s *Store, body string) *Item {
	t.Helper()
	w := s.BeginBlobWrite()
	h, n, err := w.PutBlob([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return &Item{Contents: []Content{{Type: "text/plain", Hash: h, Size: n}}, PendingBlobs: w}
}

func TestSaveReclaimsEvictedAndDiscardedCaptureBlobs(t *testing.T) {
	s := testStore(t)
	old := writeItem(t, s, "evicted history")
	s.Items = []*Item{old}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	old.ReleaseBlobs() // idempotent after Save commits the item.
	unused := writeItem(t, s, "discarded duplicate representation")
	unused.ReleaseBlobs()
	next := writeItem(t, s, "replacement history")
	s.Items = []*Item{next}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	for _, it := range []*Item{old, unused} {
		if _, err := s.ReadBlob(it.Contents[0].Hash); !os.IsNotExist(err) {
			t.Fatalf("unused blob still exists: %v", err)
		}
	}
	if b, err := s.ReadBlob(next.Contents[0].Hash); err != nil || string(b) != "replacement history" {
		t.Fatalf("replacement lost: %q %v", b, err)
	}
}

func TestFailedSavePreservesDiskAndLiveHistoryBlobs(t *testing.T) {
	s := testStore(t)
	old := writeItem(t, s, "last committed contents")
	s.Items = []*Item{old}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(s.Dir(), "history-backup.json")
	if err := os.Rename(s.indexPath(), backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.indexPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	next := writeItem(t, s, "new unsaved contents")
	s.Items = []*Item{next}
	if err := s.Save(); err == nil {
		t.Fatal("replacing a directory unexpectedly succeeded")
	}
	next.ReleaseBlobs() // mirrors the application's queued OnNew callback.
	s.GarbageCollect()
	for _, it := range []*Item{old, next} {
		if _, err := s.ReadBlob(it.Contents[0].Hash); err != nil {
			t.Fatalf("failed save lost disk/live history blob: %v", err)
		}
	}
	if err := os.Remove(s.indexPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, s.indexPath()); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil || len(reopened.Items) != 1 || reopened.Items[0].Contents[0].Hash != old.Contents[0].Hash {
		t.Fatalf("committed index changed: %#v %v", reopened, err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadBlob(old.Contents[0].Hash); !os.IsNotExist(err) {
		t.Fatal("successful retry did not reclaim old blob")
	}
}

func TestPendingCaptureSurvivesConcurrentSavesUntilUIHandoff(t *testing.T) {
	s := testStore(t)
	const count = 12
	items := make(chan *Item, count)
	var writers sync.WaitGroup
	for i := 0; i < count; i++ {
		writers.Go(func() { items <- writeItem(t, s, "queued capture") })
	}
	for i := 0; i < count; i++ {
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
		s.GarbageCollect()
	}
	writers.Wait()
	close(items)
	for it := range items {
		if b, err := s.ReadBlob(it.Contents[0].Hash); err != nil || string(b) != "queued capture" {
			t.Fatalf("pending capture removed: %q %v", b, err)
		}
		it.ReleaseBlobs()
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.blobDir())
	if err != nil || len(entries) != 0 {
		t.Fatalf("discarded captures not collected: %v %v", entries, err)
	}
}

func TestOpenDoesNotSilentlyReplaceCorruptHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	if err := os.WriteFile(path, []byte("{broken index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("corrupt history was accepted as empty history")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "{broken index" {
		t.Fatal("corrupt index was overwritten")
	}
}
