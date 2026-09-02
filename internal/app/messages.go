// Package app wires the browser domain model to the Bubble Tea runtime: the
// top-level model, message routing, asynchronous commands, and rendering.
// Filesystem work happens only inside commands; the update loop never waits
// on I/O.
package app

import (
	"gridfm/internal/browser"
	"gridfm/internal/operations"
	"gridfm/internal/places"
)

// DirectoryLoadedMsg carries the result of an asynchronous directory read.
// Results are applied only when RequestID matches the newest request, which
// rejects stale loads issued before a more recent navigation.
type DirectoryLoadedMsg struct {
	Entries   []browser.Entry
	Path      string
	Err       error
	RequestID uint64
}

// EntryResolvedMsg reports that an explicitly opened entry resolved to a
// non-directory target (typically a symlink to a file). The entry should be
// opened with the file opener. Like all load results it carries the request
// ID so stale attempts are discarded.
type EntryResolvedMsg struct {
	Path      string
	RequestID uint64
}

// PlacesLoadedMsg carries the discovered sidebar places and the home path
// used for breadcrumb abbreviation.
type PlacesLoadedMsg struct {
	Places []places.Place
	Home   string
}

// OpenFinishedMsg reports the completion of an externally opened file:
// an editor session launched with terminal hand-off, or the desktop
// opener's process. A nil Err means it exited cleanly. RequestID is the
// open that produced the completion; stale completions are discarded so a
// late opener can never supersede newer navigation.
type OpenFinishedMsg struct {
	Path      string
	Err       error
	RequestID uint64
}

// OperationEventMsg wraps one event from the operation manager: progress,
// a conflict question, or a finished result. The listener command re-arms
// after every delivery so exactly one listener blocks at a time.
type OperationEventMsg struct {
	Event operations.Event
}
