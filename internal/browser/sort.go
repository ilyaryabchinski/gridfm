package browser

import (
	"slices"
	"strings"
)

// SortEntries orders entries in place, deterministically: directories first,
// then a case-insensitive name comparison. The raw name and finally the full
// path act as tie-breakers so equal names always sort stably.
func SortEntries(entries []Entry) {
	slices.SortFunc(entries, compareEntries)
}

func compareEntries(a, b Entry) int {
	if a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}

		return 1
	}
	if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
		return c
	}
	if c := strings.Compare(a.Name, b.Name); c != 0 {
		return c
	}

	return strings.Compare(a.Path, b.Path)
}
