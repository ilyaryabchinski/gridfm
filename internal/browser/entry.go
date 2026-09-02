// Package browser contains the domain model of the file browser: directory
// entries, deterministic ordering, incremental filtering, navigation
// history, spatial grid navigation, and the browser state that ties them
// together. It deliberately has no dependency on any terminal or rendering
// library so navigation semantics can be tested without an interactive
// terminal.
package browser

import (
	"io/fs"
	"strings"
	"time"
)

// Entry is a single item in a directory listing. The full path is the entry
// identity within a loaded directory. Metadata is what a single lstat
// provides cheaply during enumeration; symlinks are never followed here.
type Entry struct {
	Name    string
	Path    string
	ModTime time.Time
	Mode    fs.FileMode
	Size    int64
	IsDir   bool
	Symlink bool
}

// Permissions renders the classic ten-character permission string, with the
// entry type as the leading character.
func (e Entry) Permissions() string {
	lead := "-"
	switch {
	case e.IsDir:
		lead = "d"
	case e.Symlink:
		lead = "l"
	}

	// Perm().String() always prefixes its own type character; drop it in
	// favor of ours.
	return lead + strings.TrimPrefix(e.Mode.Perm().String(), "-")
}
