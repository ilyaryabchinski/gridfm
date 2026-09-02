package browser

import (
	"slices"
	"strings"
)

// SortMode selects the primary comparison key for directory entries.
type SortMode int

// Sort modes, ordered by their numeric identity for stable behavior.
const (
	SortByName SortMode = iota
	SortBySize
	SortByModified
	SortByType
)

// String renders the sort mode for display and tests.
func (m SortMode) String() string {
	switch m {
	case SortByName:
		return "name"
	case SortBySize:
		return "size"
	case SortByModified:
		return "modified"
	case SortByType:
		return "type"
	default:
		return "name"
	}
}

// SortOrder is the direction of the primary comparison key. Tie-breakers
// always stay ascending so ordering remains deterministic.
type SortOrder int

// Sort orders, ascending first.
const (
	SortAscending SortOrder = iota
	SortDescending
)

// String renders the sort order for display and tests.
func (o SortOrder) String() string {
	if o == SortDescending {
		return "desc"
	}

	return "asc"
}

// Opposite returns the reversed order.
func (o SortOrder) Opposite() SortOrder {
	if o == SortAscending {
		return SortDescending
	}

	return SortAscending
}

// SortEntries orders entries in place: directories first, then the primary
// key for the mode, then case-insensitive name, raw name, and full path as
// ascending tie-breakers.
func SortEntries(entries []Entry, mode SortMode, order SortOrder) {
	slices.SortFunc(entries, func(a, b Entry) int {
		return compareEntries(a, b, mode, order)
	})
}

func compareEntries(a, b Entry, mode SortMode, order SortOrder) int {
	if a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}

		return 1
	}

	c := comparePrimary(a, b, mode)
	if c == 0 {
		c = compareNames(a, b)
	}
	if order == SortDescending {
		return -c
	}

	return c
}

func comparePrimary(a, b Entry, mode SortMode) int {
	switch mode {
	case SortByName:
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case SortBySize:
		return compareInt64(a.Size, b.Size)
	case SortByModified:
		return a.ModTime.Compare(b.ModTime)
	case SortByType:
		return strings.Compare(extensionOf(a.Name), extensionOf(b.Name))
	}

	return 0 // unreachable: the switch covers every sort mode
}

func compareNames(a, b Entry) int {
	if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
		return c
	}
	if c := strings.Compare(a.Name, b.Name); c != 0 {
		return c
	}

	return strings.Compare(a.Path, b.Path)
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func extensionOf(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 {
		// Dotfiles sort as typeless rather than by their "extension".
		return ""
	}

	return strings.ToLower(name[dot+1:])
}
