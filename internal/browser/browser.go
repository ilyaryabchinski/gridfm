package browser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Browser couples a directory location with its entries and the spatial grid
// used to navigate them. It is a plain data structure; the application layer
// drives it from the update loop and rendering reads it without mutating.
type Browser struct {
	Path    string
	Entries []Entry

	grid Grid
}

// New returns a browser rooted at path with no entries loaded yet.
func New(path string) Browser {
	return Browser{Path: path, grid: NewGrid(0, 1)}
}

// Nav exposes the spatial grid for navigation. Rendering must not mutate it.
func (b *Browser) Nav() *Grid { return &b.grid }

// Focused returns the focused entry, if any.
func (b *Browser) Focused() (Entry, bool) {
	if len(b.Entries) == 0 || b.grid.Focus() >= len(b.Entries) {
		return Entry{}, false
	}

	return b.Entries[b.grid.Focus()], true
}

// FocusedPath returns the full path of the focused entry, or an empty string
// when nothing is focused.
func (b *Browser) FocusedPath() string {
	e, ok := b.Focused()
	if !ok {
		return ""
	}

	return e.Path
}

// SetEntries applies a directory listing. Focus is preserved by entry
// identity when the path is unchanged: the previously focused entry stays
// focused if it still exists, otherwise focus falls back to the nearest
// surviving index. Navigating to a different path focuses the first entry.
func (b *Browser) SetEntries(path string, entries []Entry) {
	sameDir := path == b.Path
	previousFocus := b.grid.Focus()
	previousPath := b.FocusedPath()

	b.Path = path
	b.Entries = entries
	b.grid = NewGrid(len(entries), max(b.grid.Columns(), 1))

	if !sameDir {
		return
	}
	if previousPath != "" && b.focusEntry(previousPath) {
		return
	}
	b.grid.SetFocus(min(previousFocus, max(len(entries)-1, 0)))
}

func (b *Browser) focusEntry(path string) bool {
	for i, e := range b.Entries {
		if e.Path == path {
			b.grid.SetFocus(i)

			return true
		}
	}

	return false
}

// ReadDir enumerates the directory at path and returns entries in
// deterministic order. Only cheap metadata is collected: symlinks are
// reported as non-directories here and resolved when explicitly opened.
func ReadDir(path string) ([]Entry, error) {
	dirents, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", path, err)
	}
	entries := make([]Entry, 0, len(dirents))

	for _, d := range dirents {
		if d.Type()&fs.ModeSymlink != 0 {
			// Keep symlinks cheap to load; entering one resolves it.
			entries = append(entries, Entry{Name: d.Name(), Path: filepath.Join(path, d.Name())})

			continue
		}

		entries = append(entries, Entry{Name: d.Name(), Path: filepath.Join(path, d.Name()), IsDir: d.IsDir()})
	}

	SortEntries(entries)

	return entries, nil
}
