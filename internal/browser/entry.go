// Package browser contains the domain model of the file browser: directory
// entries, deterministic ordering, spatial grid navigation, and the browser
// state that ties them together. It deliberately has no dependency on any
// terminal or rendering library so navigation semantics can be tested without
// an interactive terminal.
package browser

// Entry is a single item in a directory listing. The full path is the entry
// identity within a loaded directory.
type Entry struct {
	Name  string
	Path  string
	IsDir bool
}
