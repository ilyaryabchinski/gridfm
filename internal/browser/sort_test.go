package browser_test

import (
	"testing"
	"time"

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

	browser.SortEntries(entries, browser.SortByName, browser.SortAscending)

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

	browser.SortEntries(entries, browser.SortByName, browser.SortAscending)
	first := entryPaths(entries)

	for range 10 {
		browser.SortEntries(entries, browser.SortByName, browser.SortAscending)
		if got := entryPaths(entries); !equalStrings(got, first) {
			t.Fatalf("sort is unstable: got %v, want %v", got, first)
		}
	}

	// Same lowercase name: the raw name tie-break orders these three.
	if entries[0].Name != "README" || entries[1].Name != "ReadMe" || entries[2].Name != "readme" {
		t.Errorf("unexpected tie-break order: %v", first)
	}
}

func TestSortBySizeAndOrder(t *testing.T) {
	t.Parallel()

	entries := []browser.Entry{
		{Name: "big", Path: "/d/big", Size: 300},
		{Name: "small", Path: "/d/small", Size: 1},
		{Name: "mid", Path: "/d/mid", Size: 100},
		{Name: "dir", Path: "/d/dir", IsDir: true, Size: 4096},
	}

	browser.SortEntries(entries, browser.SortBySize, browser.SortAscending)
	want := []string{"/d/dir", "/d/small", "/d/mid", "/d/big"}
	if got := entryPaths(entries); !equalStrings(got, want) {
		t.Errorf("ascending order = %v, want %v", got, want)
	}

	browser.SortEntries(entries, browser.SortBySize, browser.SortDescending)
	want = []string{"/d/dir", "/d/big", "/d/mid", "/d/small"}
	if got := entryPaths(entries); !equalStrings(got, want) {
		t.Errorf("descending order = %v, want %v (directories stay first)", got, want)
	}
}

func TestSortByModified(t *testing.T) {
	t.Parallel()

	base := time.Unix(1700000000, 0)
	entries := []browser.Entry{
		{Name: "newest", Path: "/d/newest", ModTime: base.Add(2 * time.Hour)},
		{Name: "oldest", Path: "/d/oldest", ModTime: base},
		{Name: "newer", Path: "/d/newer", ModTime: base.Add(time.Hour)},
	}

	browser.SortEntries(entries, browser.SortByModified, browser.SortAscending)
	want := []string{"/d/oldest", "/d/newer", "/d/newest"}
	if got := entryPaths(entries); !equalStrings(got, want) {
		t.Errorf("ascending order = %v, want %v", got, want)
	}
}

func TestSortByType(t *testing.T) {
	t.Parallel()

	entries := []browser.Entry{
		{Name: "b.txt", Path: "/d/b.txt"},
		{Name: "c.zip", Path: "/d/c.zip"},
		{Name: "a.txt", Path: "/d/a.txt"},
		{Name: "noext", Path: "/d/noext"},
		{Name: "A.TXT", Path: "/d/A.TXT"},
	}

	browser.SortEntries(entries, browser.SortByType, browser.SortAscending)
	want := []string{"/d/noext", "/d/A.TXT", "/d/a.txt", "/d/b.txt", "/d/c.zip"}
	if got := entryPaths(entries); !equalStrings(got, want) {
		t.Errorf("type order = %v, want %v (extension groups, name tie-break)", got, want)
	}
}

func TestEntryPermissionsString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		entry browser.Entry
		want  string
	}{
		{browser.Entry{Name: "d", IsDir: true, Mode: 0o755}, "drwxr-xr-x"},
		{browser.Entry{Name: "f", Mode: 0o644}, "-rw-r--r--"},
		{browser.Entry{Name: "l", Symlink: true, Mode: 0o777}, "lrwxrwxrwx"},
	}
	for _, tt := range tests {
		if got := tt.entry.Permissions(); got != tt.want {
			t.Errorf("Permissions() = %q, want %q", got, tt.want)
		}
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
