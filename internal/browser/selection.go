package browser

import "sort"

// Selection tracks chosen entry paths across directories. Selected paths
// persist through navigation, sorting, and filtering so multi-directory
// operations are possible; the application revalidates paths before any
// mutation because the filesystem may have moved on.
type Selection struct {
	paths map[string]struct{}
}

// NewSelection returns an empty selection.
func NewSelection() *Selection {
	return &Selection{paths: make(map[string]struct{})}
}

// Toggle flips the selection state of one path. It reports the new state.
func (s *Selection) Toggle(path string) bool {
	if _, ok := s.paths[path]; ok {
		delete(s.paths, path)

		return false
	}
	s.paths[path] = struct{}{}

	return true
}

// Set fixes the selection state of one path explicitly.
func (s *Selection) Set(path string, on bool) {
	if on {
		s.paths[path] = struct{}{}

		return
	}
	delete(s.paths, path)
}

// Has reports whether the path is selected.
func (s *Selection) Has(path string) bool {
	_, ok := s.paths[path]

	return ok
}

// Len returns the number of selected paths across all directories.
func (s *Selection) Len() int { return len(s.paths) }

// Clear forgets every selection.
func (s *Selection) Clear() {
	s.paths = make(map[string]struct{})
}

// Paths returns the selected paths sorted for deterministic processing.
func (s *Selection) Paths() []string {
	out := make([]string, 0, len(s.paths))
	for p := range s.paths {
		out = append(out, p)
	}
	sort.Strings(out)

	return out
}

// SelectRange marks every visible entry between two visible indexes,
// inclusive, as selected.
func (b *Browser) SelectRange(from, to int) {
	lo, hi := min(from, to), max(from, to)
	for i := lo; i <= hi && i < len(b.visible); i++ {
		b.selection.Set(b.visible[i].Path, true)
	}
}

// SelectAllVisible selects every entry in the visible view.
func (b *Browser) SelectAllVisible() {
	for _, e := range b.visible {
		b.selection.Set(e.Path, true)
	}
}

// ToggleFocused flips the selection of the focused entry and reports the
// new state.
func (b *Browser) ToggleFocused() bool {
	e, ok := b.Focused()
	if !ok {
		return false
	}

	return b.selection.Toggle(e.Path)
}

// FocusedSelected reports whether the focused entry is selected.
func (b *Browser) FocusedSelected() bool {
	e, ok := b.Focused()
	if !ok {
		return false
	}

	return b.selection.Has(e.Path)
}

// Selection exposes the selection for queries and mutation by the
// application layer.
func (b *Browser) Selection() *Selection { return b.selection }

// SelectedPaths returns every selected path, sorted, across all
// directories.
func (b *Browser) SelectedPaths() []string { return b.selection.Paths() }

// SelectedCount returns the number of selected paths.
func (b *Browser) SelectedCount() int { return b.selection.Len() }

// ClearSelection forgets every selected path.
func (b *Browser) ClearSelection() { b.selection.Clear() }
