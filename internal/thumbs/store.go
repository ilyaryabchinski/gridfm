package thumbs

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store is a bounded on-disk thumbnail cache. Recency is file mtime:
// hits refresh it, eviction removes the oldest entries until the cache
// fits its budget. All methods are safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	clock    func() time.Time
}

// Open prepares the cache directory, creating it if needed.
func Open(dir string, maxBytes int64) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("thumbnail cache: %w", err)
	}

	return &Store{dir: dir, maxBytes: maxBytes, clock: time.Now}, nil
}

// SetClockForTesting replaces the recency clock. Tests only.
func (s *Store) SetClockForTesting(now func() time.Time) { s.clock = now }

// Get returns the cached PNG for the entry, if present. A hit refreshes
// the entry's recency.
func (s *Store) Get(e Entry) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := s.file(e)
	raw, err := os.ReadFile(name)
	if err != nil {
		return nil, false
	}
	// Best-effort recency refresh; a stale mtime only affects
	// eviction order.
	now := s.clock()
	_ = os.Chtimes(name, now, now)

	return raw, true
}

// Put stores the PNG for the entry and prunes the cache back within its
// budget, evicting the least recently used files first.
func (s *Store) Put(e Entry, png []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	final := s.file(e)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, png, 0o600); err != nil {
		return fmt.Errorf("thumbnail cache write: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("thumbnail cache write: %w", err)
	}
	// Stamp recency with our clock so eviction order reflects Put/Get
	// sequence, not the real wall clock.
	now := s.clock()
	_ = os.Chtimes(final, now, now)

	s.prune()

	return nil
}

// file maps an entry to its cache path. Keys are hashes, hex-encoded so
// they are always safe file names; no path sanitization is needed.
func (s *Store) file(e Entry) string {
	return filepath.Join(s.dir, hex.EncodeToString([]byte(KeyOf(e)))+".png")
}

// prune deletes the oldest cache files until the total size fits the
// budget. Caller holds mu.
func (s *Store) prune() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}

	type cached struct {
		name  string
		size  int64
		mtime int64
	}

	var files []cached
	var total int64
	for _, ent := range entries {
		if filepath.Ext(ent.Name()) != ".png" {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		files = append(files, cached{
			name:  filepath.Join(s.dir, ent.Name()),
			size:  info.Size(),
			mtime: info.ModTime().UnixNano(),
		})
		total += info.Size()
	}

	sort.Slice(files, func(i, j int) bool { return files[i].mtime < files[j].mtime })
	for _, f := range files {
		if total <= s.maxBytes {
			return
		}
		if os.Remove(f.name) == nil {
			total -= f.size
		}
	}
}
