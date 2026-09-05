package places

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadLinesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	paths := []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
	if err := SaveBookmarks(paths); err != nil {
		t.Fatal(err)
	}

	got := LoadLines(bookmarksFile)
	if len(got) != 2 || got[0] != paths[0] || got[1] != paths[1] {
		t.Errorf("loaded = %q, want %q", got, paths)
	}
}

func TestSaveLinesIsAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := SaveRecents([]string{"/first"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveRecents([]string{"/second"}); err != nil {
		t.Fatal(err)
	}

	got := LoadLines(recentsFile)
	if len(got) != 1 || got[0] != "/second" {
		t.Errorf("loaded = %q, want [/second]", got)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "gridfm"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "recents.conf" {
			continue
		}
		t.Errorf("temp file left behind: %s", entry.Name())
	}
}

func TestLoadLinesMissingFileIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if got := LoadLines(bookmarksFile); got != nil {
		t.Errorf("missing file = %q, want nil", got)
	}
}

func TestLoadLinesSkipsCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := SaveLines(recentsFile, []string{"/a", "", "/b"}); err != nil {
		t.Fatal(err)
	}
	got := LoadLines(recentsFile)
	if len(got) != 2 {
		t.Errorf("loaded = %q, want two paths", got)
	}
}

func TestPlacesFromLinesSkipsVanishedPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keep := filepath.Join(dir, "keep")
	if err := os.Mkdir(keep, 0o755); err != nil {
		t.Fatal(err)
	}

	out := placesFromLines([]string{keep, filepath.Join(dir, "ghost")})
	if len(out) != 1 || out[0].Path != keep || out[0].Label != "keep" {
		t.Errorf("places = %+v, want only %q", out, keep)
	}
}

func TestConfigDirFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/tester")

	if got := ConfigDir(); got != filepath.Join("/home/tester", ".config", "gridfm") {
		t.Errorf("ConfigDir = %q", got)
	}
}
