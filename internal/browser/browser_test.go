package browser_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"gridfm/internal/browser"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()

	err := os.Mkdir(path, 0o755)
	if err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadDirReturnsSortedEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "Alpha"))
	mustWrite(t, filepath.Join(dir, ".hidden"), "x")
	mustWrite(t, filepath.Join(dir, "a.txt"), "x")
	mustWrite(t, filepath.Join(dir, "b.txt"), "x")

	entries, err := browser.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	wantPaths := []string{
		filepath.Join(dir, "Alpha"),
		filepath.Join(dir, ".hidden"),
		filepath.Join(dir, "a.txt"),
		filepath.Join(dir, "b.txt"),
	}
	gotPaths := entryPaths(entries)
	if !equalStrings(gotPaths, wantPaths) {
		t.Errorf("entry order = %v, want %v", gotPaths, wantPaths)
	}
	if !entries[0].IsDir {
		t.Errorf("entry %q should be a directory", entries[0].Name)
	}
	if entries[1].IsDir || entries[2].IsDir || entries[3].IsDir {
		t.Error("files should not be marked as directories")
	}
}

func TestReadDirReportsSymlinksAsNonDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	mustMkdir(t, target)

	err := os.Symlink(target, filepath.Join(dir, "link"))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := browser.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, e := range entries {
		if e.Name == "link" && e.IsDir {
			t.Error("symlink should be reported as a non-directory")
		}
	}
}

func TestReadDirMissingPathWrapsError(t *testing.T) {
	t.Parallel()

	_, err := browser.ReadDir(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("ReadDir on a missing path should fail")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error should wrap fs.ErrNotExist, got %v", err)
	}
}

func TestSetEntriesPreservesFocusByIdentityOnRefresh(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
		{Name: "c", Path: "/d/c"},
	})
	b.SetFocusIndex(1)

	// A refresh returns a new slice; entry /d/b keeps focus.
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
		{Name: "c", Path: "/d/c"},
		{Name: "d", Path: "/d/d"},
	})

	if got := b.FocusedPath(); got != "/d/b" {
		t.Errorf("FocusedPath after refresh = %q, want %q", got, "/d/b")
	}
}

func TestSetEntriesFallsBackToNearestIndexWhenFocusedEntryVanishes(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
		{Name: "c", Path: "/d/c"},
	})
	b.SetFocusIndex(2)

	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	})

	if got := b.FocusedPath(); got != "/d/b" {
		t.Errorf("FocusedPath = %q, want nearest survivor %q", got, "/d/b")
	}
}

func TestSetEntriesNewDirectoryFocusesFirstEntry(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	b.SetEntries("/d", []browser.Entry{
		{Name: "a", Path: "/d/a"},
		{Name: "b", Path: "/d/b"},
	})
	b.SetFocusIndex(1)

	b.SetEntries("/sub", []browser.Entry{
		{Name: "x", Path: "/sub/x"},
		{Name: "y", Path: "/sub/y"},
	})

	if got := b.FocusedPath(); got != "/sub/x" {
		t.Errorf("FocusedPath after navigation = %q, want %q", got, "/sub/x")
	}
}

func TestFocusedOnEmptyBrowser(t *testing.T) {
	t.Parallel()

	b := browser.New("/d")
	if _, ok := b.Focused(); ok {
		t.Error("Focused on an empty browser should report no entry")
	}
	if got := b.FocusedPath(); got != "" {
		t.Errorf("FocusedPath = %q, want empty", got)
	}
}
