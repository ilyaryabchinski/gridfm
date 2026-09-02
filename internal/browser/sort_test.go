package browser_test

import (
	"testing"

	"gridfm/internal/browser"
)

func TestSortOrdersDirectoriesFirstThenCaseInsensitiveNames(t *testing.T) {
	t.Parallel()

	entries := []browser.Entry{
		{Name: "zeta.txt", Path: "/d/zeta.txt"},
		{Name: "Beta", Path: "/d/Beta", IsDir: true},
		{Name: "alpha", Path: "/d/alpha"},
		{Name: "Apple", Path: "/d/Apple"},
		{Name: "apple2", Path: "/d/apple2"},
		{Name: "adir", Path: "/d/adir", IsDir: true},
	}

	browser.SortEntries(entries)

	want := []string{"/d/adir", "/d/Beta", "/d/alpha", "/d/Apple", "/d/apple2", "/d/zeta.txt"}
	got := entryPaths(entries)
	for i, e := range entries {
		if e.Path != want[i] {
			t.Fatalf("entries[%d].Path = %q, want %q (full order %v)", i, e.Path, want[i], got)
		}
	}
}

func TestSortIsDeterministicOnEqualLowercaseNames(t *testing.T) {
	t.Parallel()

	entries := []browser.Entry{
		{Name: "README", Path: "/d/README"},
		{Name: "readme", Path: "/d/readme"},
		{Name: "ReadMe", Path: "/d/ReadMe"},
	}

	browser.SortEntries(entries)
	first := entryPaths(entries)

	for range 10 {
		browser.SortEntries(entries)
		if got := entryPaths(entries); !equalStrings(got, first) {
			t.Fatalf("sort is unstable: got %v, want %v", got, first)
		}
	}

	// Same lowercase name: the raw name tie-break orders these three.
	if entries[0].Name != "README" || entries[1].Name != "ReadMe" || entries[2].Name != "readme" {
		t.Errorf("unexpected tie-break order: %v", first)
	}
}

func entryPaths(entries []browser.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Path
	}

	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
