// Package model defines a clipboard history item and its persisted store.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Content is one pasteboard representation of an item, stored as a blob.
type Content struct {
	Type string `json:"type"`
	Hash string `json:"hash"`
	Size int    `json:"size"`
}

// Item is one entry of the clipboard history.
type Item struct {
	ID              string    `json:"id"`
	Application     string    `json:"application,omitempty"`
	ApplicationName string    `json:"applicationName,omitempty"`
	FirstCopiedAt   time.Time `json:"firstCopiedAt"`
	LastCopiedAt    time.Time `json:"lastCopiedAt"`
	NumberOfCopies  int       `json:"numberOfCopies"`
	Pin             string    `json:"pin,omitempty"`
	Pinned          bool      `json:"pinned,omitempty"`
	Title           string    `json:"title"`
	Files           []string  `json:"files,omitempty"`
	Contents        []Content `json:"contents"`
	// ClipboardChangeCount relates ModifiedType events within this run.
	// Platform clipboard sequence numbers are not meaningful after restart.
	ClipboardChangeCount int64 `json:"-"`
	// PendingBlobs protects a captured item until its queued history update
	// has either persisted it or discarded it as a duplicate.
	PendingBlobs *BlobWrite `json:"-"`
}

func (i *Item) ReleaseBlobs() {
	if i != nil && i.PendingBlobs != nil {
		i.PendingBlobs.Release()
	}
}

// IsPinned also recognizes histories written before Pinned was stored.
// A pinned item may have no shortcut when every supported key is assigned.
func (i *Item) IsPinned() bool { return i != nil && (i.Pinned || i.Pin != "") }

// PinOrderKey preserves existing key-based orders and gives keyless pins
// an identity of their own, so several such pins retain a stable order.
func (i *Item) PinOrderKey() string {
	if !i.IsPinned() {
		return ""
	}
	if i.Pin != "" {
		return i.Pin
	}
	return "id:" + i.ID
}

// Store is the on-disk history: an index plus a directory of blobs.
type Store struct {
	dir       string
	mu        sync.Mutex
	Items     []*Item `json:"items"`
	committed map[string]bool
	current   map[string]bool
	pending   map[string]int
	unclaimed map[string]bool
}

// BlobWrite reserves newly captured representations across the asynchronous
// handoff to the UI. All its state is protected by its store's mutex.
type BlobWrite struct {
	store    *Store
	hashes   map[string]bool
	released bool
}

func (s *Store) BeginBlobWrite() *BlobWrite {
	return &BlobWrite{store: s, hashes: make(map[string]bool)}
}

func (w *BlobWrite) PutBlob(b []byte) (string, int, error) {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if w.released {
		return "", 0, errors.New("model: blob write already released")
	}
	h, n, err := w.store.putBlobLocked(b)
	if err == nil && !w.hashes[h] {
		w.hashes[h] = true
		if w.store.pending == nil {
			w.store.pending = make(map[string]int)
		}
		w.store.pending[h]++
	}
	return h, n, err
}

func (w *BlobWrite) Release() {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	w.releaseLocked()
}

func (w *BlobWrite) releaseLocked() {
	if w.released {
		return
	}
	w.released = true
	for h := range w.hashes {
		w.store.pending[h]--
		if w.store.pending[h] == 0 {
			delete(w.store.pending, h)
		}
	}
}

type indexFile struct {
	Items []*Item `json:"items"`
}

// Open loads (or creates) the store rooted at dir.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir}
	if err := os.MkdirAll(s.blobDir(), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.indexPath())
	if err == nil {
		var idx indexFile
		if err := json.Unmarshal(data, &idx); err == nil {
			s.Items = idx.Items
		} else {
			return nil, fmt.Errorf("model: read history index: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	s.committed = referencedBlobs(s.Items)
	s.current = s.committed
	return s, nil
}

func (s *Store) indexPath() string        { return filepath.Join(s.dir, "history.json") }
func (s *Store) blobDir() string          { return filepath.Join(s.dir, "blobs") }
func (s *Store) blobPath(h string) string { return filepath.Join(s.blobDir(), h) }

// PutBlob stores b and returns its hash and size.
func (s *Store) PutBlob(b []byte) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, n, err := s.putBlobLocked(b)
	if err == nil {
		if s.unclaimed == nil {
			s.unclaimed = make(map[string]bool)
		}
		s.unclaimed[h] = true
	}
	return h, n, err
}

func (s *Store) putBlobLocked(b []byte) (string, int, error) {
	sum := sha256.Sum256(b)
	h := hex.EncodeToString(sum[:])
	path := s.blobPath(h)
	if _, err := os.Stat(path); err == nil {
		return h, len(b), nil
	}
	if err := writeAtomic(path, b); err != nil {
		return "", 0, err
	}
	return h, len(b), nil
}

// ReadBlob returns the bytes of a blob.
func (s *Store) ReadBlob(h string) ([]byte, error) {
	return os.ReadFile(s.blobPath(h))
}

// Save writes the index atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Keep the live history usable after a failed save as well. Its newly
	// captured items may be retried after their queue leases are released.
	s.current = referencedBlobs(s.Items)
	data, err := json.MarshalIndent(indexFile{Items: s.Items}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(s.indexPath(), data); err != nil {
		return err
	}
	// Update the GC authority only after the new index is safely installed.
	// A failed save must leave every blob needed by the previous index intact.
	s.committed = s.current
	for h := range s.committed {
		delete(s.unclaimed, h)
	}
	for _, item := range s.Items {
		if item != nil && item.PendingBlobs != nil && item.PendingBlobs.store == s {
			item.PendingBlobs.releaseLocked()
		}
	}
	s.garbageCollectLocked()
	return nil
}

func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".maccy-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0o644); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func referencedBlobs(items []*Item) map[string]bool {
	used := make(map[string]bool)
	for _, it := range items {
		if it == nil {
			continue
		}
		for _, c := range it.Contents {
			used[c.Hash] = true
		}
	}
	return used
}

// Dir returns the store's directory.
func (s *Store) Dir() string { return s.dir }

// GarbageCollect removes blobs no item references.
func (s *Store) GarbageCollect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.garbageCollectLocked()
}

func (s *Store) garbageCollectLocked() {
	entries, err := os.ReadDir(s.blobDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if !s.committed[e.Name()] && !s.current[e.Name()] && !s.unclaimed[e.Name()] && s.pending[e.Name()] == 0 {
			_ = os.Remove(filepath.Join(s.blobDir(), e.Name()))
		}
	}
}
