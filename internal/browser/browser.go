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

// The navigation methods below are the only way to move the grid: the
// spatial model stays encapsulated so callers express intent (move left,
// page down) instead of reaching into mutable browser state.

// Left moves focus one entry left without wrapping across rows. It reports
// whether focus changed.
func (b *Browser) Left() bool { return b.grid.Left() }

// Right moves focus one entry right without wrapping across rows. It reports
// whether focus changed.
func (b *Browser) Right() bool { return b.grid.Right() }

// Up moves focus to the preferred column of the row above. It reports
// whether focus changed.
func (b *Browser) Up() bool { return b.grid.Up() }

// Down moves focus toward the preferred column of the row below, clamping
// to the nearest valid column in a shorter last row. It reports whether
// focus changed.
func (b *Browser) Down() bool { return b.grid.Down() }

// PageUp moves focus up by the given number of rows, preserving the
// preferred column. It reports whether focus changed.
func (b *Browser) PageUp(rows int) bool { return b.grid.PageUp(rows) }

// PageDown moves focus down by the given number of rows, preserving the
// preferred column. It reports whether focus changed.
func (b *Browser) PageDown(rows int) bool { return b.grid.PageDown(rows) }

// Home moves focus to the first entry. It reports whether focus changed.
func (b *Browser) Home() bool { return b.grid.Home() }

// End moves focus to the last entry. It reports whether focus changed.
func (b *Browser) End() bool { return b.grid.End() }

// SetColumns resizes the navigation grid, preserving the focused entry and
// re-deriving the preferred column from its new position.
func (b *Browser) SetColumns(columns int) { b.grid.SetColumns(columns) }

// FocusIndex returns the focused index, or 0 for an empty browser. It is
// the read-only geometry rendering needs to mark the focused card.
func (b *Browser) FocusIndex() int { return b.grid.Focus() }

// FocusedRow returns the row of the focused entry.
func (b *Browser) FocusedRow() int { return b.grid.FocusedRow() }

// SetFocusIndex moves focus to a specific index, clamped to the valid
// range. Callers should prefer FocusEntry for identity-based restoration.
func (b *Browser) SetFocusIndex(index int) { b.grid.SetFocus(index) }

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
