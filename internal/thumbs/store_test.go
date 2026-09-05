package thumbs_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gridfm/internal/thumbs"
)

func entryAt(name string, offset time.Duration) thumbs.Entry {
	return thumbs.Entry{
		Path:       filepath.Join("/d", name),
		Size:       123,
		MtimeNanos: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC).Add(offset).UnixNano(),
	}
}

func pngBytes(size int) []byte { return bytes.Repeat([]byte{0x89}, size) }

func TestStoreRoundtrip(t *testing.T) {
	t.Parallel()

	s, err := thumbs.Open(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	e := entryAt("pic.png", 0)
	if _, ok := s.Get(e); ok {
		t.Fatal("an empty cache must miss")
	}

	if err := s.Put(e, pngBytes(64)); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Get(e)
	if !ok || !bytes.Equal(got, pngBytes(64)) {
		t.Fatalf("Get = (%v, %v), want the stored bytes", got, ok)
	}
}

func TestKeyBustsOnChange(t *testing.T) {
	t.Parallel()

	base := thumbs.Entry{Path: "/d/pic.png", Size: 10, MtimeNanos: 1}

	first, second := thumbs.KeyOf(base), thumbs.KeyOf(base)
	if first != second {
		t.Fatal("the key must be deterministic")
	}
	for _, changed := range []thumbs.Entry{
		{Path: "/d/other.png", Size: 10, MtimeNanos: 1},
		{Path: "/d/pic.png", Size: 11, MtimeNanos: 1},
		{Path: "/d/pic.png", Size: 10, MtimeNanos: 2},
	} {
		if thumbs.KeyOf(base) == thumbs.KeyOf(changed) {
			t.Fatalf("key must change for %+v", changed)
		}
	}
}

func TestStoreEvictsOldestFirst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := thumbs.Open(dir, 160) // fits two 64-byte entries, not three
	if err != nil {
		t.Fatal(err)
	}

	first := entryAt("a.png", 0)
	second := entryAt("b.png", time.Minute)
	third := entryAt("c.png", 2*time.Minute)

	for _, e := range []thumbs.Entry{first, second, third} {
		if err := s.Put(e, pngBytes(64)); err != nil {
			t.Fatal(err)
		}
	}

	if _, ok := s.Get(first); ok {
		t.Error("the oldest entry should have been evicted")
	}
	if _, ok := s.Get(second); !ok {
		t.Error("the middle entry should have survived")
	}
	if _, ok := s.Get(third); !ok {
		t.Error("the newest entry should have survived")
	}
}

func TestStoreGetRefreshesRecency(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := thumbs.Open(dir, 160)
	if err != nil {
		t.Fatal(err)
	}

	// Stage 1: two entries land at the same instant.
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	s.SetClockForTesting(func() time.Time { return base })

	older := entryAt("old.png", 0)
	newer := entryAt("new.png", 0)
	if putErr := s.Put(older, pngBytes(64)); putErr != nil {
		t.Fatal(putErr)
	}
	if putErr := s.Put(newer, pngBytes(64)); putErr != nil {
		t.Fatal(putErr)
	}

	// Stage 2: the clock advances and a Get touches the older entry,
	// moving it ahead of the untouched one.
	s.SetClockForTesting(func() time.Time { return base.Add(10 * time.Minute) })
	if _, ok := s.Get(older); !ok {
		t.Fatal("expected the entry to be cached")
	}

	// Stage 3: a third entry arrives; the budget now evicts the true
	// oldest — the untouched one — even though both were "first".
	s.SetClockForTesting(func() time.Time { return base.Add(20 * time.Minute) })
	if putErr := s.Put(entryAt("x.png", 0), pngBytes(64)); putErr != nil {
		t.Fatal(putErr)
	}

	if _, ok := s.Get(newer); ok {
		t.Error("the untouched entry should have been evicted before the touched one")
	}
	if _, ok := s.Get(older); !ok {
		t.Error("the touched entry should have survived")
	}
}

func TestStoreSurvivesReopen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := thumbs.Open(dir, 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	e := entryAt("keep.png", 0)
	if putErr := s.Put(e, pngBytes(32)); putErr != nil {
		t.Fatal(putErr)
	}

	reopened, openErr := thumbs.Open(dir, 1<<20)
	if openErr != nil {
		t.Fatal(openErr)
	}
	if _, ok := reopened.Get(e); !ok {
		t.Error("entries must survive a restart")
	}
}

func TestStoreFilesAreOwnerOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := thumbs.Open(dir, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(entryAt("perm.png", 0), pngBytes(32)); err != nil {
		t.Fatal(err)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one cache file, got %d (err %v)", len(entries), readErr)
	}
	info, infoErr := entries[0].Info()
	if infoErr != nil {
		t.Fatal(infoErr)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file mode = %o, want 600", perm)
	}
}
