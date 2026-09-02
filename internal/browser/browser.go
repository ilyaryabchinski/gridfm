package browser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Browser couples a directory location with its entries, the derived
// visible view, the spatial grid, and location history. It is a plain data
// structure; the application layer drives it from the update loop and
// rendering reads it without mutating.
type Browser struct {
	Path    string
	Entries []Entry

	grid       Grid
	visible    []Entry
	filter     string
	showHidden bool
	sortMode   SortMode
	sortOrder  SortOrder

	history    []string
	historyPos int
	// pendingStep records an in-flight back or forward move. The history
	// position commits only when the matching load confirms it, so a
	// failed traversal leaves the cursor consistent for a retry.
	pendingStep histStep

	selection *Selection
}

// histStep is the direction of an in-flight history traversal.
type histStep int

const (
	histNone histStep = iota
	histBack
	histForward
)

// New returns a browser rooted at path with no entries loaded yet.
func New(path string) Browser {
	return Browser{Path: path, grid: NewGrid(0, 1), selection: NewSelection()}
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

// FocusIndex returns the focused index into the visible view, or 0 for an
// empty browser. It is the read-only geometry rendering needs to mark the
// focused card.
func (b *Browser) FocusIndex() int { return b.grid.Focus() }

// FocusedRow returns the row of the focused entry.
func (b *Browser) FocusedRow() int { return b.grid.FocusedRow() }

// SetFocusIndex moves focus to a specific visible index, clamped to the
// valid range.
func (b *Browser) SetFocusIndex(index int) { b.grid.SetFocus(index) }

// Focused returns the focused entry from the visible view, if any.
func (b *Browser) Focused() (Entry, bool) {
	if b.FocusIndex() >= len(b.visible) {
		return Entry{}, false
	}

	return b.visible[b.FocusIndex()], true
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

// Visible returns the entries surviving the hidden-file rule and the
// active filter, in sorted order.
func (b *Browser) Visible() []Entry { return b.visible }

// Filter returns the active incremental filter query.
func (b *Browser) Filter() string { return b.filter }

// ShowHidden reports whether hidden (dot-prefixed) entries are visible.
func (b *Browser) ShowHidden() bool { return b.showHidden }

// SortMode returns the active sort mode.
func (b *Browser) SortMode() SortMode { return b.sortMode }

// SortOrder returns the active sort order.
func (b *Browser) SortOrder() SortOrder { return b.sortOrder }

// SetSort changes the ordering and re-derives the visible view.
func (b *Browser) SetSort(mode SortMode, order SortOrder) {
	b.sortMode = mode
	b.sortOrder = order
	SortEntries(b.Entries, mode, order)
	b.rebuild(b.grid.Focus())
}

// SetFilter changes the incremental filter and re-derives the visible view.
// Focus is preserved by entry identity when the focused entry survives.
func (b *Browser) SetFilter(query string) {
	b.filter = query
	b.rebuild(b.grid.Focus())
}

// SetShowHidden toggles dot-prefixed entries and re-derives the visible
// view. Focus is preserved by entry identity when the focused entry
// survives.
func (b *Browser) SetShowHidden(show bool) {
	b.showHidden = show
	b.rebuild(b.grid.Focus())
}

// SetEntries applies a directory listing. Focus is preserved by entry
// identity when the path is unchanged: the previously focused entry stays
// focused if it still exists, otherwise focus falls back to the nearest
// surviving index. Navigating to a different path focuses the first entry
// and records the location in history; confirming a back or forward move
// updates the history position instead of pushing a duplicate.
func (b *Browser) SetEntries(path string, entries []Entry) {
	sameDir := path == b.Path
	previousFocus := b.grid.Focus()

	b.sortEntries(entries)
	b.Path = path
	b.Entries = entries
	if sameDir {
		// A refresh falls back to the nearest surviving index.
		b.rebuild(previousFocus)
	} else {
		// A new location focuses the first entry.
		b.rebuild(0)
	}

	if !sameDir || len(b.history) == 0 {
		b.commitHistory(path)
	}
}

// CancelHistoryStep abandons an in-flight back or forward move after a
// failed load, keeping the history position on the current location so the
// traversal can be retried.
func (b *Browser) CancelHistoryStep() {
	b.pendingStep = histNone
}

// FocusEntryInVisible moves focus to the visible entry at path. It reports
// whether the entry was found.
func (b *Browser) FocusEntryInVisible(path string) bool {
	for i, e := range b.visible {
		if e.Path == path {
			b.grid.SetFocus(i)

			return true
		}
	}

	return false
}

// CanBack reports whether there is a location behind the current one. It is
// false while a traversal is in flight so steps cannot stack up.
func (b *Browser) CanBack() bool {
	return b.pendingStep == histNone && b.historyPos > 1
}

// CanForward reports whether there is a location ahead of the current one.
// It is false while a traversal is in flight.
func (b *Browser) CanForward() bool {
	return b.pendingStep == histNone && b.historyPos < len(b.history)
}

// Back returns the previous location and marks the move in flight. The
// history position commits only when the caller confirms the move by
// loading that path; a failed load must call CancelHistoryStep.
func (b *Browser) Back() (string, bool) {
	if !b.CanBack() {
		return "", false
	}

	b.pendingStep = histBack

	return b.history[b.historyPos-2], true
}

// Forward returns the next location and marks the move in flight, mirroring
// Back.
func (b *Browser) Forward() (string, bool) {
	if !b.CanForward() {
		return "", false
	}

	b.pendingStep = histForward

	return b.history[b.historyPos], true
}

func (b *Browser) sortEntries(entries []Entry) {
	SortEntries(entries, b.sortMode, b.sortOrder)
}

// rebuild re-derives the visible view from the entry list and the current
// filter state, keeping the focused entry by identity when it survives and
// otherwise falling back to the given index (clamped to the valid range).
func (b *Browser) rebuild(fallback int) {
	previousPath := b.FocusedPath()
	b.visible = FilterEntries(b.Entries, b.showHidden, b.filter)
	b.grid.SetCount(len(b.visible))
	b.restoreFocus(previousPath, fallback)
}

func (b *Browser) restoreFocus(previousPath string, previousIndex int) {
	if previousPath != "" && b.FocusEntryInVisible(previousPath) {
		return
	}
	b.grid.SetFocus(min(previousIndex, max(len(b.visible)-1, 0)))
}

// commitHistory records a confirmed location. An in-flight back or forward
// move commits its position change when the loaded path matches; anything
// else is a fresh navigation that drops any forward entries. The history
// position indexes one past the current location.
func (b *Browser) commitHistory(path string) {
	switch {
	case b.pendingStep == histBack && b.historyPos >= 2 && b.history[b.historyPos-2] == path:
		b.historyPos--
		b.pendingStep = histNone

		return
	case b.pendingStep == histForward && b.historyPos < len(b.history) && b.history[b.historyPos] == path:
		b.historyPos++
		b.pendingStep = histNone

		return
	case b.pendingStep != histNone:
		// A stale or mismatched step resolved elsewhere: drop it and treat
		// the load as fresh navigation.
		b.pendingStep = histNone
	}

	if len(b.history) > 0 && b.history[b.historyPos-1] == path {
		return
	}

	b.history = append(b.history[:b.historyPos], path)
	b.historyPos = len(b.history)
}

// ReadDir enumerates the directory at path and returns entries in
// deterministic order under the current sort. Only cheap metadata is
// collected; symlinks are reported as non-directories here and resolved
// when explicitly opened.
func ReadDir(path string) ([]Entry, error) {
	dirents, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", path, err)
	}

	entries := make([]Entry, 0, len(dirents))

	for _, d := range dirents {
		entry := Entry{
			Name:    d.Name(),
			Path:    filepath.Join(path, d.Name()),
			IsDir:   d.IsDir(),
			Symlink: d.Type()&fs.ModeSymlink != 0,
			Mode:    d.Type(),
		}
		// Keep symlinks cheap to load; entering one resolves it.
		if !entry.Symlink {
			info, infoErr := d.Info()
			if infoErr == nil {
				entry.Size = info.Size()
				entry.ModTime = info.ModTime()
				entry.Mode = info.Mode()
			}
		}

		entries = append(entries, entry)
	}

	SortEntries(entries, SortByName, SortAscending)

	return entries, nil
}
