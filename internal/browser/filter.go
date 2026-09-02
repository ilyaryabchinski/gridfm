package browser

import "strings"

// FilterEntries applies the hidden-file rule and the incremental filter to a
// sorted entry list, returning the visible subset in the original order.
// Hidden files are dot-prefixed names; they are excluded unless shown. The
// filter is a case-insensitive substring match on the name.
func FilterEntries(entries []Entry, showHidden bool, query string) []Entry {
	query = strings.ToLower(query)
	visible := make([]Entry, 0, len(entries))

	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name, ".") {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Name), query) {
			continue
		}

		visible = append(visible, e)
	}

	return visible
}
